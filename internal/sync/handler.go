package sync

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// EventHandler processes a sync event for a user.
type EventHandler interface {
	Handle(ctx context.Context, userID string, event Event) error
}

// BackgroundDispatcher runs event handlers in background goroutines.
// Errors are logged with context and counted via an atomic error counter.
type BackgroundDispatcher struct {
	handler    EventHandler
	logger     *slog.Logger
	errorCount atomic.Int64
}

// NewBackgroundDispatcher creates a BackgroundDispatcher that dispatches events
// to handler in background goroutines. If logger is nil, slog.Default() is used.
func NewBackgroundDispatcher(handler EventHandler, logger *slog.Logger) *BackgroundDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackgroundDispatcher{
		handler: handler,
		logger:  logger,
	}
}

// Dispatch runs the handler in a background goroutine. Errors are logged and
// counted; they are never silently dropped.
func (d *BackgroundDispatcher) Dispatch(ctx context.Context, userID string, event Event, attempt int) {
	go func() {
		if err := d.handler.Handle(ctx, userID, event); err != nil {
			d.errorCount.Add(1)
			d.logger.Error("background sync handler error",
				"user", userID,
				"event_type", event.Type,
				"mailbox", event.MailboxID,
				"attempt", attempt,
				"error", err,
			)
		}
	}()
}

// ErrorCount returns the cumulative number of handler errors.
func (d *BackgroundDispatcher) ErrorCount() int64 {
	return d.errorCount.Load()
}
