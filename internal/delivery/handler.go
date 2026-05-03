package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
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
	Subject      string             `json:"subject"`
	StoragePath  string             `json:"storage_path"`
	Size         int64              `json:"size"`
	RetryAttempt int                `json:"retry_attempt"`
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
}

func NewHandler(store storage.Store, transport Transport, recorder Recorder, retry RetryScheduler) *Handler {
	if recorder == nil {
		recorder = noopRecorder{}
	}
	return &Handler{store: store, transport: transport, recorder: recorder, retry: retry}
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
		return err
	}
	if queued.StoragePath == "" {
		return fmt.Errorf("mail.queued payload is missing storage_path")
	}

	job := Job{
		QueuedMessage: queued,
		OpenMessage: func(openCtx context.Context) (io.ReadCloser, error) {
			return h.store.Get(openCtx, queued.StoragePath)
		},
	}

	if err := h.transport.Deliver(ctx, job); err != nil {
		if recordErr := h.recordAttempts(ctx, job, AttemptFailed, err); recordErr != nil {
			return recordErr
		}
		if h.retry != nil {
			retryErr := h.retry.ScheduleRetry(ctx, job, err)
			if retryErr == nil || errors.Is(retryErr, ErrRetryExhausted) {
				return nil
			}
			return retryErr
		}
		return err
	}
	return h.recordAttempts(ctx, job, AttemptDelivered, nil)
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
	if len(queued.Recipients()) == 0 {
		return QueuedMessage{}, fmt.Errorf("mail.queued payload has no recipients")
	}
	return queued, nil
}

func (m QueuedMessage) Recipients() []outbound.Address {
	recipients := make([]outbound.Address, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	recipients = append(recipients, m.To...)
	recipients = append(recipients, m.Cc...)
	recipients = append(recipients, m.Bcc...)
	return recipients
}

func (h *Handler) recordAttempts(ctx context.Context, job Job, status AttemptStatus, cause error) error {
	for _, attempt := range attemptsFor(job, status, cause, timeNow()) {
		if err := h.recorder.RecordAttempt(ctx, attempt); err != nil {
			return fmt.Errorf("record delivery attempt: %w", err)
		}
	}
	return nil
}

var timeNow = func() time.Time {
	return time.Now().UTC()
}
