package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/storage"
)

type QueuedMessage struct {
	Event        string             `json:"event"`
	MessageID    string             `json:"message_id"`
	RFCMessageID string             `json:"rfc_message_id"`
	CompanyID    string             `json:"company_id"`
	DomainID     string             `json:"domain_id"`
	UserID       string             `json:"user_id"`
	Farm         outbound.Farm      `json:"farm"`
	From         outbound.Address   `json:"from"`
	To           []outbound.Address `json:"to"`
	Cc           []outbound.Address `json:"cc"`
	Bcc          []outbound.Address `json:"bcc"`
	DSN          DSNOptions         `json:"dsn"`
	Subject      string             `json:"subject"`
	StoragePath  string             `json:"storage_path"`
	Size         int64              `json:"size"`
	RetryAttempt int                `json:"retry_attempt"`
}

type DSNOptions struct {
	Return     string                `json:"return"`
	EnvelopeID string                `json:"envelope_id"`
	Recipients []DSNRecipientOptions `json:"recipients"`
}

type DSNRecipientOptions struct {
	Address           string   `json:"address"`
	Notify            []string `json:"notify"`
	OriginalRecipient string   `json:"original_recipient"`
}

type MessageOpener func(ctx context.Context) (io.ReadCloser, error)

type Job struct {
	QueuedMessage
	OpenMessage MessageOpener
}

type Transport interface {
	Deliver(ctx context.Context, job Job) error
}

type Handler struct {
	store     storage.Store
	transport Transport
	recorder  Recorder
	retry     RetryScheduler
	metrics   Metrics
	throttler Throttler
}

func NewHandler(store storage.Store, transport Transport, recorder Recorder, retry RetryScheduler) *Handler {
	if recorder == nil {
		recorder = noopRecorder{}
	}
	return &Handler{store: store, transport: transport, recorder: recorder, retry: retry, metrics: noopMetrics{}}
}

func (h *Handler) WithMetrics(metrics Metrics) *Handler {
	h.metrics = metricsOrDefault(metrics)
	return h
}

func (h *Handler) WithThrottler(throttler Throttler) *Handler {
	h.throttler = throttler
	return h
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.store == nil {
		return fmt.Errorf("delivery storage is required")
	}
	if h.transport == nil {
		return fmt.Errorf("delivery transport is required")
	}

	queued, err := DecodeQueuedMessage(msg.Payload)
	if err != nil {
		h.observe(ctx, MetricEvent{Stage: MetricQueuedDecoded, Result: MetricFailed, Error: err.Error()})
		return err
	}
	h.observe(ctx, metricEvent(queued, MetricQueuedDecoded, MetricOK, nil))
	if queued.StoragePath == "" {
		return fmt.Errorf("mail.queued payload is missing storage_path")
	}

	job := Job{
		QueuedMessage: queued,
		OpenMessage: func(openCtx context.Context) (io.ReadCloser, error) {
			return h.store.Get(openCtx, queued.StoragePath)
		},
	}

	if h.throttler != nil {
		release, err := h.throttler.Acquire(ctx, job)
		if err != nil {
			h.observe(ctx, metricEvent(queued, MetricThrottled, MetricDeferred, err))
			if h.retry != nil {
				if retryErr := h.retry.ScheduleRetry(ctx, job, err); retryErr == nil || errors.Is(retryErr, ErrRetryExhausted) {
					return nil
				} else {
					return retryErr
				}
			}
			return err
		}
		defer release()
	}

	if err := h.transport.Deliver(ctx, job); err != nil {
		var partial *PartialDeliveryError
		if errors.As(err, &partial) {
			h.observe(ctx, metricEvent(queued, MetricTransportFailed, MetricDeferred, err))
			if recordErr := h.recordPartialAttempts(ctx, job, partial); recordErr != nil {
				return recordErr
			}
			temporary := partial.TemporaryFailures()
			if len(temporary) == 0 || h.retry == nil {
				return nil
			}
			retryJob := job
			retryJob.QueuedMessage = queuedMessageForRecipients(job.QueuedMessage, temporary)
			retryErr := h.retry.ScheduleRetry(ctx, retryJob, err)
			if retryErr == nil || errors.Is(retryErr, ErrRetryExhausted) {
				if errors.Is(retryErr, ErrRetryExhausted) {
					h.observe(ctx, metricEvent(retryJob.QueuedMessage, MetricRetryExhausted, MetricFailed, retryErr))
				} else {
					h.observe(ctx, metricEvent(retryJob.QueuedMessage, MetricRetryScheduled, MetricDeferred, err))
				}
				return nil
			}
			return retryErr
		}
		status := AttemptFailed
		result := MetricFailed
		if IsPermanentFailure(err) {
			status = AttemptBounced
			result = MetricBounced
		}
		h.observe(ctx, metricEvent(queued, MetricTransportFailed, result, err))
		if recordErr := h.recordAttempts(ctx, job, status, err); recordErr != nil {
			return recordErr
		}
		if IsPermanentFailure(err) {
			return nil
		}
		if h.retry != nil {
			retryErr := h.retry.ScheduleRetry(ctx, job, err)
			if retryErr == nil || errors.Is(retryErr, ErrRetryExhausted) {
				if errors.Is(retryErr, ErrRetryExhausted) {
					h.observe(ctx, metricEvent(queued, MetricRetryExhausted, MetricFailed, retryErr))
				} else {
					h.observe(ctx, metricEvent(queued, MetricRetryScheduled, MetricDeferred, err))
				}
				return nil
			}
			return retryErr
		}
		return err
	}
	h.observe(ctx, metricEvent(queued, MetricTransportDelivered, MetricOK, nil))
	return h.recordAttempts(ctx, job, AttemptDelivered, nil)
}

func queuedMessageForRecipients(queued QueuedMessage, recipients []outbound.Address) QueuedMessage {
	queued.To = append([]outbound.Address(nil), recipients...)
	queued.Cc = nil
	queued.Bcc = nil
	queued.DSN.Recipients = filterDSNRecipients(queued.DSN.Recipients, recipients)
	return queued
}

func filterDSNRecipients(dsnRecipients []DSNRecipientOptions, recipients []outbound.Address) []DSNRecipientOptions {
	if len(dsnRecipients) == 0 || len(recipients) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		normalized, err := mail.NormalizeAddress(recipient.Email)
		if err != nil {
			continue
		}
		wanted[normalized] = struct{}{}
	}
	if len(wanted) == 0 {
		return nil
	}
	filtered := make([]DSNRecipientOptions, 0, len(dsnRecipients))
	for _, recipient := range dsnRecipients {
		normalized, err := mail.NormalizeAddress(recipient.Address)
		if err != nil {
			continue
		}
		if _, ok := wanted[normalized]; !ok {
			continue
		}
		recipient.Address = normalized
		filtered = append(filtered, recipient)
	}
	return filtered
}

func DecodeQueuedMessage(payload json.RawMessage) (QueuedMessage, error) {
	var queued QueuedMessage
	if err := json.Unmarshal(payload, &queued); err != nil {
		return QueuedMessage{}, fmt.Errorf("decode mail.queued payload: %w", err)
	}
	if queued.Event != "mail.queued" {
		return QueuedMessage{}, fmt.Errorf("unexpected delivery event %q", queued.Event)
	}
	if queued.MessageID == "" {
		return QueuedMessage{}, fmt.Errorf("mail.queued payload is missing message_id")
	}
	if queued.From.Email == "" {
		return QueuedMessage{}, fmt.Errorf("mail.queued payload is missing from.email")
	}
	from, err := mail.NormalizeAddress(queued.From.Email)
	if err != nil {
		return QueuedMessage{}, fmt.Errorf("mail.queued payload has invalid from.email: %w", err)
	}
	queued.From.Email = from
	if err := normalizeQueuedRecipients(&queued); err != nil {
		return QueuedMessage{}, err
	}
	if len(queued.Recipients()) == 0 {
		return QueuedMessage{}, fmt.Errorf("mail.queued payload has no recipients")
	}
	return queued, nil
}

func (m QueuedMessage) Recipients() []outbound.Address {
	raw := make([]outbound.Address, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	raw = append(raw, m.To...)
	raw = append(raw, m.Cc...)
	raw = append(raw, m.Bcc...)

	seen := make(map[string]struct{}, len(raw))
	recipients := make([]outbound.Address, 0, len(raw))
	for _, recipient := range raw {
		normalized, err := mail.NormalizeAddress(recipient.Email)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		recipient.Email = normalized
		recipients = append(recipients, recipient)
	}
	return recipients
}

func normalizeQueuedRecipients(queued *QueuedMessage) error {
	var err error
	if queued.To, err = normalizeAddressList("to", queued.To); err != nil {
		return err
	}
	if queued.Cc, err = normalizeAddressList("cc", queued.Cc); err != nil {
		return err
	}
	if queued.Bcc, err = normalizeAddressList("bcc", queued.Bcc); err != nil {
		return err
	}
	return nil
}

func normalizeAddressList(field string, addresses []outbound.Address) ([]outbound.Address, error) {
	if len(addresses) == 0 {
		return addresses, nil
	}
	normalized := addresses[:0]
	for _, address := range addresses {
		email, err := mail.NormalizeAddress(address.Email)
		if err != nil {
			return nil, fmt.Errorf("mail.queued payload has invalid %s recipient %q: %w", field, address.Email, err)
		}
		address.Email = email
		normalized = append(normalized, address)
	}
	return normalized, nil
}

func (h *Handler) recordAttempts(ctx context.Context, job Job, status AttemptStatus, cause error) error {
	for _, attempt := range attemptsFor(job, status, cause, timeNow()) {
		if err := h.recorder.RecordAttempt(ctx, attempt); err != nil {
			return fmt.Errorf("record delivery attempt: %w", err)
		}
	}
	return nil
}

func (h *Handler) recordPartialAttempts(ctx context.Context, job Job, partial *PartialDeliveryError) error {
	attemptedAt := timeNow()
	for _, attempt := range attemptsFor(Job{QueuedMessage: queuedMessageForRecipients(job.QueuedMessage, partial.Delivered)}, AttemptDelivered, nil, attemptedAt) {
		if err := h.recorder.RecordAttempt(ctx, attempt); err != nil {
			return fmt.Errorf("record partial delivered attempt: %w", err)
		}
	}
	for _, failure := range partial.Failed {
		status := AttemptFailed
		if IsPermanentFailure(failure.Err) {
			status = AttemptBounced
		}
		for _, attempt := range attemptsFor(Job{QueuedMessage: queuedMessageForRecipients(job.QueuedMessage, []outbound.Address{failure.Recipient})}, status, failure.Err, attemptedAt) {
			if err := h.recorder.RecordAttempt(ctx, attempt); err != nil {
				return fmt.Errorf("record partial failed attempt: %w", err)
			}
		}
	}
	return nil
}

func (h *Handler) observe(ctx context.Context, event MetricEvent) {
	h.metrics.ObserveDelivery(ctx, event)
}

func metricEvent(queued QueuedMessage, stage MetricStage, result MetricResult, err error) MetricEvent {
	event := MetricEvent{
		Stage:          stage,
		Result:         result,
		MessageID:      queued.MessageID,
		RFCMessageID:   queued.RFCMessageID,
		DomainID:       queued.DomainID,
		Farm:           string(queued.Farm),
		RecipientCount: len(queued.Recipients()),
	}
	if err != nil {
		event.Error = err.Error()
	}
	return event
}

var timeNow = func() time.Time {
	return time.Now().UTC()
}
