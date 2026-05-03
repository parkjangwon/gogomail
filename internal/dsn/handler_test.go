package dsn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/outbound"
)

func TestBounceHandlerQueuesFailureDSN(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{}}
	queue := &captureQueue{}
	handler := NewBounceHandler(HandlerOptions{
		Store:        store,
		Queue:        queue,
		ReportingMTA: "mx.example.com",
		Farm:         outbound.FarmGeneral,
		Now: func() time.Time {
			return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
		},
	})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.bounced",
		"message_id":"018f0000-0000-7000-8000-000000000001",
		"rfc_message_id":"<original@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"sender":"sender@example.com",
		"recipient":"bad@example.net",
		"recipient_domain":"example.net",
		"enhanced_status":"5.1.1",
		"error_message":"550 5.1.1 no such user",
		"attempted_at":"2026-05-04T01:00:00Z",
		"dsn":{"envelope_id":"env-1","notify":["FAILURE"],"original_recipient":"rfc822;alias+40example.net"}
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if queue.topic != "mail.outbound.general" {
		t.Fatalf("topic = %q, want mail.outbound.general", queue.topic)
	}
	if len(store.values) != 1 {
		t.Fatalf("stored messages = %d, want 1", len(store.values))
	}

	var queued delivery.QueuedMessage
	if err := json.Unmarshal(queue.payload, &queued); err != nil {
		t.Fatalf("decode queued payload: %v", err)
	}
	if queued.From.Email != "" {
		t.Fatalf("queued From = %q, want null reverse-path", queued.From.Email)
	}
	if len(queued.To) != 1 || queued.To[0].Email != "sender@example.com" {
		t.Fatalf("queued To = %+v, want original sender", queued.To)
	}
	if queued.StoragePath == "" || !strings.HasPrefix(queued.StoragePath, "dsn/2026/05/") {
		t.Fatalf("StoragePath = %q, want DSN storage path", queued.StoragePath)
	}
	raw := string(store.values[queued.StoragePath])
	for _, want := range []string{
		"Content-Type: message/delivery-status",
		"Original-Envelope-Id: env-1",
		"Final-Recipient: rfc822; bad@example.net",
		"Original-Recipient: rfc822;alias+40example.net",
		"Status: 5.1.1",
		"Diagnostic-Code: smtp; 550 5.1.1 no such user",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("stored DSN missing %q:\n%s", want, raw)
		}
	}
}

func TestBounceHandlerSkipsNullReversePathAndNotifyNever(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"event":"mail.bounced","message_id":"msg-1","recipient":"bad@example.net","sender":"","dsn":{"notify":["FAILURE"]}}`,
		`{"event":"mail.bounced","message_id":"msg-1","recipient":"bad@example.net","sender":"sender@example.com","dsn":{"notify":["NEVER"]}}`,
	} {
		store := &memoryStore{values: map[string][]byte{}}
		queue := &captureQueue{}
		handler := NewBounceHandler(HandlerOptions{Store: store, Queue: queue})
		if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(payload)}); err != nil {
			t.Fatalf("HandleEvent returned error: %v", err)
		}
		if len(store.values) != 0 || len(queue.payload) != 0 {
			t.Fatalf("handler generated DSN for payload %s", payload)
		}
	}
}

func TestDecodeBounceEventNormalizesAddresses(t *testing.T) {
	t.Parallel()

	event, err := decodeBounceEvent([]byte(`{
		"event":"mail.bounced",
		"message_id":"msg-1",
		"sender":" Sender@Example.COM ",
		"recipient":" Bad@Example.NET "
	}`))
	if err != nil {
		t.Fatalf("decodeBounceEvent returned error: %v", err)
	}
	if event.Sender != "sender@example.com" || event.Recipient != "bad@example.net" {
		t.Fatalf("addresses = sender %q recipient %q, want normalized", event.Sender, event.Recipient)
	}
}

func TestDecodeBounceEventRejectsInvalidRecipient(t *testing.T) {
	t.Parallel()

	_, err := decodeBounceEvent([]byte(`{
		"event":"mail.bounced",
		"message_id":"msg-1",
		"sender":"sender@example.com",
		"recipient":"not an address"
	}`))
	if err == nil {
		t.Fatal("decodeBounceEvent accepted invalid recipient")
	}
}

func TestBounceHandlerDeletesStoredMessageWhenQueueFails(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{}}
	handler := NewBounceHandler(HandlerOptions{
		Store: store,
		Queue: failingQueue{err: errors.New("database down")},
		Now: func() time.Time {
			return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
		},
	})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.bounced",
		"message_id":"018f0000-0000-7000-8000-000000000001",
		"sender":"sender@example.com",
		"recipient":"bad@example.net"
	}`)})
	if err == nil {
		t.Fatal("HandleEvent returned nil, want queue failure")
	}
	if len(store.values) != 0 {
		t.Fatalf("stored messages = %d, want compensation delete", len(store.values))
	}
}

type failingQueue struct {
	err error
}

func (q failingQueue) Enqueue(context.Context, string, string, []byte) error {
	return q.err
}

type captureQueue struct {
	topic        string
	partitionKey string
	payload      []byte
}

func (q *captureQueue) Enqueue(_ context.Context, topic string, partitionKey string, payload []byte) error {
	q.topic = topic
	q.partitionKey = partitionKey
	q.payload = append([]byte(nil), payload...)
	return nil
}

type memoryStore struct {
	values map[string][]byte
}

func (s *memoryStore) Put(_ context.Context, path string, body io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(body); err != nil {
		return err
	}
	s.values[path] = append([]byte(nil), buf.Bytes()...)
	return nil
}

func (s *memoryStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (s *memoryStore) Delete(_ context.Context, path string) error {
	delete(s.values, path)
	return nil
}
