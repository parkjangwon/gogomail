package pushnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
)

const EventMailStored = "mail.stored"

type Event struct {
	Event        string `json:"event"`
	MessageID    string `json:"message_id"`
	RFCMessageID string `json:"rfc_message_id"`
	CompanyID    string `json:"company_id"`
	DomainID     string `json:"domain_id"`
	UserID       string `json:"user_id"`
	Recipient    string `json:"recipient"`
	Subject      string `json:"subject"`
	ReceivedAt   string `json:"received_at"`
}

type Notification struct {
	MessageID    string
	RFCMessageID string
	CompanyID    string
	DomainID     string
	UserID       string
	Recipient    string
	Subject      string
	ReceivedAt   string
	Targets      []Target
}

type Sink interface {
	EnqueuePush(ctx context.Context, notification Notification) error
}

type Target struct {
	DeviceID    string
	Platform    string
	Token       string
	TokenSuffix string
	Label       string
}

type TargetResolver interface {
	ResolvePushTargets(ctx context.Context, event Event) ([]Target, error)
}

type CandidateRecord struct {
	MessageID    string
	RFCMessageID string
	CompanyID    string
	DomainID     string
	UserID       string
	Recipient    string
	Subject      string
	DeviceID     string
	Platform     string
	TokenSuffix  string
	Status       string
	ErrorMessage string
}

type CandidateRecorder interface {
	RecordCandidate(ctx context.Context, record CandidateRecord) error
}

type Handler struct {
	sink           Sink
	targetResolver TargetResolver
	recorder       CandidateRecorder
}

type HandlerOption func(*Handler)

func WithTargetResolver(resolver TargetResolver) HandlerOption {
	return func(h *Handler) {
		h.targetResolver = resolver
	}
}

func WithCandidateRecorder(recorder CandidateRecorder) HandlerOption {
	return func(h *Handler) {
		h.recorder = recorder
	}
}

func NewHandler(sink Sink, opts ...HandlerOption) *Handler {
	handler := &Handler{sink: sink}
	for _, opt := range opts {
		opt(handler)
	}
	return handler
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h == nil || h.sink == nil {
		return fmt.Errorf("push notification sink is required")
	}
	event, err := DecodeEvent(msg.Payload)
	if err != nil {
		return err
	}
	notification := notificationFromEvent(event)
	if h.targetResolver != nil {
		targets, err := h.targetResolver.ResolvePushTargets(ctx, event)
		if err != nil {
			return fmt.Errorf("resolve push notification targets: %w", err)
		}
		if len(targets) == 0 {
			return nil
		}
		notification.Targets = targets
	}
	if h.recorder != nil {
		for _, target := range notification.Targets {
			if err := h.recorder.RecordCandidate(ctx, candidateRecordFromNotification(notification, target)); err != nil {
				return fmt.Errorf("record push notification candidate: %w", err)
			}
		}
	}
	if err := h.sink.EnqueuePush(ctx, notification); err != nil {
		return fmt.Errorf("enqueue push notification candidate: %w", err)
	}
	return nil
}

func DecodeEvent(payload json.RawMessage) (Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, fmt.Errorf("decode mail.stored push payload: %w", err)
	}
	if err := validateEvent(&event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func validateEvent(event *Event) error {
	event.Event = strings.TrimSpace(event.Event)
	if event.Event != EventMailStored {
		return fmt.Errorf("unexpected push notification event %q", event.Event)
	}
	var err error
	if event.MessageID, err = requiredValue("message_id", event.MessageID); err != nil {
		return err
	}
	if event.UserID, err = requiredValue("user_id", event.UserID); err != nil {
		return err
	}
	event.RFCMessageID = strings.TrimSpace(event.RFCMessageID)
	event.CompanyID = strings.TrimSpace(event.CompanyID)
	event.DomainID = strings.TrimSpace(event.DomainID)
	event.Recipient = strings.TrimSpace(event.Recipient)
	event.Subject = strings.TrimSpace(event.Subject)
	event.ReceivedAt = strings.TrimSpace(event.ReceivedAt)
	return nil
}

func requiredValue(name string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("mail.stored push payload is missing %s", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mail.stored push payload has invalid %s", name)
	}
	return value, nil
}

func notificationFromEvent(event Event) Notification {
	return Notification{
		MessageID:    event.MessageID,
		RFCMessageID: event.RFCMessageID,
		CompanyID:    event.CompanyID,
		DomainID:     event.DomainID,
		UserID:       event.UserID,
		Recipient:    event.Recipient,
		Subject:      event.Subject,
		ReceivedAt:   event.ReceivedAt,
	}
}

func candidateRecordFromNotification(notification Notification, target Target) CandidateRecord {
	return CandidateRecord{
		MessageID:    notification.MessageID,
		RFCMessageID: notification.RFCMessageID,
		CompanyID:    notification.CompanyID,
		DomainID:     notification.DomainID,
		UserID:       notification.UserID,
		Recipient:    notification.Recipient,
		Subject:      notification.Subject,
		DeviceID:     target.DeviceID,
		Platform:     target.Platform,
		TokenSuffix:  target.TokenSuffix,
		Status:       "candidate",
	}
}

type SlogSink struct {
	Logger *slog.Logger
}

func (s SlogSink) EnqueuePush(_ context.Context, notification Notification) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		"push notification candidate",
		"message_id", notification.MessageID,
		"rfc_message_id", notification.RFCMessageID,
		"company_id", notification.CompanyID,
		"domain_id", notification.DomainID,
		"user_id", notification.UserID,
		"recipient", notification.Recipient,
		"subject", notification.Subject,
		"received_at", notification.ReceivedAt,
		"targets", len(notification.Targets),
	)
	return nil
}
