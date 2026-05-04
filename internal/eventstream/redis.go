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
	client   *redis.Client
	stream   string
	group    string
	consumer string
	count    int64
	block    time.Duration
	handler  Handler
	logger   *slog.Logger
}

type RedisConsumerOptions struct {
	Client   *redis.Client
	Stream   string
	Group    string
	Consumer string
	Count    int64
	Block    time.Duration
	Handler  Handler
	Logger   *slog.Logger
}

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
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &RedisConsumer{
		client:   opts.Client,
		stream:   opts.Stream,
		group:    opts.Group,
		consumer: opts.Consumer,
		count:    opts.Count,
		block:    opts.Block,
		handler:  opts.Handler,
		logger:   opts.Logger,
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
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, ">"},
		Count:    c.count,
		Block:    c.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read redis stream group %q/%q: %w", c.stream, c.group, err)
	}

	processed := 0
	for _, stream := range streams {
		for _, redisMessage := range stream.Messages {
			acked, err := c.processRedisMessage(ctx, stream.Stream, redisMessage, func(ctx context.Context, id string) error {
				return c.client.XAck(ctx, c.stream, c.group, id).Err()
			})
			if err != nil {
				return processed, err
			}
			if acked {
				processed++
			}
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
	outboxID, err := stringValue(msg.Values, "outbox_id")
	if err != nil {
		return Message{}, err
	}
	partitionKey, err := stringValue(msg.Values, "partition_key")
	if err != nil {
		return Message{}, err
	}
	payloadRaw, err := stringValue(msg.Values, "payload")
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

func stringValue(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", fmt.Errorf("redis stream message is missing %q", key)
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", fmt.Errorf("redis stream message has empty %q", key)
		}
		return typed, nil
	case []byte:
		if len(typed) == 0 {
			return "", fmt.Errorf("redis stream message has empty %q", key)
		}
		return string(typed), nil
	default:
		return "", fmt.Errorf("redis stream message field %q has unsupported type %T", key, value)
	}
}

func isRedisBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
