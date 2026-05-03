package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogomail/gogomail/internal/eventstream"
)

type Repository interface {
	Insert(ctx context.Context, log Log) error
}

type MailStoredHandler struct {
	repository Repository
}

func NewMailStoredHandler(repository Repository) *MailStoredHandler {
	return &MailStoredHandler{repository: repository}
}

func (h *MailStoredHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.repository == nil {
		return fmt.Errorf("audit repository is required")
	}

	log, err := MailStoredAuditLog(msg.Payload)
	if err != nil {
		return err
	}
	return h.repository.Insert(ctx, log)
}

func MailStoredAuditLog(payload json.RawMessage) (Log, error) {
	var event struct {
		Event        string `json:"event"`
		MessageID    string `json:"message_id"`
		RFCMessageID string `json:"rfc_message_id"`
		CompanyID    string `json:"company_id"`
		DomainID     string `json:"domain_id"`
		UserID       string `json:"user_id"`
		Recipient    string `json:"recipient"`
		Subject      string `json:"subject"`
		StoragePath  string `json:"storage_path"`
		ReceivedAt   string `json:"received_at"`
		Size         int64  `json:"size"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return Log{}, fmt.Errorf("decode mail.stored audit payload: %w", err)
	}
	if event.Event != "mail.stored" {
		return Log{}, fmt.Errorf("unexpected audit event %q", event.Event)
	}
	if event.MessageID == "" {
		return Log{}, fmt.Errorf("mail.stored audit payload is missing message_id")
	}

	detail, err := json.Marshal(map[string]any{
		"recipient":      event.Recipient,
		"rfc_message_id": event.RFCMessageID,
		"subject":        event.Subject,
		"storage_path":   event.StoragePath,
		"received_at":    event.ReceivedAt,
		"size":           event.Size,
	})
	if err != nil {
		return Log{}, fmt.Errorf("marshal mail.received audit detail: %w", err)
	}

	return Log{
		CompanyID:  event.CompanyID,
		DomainID:   event.DomainID,
		UserID:     event.UserID,
		Category:   "mail",
		Action:     "mail.received",
		TargetType: "message",
		TargetID:   event.MessageID,
		Result:     "success",
		Detail:     detail,
	}, nil
}
