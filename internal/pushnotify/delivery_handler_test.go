package pushnotify_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/pushnotify"
)

type fakeDeliverySink struct {
	notifications []pushnotify.Notification
}

func (f *fakeDeliverySink) EnqueuePush(_ context.Context, n pushnotify.Notification) error {
	f.notifications = append(f.notifications, n)
	return nil
}

type fakeMessageUserLookup struct {
	userID string
}

func (f *fakeMessageUserLookup) GetMessageSenderUserID(_ context.Context, messageID string) (string, error) {
	return f.userID, nil
}

func TestDeliveryExhaustedHandler_HandleEvent(t *testing.T) {
	sink := &fakeDeliverySink{}
	lookup := &fakeMessageUserLookup{userID: "user-123"}
	handler := pushnotify.NewDeliveryExhaustedHandler(sink, lookup)

	payload, _ := json.Marshal(map[string]any{
		"event":      "mail.delivery_exhausted",
		"message_id": "msg-abc",
		"company_id": "co-1",
		"domain_id":  "d-1",
		"sender":     "user@example.com",
		"recipients": []string{"bob@external.com"},
	})
	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload}); err != nil {
		t.Fatalf("HandleEvent error: %v", err)
	}
	if len(sink.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sink.notifications))
	}
	n := sink.notifications[0]
	if n.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %q", n.UserID)
	}
	if n.Subject != "발송 최종 실패" {
		t.Errorf("expected Subject='발송 최종 실패', got %q", n.Subject)
	}
}

func TestDeliveryExhaustedHandler_NoUserID_SkipsPush(t *testing.T) {
	sink := &fakeDeliverySink{}
	lookup := &fakeMessageUserLookup{userID: ""}
	handler := pushnotify.NewDeliveryExhaustedHandler(sink, lookup)

	payload, _ := json.Marshal(map[string]any{
		"event":      "mail.delivery_exhausted",
		"message_id": "msg-xyz",
	})
	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload}); err != nil {
		t.Fatalf("HandleEvent error: %v", err)
	}
	if len(sink.notifications) != 0 {
		t.Errorf("expected 0 notifications when userID is empty, got %d", len(sink.notifications))
	}
}
