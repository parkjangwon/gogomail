package eventstream

import (
	"context"
	"io"
	"log/slog"
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

func TestRedisConsumerAcksMalformedMessage(t *testing.T) {
	t.Parallel()

	consumer := &RedisConsumer{
		handler: HandlerFunc(func(context.Context, Message) error {
			t.Fatal("handler should not receive malformed message")
			return nil
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	var acked []string
	processed, err := consumer.processRedisMessage(context.Background(), "mail.event", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"outbox_id":     "outbox-1",
			"partition_key": "message-1",
			"payload":       `{invalid`,
		},
	}, func(_ context.Context, id string) error {
		acked = append(acked, id)
		return nil
	})
	if err != nil {
		t.Fatalf("processRedisMessage returned error: %v", err)
	}
	if !processed || len(acked) != 1 || acked[0] != "1-0" {
		t.Fatalf("processed = %v, acked = %+v", processed, acked)
	}
}
