package eventstream

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDecodeRedisMessage(t *testing.T) {
	t.Parallel()

	msg, err := decodeRedisMessage("mail.event", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"outbox_id":     " outbox-1 ",
			"partition_key": []byte(" message-1 "),
			"payload":       ` {"event":"mail.stored"} `,
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
	if msg.PartitionKey != "message-1" {
		t.Fatalf("PartitionKey = %q, want message-1", msg.PartitionKey)
	}
	if string(msg.Payload) != `{"event":"mail.stored"}` {
		t.Fatalf("Payload = %q", string(msg.Payload))
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

func TestDecodeRedisMessageRejectsBlankMetadata(t *testing.T) {
	t.Parallel()

	_, err := decodeRedisMessage("mail.event", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"outbox_id":     "   ",
			"partition_key": "message-1",
			"payload":       `{"event":"mail.stored"}`,
		},
	})
	if err == nil {
		t.Fatal("decodeRedisMessage accepted blank outbox_id")
	}
}

func TestDecodeRedisMessageRejectsUnsafeMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]any
	}{
		{
			name: "outbox_crlf",
			values: map[string]any{
				"outbox_id":     "outbox-1\nbad",
				"partition_key": "message-1",
				"payload":       `{"event":"mail.stored"}`,
			},
		},
		{
			name: "partition_too_long",
			values: map[string]any{
				"outbox_id":     "outbox-1",
				"partition_key": strings.Repeat("p", maxRedisMetadataBytes+1),
				"payload":       `{"event":"mail.stored"}`,
			},
		},
		{
			name: "payload_too_long",
			values: map[string]any{
				"outbox_id":     "outbox-1",
				"partition_key": "message-1",
				"payload":       strings.Repeat(" ", maxRedisPayloadBytes+1),
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := decodeRedisMessage("mail.event", redis.XMessage{ID: "1-0", Values: tc.values}); err == nil {
				t.Fatal("decodeRedisMessage accepted unsafe metadata")
			}
		})
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

func TestRedisConsumerClaimsIdlePendingMessagesBeforeNewReads(t *testing.T) {
	t.Parallel()

	client := &fakeRedisConsumerClient{
		claimed: []redis.XMessage{{
			ID: "1-0",
			Values: map[string]any{
				"outbox_id":     "outbox-1",
				"partition_key": "message-1",
				"payload":       `{"event":"mail.stored"}`,
			},
		}},
		nextClaimStart: "0-0",
		readErr:        redis.Nil,
	}
	var handled []Message
	consumer := &RedisConsumer{
		client:     client,
		stream:     "mail.event",
		group:      "gogomail.event-worker",
		consumer:   "worker-2",
		count:      10,
		block:      time.Millisecond,
		claimIdle:  5 * time.Minute,
		claimStart: redisClaimStartID,
		handler: HandlerFunc(func(_ context.Context, msg Message) error {
			handled = append(handled, msg)
			return nil
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	processed, err := consumer.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if len(handled) != 1 || handled[0].ID != "1-0" {
		t.Fatalf("handled = %+v, want claimed message", handled)
	}
	if len(client.acked) != 1 || client.acked[0] != "1-0" {
		t.Fatalf("acked = %+v, want claimed message ack", client.acked)
	}
	if client.claimArgs == nil {
		t.Fatal("XAutoClaim was not called")
	}
	if client.claimArgs.MinIdle != 5*time.Minute {
		t.Fatalf("claim MinIdle = %s, want 5m", client.claimArgs.MinIdle)
	}
	if client.claimArgs.Consumer != "worker-2" {
		t.Fatalf("claim Consumer = %q, want worker-2", client.claimArgs.Consumer)
	}
}

func TestRedisConsumerSkipsPendingClaimWhenDisabled(t *testing.T) {
	t.Parallel()

	client := &fakeRedisConsumerClient{readErr: redis.Nil}
	consumer := &RedisConsumer{
		client:   client,
		stream:   "mail.event",
		group:    "gogomail.event-worker",
		consumer: "worker-2",
		count:    10,
		block:    time.Millisecond,
		handler:  HandlerFunc(func(context.Context, Message) error { return nil }),
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if _, err := consumer.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if client.claimArgs != nil {
		t.Fatalf("XAutoClaim called with %+v, want disabled", client.claimArgs)
	}
}

type fakeRedisConsumerClient struct {
	claimed        []redis.XMessage
	nextClaimStart string
	claimErr       error
	claimArgs      *redis.XAutoClaimArgs
	readStreams    []redis.XStream
	readErr        error
	acked          []string
}

func (f *fakeRedisConsumerClient) XAck(ctx context.Context, stream string, group string, id ...string) *redis.IntCmd {
	f.acked = append(f.acked, id...)
	return redis.NewIntResult(int64(len(id)), nil)
}

func (f *fakeRedisConsumerClient) XAutoClaim(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimCmd {
	args := *a
	f.claimArgs = &args
	cmd := redis.NewXAutoClaimCmd(ctx)
	cmd.SetVal(f.claimed, f.nextClaimStart)
	cmd.SetErr(f.claimErr)
	return cmd
}

func (f *fakeRedisConsumerClient) XGroupCreateMkStream(ctx context.Context, stream string, group string, start string) *redis.StatusCmd {
	return redis.NewStatusResult("OK", nil)
}

func (f *fakeRedisConsumerClient) XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	return redis.NewXStreamSliceCmdResult(f.readStreams, f.readErr)
}
