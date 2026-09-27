package outbox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// kafkaWriter is the minimal surface of *kafka.Writer used by KafkaPublisher.
// It is an interface so unit tests can substitute a fake without requiring a
// real broker.
type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// KafkaPublisher implements the outbox Publisher interface by producing each
// relayed outbox event to a Kafka topic.
//
// Graceful degradation contract: KafkaPublisher NEVER touches the mail-accept
// path. It is only invoked by the outbox-relay worker, which is fully
// decoupled from SMTP/IMAP/JMAP ingestion via the Postgres outbox table. When
// the broker is unreachable, WriteMessages returns an error, the relay marks
// the event failed (incrementing its attempt counter), and the event is
// retried on the next poll. Mail is already durably stored before any event is
// published, so a Kafka outage can never lose or block mail — it only delays
// downstream event fan-out until the broker recovers.
type KafkaPublisher struct {
	writer      kafkaWriter
	topicPrefix string
}

// KafkaOptions configures a KafkaPublisher backed by a real *kafka.Writer.
type KafkaOptions struct {
	Brokers       []string
	TopicPrefix   string
	ClientID      string
	BatchSize     int
	BatchTimeout  time.Duration
	WriteTimeout  time.Duration
	MaxAttempts   int
	RequiredAcks  string // none|leader|all
	TLSEnabled    bool
	TLSSkipVerify bool
	SASLMechanism string // none|plain|scram-sha-256|scram-sha-512
	SASLUsername  string
	SASLPassword  string
}

// NewKafkaPublisher builds a KafkaPublisher backed by a real kafka.Writer.
func NewKafkaPublisher(opts KafkaOptions) (*KafkaPublisher, error) {
	brokers := make([]string, 0, len(opts.Brokers))
	for _, broker := range opts.Brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are required")
	}

	acks, err := parseRequiredAcks(opts.RequiredAcks)
	if err != nil {
		return nil, err
	}

	transport, err := buildKafkaTransport(opts)
	if err != nil {
		return nil, err
	}

	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	batchTimeout := opts.BatchTimeout
	if batchTimeout <= 0 {
		batchTimeout = 100 * time.Millisecond
	}
	writeTimeout := opts.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 10 * time.Second
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.Hash{}, // partition by message key for per-key ordering
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		WriteTimeout: writeTimeout,
		MaxAttempts:  maxAttempts,
		RequiredAcks: acks,
		Async:        false, // synchronous so publish errors propagate to the relay
		Transport:    transport,
	}

	return &KafkaPublisher{
		writer:      writer,
		topicPrefix: strings.TrimSpace(opts.TopicPrefix),
	}, nil
}

// newKafkaPublisherWithWriter is used by tests to inject a fake writer.
func newKafkaPublisherWithWriter(writer kafkaWriter, topicPrefix string) *KafkaPublisher {
	return &KafkaPublisher{writer: writer, topicPrefix: strings.TrimSpace(topicPrefix)}
}

func parseRequiredAcks(value string) (kafka.RequiredAcks, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all", "-1":
		return kafka.RequireAll, nil
	case "leader", "one", "1":
		return kafka.RequireOne, nil
	case "none", "0":
		return kafka.RequireNone, nil
	default:
		return 0, fmt.Errorf("unsupported kafka required acks %q", value)
	}
}

func buildKafkaTransport(opts KafkaOptions) (*kafka.Transport, error) {
	transport := &kafka.Transport{
		ClientID:    strings.TrimSpace(opts.ClientID),
		DialTimeout: 5 * time.Second,
	}
	if opts.TLSEnabled {
		transport.TLS = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opts.TLSSkipVerify, //nolint:gosec // operator opt-in, rejected in production by config validation
		}
	}
	mechanism, err := buildKafkaSASL(opts)
	if err != nil {
		return nil, err
	}
	if mechanism != nil {
		transport.SASL = mechanism
	}
	return transport, nil
}

func buildKafkaSASL(opts KafkaOptions) (sasl.Mechanism, error) {
	mechanism := strings.ToLower(strings.TrimSpace(opts.SASLMechanism))
	switch mechanism {
	case "", "none":
		return nil, nil
	case "plain":
		return plain.Mechanism{Username: opts.SASLUsername, Password: opts.SASLPassword}, nil
	case "scram-sha-256":
		return scram.Mechanism(scram.SHA256, opts.SASLUsername, opts.SASLPassword)
	case "scram-sha-512":
		return scram.Mechanism(scram.SHA512, opts.SASLUsername, opts.SASLPassword)
	default:
		return nil, fmt.Errorf("unsupported kafka sasl mechanism %q", opts.SASLMechanism)
	}
}

// Publish produces a single outbox event to Kafka. The event topic maps to the
// Kafka topic (with optional prefix); the partition key preserves per-entity
// ordering via the Hash balancer.
func (p *KafkaPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil || p.writer == nil {
		return fmt.Errorf("kafka writer is required")
	}
	event, err := normalizeKafkaEvent(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: p.kafkaTopic(event.Topic),
		Key:   []byte(event.PartitionKey),
		Value: []byte(event.Payload),
		Headers: []kafka.Header{
			{Key: "outbox_id", Value: []byte(event.ID)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish outbox event to kafka topic %q: %w", msg.Topic, err)
	}
	return nil
}

// Close releases the underlying Kafka writer.
func (p *KafkaPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func (p *KafkaPublisher) kafkaTopic(eventTopic string) string {
	if p.topicPrefix == "" {
		return eventTopic
	}
	return p.topicPrefix + eventTopic
}

func normalizeKafkaEvent(event Event) (Event, error) {
	event.ID = strings.TrimSpace(event.ID)
	event.Topic = strings.TrimSpace(event.Topic)
	event.PartitionKey = strings.TrimSpace(event.PartitionKey)
	event.Payload = []byte(strings.TrimSpace(string(event.Payload)))
	if event.Topic == "" {
		return Event{}, fmt.Errorf("outbox event topic is required")
	}
	if strings.ContainsAny(event.Topic, "\r\n") {
		return Event{}, fmt.Errorf("outbox event topic is invalid")
	}
	if event.ID == "" {
		return Event{}, fmt.Errorf("outbox event id is required")
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return Event{}, fmt.Errorf("outbox event payload must be valid json")
	}
	return event, nil
}
