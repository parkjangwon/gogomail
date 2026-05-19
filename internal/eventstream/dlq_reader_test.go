package eventstream

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDecodeDLQEntry(t *testing.T) {
	t.Parallel()

	entry, err := decodeDLQEntry(redis.XMessage{
		ID: "1700000000000-0",
		Values: map[string]any{
			"original_stream":  "mail.event",
			"group":            "processor",
			"consumer":         "worker-1",
			"original_id":      "1699000000000-0",
			"outbox_id":        "outbox-abc",
			"partition_key":    "tenant-1",
			"payload":          `{"event":"mail.stored"}`,
			"error":            "handler timed out",
			"deliveries":       "11",
			"dead_lettered_at": "2024-01-15T10:30:00.123456789Z",
		},
	})
	if err != nil {
		t.Fatalf("decodeDLQEntry returned error: %v", err)
	}
	if entry.ID != "1700000000000-0" {
		t.Errorf("ID = %q, want 1700000000000-0", entry.ID)
	}
	if entry.OriginalStream != "mail.event" {
		t.Errorf("OriginalStream = %q, want mail.event", entry.OriginalStream)
	}
	if entry.Group != "processor" {
		t.Errorf("Group = %q, want processor", entry.Group)
	}
	if entry.Consumer != "worker-1" {
		t.Errorf("Consumer = %q, want worker-1", entry.Consumer)
	}
	if entry.OriginalID != "1699000000000-0" {
		t.Errorf("OriginalID = %q, want 1699000000000-0", entry.OriginalID)
	}
	if entry.OutboxID != "outbox-abc" {
		t.Errorf("OutboxID = %q, want outbox-abc", entry.OutboxID)
	}
	if entry.PartitionKey != "tenant-1" {
		t.Errorf("PartitionKey = %q, want tenant-1", entry.PartitionKey)
	}
	if !json.Valid(entry.Payload) || string(entry.Payload) != `{"event":"mail.stored"}` {
		t.Errorf("Payload = %q", string(entry.Payload))
	}
	if entry.Error != "handler timed out" {
		t.Errorf("Error = %q, want 'handler timed out'", entry.Error)
	}
	if entry.Deliveries != 11 {
		t.Errorf("Deliveries = %d, want 11", entry.Deliveries)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	if !entry.DeadLetteredAt.Equal(expected) {
		t.Errorf("DeadLetteredAt = %v, want %v", entry.DeadLetteredAt, expected)
	}
}

func TestDecodeDLQEntryWithByteValues(t *testing.T) {
	t.Parallel()

	entry, err := decodeDLQEntry(redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"original_stream":  []byte("mail.event"),
			"group":            []byte("processor"),
			"consumer":         []byte("worker-1"),
			"original_id":      []byte("0-1"),
			"outbox_id":        []byte("outbox-1"),
			"partition_key":    []byte("tenant-1"),
			"payload":          []byte(`{"k":"v"}`),
			"error":            []byte("some error"),
			"deliveries":       []byte("5"),
			"dead_lettered_at": []byte("2024-06-01T00:00:00Z"),
		},
	})
	if err != nil {
		t.Fatalf("decodeDLQEntry returned error: %v", err)
	}
	if entry.OriginalStream != "mail.event" {
		t.Errorf("OriginalStream = %q, want mail.event", entry.OriginalStream)
	}
	if entry.Deliveries != 5 {
		t.Errorf("Deliveries = %d, want 5", entry.Deliveries)
	}
}

func TestDecodeDLQEntryInvalidPayloadKept(t *testing.T) {
	t.Parallel()

	// Invalid JSON payload should result in nil Payload (silently skipped).
	entry, err := decodeDLQEntry(redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"original_stream":  "s",
			"group":            "g",
			"consumer":         "c",
			"original_id":      "0-1",
			"outbox_id":        "o",
			"partition_key":    "p",
			"payload":          `{bad json`,
			"error":            "",
			"deliveries":       "1",
			"dead_lettered_at": "",
		},
	})
	if err != nil {
		t.Fatalf("decodeDLQEntry returned error: %v", err)
	}
	if entry.Payload != nil {
		t.Errorf("expected nil Payload for invalid JSON, got %q", string(entry.Payload))
	}
}

func TestNewRedisDLQReaderRequiresClient(t *testing.T) {
	t.Parallel()

	_, err := NewRedisDLQReader(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}
