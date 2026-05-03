package eventstream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type Message struct {
	ID           string
	Stream       string
	OutboxID     string
	PartitionKey string
	Payload      json.RawMessage
}

type Handler interface {
	HandleEvent(ctx context.Context, msg Message) error
}

type HandlerFunc func(ctx context.Context, msg Message) error

func (f HandlerFunc) HandleEvent(ctx context.Context, msg Message) error {
	return f(ctx, msg)
}

type MultiHandler []Handler

func (h MultiHandler) HandleEvent(ctx context.Context, msg Message) error {
	for _, handler := range h {
		if handler == nil {
			continue
		}
		if err := handler.HandleEvent(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

type Router struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[string]Handler)}
}

func (r *Router) Register(event string, handler Handler) error {
	if event == "" {
		return fmt.Errorf("event name is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[event] = handler
	return nil
}

func (r *Router) HandleEvent(ctx context.Context, msg Message) error {
	eventName, err := EventName(msg.Payload)
	if err != nil {
		return err
	}

	r.mu.RLock()
	handler := r.handlers[eventName]
	r.mu.RUnlock()
	if handler == nil {
		return nil
	}
	return handler.HandleEvent(ctx, msg)
}

func EventName(payload json.RawMessage) (string, error) {
	var envelope struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", fmt.Errorf("decode event payload: %w", err)
	}
	if envelope.Event == "" {
		return "", fmt.Errorf("event payload is missing event field")
	}
	return envelope.Event, nil
}
