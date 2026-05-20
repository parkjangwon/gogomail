package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	logger       *slog.Logger
}

type RelayOptions struct {
	Store        Store
	Publisher    Publisher
	BatchSize    int
	PollInterval time.Duration
	Logger       *slog.Logger
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
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &Relay{
		store:        opts.Store,
		publisher:    opts.Publisher,
		batchSize:    opts.BatchSize,
		pollInterval: opts.PollInterval,
		logger:       opts.Logger,
	}, nil
}

func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if _, err := r.ProcessOnce(ctx); err != nil {
			r.logger.Error("outbox relay batch failed", "error", err)
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
