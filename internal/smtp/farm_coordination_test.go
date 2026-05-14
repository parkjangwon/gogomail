package smtpd

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNoOpFarmCoordinatorEnqueueDequeue(t *testing.T) {
	coord := NewNoOpFarmCoordinator()
	ctx := context.Background()

	job := DeliveryJob{
		JobID:           "test-1",
		RecipientDomain: "example.com",
		StoragePath:     "/path/to/message.eml",
		EnvelopeFrom:    "sender@domain.com",
		CreatedAt:       time.Now(),
	}

	// Enqueue job
	if err := coord.EnqueueDelivery(ctx, job); err != nil {
		t.Fatalf("EnqueueDelivery failed: %v", err)
	}

	// Dequeue job
	dequeued, err := coord.DequeueDelivery(ctx, "node1", "")
	if err != nil {
		t.Fatalf("DequeueDelivery failed: %v", err)
	}

	if dequeued == nil {
		t.Fatal("expected job, got nil")
	}

	if dequeued.JobID != "test-1" {
		t.Errorf("expected job ID test-1, got %s", dequeued.JobID)
	}

	if dequeued.AssignedToNode != "node1" {
		t.Errorf("expected assigned to node1, got %s", dequeued.AssignedToNode)
	}
}

func TestNoOpFarmCoordinatorEmptyQueue(t *testing.T) {
	coord := NewNoOpFarmCoordinator()
	ctx := context.Background()

	job, err := coord.DequeueDelivery(ctx, "node1", "")
	if err != nil {
		t.Fatalf("DequeueDelivery failed: %v", err)
	}

	if job != nil {
		t.Error("expected nil job from empty queue")
	}
}

func TestNoOpFarmCoordinatorQueueStats(t *testing.T) {
	coord := NewNoOpFarmCoordinator()
	ctx := context.Background()

	// Enqueue multiple jobs
	for i := 0; i < 5; i++ {
		job := DeliveryJob{
			JobID:           "test-" + string(rune(i)),
			RecipientDomain: "example.com",
			CreatedAt:       time.Now(),
		}
		coord.EnqueueDelivery(ctx, job)
	}

	stats, err := coord.GetQueueStats(ctx)
	if err != nil {
		t.Fatalf("GetQueueStats failed: %v", err)
	}

	if stats["queue_length"] != 5 {
		t.Errorf("expected queue_length 5, got %v", stats["queue_length"])
	}

	if stats["mode"] != "noop" {
		t.Errorf("expected mode noop, got %v", stats["mode"])
	}
}

func TestDeliveryJobCodec(t *testing.T) {
	original := &DeliveryJob{
		JobID:              "test-job-1",
		RecipientDomain:    "example.com",
		StoragePath:        "/mail/store/path.eml",
		EnvelopeFrom:       "sender@example.net",
		RecipientAddress:   "user@example.com",
		Priority:           5,
		RetryCount:         2,
		MaxRetries:         5,
		CreatedAt:          time.Now(),
		NextRetryAt:        time.Now().Add(1 * time.Hour),
		LastErrorMessage:   "connection timeout",
		AssignedToNode:     "smtp-node-1",
	}

	// Encode
	encoded, err := EncodeDeliveryJob(original)
	if err != nil {
		t.Fatalf("EncodeDeliveryJob failed: %v", err)
	}

	// Decode
	decoded, err := DecodeDeliveryJob(encoded)
	if err != nil {
		t.Fatalf("DecodeDeliveryJob failed: %v", err)
	}

	// Compare key fields
	if decoded.JobID != original.JobID {
		t.Errorf("JobID mismatch: %s vs %s", decoded.JobID, original.JobID)
	}

	if decoded.RecipientDomain != original.RecipientDomain {
		t.Errorf("RecipientDomain mismatch: %s vs %s", decoded.RecipientDomain, original.RecipientDomain)
	}

	if decoded.Priority != original.Priority {
		t.Errorf("Priority mismatch: %d vs %d", decoded.Priority, original.Priority)
	}

	if decoded.LastErrorMessage != original.LastErrorMessage {
		t.Errorf("LastErrorMessage mismatch: %s vs %s", decoded.LastErrorMessage, original.LastErrorMessage)
	}
}

func TestDeliveryJobCodecInvalidJSON(t *testing.T) {
	_, err := DecodeDeliveryJob("invalid json {")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFarmNodeTypes(t *testing.T) {
	node := FarmNode{
		NodeID:           "smtp-1",
		LastHeartbeat:    time.Now(),
		ActiveDeliveries: 42,
		UpSince:          time.Now().Add(-1 * time.Hour),
		Status:           "healthy",
	}

	// Verify it can be marshaled
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled FarmNode
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if unmarshaled.NodeID != "smtp-1" {
		t.Errorf("NodeID mismatch after marshal/unmarshal")
	}
}

func TestNoOpFarmCoordinatorConcurrentEnqueue(t *testing.T) {
	coord := NewNoOpFarmCoordinator()
	ctx := context.Background()

	// Enqueue jobs concurrently
	for i := 0; i < 100; i++ {
		job := DeliveryJob{
			JobID:           "job-" + string(rune(i)),
			RecipientDomain: "example.com",
			CreatedAt:       time.Now(),
		}
		if err := coord.EnqueueDelivery(ctx, job); err != nil {
			t.Fatalf("EnqueueDelivery failed: %v", err)
		}
	}

	stats, _ := coord.GetQueueStats(ctx)
	if stats["queue_length"] != 100 {
		t.Errorf("expected 100 jobs, got %v", stats["queue_length"])
	}
}
