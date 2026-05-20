package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
)

// mailEventPayload is the common envelope used by mail.* events in the event stream.
type mailEventPayload struct {
	Event         string          `json:"event"`
	SchemaVersion string          `json:"schema_version,omitempty"`
	MessageID     string          `json:"message_id"`
	RFCMessageID  string          `json:"rfc_message_id,omitempty"`
	CompanyID     string          `json:"company_id"`
	DomainID      string          `json:"domain_id,omitempty"`
	UserID        string          `json:"user_id,omitempty"`
	Recipient     string          `json:"recipient,omitempty"`
	Subject       string          `json:"subject,omitempty"`
	ReceivedAt    string          `json:"received_at,omitempty"`
	Extra         json.RawMessage `json:"-"`
}

// EventHandler dispatches webhook events when it receives events from the event stream.
type EventHandler struct {
	dispatcher *WebhookDispatcher
	eventType  string // e.g. "mail.received", "mail.sent", "mail.bounced"
}

// NewMailStoredWebhookHandler creates an eventstream.Handler that dispatches
// webhook events for mail.stored (treated as mail.received externally).
func NewMailStoredWebhookHandler(dispatcher *WebhookDispatcher) eventstream.Handler {
	return &EventHandler{dispatcher: dispatcher, eventType: "mail.received"}
}

// NewMailBouncedWebhookHandler creates an eventstream.Handler for mail.bounced.
func NewMailBouncedWebhookHandler(dispatcher *WebhookDispatcher) eventstream.Handler {
	return &EventHandler{dispatcher: dispatcher, eventType: "mail.bounced"}
}

// NewMailSentWebhookHandler creates an eventstream.Handler for mail.delivered (externally mail.sent).
func NewMailSentWebhookHandler(dispatcher *WebhookDispatcher) eventstream.Handler {
	return &EventHandler{dispatcher: dispatcher, eventType: "mail.sent"}
}

// HandleEvent implements eventstream.Handler.
func (h *EventHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	var p mailEventPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("webhook handler: decode payload: %w", err)
	}
	companyID := strings.TrimSpace(p.CompanyID)
	if companyID == "" {
		// No company ID — nothing to dispatch to.
		return nil
	}

	occurredAt := strings.TrimSpace(p.ReceivedAt)
	if occurredAt == "" {
		occurredAt = time.Now().UTC().Format(time.RFC3339)
	}

	data := map[string]interface{}{
		"message_id":     p.MessageID,
		"rfc_message_id": p.RFCMessageID,
		"domain_id":      p.DomainID,
		"user_id":        p.UserID,
		"recipient":      p.Recipient,
		"subject":        p.Subject,
	}

	event := WebhookEvent{
		Event:      h.eventType,
		CompanyID:  companyID,
		OccurredAt: occurredAt,
		Data:       data,
	}

	// Dispatch is best-effort: errors are logged inside Dispatch, not propagated.
	_ = h.dispatcher.Dispatch(ctx, event)
	return nil
}
