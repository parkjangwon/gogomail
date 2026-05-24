package outbox

import "testing"

func TestNormalizeRedisStreamEventTrimsMetadata(t *testing.T) {
	t.Parallel()

	event, err := normalizeRedisStreamEvent(Event{
		ID:           " event-1 ",
		Topic:        " mail.event ",
		PartitionKey: " message-1 ",
		Payload:      []byte(` {"event":"mail.stored"} `),
	})
	if err != nil {
		t.Fatalf("normalizeRedisStreamEvent returned error: %v", err)
	}
	if event.ID != "event-1" || event.Topic != "mail.event" || event.PartitionKey != "message-1" || string(event.Payload) != `{"event":"mail.stored"}` {
		t.Fatalf("event = %#v payload=%q", event, string(event.Payload))
	}
}

func TestNormalizeRedisStreamEventRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
	}{
		{name: "blank id", event: Event{ID: " ", Topic: "mail.event", Payload: []byte(`{"event":"mail.stored"}`)}},
		{name: "blank topic", event: Event{ID: "event-1", Topic: " ", Payload: []byte(`{"event":"mail.stored"}`)}},
		{name: "invalid topic", event: Event{ID: "event-1", Topic: "mail.event\nmail.other", Payload: []byte(`{"event":"mail.stored"}`)}},
		{name: "invalid payload", event: Event{ID: "event-1", Topic: "mail.event", Payload: []byte(`{invalid`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeRedisStreamEvent(tt.event); err == nil {
				t.Fatal("normalizeRedisStreamEvent accepted invalid event")
			}
		})
	}
}

func TestRedisStreamForEventUsesOutboxTopic(t *testing.T) {
	t.Parallel()

	if got := redisStreamForEvent("mail.event", " mail.outbound.general "); got != "mail.outbound.general" {
		t.Fatalf("redisStreamForEvent = %q, want mail.outbound.general", got)
	}
}

func TestRedisStreamForEventFallsBackToDefault(t *testing.T) {
	t.Parallel()

	if got := redisStreamForEvent(" mail.event ", " "); got != "mail.event" {
		t.Fatalf("redisStreamForEvent fallback = %q, want mail.event", got)
	}
}
