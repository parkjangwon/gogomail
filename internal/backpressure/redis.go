package backpressure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultStateKey = "backpressure:smtp:state"

type RedisBackpressure struct {
	client *redis.Client
	key    string
}

type State struct {
	Level     string     `json:"level"`
	Reason    string     `json:"reason,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	Until     *time.Time `json:"until,omitempty"`
}

type StateUpdate struct {
	Level  string     `json:"level"`
	Reason string     `json:"reason,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
}

func NewRedisBackpressure(client *redis.Client, key string) *RedisBackpressure {
	if strings.TrimSpace(key) == "" {
		key = DefaultStateKey
	}
	return &RedisBackpressure{client: client, key: key}
}

func (b *RedisBackpressure) Accept(ctx context.Context) (bool, error) {
	state, err := b.State(ctx)
	if err != nil {
		return false, err
	}
	return acceptsState(state.Level), nil
}

func (b *RedisBackpressure) State(ctx context.Context) (State, error) {
	raw, err := b.client.Get(ctx, b.key).Result()
	if err == redis.Nil {
		return State{Level: "normal"}, nil
	}
	if err != nil {
		return State{}, err
	}
	state, err := decodeState(raw)
	if err != nil {
		return State{}, err
	}
	if state.Until != nil && time.Now().After(*state.Until) {
		return State{Level: "normal"}, nil
	}
	return state, nil
}

func (b *RedisBackpressure) SetState(ctx context.Context, update StateUpdate) (State, error) {
	state, err := normalizeStateUpdate(update)
	if err != nil {
		return State{}, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return State{}, fmt.Errorf("marshal backpressure state: %w", err)
	}
	if err := b.client.Set(ctx, b.key, raw, 0).Err(); err != nil {
		return State{}, err
	}
	return state, nil
}

func decodeState(raw string) (State, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return State{Level: "normal"}, nil
	}
	if strings.HasPrefix(raw, "{") {
		var state State
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			return State{}, fmt.Errorf("decode backpressure state: %w", err)
		}
		state.Level = normalizeLevel(state.Level)
		if state.UpdatedAt.IsZero() {
			state.UpdatedAt = time.Now().UTC()
		}
		return state, nil
	}
	return State{Level: normalizeLevel(raw), UpdatedAt: time.Now().UTC()}, nil
}

func normalizeStateUpdate(update StateUpdate) (State, error) {
	level := normalizeLevel(update.Level)
	if !validLevel(level) {
		return State{}, fmt.Errorf("unsupported backpressure level %q", update.Level)
	}
	reason := strings.TrimSpace(update.Reason)
	if strings.ContainsAny(reason, "\r\n") {
		return State{}, fmt.Errorf("reason must not contain newlines")
	}
	if len(reason) > 512 {
		return State{}, fmt.Errorf("reason is too long")
	}
	return State{
		Level:     level,
		Reason:    reason,
		Until:     update.Until,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func normalizeLevel(level string) string {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		return "normal"
	}
	return level
}

func validLevel(level string) bool {
	switch level {
	case "normal", "warning", "danger", "critical":
		return true
	default:
		return false
	}
}

func acceptsState(state string) bool {
	switch normalizeLevel(state) {
	case "", "normal", "warning":
		return true
	case "danger", "critical":
		return false
	default:
		return true
	}
}
