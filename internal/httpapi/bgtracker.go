package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// BackgroundTracker tracks fire-and-forget goroutines spawned by HTTP handlers
// (invite email, welcome email, password reset token issue, etc.) so that
// graceful shutdown can wait for them to finish before the process exits.
//
// Tracked goroutines run with a context detached from the caller's request
// context (so client disconnects do not cancel them) but values from the
// caller's context (request ID, etc.) are preserved via context.WithoutCancel.
type BackgroundTracker struct {
	wg     sync.WaitGroup
	logger *slog.Logger

	mu     sync.Mutex
	closed bool
}

// NewBackgroundTracker returns a tracker. If logger is nil, slog.Default() is used.
func NewBackgroundTracker(logger *slog.Logger) *BackgroundTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackgroundTracker{logger: logger}
}

// Track runs fn in a new goroutine while keeping the tracker's WaitGroup busy.
//
// The supplied parentCtx contributes only its values (request ID, etc.) —
// cancellation is detached via context.WithoutCancel, then a per-goroutine
// timeout is applied. If timeout is zero, a default of 30 seconds is used.
//
// After Wait has been called the tracker rejects new work and runs fn
// synchronously with the supplied ctx so that callers do not silently lose
// work during shutdown races.
func (t *BackgroundTracker) Track(parentCtx context.Context, timeout time.Duration, fn func(ctx context.Context)) {
	if t == nil || fn == nil {
		return
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		// Running synchronously is safer than dropping work; Wait was already
		// called so we are during shutdown and the caller is unlikely to be a
		// hot HTTP request.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), timeout)
		defer cancel()
		fn(ctx)
		return
	}
	t.wg.Add(1)
	t.mu.Unlock()

	go func() {
		defer t.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				t.logger.Error("background task panicked", "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), timeout)
		defer cancel()
		fn(ctx)
	}()
}

// Wait blocks until all tracked goroutines complete or ctx is cancelled.
// Once Wait has been called, subsequent Track calls run synchronously.
func (t *BackgroundTracker) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.Join(ctx.Err(), errors.New("background tracker: timeout waiting for goroutines"))
	}
}
