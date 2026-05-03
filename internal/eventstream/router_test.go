package eventstream

import (
	"context"
	"testing"
)

func TestRouterRoutesByPayloadEventName(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	var handled Message
	if err := router.Register("mail.stored", HandlerFunc(func(_ context.Context, msg Message) error {
		handled = msg
		return nil
	})); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	err := router.HandleEvent(context.Background(), Message{
		ID:      "redis-1",
		Payload: []byte(`{"event":"mail.stored","message_id":"msg-1"}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if handled.ID != "redis-1" {
		t.Fatalf("handled ID = %q, want redis-1", handled.ID)
	}
}

func TestRouterIgnoresUnknownEvents(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	err := router.HandleEvent(context.Background(), Message{
		ID:      "redis-1",
		Payload: []byte(`{"event":"mail.unknown"}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
}

func TestEventNameRejectsMissingEvent(t *testing.T) {
	t.Parallel()

	_, err := EventName([]byte(`{"message_id":"msg-1"}`))
	if err == nil {
		t.Fatal("EventName accepted payload without event")
	}
}
