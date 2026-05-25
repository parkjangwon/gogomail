package outbox

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRelayWorkerCountDefaultsToOne verifies backward compatibility: with
// WorkerCount unset (0), NewRelay normalizes it to 1.
func TestRelayWorkerCountDefaultsToOne(t *testing.T) {
	relay, err := NewRelay(RelayOptions{
		Store:     &fakeStore{},
		Publisher: &fakePublisher{},
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	if relay.workerCount != 1 {
		t.Errorf("workerCount = %d, want 1", relay.workerCount)
	}
}

// TestRelayWorkerCountNegativeDefaultsToOne verifies negative values normalize.
func TestRelayWorkerCountNegativeDefaultsToOne(t *testing.T) {
	relay, err := NewRelay(RelayOptions{
		Store:       &fakeStore{},
		Publisher:   &fakePublisher{},
		WorkerCount: -5,
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	if relay.workerCount != 1 {
		t.Errorf("workerCount = %d, want 1", relay.workerCount)
	}
}

// TestRelayRunSingleWorkerStopsOnCancel verifies the Run loop exits when the
// context is cancelled (single-worker path).
func TestRelayRunSingleWorkerStopsOnCancel(t *testing.T) {
	store := &fakeStore{events: []Event{}}
	relay, err := NewRelay(RelayOptions{
		Store:        store,
		Publisher:    &fakePublisher{},
		PollInterval: 50 * time.Millisecond,
		WorkerCount:  1,
		Logger:       slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	// Should terminate within a reasonable window around the context deadline.
	if elapsed > 500*time.Millisecond {
		t.Errorf("Run took too long: %v", elapsed)
	}
}

// TestRelayRunMultipleWorkersStopsOnCancel verifies the multi-worker path
// shuts down cleanly when the context is cancelled.
func TestRelayRunMultipleWorkersStopsOnCancel(t *testing.T) {
	store := &fakeStore{events: []Event{}}
	relay, err := NewRelay(RelayOptions{
		Store:        store,
		Publisher:    &fakePublisher{},
		PollInterval: 50 * time.Millisecond,
		WorkerCount:  3,
		Logger:       slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("Run (3 workers) took too long: %v", elapsed)
	}
}

// TestRelayMultipleWorkersProcessEvents verifies that multiple workers can all
// consume events concurrently.  We use a thread-safe fake store.
func TestRelayMultipleWorkersProcessEvents(t *testing.T) {
	const numEvents = 30

	events := make([]Event, numEvents)
	for i := range numEvents {
		events[i] = Event{ID: "e" + string(rune('0'+i%10)) + string(rune('a'+i/10)), Topic: "t", PartitionKey: "pk"}
	}

	store := &threadSafeFakeStore{events: events}
	publisher := &atomicPublisher{}

	relay, err := NewRelay(RelayOptions{
		Store:        store,
		Publisher:    publisher,
		BatchSize:    5,
		PollInterval: 20 * time.Millisecond,
		WorkerCount:  4,
		Logger:       slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = relay.Run(ctx)

	published := int(publisher.count.Load())
	if published == 0 {
		t.Error("expected at least some events to be published by multi-worker relay")
	}
}

// threadSafeFakeStore is a fakeStore safe for concurrent access.
type threadSafeFakeStore struct {
	mu     sync.Mutex
	events []Event
	done   []string
}

func (s *threadSafeFakeStore) FetchPending(_ context.Context, limit int) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return nil, nil
	}
	if limit > len(s.events) {
		limit = len(s.events)
	}
	batch := make([]Event, limit)
	copy(batch, s.events[:limit])
	s.events = s.events[limit:]
	return batch, nil
}

func (s *threadSafeFakeStore) MarkDone(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = append(s.done, id)
	return nil
}

func (s *threadSafeFakeStore) MarkFailed(_ context.Context, _ string, _ error) error {
	return nil
}

// atomicPublisher counts published events atomically.
type atomicPublisher struct {
	count atomic.Int64
}

func (p *atomicPublisher) Publish(_ context.Context, _ Event) error {
	p.count.Add(1)
	return nil
}

// TestNewShardedPostgresStoreValidation verifies the constructor rejects bad inputs.
func TestNewShardedPostgresStoreValidation(t *testing.T) {
	_, err := NewShardedPostgresStore(nil, 10, 3, -1)
	if err == nil {
		t.Error("expected error for shardIndex=-1, got nil")
	}
	_, err = NewShardedPostgresStore(nil, 10, 3, 3)
	if err == nil {
		t.Error("expected error for shardIndex==totalShards, got nil")
	}
	_, err = NewShardedPostgresStore(nil, 10, 3, 0)
	if err != nil {
		t.Errorf("unexpected error for valid shardIndex=0: %v", err)
	}
}
