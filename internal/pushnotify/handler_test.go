package pushnotify

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestHandlerEnqueuesMailStoredNotification(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	handler := NewHandler(sink)
	payload := json.RawMessage(`{
		"event":"mail.stored",
		"message_id":"msg-1",
		"rfc_message_id":"<msg-1@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"user_id":"user-1",
		"recipient":"user@example.com",
		"subject":"Hello",
		"received_at":"2026-05-04T00:00:00Z"
	}`)

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if sink.last.UserID != "user-1" || sink.last.MessageID != "msg-1" || sink.last.Subject != "Hello" {
		t.Fatalf("notification = %+v", sink.last)
	}
}

func TestDecodeEventRequiresStoredMessageIdentity(t *testing.T) {
	t.Parallel()

	_, err := DecodeEvent(json.RawMessage(`{"event":"mail.stored","user_id":"user-1"}`))
	if err == nil {
		t.Fatal("DecodeEvent accepted payload without message_id")
	}
}

type fakeSink struct {
	last Notification
}

func (s *fakeSink) EnqueuePush(_ context.Context, notification Notification) error {
	s.last = notification
	return nil
}
