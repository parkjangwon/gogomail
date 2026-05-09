package sync

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// fakeHandler is a controllable EventHandler for tests.
type fakeHandler struct {
	mu       sync.Mutex
	calls    []Event
	returnErr error
}

func (f *fakeHandler) Handle(_ context.Context, _ string, event Event) error {
	f.mu.Lock()
	f.calls = append(f.calls, event)
	f.mu.Unlock()
	return f.returnErr
}

func (f *fakeHandler) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestBackgroundDispatcherSuccess(t *testing.T) {
	h := &fakeHandler{}
	d := NewBackgroundDispatcher(h, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	event := Event{MailboxID: "inbox", UserID: "u1", Type: "add", Version: 1}
	d.Dispatch(context.Background(), "u1", event, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if h.callCount() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if h.callCount() != 1 {
		t.Fatalf("expected 1 handler call, got %d", h.callCount())
	}
	if d.ErrorCount() != 0 {
		t.Fatalf("expected 0 errors, got %d", d.ErrorCount())
	}
}

// TestBackgroundDispatcherErrorLogged verifies that handler errors are counted
// and not silently dropped.
func TestBackgroundDispatcherErrorLogged(t *testing.T) {
	h := &fakeHandler{returnErr: errors.New("handler failure")}
	d := NewBackgroundDispatcher(h, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	event := Event{MailboxID: "inbox", UserID: "u1", Type: "add", Version: 1}
	d.Dispatch(context.Background(), "u1", event, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if d.ErrorCount() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if d.ErrorCount() != 1 {
		t.Fatalf("expected error count 1, got %d", d.ErrorCount())
	}
}

// TestBackgroundDispatcherMultipleErrors verifies cumulative error counting.
func TestBackgroundDispatcherMultipleErrors(t *testing.T) {
	const dispatches = 10
	h := &fakeHandler{returnErr: errors.New("always fails")}
	d := NewBackgroundDispatcher(h, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	for i := 0; i < dispatches; i++ {
		event := Event{MailboxID: "inbox", UserID: "u1", Type: "add", Version: int64(i)}
		d.Dispatch(context.Background(), "u1", event, i+1)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d.ErrorCount() == dispatches {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if d.ErrorCount() != dispatches {
		t.Fatalf("expected %d errors, got %d", dispatches, d.ErrorCount())
	}
}

// TestBackgroundDispatcherNilLogger verifies nil logger falls back to slog.Default().
func TestBackgroundDispatcherNilLogger(t *testing.T) {
	h := &fakeHandler{}
	d := NewBackgroundDispatcher(h, nil)
	if d.logger == nil {
		t.Fatal("expected logger to be set to slog.Default(), got nil")
	}
}
