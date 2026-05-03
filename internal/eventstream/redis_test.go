package eventstream

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDecodeRedisMessage(t *testing.T) {
	t.Parallel()

	msg, err := decodeRedisMessage("mail.event", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"outbox_id":     "outbox-1",
			"partition_key": "message-1",
			"payload":       `{"event":"mail.stored"}`,
		},
	})
	if err != nil {
		t.Fatalf("decodeRedisMessage returned error: %v", err)
	}
	if msg.ID != "1-0" {
		t.Fatalf("ID = %q, want 1-0", msg.ID)
	}
	if msg.Stream != "mail.event" {
		t.Fatalf("Stream = %q, want mail.event", msg.Stream)
	}
	if msg.OutboxID != "outbox-1" {
		t.Fatalf("OutboxID = %q, want outbox-1", msg.OutboxID)
	}
}

func TestDecodeRedisMessageRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	_, err := decodeRedisMessage("mail.event", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"outbox_id":     "outbox-1",
			"partition_key": "message-1",
			"payload":       `{invalid`,
		},
	})
	if err == nil {
		t.Fatal("decodeRedisMessage accepted invalid payload")
	}
}
