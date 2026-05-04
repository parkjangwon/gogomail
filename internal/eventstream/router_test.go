package eventstream

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRouterRoutesByPayloadEventName(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	var handled Message
	if err := router.Register(" mail.stored ", HandlerFunc(func(_ context.Context, msg Message) error {
		handled = msg
		return nil
	})); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	err := router.HandleEvent(context.Background(), Message{
		ID:      "redis-1",
		Payload: []byte(`{"event":" mail.stored ","message_id":"msg-1"}`),
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

func TestEventNameRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	_, err := EventName([]byte("{\"event\":\"mail.stored\\nmail.bounced\"}"))
	if err == nil {
		t.Fatal("EventName accepted invalid event name")
	}
}

func TestEventNameRejectsOversizedEvent(t *testing.T) {
	t.Parallel()

	_, err := EventName([]byte(`{"event":"` + strings.Repeat("e", maxEventNameBytes+1) + `"}`))
	if err == nil {
		t.Fatal("EventName accepted oversized event name")
	}
}

func TestRouterRejectsInvalidRegisteredEvent(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	err := router.Register("mail.stored\nmail.bounced", HandlerFunc(func(context.Context, Message) error {
		return nil
	}))
	if err == nil {
		t.Fatal("Register accepted invalid event name")
	}
}

func TestRouterRejectsOversizedRegisteredEvent(t *testing.T) {
	t.Parallel()

	router := NewRouter()
	err := router.Register(strings.Repeat("e", maxEventNameBytes+1), HandlerFunc(func(context.Context, Message) error {
		return nil
	}))
	if err == nil {
		t.Fatal("Register accepted oversized event name")
	}
}

func TestMultiHandlerFansOutInOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	handler := MultiHandler{
		HandlerFunc(func(context.Context, Message) error {
			calls = append(calls, "first")
			return nil
		}),
		nil,
		HandlerFunc(func(context.Context, Message) error {
			calls = append(calls, "second")
			return nil
		}),
	}

	if err := handler.HandleEvent(context.Background(), Message{Payload: []byte(`{"event":"mail.bounced"}`)}); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("calls = %+v, want ordered fan-out", calls)
	}
}

func TestMultiHandlerStopsOnError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("stop")
	calledSecond := false
	handler := MultiHandler{
		HandlerFunc(func(context.Context, Message) error {
			return wantErr
		}),
		HandlerFunc(func(context.Context, Message) error {
			calledSecond = true
			return nil
		}),
	}

	err := handler.HandleEvent(context.Background(), Message{Payload: []byte(`{"event":"mail.bounced"}`)})
	if !errors.Is(err, wantErr) {
		t.Fatalf("HandleEvent() error = %v, want %v", err, wantErr)
	}
	if calledSecond {
		t.Fatal("second handler was called after first handler failed")
	}
}
