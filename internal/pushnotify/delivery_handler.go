package pushnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
)

const EventMailDeliveryExhausted = "mail.delivery_exhausted"

// MessageUserLookup looks up the sender's user_id from a message_id.
type MessageUserLookup interface {
	GetMessageSenderUserID(ctx context.Context, messageID string) (string, error)
}

// DeliveryExhaustedHandler handles mail.delivery_exhausted events and sends push notifications.
type DeliveryExhaustedHandler struct {
	sink   Sink
	lookup MessageUserLookup
}

// NewDeliveryExhaustedHandler creates a handler for delivery exhausted events.
func NewDeliveryExhaustedHandler(sink Sink, lookup MessageUserLookup) *DeliveryExhaustedHandler {
	return &DeliveryExhaustedHandler{sink: sink, lookup: lookup}
}

type deliveryExhaustedEvent struct {
	Event      string   `json:"event"`
	MessageID  string   `json:"message_id"`
	CompanyID  string   `json:"company_id"`
	DomainID   string   `json:"domain_id"`
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
}

// HandleEvent implements eventstream.Handler for mail.delivery_exhausted.
func (h *DeliveryExhaustedHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	var ev deliveryExhaustedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		return fmt.Errorf("delivery exhausted: decode: %w", err)
	}
	ev.MessageID = strings.TrimSpace(ev.MessageID)
	if ev.MessageID == "" {
		return fmt.Errorf("delivery exhausted: message_id is required")
	}

	userID, err := h.lookup.GetMessageSenderUserID(ctx, ev.MessageID)
	if err != nil {
		return fmt.Errorf("delivery exhausted: lookup sender user_id: %w", err)
	}
	if userID == "" {
		return nil
	}

	recipient := ""
	if len(ev.Recipients) > 0 {
		recipient = strings.Join(ev.Recipients, ", ")
		if len(recipient) > 100 {
			recipient = recipient[:100] + "…"
		}
	}

	return h.sink.EnqueuePush(ctx, Notification{
		MessageID: ev.MessageID,
		CompanyID: strings.TrimSpace(ev.CompanyID),
		DomainID:  strings.TrimSpace(ev.DomainID),
		UserID:    userID,
		Subject:   "발송 최종 실패",
		Recipient: recipient,
	})
}
