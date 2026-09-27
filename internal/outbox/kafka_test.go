package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// fakeKafkaWriter records written messages and can be configured to fail.
type fakeKafkaWriter struct {
	mu       sync.Mutex
	messages []kafka.Message
	failWith error
	closed   bool
	calls    int
}

func (f *fakeKafkaWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failWith != nil {
		return f.failWith
	}
	f.messages = append(f.messages, msgs...)
	return nil
}

func (f *fakeKafkaWriter) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeKafkaWriter) written() []kafka.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]kafka.Message, len(f.messages))
	copy(out, f.messages)
	return out
}

func validEvent() Event {
	return Event{
		ID:           "outbox-1",
		Topic:        "mail.stored",
		PartitionKey: "user-42",
		Payload:      json.RawMessage(`{"event":"mail.stored"}`),
	}
}

func TestKafkaPublisherPublishesMessage(t *testing.T) {
	writer := &fakeKafkaWriter{}
	pub := newKafkaPublisherWithWriter(writer, "")

	if err := pub.Publish(context.Background(), validEvent()); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	msgs := writer.written()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg := msgs[0]
	if msg.Topic != "mail.stored" {
		t.Fatalf("unexpected topic %q", msg.Topic)
	}
	if string(msg.Key) != "user-42" {
		t.Fatalf("unexpected key %q", string(msg.Key))
	}
	if string(msg.Value) != `{"event":"mail.stored"}` {
		t.Fatalf("unexpected value %q", string(msg.Value))
	}
	var sawOutboxID bool
	for _, h := range msg.Headers {
		if h.Key == "outbox_id" && string(h.Value) == "outbox-1" {
			sawOutboxID = true
		}
	}
	if !sawOutboxID {
		t.Fatalf("expected outbox_id header, headers=%v", msg.Headers)
	}
}

func TestKafkaPublisherAppliesTopicPrefix(t *testing.T) {
	writer := &fakeKafkaWriter{}
	pub := newKafkaPublisherWithWriter(writer, "gogomail.")

	if err := pub.Publish(context.Background(), validEvent()); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	msgs := writer.written()
	if len(msgs) != 1 || msgs[0].Topic != "gogomail.mail.stored" {
		t.Fatalf("expected prefixed topic, got %+v", msgs)
	}
}

func TestKafkaPublisherPropagatesWriteError(t *testing.T) {
	// Graceful degradation contract: a broker error is returned to the relay,
	// which marks the event failed and retries later. The publisher must never
	// panic or silently drop the event.
	brokerDown := errors.New("dial tcp: connection refused")
	writer := &fakeKafkaWriter{failWith: brokerDown}
	pub := newKafkaPublisherWithWriter(writer, "")

	err := pub.Publish(context.Background(), validEvent())
	if err == nil {
		t.Fatal("expected error when broker write fails")
	}
	if !errors.Is(err, brokerDown) {
		t.Fatalf("expected wrapped broker error, got %v", err)
	}
	if len(writer.written()) != 0 {
		t.Fatal("no message should be recorded on failure")
	}
}

func TestKafkaPublisherRejectsInvalidEvents(t *testing.T) {
	writer := &fakeKafkaWriter{}
	pub := newKafkaPublisherWithWriter(writer, "")

	cases := map[string]Event{
		"missing topic":   {ID: "1", PartitionKey: "k", Payload: json.RawMessage(`{}`)},
		"missing id":      {Topic: "t", PartitionKey: "k", Payload: json.RawMessage(`{}`)},
		"empty payload":   {ID: "1", Topic: "t", PartitionKey: "k", Payload: json.RawMessage(``)},
		"invalid json":    {ID: "1", Topic: "t", PartitionKey: "k", Payload: json.RawMessage(`not json`)},
		"topic with crlf": {ID: "1", Topic: "t\ntopic", PartitionKey: "k", Payload: json.RawMessage(`{}`)},
	}
	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			if err := pub.Publish(context.Background(), event); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
	if writer.calls != 0 {
		t.Fatalf("writer should not be called for invalid events, calls=%d", writer.calls)
	}
}

func TestKafkaPublisherClose(t *testing.T) {
	writer := &fakeKafkaWriter{}
	pub := newKafkaPublisherWithWriter(writer, "")
	if err := pub.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !writer.closed {
		t.Fatal("expected underlying writer to be closed")
	}
}

func TestParseRequiredAcks(t *testing.T) {
	cases := map[string]kafka.RequiredAcks{
		"":       kafka.RequireAll,
		"all":    kafka.RequireAll,
		"leader": kafka.RequireOne,
		"one":    kafka.RequireOne,
		"none":   kafka.RequireNone,
	}
	for value, want := range cases {
		got, err := parseRequiredAcks(value)
		if err != nil {
			t.Fatalf("parseRequiredAcks(%q) error: %v", value, err)
		}
		if got != want {
			t.Fatalf("parseRequiredAcks(%q)=%v want %v", value, got, want)
		}
	}
	if _, err := parseRequiredAcks("bogus"); err == nil {
		t.Fatal("expected error for unsupported acks value")
	}
}

func TestNewKafkaPublisherValidatesBrokers(t *testing.T) {
	if _, err := NewKafkaPublisher(KafkaOptions{Brokers: nil}); err == nil {
		t.Fatal("expected error when no brokers are configured")
	}
	if _, err := NewKafkaPublisher(KafkaOptions{Brokers: []string{"  "}}); err == nil {
		t.Fatal("expected error when brokers are blank")
	}
}

func TestNewKafkaPublisherRejectsBadSASL(t *testing.T) {
	_, err := NewKafkaPublisher(KafkaOptions{
		Brokers:       []string{"localhost:9092"},
		SASLMechanism: "totally-unknown",
	})
	if err == nil {
		t.Fatal("expected error for unsupported sasl mechanism")
	}
}

// TestKafkaPublisherIntegration exercises a real broker. It is skipped unless
// GOGOMAIL_KAFKA_INTEGRATION is set and never runs under `go test -short`.
func TestKafkaPublisherIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping kafka integration test under -short")
	}
	brokers := strings.TrimSpace(os.Getenv("GOGOMAIL_KAFKA_INTEGRATION"))
	if brokers == "" {
		t.Skip("set GOGOMAIL_KAFKA_INTEGRATION=host:port[,host:port] to run the kafka integration test")
	}

	pub, err := NewKafkaPublisher(KafkaOptions{
		Brokers:      splitBrokers(brokers),
		ClientID:     "gogomail-kafka-integration-test",
		BatchTimeout: 50 * time.Millisecond,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: "all",
	})
	if err != nil {
		t.Fatalf("NewKafkaPublisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := pub.Publish(ctx, validEvent()); err != nil {
		t.Fatalf("Publish to real broker failed: %v", err)
	}
}

func splitBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
