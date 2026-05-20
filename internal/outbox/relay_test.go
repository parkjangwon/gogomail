package outbox

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestRelayProcessOncePublishesAndMarksDone(t *testing.T) {
	t.Parallel()

	store := &fakeStore{events: []Event{{
		ID:           "event-1",
		Topic:        "mail.event",
		PartitionKey: "message-1",
		Payload:      []byte(`{"event":"mail.stored"}`),
	}}}
	publisher := &fakePublisher{}
	relay, err := NewRelay(RelayOptions{
		Store:        store,
		Publisher:    publisher,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		Logger:       slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay returned error: %v", err)
	}

	processed, err := relay.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if len(publisher.published) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.published))
	}
	if store.done[0] != "event-1" {
		t.Fatalf("done id = %q, want event-1", store.done[0])
	}
}

func TestRelayProcessOnceMarksPublishFailure(t *testing.T) {
	t.Parallel()

	store := &fakeStore{events: []Event{{
		ID:           "event-1",
		Topic:        "mail.event",
		PartitionKey: "message-1",
		Payload:      []byte(`{"event":"mail.stored"}`),
	}}}
	publisher := &fakePublisher{err: errors.New("redis down")}
	relay, err := NewRelay(RelayOptions{
		Store:     store,
		Publisher: publisher,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay returned error: %v", err)
	}

	processed, err := relay.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}
	if len(store.failed) != 1 || store.failed[0] != "event-1" {
		t.Fatalf("failed ids = %+v, want [event-1]", store.failed)
	}
}

func TestRelayProcessOnceUsesBatchMarkingWhenAvailable(t *testing.T) {
	t.Parallel()

	store := &fakeBatchStore{fakeStore: fakeStore{events: []Event{
		{ID: "event-1", Topic: "mail.event", PartitionKey: "message-1", Payload: []byte(`{"event":"one"}`)},
		{ID: "event-2", Topic: "mail.event", PartitionKey: "message-2", Payload: []byte(`{"event":"two"}`)},
	}}}
	publisher := &fakePublisher{}
	relay, err := NewRelay(RelayOptions{
		Store:     store,
		Publisher: publisher,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay returned error: %v", err)
	}

	processed, err := relay.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}
	if len(store.doneBatches) != 1 {
		t.Fatalf("done batch calls = %d, want 1", len(store.doneBatches))
	}
	if got := store.doneBatches[0]; len(got) != 2 || got[0] != "event-1" || got[1] != "event-2" {
		t.Fatalf("done batch = %+v, want [event-1 event-2]", got)
	}
	if len(store.done) != 0 {
		t.Fatalf("individual done calls = %+v, want none", store.done)
	}
}

func TestRelayProcessOnceUsesBatchFailureMarkingWhenAvailable(t *testing.T) {
	t.Parallel()

	store := &fakeBatchStore{fakeStore: fakeStore{events: []Event{
		{ID: "event-1", Topic: "mail.event", PartitionKey: "message-1", Payload: []byte(`{"event":"one"}`)},
		{ID: "event-2", Topic: "mail.event", PartitionKey: "message-2", Payload: []byte(`{"event":"two"}`)},
	}}}
	publisher := &fakeSelectivePublisher{fail: map[string]error{"event-2": errors.New("redis down")}}
	relay, err := NewRelay(RelayOptions{
		Store:     store,
		Publisher: publisher,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewRelay returned error: %v", err)
	}

	processed, err := relay.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if len(store.failedBatches) != 1 {
		t.Fatalf("failed batch calls = %d, want 1", len(store.failedBatches))
	}
	if got := store.failedBatches[0]; len(got) != 1 || got[0].ID != "event-2" {
		t.Fatalf("failed batch = %+v, want event-2", got)
	}
	if len(store.doneBatches) != 1 || len(store.doneBatches[0]) != 1 || store.doneBatches[0][0] != "event-1" {
		t.Fatalf("done batch = %+v, want [[event-1]]", store.doneBatches)
	}
	if len(store.failed) != 0 {
		t.Fatalf("individual failed calls = %+v, want none", store.failed)
	}
}

type fakeStore struct {
	events []Event
	done   []string
	failed []string
}

func (s *fakeStore) FetchPending(context.Context, int) ([]Event, error) {
	return s.events, nil
}

func (s *fakeStore) MarkDone(_ context.Context, id string) error {
	s.done = append(s.done, id)
	return nil
}

func (s *fakeStore) MarkFailed(_ context.Context, id string, _ error) error {
	s.failed = append(s.failed, id)
	return nil
}

type fakeBatchStore struct {
	fakeStore
	doneBatches   [][]string
	failedBatches [][]FailedEvent
}

func (s *fakeBatchStore) MarkDoneBatch(_ context.Context, ids []string) error {
	s.doneBatches = append(s.doneBatches, append([]string(nil), ids...))
	return nil
}

func (s *fakeBatchStore) MarkFailedBatch(_ context.Context, failures []FailedEvent) error {
	s.failedBatches = append(s.failedBatches, append([]FailedEvent(nil), failures...))
	return nil
}

type fakePublisher struct {
	err       error
	published []Event
}

func (p *fakePublisher) Publish(_ context.Context, event Event) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, event)
	return nil
}

type fakeSelectivePublisher struct {
	fail      map[string]error
	published []Event
}

func (p *fakeSelectivePublisher) Publish(_ context.Context, event Event) error {
	if err := p.fail[event.ID]; err != nil {
		return err
	}
	p.published = append(p.published, event)
	return nil
}
