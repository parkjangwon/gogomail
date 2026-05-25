package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Event struct {
	ID           string
	Topic        string
	PartitionKey string
	Payload      json.RawMessage
}

type Store interface {
	FetchPending(ctx context.Context, limit int) ([]Event, error)
	MarkDone(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, cause error) error
}

type FailedEvent struct {
	ID    string
	Cause error
}

type BatchStore interface {
	Store
	MarkDoneBatch(ctx context.Context, ids []string) error
	MarkFailedBatch(ctx context.Context, failures []FailedEvent) error
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type Relay struct {
	store        Store
	publisher    Publisher
	batchSize    int
	pollInterval time.Duration
	workerCount  int
	logger       *slog.Logger
}

// RelayOptions configures a Relay.
//
// Horizontal scaling note: PostgresStore uses FOR UPDATE … SKIP LOCKED so
// multiple Relay instances (or multiple workers within one Relay) can safely
// run against the same database without double-processing. Each concurrent
// worker claims a distinct set of rows.
//
// For strict per-partition ordering across multiple relay *processes*, use
// ShardedPostgresStore so each process handles a disjoint subset of
// partition keys.
type RelayOptions struct {
	Store        Store
	Publisher    Publisher
	BatchSize    int
	PollInterval time.Duration
	// WorkerCount is the number of parallel goroutines each Relay runs.
	// Zero or negative defaults to 1 (single worker, backward-compatible).
	// With WorkerCount > 1 the Relay launches N independent poll loops;
	// Postgres SKIP LOCKED prevents double-claiming.
	WorkerCount int
	Logger      *slog.Logger
}

func NewRelay(opts RelayOptions) (*Relay, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("outbox store is required")
	}
	if opts.Publisher == nil {
		return nil, fmt.Errorf("outbox publisher is required")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.WorkerCount <= 0 {
		opts.WorkerCount = 1
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &Relay{
		store:        opts.Store,
		publisher:    opts.Publisher,
		batchSize:    opts.BatchSize,
		pollInterval: opts.PollInterval,
		workerCount:  opts.WorkerCount,
		logger:       opts.Logger,
	}, nil
}

// Run starts the relay and blocks until ctx is cancelled.
//
// When WorkerCount == 1 (the default) it runs a single poll loop identical to
// the pre-scaling behaviour.  When WorkerCount > 1 it launches N independent
// goroutines, each running their own poll loop.  Postgres FOR UPDATE … SKIP
// LOCKED prevents double-claiming across goroutines and across relay
// *processes*, so this is safe to scale both horizontally (multiple Relay
// processes) and vertically (multiple workers within one process).
func (r *Relay) Run(ctx context.Context) error {
	if r.workerCount <= 1 {
		return r.runWorker(ctx, 0)
	}

	var wg sync.WaitGroup
	for i := range r.workerCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := r.runWorker(ctx, idx); err != nil {
				r.logger.Error("outbox relay worker exited with error", "worker", idx, "error", err)
			}
		}(i)
	}
	wg.Wait()
	return nil
}

// runWorker runs a single poll loop for worker idx.
func (r *Relay) runWorker(ctx context.Context, idx int) error {
	log := r.logger
	if r.workerCount > 1 {
		log = r.logger.With("worker", idx)
	}
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if _, err := r.ProcessOnce(ctx); err != nil {
			log.Error("outbox relay batch failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) ProcessOnce(ctx context.Context) (int, error) {
	events, err := r.store.FetchPending(ctx, r.batchSize)
	if err != nil {
		return 0, fmt.Errorf("fetch pending outbox events: %w", err)
	}

	if batchStore, ok := r.store.(BatchStore); ok {
		return r.processBatch(ctx, batchStore, events)
	}

	return r.processIndividually(ctx, events)
}

func (r *Relay) processIndividually(ctx context.Context, events []Event) (int, error) {
	processed := 0
	for _, event := range events {
		if err := r.publisher.Publish(ctx, event); err != nil {
			if markErr := r.store.MarkFailed(ctx, event.ID, err); markErr != nil {
				return processed, fmt.Errorf("mark outbox event failed after publish error: %w", markErr)
			}
			r.logger.Warn("outbox event publish failed", "id", event.ID, "topic", event.Topic, "error", err)
			continue
		}
		if err := r.store.MarkDone(ctx, event.ID); err != nil {
			return processed, fmt.Errorf("mark outbox event done: %w", err)
		}
		r.logger.Info("outbox event relayed",
			"outbox_id", event.ID,
			"topic", event.Topic,
			"message_id", event.PartitionKey,
		)
		processed++
	}
	return processed, nil
}

func (r *Relay) processBatch(ctx context.Context, store BatchStore, events []Event) (int, error) {
	doneIDs := make([]string, 0, len(events))
	failures := make([]FailedEvent, 0)
	for _, event := range events {
		if err := r.publisher.Publish(ctx, event); err != nil {
			failures = append(failures, FailedEvent{ID: event.ID, Cause: err})
			r.logger.Warn("outbox event publish failed", "id", event.ID, "topic", event.Topic, "error", err)
			continue
		}
		r.logger.Info("outbox event relayed",
			"outbox_id", event.ID,
			"topic", event.Topic,
			"message_id", event.PartitionKey,
		)
		doneIDs = append(doneIDs, event.ID)
	}

	if len(failures) > 0 {
		if err := store.MarkFailedBatch(ctx, failures); err != nil {
			return 0, fmt.Errorf("mark outbox event failures: %w", err)
		}
	}
	if len(doneIDs) == 0 {
		return 0, nil
	}
	if err := store.MarkDoneBatch(ctx, doneIDs); err != nil {
		return 0, fmt.Errorf("mark outbox events done: %w", err)
	}
	return len(doneIDs), nil
}
