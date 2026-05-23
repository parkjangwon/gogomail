package sync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
)

// EventHandler processes a sync event for a user.
type EventHandler interface {
	Handle(ctx context.Context, userID string, event Event) error
}

// BackgroundDispatcher runs event handlers in background goroutines.
// Errors are logged with context and counted via an atomic error counter.
//
// Pending goroutines are tracked by an internal WaitGroup so callers can
// drain in-flight work during graceful shutdown via Wait.
type BackgroundDispatcher struct {
	handler    EventHandler
	logger     *slog.Logger
	errorCount atomic.Int64

	wg sync.WaitGroup

	mu     sync.Mutex
	closed bool
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
//
// The caller's ctx contributes only its values (request ID, trace info) — its
// cancellation is detached via context.WithoutCancel so that an HTTP request
// returning before the goroutine runs does not poison the handler.
func (d *BackgroundDispatcher) Dispatch(ctx context.Context, userID string, event Event, attempt int) {
	handlerCtx := context.WithoutCancel(ctx)

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		// Run synchronously during/after shutdown to avoid silently dropping
		// the event. Callers reaching here are exceptional.
		if err := d.handler.Handle(handlerCtx, userID, event); err != nil {
			d.errorCount.Add(1)
			d.logger.Error("background sync handler error (sync)",
				"user", userID,
				"event_type", event.Type,
				"mailbox", event.MailboxID,
				"attempt", attempt,
				"error", err,
			)
		}
		return
	}
	d.wg.Add(1)
	d.mu.Unlock()

	go func() {
		defer d.wg.Done()
		if err := d.handler.Handle(handlerCtx, userID, event); err != nil {
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

// Wait blocks until all in-flight goroutines complete or ctx is cancelled.
// After Wait has been called the dispatcher refuses to launch new goroutines
// and instead processes incoming Dispatch calls synchronously.
func (d *BackgroundDispatcher) Wait(ctx context.Context) error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.Join(ctx.Err(), errors.New("background dispatcher: timeout waiting for goroutines"))
	}
}

// ErrorCount returns the cumulative number of handler errors.
func (d *BackgroundDispatcher) ErrorCount() int64 {
	return d.errorCount.Load()
}
