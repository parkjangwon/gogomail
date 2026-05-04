package eventstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	client     redisConsumerClient
	stream     string
	group      string
	consumer   string
	count      int64
	block      time.Duration
	claimIdle  time.Duration
	claimStart string
	handler    Handler
	logger     *slog.Logger
}

type redisConsumerClient interface {
	XAck(ctx context.Context, stream string, group string, id ...string) *redis.IntCmd
	XAutoClaim(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
	XGroupCreateMkStream(ctx context.Context, stream string, group string, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

type RedisConsumerOptions struct {
	Client    *redis.Client
	Stream    string
	Group     string
	Consumer  string
	Count     int64
	Block     time.Duration
	ClaimIdle time.Duration
	Handler   Handler
	Logger    *slog.Logger
}

const redisClaimStartID = "0-0"

func NewRedisConsumer(opts RedisConsumerOptions) (*RedisConsumer, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if opts.Stream == "" {
		return nil, fmt.Errorf("redis stream is required")
	}
	if opts.Group == "" {
		return nil, fmt.Errorf("redis consumer group is required")
	}
	if opts.Consumer == "" {
		return nil, fmt.Errorf("redis consumer name is required")
	}
	if opts.Handler == nil {
		return nil, fmt.Errorf("event handler is required")
	}
	if opts.Count <= 0 {
		opts.Count = 100
	}
	if opts.Block <= 0 {
		opts.Block = time.Second
	}
	if opts.ClaimIdle < 0 {
		return nil, fmt.Errorf("redis consumer claim idle duration must not be negative")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &RedisConsumer{
		client:     opts.Client,
		stream:     opts.Stream,
		group:      opts.Group,
		consumer:   opts.Consumer,
		count:      opts.Count,
		block:      opts.Block,
		claimIdle:  opts.ClaimIdle,
		claimStart: redisClaimStartID,
		handler:    opts.Handler,
		logger:     opts.Logger,
	}, nil
}

func (c *RedisConsumer) EnsureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err == nil || isRedisBusyGroup(err) {
		return nil
	}
	return fmt.Errorf("create redis stream group %q/%q: %w", c.stream, c.group, err)
}

func (c *RedisConsumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}

	for {
		if _, err := c.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("redis stream consumer batch failed", "stream", c.stream, "group", c.group, "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (c *RedisConsumer) ProcessOnce(ctx context.Context) (int, error) {
	processed, err := c.claimPending(ctx)
	if err != nil {
		return 0, err
	}

	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, ">"},
		Count:    c.count,
		Block:    c.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return processed, nil
	}
	if err != nil {
		return processed, fmt.Errorf("read redis stream group %q/%q: %w", c.stream, c.group, err)
	}

	for _, stream := range streams {
		count, err := c.processRedisMessages(ctx, stream.Stream, stream.Messages)
		if err != nil {
			return processed, err
		}
		processed += count
	}
	return processed, nil
}

func (c *RedisConsumer) claimPending(ctx context.Context) (int, error) {
	if c.claimIdle <= 0 {
		return 0, nil
	}
	start := c.claimStart
	if start == "" {
		start = redisClaimStartID
	}
	messages, nextStart, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  c.claimIdle,
		Start:    start,
		Count:    c.count,
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("claim pending redis stream messages %q/%q: %w", c.stream, c.group, err)
	}
	if nextStart == "" {
		nextStart = redisClaimStartID
	}
	c.claimStart = nextStart
	if len(messages) == 0 {
		return 0, nil
	}
	c.logger.Info(
		"claimed pending redis stream messages",
		"stream", c.stream,
		"group", c.group,
		"consumer", c.consumer,
		"count", len(messages),
		"min_idle", c.claimIdle.String(),
	)
	return c.processRedisMessages(ctx, c.stream, messages)
}

func (c *RedisConsumer) processRedisMessages(ctx context.Context, stream string, messages []redis.XMessage) (int, error) {
	processed := 0
	for _, redisMessage := range messages {
		acked, err := c.processRedisMessage(ctx, stream, redisMessage, func(ctx context.Context, id string) error {
			return c.client.XAck(ctx, c.stream, c.group, id).Err()
		})
		if err != nil {
			return processed, err
		}
		if acked {
			processed++
		}
	}
	return processed, nil
}

func (c *RedisConsumer) processRedisMessage(ctx context.Context, stream string, redisMessage redis.XMessage, ack func(context.Context, string) error) (bool, error) {
	msg, err := decodeRedisMessage(stream, redisMessage)
	if err != nil {
		c.logger.Warn("dropping malformed redis stream message", "stream", stream, "id", redisMessage.ID, "error", err)
		if ackErr := ack(ctx, redisMessage.ID); ackErr != nil {
			return false, fmt.Errorf("ack malformed redis stream message %q: %w", redisMessage.ID, ackErr)
		}
		return true, nil
	}
	if err := c.handler.HandleEvent(ctx, msg); err != nil {
		c.logger.Warn("event handler failed", "stream", msg.Stream, "id", msg.ID, "error", err)
		return false, nil
	}
	if err := ack(ctx, redisMessage.ID); err != nil {
		return false, fmt.Errorf("ack redis stream message %q: %w", redisMessage.ID, err)
	}
	return true, nil
}

func decodeRedisMessage(stream string, msg redis.XMessage) (Message, error) {
	outboxID, err := stringValue(msg.Values, "outbox_id", maxRedisMetadataBytes)
	if err != nil {
		return Message{}, err
	}
	partitionKey, err := stringValue(msg.Values, "partition_key", maxRedisMetadataBytes)
	if err != nil {
		return Message{}, err
	}
	payloadRaw, err := stringValue(msg.Values, "payload", maxRedisPayloadBytes)
	if err != nil {
		return Message{}, err
	}
	if !json.Valid([]byte(payloadRaw)) {
		return Message{}, fmt.Errorf("redis stream message %q has invalid json payload", msg.ID)
	}

	return Message{
		ID:           msg.ID,
		Stream:       stream,
		OutboxID:     outboxID,
		PartitionKey: partitionKey,
		Payload:      json.RawMessage(payloadRaw),
	}, nil
}

const (
	maxRedisMetadataBytes = 1024
	maxRedisPayloadBytes  = 1 << 20
)

func stringValue(values map[string]any, key string, maxBytes int) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", fmt.Errorf("redis stream message is missing %q", key)
	}
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return "", fmt.Errorf("redis stream message has empty %q", key)
		}
		if strings.ContainsAny(typed, "\r\n") && key != "payload" {
			return "", fmt.Errorf("redis stream message has invalid %q", key)
		}
		if len(typed) > maxBytes {
			return "", fmt.Errorf("redis stream message %q is too long", key)
		}
		return typed, nil
	case []byte:
		typed = []byte(strings.TrimSpace(string(typed)))
		if len(typed) == 0 {
			return "", fmt.Errorf("redis stream message has empty %q", key)
		}
		if strings.ContainsAny(string(typed), "\r\n") && key != "payload" {
			return "", fmt.Errorf("redis stream message has invalid %q", key)
		}
		if len(typed) > maxBytes {
			return "", fmt.Errorf("redis stream message %q is too long", key)
		}
		return string(typed), nil
	default:
		return "", fmt.Errorf("redis stream message field %q has unsupported type %T", key, value)
	}
}

func isRedisBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
