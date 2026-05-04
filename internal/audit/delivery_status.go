package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
)

type DeliveryStatusHandler struct {
	repository Repository
}

func NewDeliveryStatusHandler(repository Repository) *DeliveryStatusHandler {
	return &DeliveryStatusHandler{repository: repository}
}

func (h *DeliveryStatusHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.repository == nil {
		return fmt.Errorf("audit repository is required")
	}

	log, err := DeliveryStatusAuditLog(msg.Payload)
	if err != nil {
		return err
	}
	return h.repository.Insert(ctx, log)
}

func DeliveryStatusAuditLog(payload json.RawMessage) (Log, error) {
	var event struct {
		Event           string `json:"event"`
		MessageID       string `json:"message_id"`
		RFCMessageID    string `json:"rfc_message_id"`
		CompanyID       string `json:"company_id"`
		DomainID        string `json:"domain_id"`
		Farm            string `json:"farm"`
		Sender          string `json:"sender"`
		Recipient       string `json:"recipient"`
		RecipientDomain string `json:"recipient_domain"`
		Status          string `json:"status"`
		ErrorMessage    string `json:"error_message"`
		AttemptedAt     string `json:"attempted_at"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return Log{}, fmt.Errorf("decode delivery audit payload: %w", err)
	}
	event.Event = strings.TrimSpace(event.Event)
	action, err := deliveryAuditAction(event.Event)
	if err != nil {
		return Log{}, err
	}
	if event.MessageID, err = requiredDeliveryEventValue("message_id", event.MessageID); err != nil {
		return Log{}, err
	}
	event.RFCMessageID = strings.TrimSpace(event.RFCMessageID)
	event.CompanyID = strings.TrimSpace(event.CompanyID)
	event.DomainID = strings.TrimSpace(event.DomainID)
	event.Farm = strings.TrimSpace(event.Farm)
	event.Sender = strings.TrimSpace(event.Sender)
	event.Recipient = strings.TrimSpace(event.Recipient)
	event.RecipientDomain = strings.TrimSpace(event.RecipientDomain)
	event.Status = strings.TrimSpace(event.Status)
	event.ErrorMessage = strings.TrimSpace(event.ErrorMessage)
	event.AttemptedAt = strings.TrimSpace(event.AttemptedAt)

	detail, err := json.Marshal(map[string]any{
		"sender":           event.Sender,
		"recipient":        event.Recipient,
		"recipient_domain": event.RecipientDomain,
		"rfc_message_id":   event.RFCMessageID,
		"farm":             event.Farm,
		"status":           event.Status,
		"error_message":    event.ErrorMessage,
		"attempted_at":     event.AttemptedAt,
	})
	if err != nil {
		return Log{}, fmt.Errorf("marshal delivery audit detail: %w", err)
	}

	return Log{
		CompanyID:  event.CompanyID,
		DomainID:   event.DomainID,
		Category:   "mail",
		Action:     action,
		TargetType: "message",
		TargetID:   event.MessageID,
		Result:     deliveryAuditResult(event.Event),
		Detail:     detail,
	}, nil
}

func deliveryAuditAction(event string) (string, error) {
	switch event {
	case "mail.delivered":
		return "mail.delivered", nil
	case "mail.bounced":
		return "mail.bounced", nil
	case "mail.delivery_failed":
		return "mail.delivery_failed", nil
	default:
		return "", fmt.Errorf("unexpected delivery audit event %q", event)
	}
}

func deliveryAuditResult(event string) string {
	if event == "mail.delivered" {
		return "success"
	}
	return "failure"
}

func requiredDeliveryEventValue(name string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("delivery audit payload is missing %s", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("delivery audit payload has invalid %s", name)
	}
	return value, nil
}
