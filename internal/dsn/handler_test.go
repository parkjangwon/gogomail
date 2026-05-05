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
	"github.com/gogomail/gogomail/internal/storage"
)

func TestBounceHandlerQueuesFailureDSN(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{
		"mailstore/original.eml": []byte("From: Sender <sender@example.com>\r\nTo: Bad <bad@example.net>\r\nSubject: Original\r\nMessage-ID: <original@example.com>\r\n\r\nbody"),
	}}
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
		"storage_path":"mailstore/original.eml",
		"attempted_at":"2026-05-04T01:00:00Z",
		"dsn":{"return":"HDRS","envelope_id":"env-1","notify":["FAILURE"],"original_recipient":"rfc822;alias+40example.net"}
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if queue.topic != "mail.outbound.general" {
		t.Fatalf("topic = %q, want mail.outbound.general", queue.topic)
	}
	if queue.dedupeKey != "dsn:bounce:018f0000-0000-7000-8000-000000000001:bad@example.net" {
		t.Fatalf("dedupeKey = %q, want stable bounce DSN key", queue.dedupeKey)
	}
	if len(store.values) != 2 {
		t.Fatalf("stored messages = %d, want original plus DSN", len(store.values))
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
	if queued.StoragePath != "dsn/2026/05/018f0000-0000-7000-8000-000000000001-bad@example.net.eml" {
		t.Fatalf("StoragePath = %q, want DSN storage path", queued.StoragePath)
	}
	raw := string(store.values[queued.StoragePath])
	for _, want := range []string{
		"Content-Type: message/delivery-status",
		"Original-Envelope-Id: env-1",
		"Final-Recipient: rfc822; bad@example.net",
		"Original-Recipient: rfc822; alias+40example.net",
		"Status: 5.1.1",
		"Diagnostic-Code: smtp; 550 5.1.1 no such user",
		"Content-Type: text/rfc822-headers",
		"Message-Id: <original@example.com>",
		"Subject: Original",
		"Message-ID: <dsn-018f0000-0000-7000-8000-000000000001-bad-example@mx.example.com>",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("stored DSN missing %q:\n%s", want, raw)
		}
	}
}

func TestBounceHandlerQueuesFailureDSNForExhaustedDelivery(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{
		"mailstore/original.eml": []byte("From: Sender <sender@example.com>\r\nTo: Slow <slow@example.net>\r\nSubject: Original\r\nMessage-ID: <original@example.com>\r\n\r\nbody"),
	}}
	queue := &captureQueue{}
	handler := NewBounceHandler(HandlerOptions{
		Store:        store,
		Queue:        queue,
		ReportingMTA: "mx.example.com",
		Now: func() time.Time {
			return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
		},
	})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.delivery_exhausted",
		"message_id":"018f0000-0000-7000-8000-000000000002",
		"rfc_message_id":"<original@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"sender":"sender@example.com",
		"error_message":"451 4.4.7 retry timeout",
		"storage_path":"mailstore/original.eml",
		"dsn_return":"HDRS",
		"dsn_envelope_id":"env-1",
		"exhausted_at":"2026-05-04T01:00:00Z",
		"recipient_details":[{
			"recipient":"slow@example.net",
			"recipient_domain":"example.net",
			"enhanced_status":"4.4.7",
			"dsn_notify":["FAILURE"],
			"original_recipient":"rfc822;alias+40example.net"
		}]
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if queue.dedupeKey != "dsn:bounce:018f0000-0000-7000-8000-000000000002:slow@example.net" {
		t.Fatalf("dedupeKey = %q, want stable exhausted DSN key", queue.dedupeKey)
	}
	var queued delivery.QueuedMessage
	if err := json.Unmarshal(queue.payload, &queued); err != nil {
		t.Fatalf("decode queued payload: %v", err)
	}
	raw := string(store.values[queued.StoragePath])
	for _, want := range []string{
		"Action: failed",
		"Status: 4.4.7",
		"Diagnostic-Code: smtp; 451 4.4.7 retry timeout",
		"Original-Recipient: rfc822; alias+40example.net",
		"Content-Type: text/rfc822-headers",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("stored exhausted DSN missing %q:\n%s", want, raw)
		}
	}
}

func TestBounceHandlerQueuesFailureDSNWithRetFull(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{
		"mailstore/original.eml": []byte("From: Sender <sender@example.com>\r\nTo: Bad <bad@example.net>\r\nSubject: Original FULL\r\nMessage-ID: <original@example.com>\r\n\r\nbody"),
	}}
	queue := &captureQueue{}
	handler := NewBounceHandler(HandlerOptions{
		Store:        store,
		Queue:        queue,
		ReportingMTA: "mx.example.com",
		Now: func() time.Time {
			return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
		},
	})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.bounced",
		"message_id":"018f0000-0000-7000-8000-000000000003",
		"rfc_message_id":"<original@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"sender":"sender@example.com",
		"recipient":"bad@example.net",
		"recipient_domain":"example.net",
		"enhanced_status":"5.1.1",
		"error_message":"550 5.1.1 no such user",
		"storage_path":"mailstore/original.eml",
		"attempted_at":"2026-05-04T01:00:00Z",
		"dsn":{"return":"FULL","notify":["FAILURE"]}
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	var queued delivery.QueuedMessage
	if err := json.Unmarshal(queue.payload, &queued); err != nil {
		t.Fatalf("decode queued payload: %v", err)
	}
	raw := string(store.values[queued.StoragePath])
	for _, want := range []string{
		"Content-Type: message/delivery-status",
		"Content-Type: message/rfc822",
		"Subject: Original FULL",
		"\r\n\r\nbody\r\n--gogomail-dsn-",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("stored DSN missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(raw, "Content-Type: text/rfc822-headers") {
		t.Fatalf("stored DSN contains header-only returned content part:\n%s", raw)
	}
}

func TestBounceHandlerSkipsNullReversePathAndNotifyNever(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"event":"mail.bounced","message_id":"msg-1","recipient":"bad@example.net","sender":"","dsn":{"notify":["FAILURE"]}}`,
		`{"event":"mail.bounced","message_id":"msg-1","recipient":"bad@example.net","sender":"sender@example.com","dsn":{"notify":["NEVER"]}}`,
		`{"event":"mail.bounced","message_id":"msg-1","recipient":"bad@example.net","sender":"sender@example.com","dsn":{"notify":["SUCCESS","DELAY"]}}`,
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

func TestDecodeBounceEventRejectsNewlineIdentity(t *testing.T) {
	t.Parallel()

	_, err := decodeBounceEvent([]byte("{\"event\":\"mail.bounced\",\"message_id\":\"msg-1\\nInjected\",\"recipient\":\"bad@example.net\"}"))
	if err == nil {
		t.Fatal("decodeBounceEvent accepted newline-bearing message identity")
	}
}

func TestDecodeBounceEventRejectsInvalidEnhancedStatus(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"4.2.0", "5.9999.1", "5.x.1", "5.1.1\nInjected"} {
		_, err := decodeBounceEvent([]byte(`{
			"event":"mail.bounced",
			"message_id":"msg-1",
			"sender":"sender@example.com",
			"recipient":"bad@example.net",
			"enhanced_status":"` + status + `"
		}`))
		if err == nil {
			t.Fatalf("decodeBounceEvent accepted enhanced_status %q", status)
		}
	}
}

func TestDecodeBounceEventNormalizesAndValidatesNotify(t *testing.T) {
	t.Parallel()

	event, err := decodeBounceEvent([]byte(`{
		"event":"mail.bounced",
		"message_id":"msg-1",
		"sender":"sender@example.com",
		"recipient":"bad@example.net",
		"dsn":{"notify":[" failure ","FAILURE","delay"]}
	}`))
	if err != nil {
		t.Fatalf("decodeBounceEvent returned error: %v", err)
	}
	if strings.Join(event.DSN.Notify, ",") != "FAILURE,DELAY" {
		t.Fatalf("notify = %v, want normalized unique values", event.DSN.Notify)
	}

	for _, payload := range []string{
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"notify":["BOGUS"]}}`,
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"notify":["NEVER","FAILURE"]}}`,
	} {
		if _, err := decodeBounceEvent([]byte(payload)); err == nil {
			t.Fatalf("decodeBounceEvent accepted invalid notify payload %s", payload)
		}
	}
}

func TestDecodeBounceEventRejectsNewlineDSNMetadata(t *testing.T) {
	t.Parallel()

	_, err := decodeBounceEvent([]byte(`{
		"event":"mail.bounced",
		"message_id":"msg-1",
		"sender":"sender@example.com",
		"recipient":"bad@example.net",
		"dsn":{"envelope_id":"env-1\nInjected"}
	}`))
	if err == nil {
		t.Fatal("decodeBounceEvent accepted newline-bearing DSN metadata")
	}
}

func TestDecodeBounceEventRejectsInvalidReturnAndStoragePath(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","storage_path":"../msg.eml"}`,
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"return":"BODY"}}`,
	} {
		if _, err := decodeBounceEvent([]byte(payload)); err == nil {
			t.Fatalf("decodeBounceEvent accepted invalid payload %s", payload)
		}
	}
}

func TestDecodeBounceEventRejectsInvalidDSNXTextMetadata(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"envelope_id":"env 1"}}`,
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"envelope_id":"env+XX1"}}`,
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"original_recipient":"rfc822;alias example.net"}}`,
		`{"event":"mail.bounced","message_id":"msg-1","sender":"sender@example.com","recipient":"bad@example.net","dsn":{"original_recipient":"rfc822;alias+XXexample.net"}}`,
	} {
		if _, err := decodeBounceEvent([]byte(payload)); err == nil {
			t.Fatalf("decodeBounceEvent accepted invalid DSN metadata payload %s", payload)
		}
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

func TestBounceHandlerUsesSingleClockSnapshot(t *testing.T) {
	t.Parallel()

	times := []time.Time{
		time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	calls := 0
	store := &memoryStore{values: map[string][]byte{}}
	queue := &captureQueue{}
	handler := NewBounceHandler(HandlerOptions{
		Store: store,
		Queue: queue,
		Now: func() time.Time {
			out := times[calls]
			calls++
			return out
		},
	})

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.bounced",
		"message_id":"018f0000-0000-7000-8000-000000000001",
		"sender":"sender@example.com",
		"recipient":"bad@example.net"
	}`)}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("clock calls = %d, want one snapshot", calls)
	}
}

func TestBounceHandlerFormatsNamedPostmaster(t *testing.T) {
	t.Parallel()

	store := &memoryStore{values: map[string][]byte{}}
	queue := &captureQueue{}
	handler := NewBounceHandler(HandlerOptions{
		Store:      store,
		Queue:      queue,
		Postmaster: "Ops Bounces <bounces@example.com>",
		Now: func() time.Time {
			return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
		},
	})

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.bounced",
		"message_id":"018f0000-0000-7000-8000-000000000001",
		"sender":"sender@example.com",
		"recipient":"bad@example.net"
	}`)}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	var queued delivery.QueuedMessage
	if err := json.Unmarshal(queue.payload, &queued); err != nil {
		t.Fatalf("decode queued payload: %v", err)
	}
	raw := string(store.values[queued.StoragePath])
	if !strings.Contains(raw, `From: Ops Bounces <bounces@example.com>`) {
		t.Fatalf("stored DSN From header not formatted from named postmaster:\n%s", raw)
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
	dedupeKey    string
	payload      []byte
}

func (q *captureQueue) Enqueue(_ context.Context, topic string, partitionKey string, payload []byte) error {
	return q.EnqueueOnce(context.Background(), topic, partitionKey, "", payload)
}

func (q *captureQueue) EnqueueOnce(_ context.Context, topic string, partitionKey string, dedupeKey string, payload []byte) error {
	q.topic = topic
	q.partitionKey = partitionKey
	q.dedupeKey = dedupeKey
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

func (s *memoryStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.values[path])), nil
}

func (s *memoryStore) Stat(_ context.Context, path string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Path: path, Size: int64(len(s.values[path]))}, nil
}

func (s *memoryStore) Copy(_ context.Context, sourcePath string, destPath string) error {
	s.values[destPath] = append([]byte(nil), s.values[sourcePath]...)
	return nil
}

func (s *memoryStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s *memoryStore) Delete(_ context.Context, path string) error {
	delete(s.values, path)
	return nil
}
