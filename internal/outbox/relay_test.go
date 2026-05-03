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
