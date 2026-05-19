package smtpd

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func redisClientForFarmTest(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Skip("redis not available:", err)
	}
	return client
}

func cleanupFarmKeys(t *testing.T, client *redis.Client, prefix string) {
	t.Helper()
	ctx := context.Background()
	keys, _ := client.Keys(ctx, prefix+":*").Result()
	if len(keys) > 0 {
		client.Del(ctx, keys...)
	}
}

func TestRedisFarmCoordinator_RegisterUnregisterNode(t *testing.T) {
	client := redisClientForFarmTest(t)
	defer client.Close()

	prefix := "gogomail:farmtest:regtest"
	cleanupFarmKeys(t, client, prefix)
	defer cleanupFarmKeys(t, client, prefix)

	coord := NewRedisFarmCoordinator(client, RedisFarmCoordinatorOptions{
		NodeHeartbeatTTL: 5 * time.Second,
		KeyPrefix:        prefix,
	})
	ctx := context.Background()

	node := FarmNode{
		NodeID:  "test-node-1",
		Status:  "healthy",
		UpSince: time.Now().UTC(),
	}
	if err := coord.RegisterNode(ctx, node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	nodes, err := coord.GetHealthyNodes(ctx)
	if err != nil {
		t.Fatalf("GetHealthyNodes failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 healthy node, got %d", len(nodes))
	}
	if nodes[0].NodeID != "test-node-1" {
		t.Errorf("expected node ID test-node-1, got %s", nodes[0].NodeID)
	}

	if err := coord.UnregisterNode(ctx, "test-node-1"); err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	nodes, err = coord.GetHealthyNodes(ctx)
	if err != nil {
		t.Fatalf("GetHealthyNodes after unregister failed: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 healthy nodes after unregister, got %d", len(nodes))
	}
}

func TestRedisFarmCoordinator_Heartbeat_Expiry(t *testing.T) {
	client := redisClientForFarmTest(t)
	defer client.Close()

	prefix := "gogomail:farmtest:heartbeat"
	cleanupFarmKeys(t, client, prefix)
	defer cleanupFarmKeys(t, client, prefix)

	coord := NewRedisFarmCoordinator(client, RedisFarmCoordinatorOptions{
		NodeHeartbeatTTL: 1 * time.Second,
		KeyPrefix:        prefix,
	})
	ctx := context.Background()

	node := FarmNode{
		NodeID:  "hb-node-1",
		Status:  "healthy",
		UpSince: time.Now().UTC(),
	}
	if err := coord.RegisterNode(ctx, node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	// Record heartbeat should refresh TTL
	if err := coord.RecordHeartbeat(ctx, "hb-node-1"); err != nil {
		t.Fatalf("RecordHeartbeat failed: %v", err)
	}

	nodes, err := coord.GetHealthyNodes(ctx)
	if err != nil {
		t.Fatalf("GetHealthyNodes failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node after heartbeat, got %d", len(nodes))
	}

	// Wait for TTL to expire
	time.Sleep(1500 * time.Millisecond)

	nodes, err = coord.GetHealthyNodes(ctx)
	if err != nil {
		t.Fatalf("GetHealthyNodes after expiry failed: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after TTL expiry, got %d", len(nodes))
	}
}

func TestRedisFarmCoordinator_EnqueueDequeue_Priority(t *testing.T) {
	client := redisClientForFarmTest(t)
	defer client.Close()

	prefix := "gogomail:farmtest:priority"
	cleanupFarmKeys(t, client, prefix)
	defer cleanupFarmKeys(t, client, prefix)

	coord := NewRedisFarmCoordinator(client, RedisFarmCoordinatorOptions{
		NodeHeartbeatTTL: 30 * time.Second,
		KeyPrefix:        prefix,
	})
	ctx := context.Background()

	// Enqueue low priority first, then high priority
	lowPriority := DeliveryJob{
		JobID:           "low-priority-job",
		RecipientDomain: "example.com",
		Priority:        1, // lower priority
		CreatedAt:       time.Now().UTC(),
	}
	highPriority := DeliveryJob{
		JobID:           "high-priority-job",
		RecipientDomain: "example.com",
		Priority:        10, // highest priority
		CreatedAt:       time.Now().UTC(),
	}

	if err := coord.EnqueueDelivery(ctx, lowPriority); err != nil {
		t.Fatalf("EnqueueDelivery low-priority failed: %v", err)
	}
	if err := coord.EnqueueDelivery(ctx, highPriority); err != nil {
		t.Fatalf("EnqueueDelivery high-priority failed: %v", err)
	}

	// Dequeue: high priority should come first (lower score)
	job1, err := coord.DequeueDelivery(ctx, "node-1", "")
	if err != nil {
		t.Fatalf("DequeueDelivery 1 failed: %v", err)
	}
	if job1 == nil {
		t.Fatal("expected a job, got nil")
	}
	if job1.JobID != "high-priority-job" {
		t.Errorf("expected high-priority-job first, got %s", job1.JobID)
	}
	if job1.AssignedToNode != "node-1" {
		t.Errorf("expected assigned to node-1, got %s", job1.AssignedToNode)
	}

	job2, err := coord.DequeueDelivery(ctx, "node-1", "")
	if err != nil {
		t.Fatalf("DequeueDelivery 2 failed: %v", err)
	}
	if job2 == nil {
		t.Fatal("expected second job, got nil")
	}
	if job2.JobID != "low-priority-job" {
		t.Errorf("expected low-priority-job second, got %s", job2.JobID)
	}
}

func TestRedisFarmCoordinator_RequeueJob(t *testing.T) {
	client := redisClientForFarmTest(t)
	defer client.Close()

	prefix := "gogomail:farmtest:requeue"
	cleanupFarmKeys(t, client, prefix)
	defer cleanupFarmKeys(t, client, prefix)

	coord := NewRedisFarmCoordinator(client, RedisFarmCoordinatorOptions{
		NodeHeartbeatTTL: 30 * time.Second,
		KeyPrefix:        prefix,
	})
	ctx := context.Background()

	job := DeliveryJob{
		JobID:           "requeue-test-job",
		RecipientDomain: "example.com",
		Priority:        5,
		RetryCount:      0,
		MaxRetries:      3,
		CreatedAt:       time.Now().UTC(),
	}

	if err := coord.EnqueueDelivery(ctx, job); err != nil {
		t.Fatalf("EnqueueDelivery failed: %v", err)
	}

	// Dequeue it
	dequeued, err := coord.DequeueDelivery(ctx, "node-1", "")
	if err != nil {
		t.Fatalf("DequeueDelivery failed: %v", err)
	}
	if dequeued == nil {
		t.Fatal("expected job, got nil")
	}

	// Requeue with a short delay
	if err := coord.RequeueJob(ctx, "requeue-test-job", 100*time.Millisecond); err != nil {
		t.Fatalf("RequeueJob failed: %v", err)
	}

	// The job should be back in the queue
	stats, err := coord.GetQueueStats(ctx)
	if err != nil {
		t.Fatalf("GetQueueStats failed: %v", err)
	}
	if stats["queue_length"].(int64) < 1 {
		t.Errorf("expected at least 1 job in queue after requeue, got %v", stats["queue_length"])
	}

	// Verify retry count incremented
	requeued, err := coord.GetJobStatus(ctx, "requeue-test-job")
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}
	if requeued == nil {
		t.Fatal("expected job status, got nil")
	}
	if requeued.RetryCount != 1 {
		t.Errorf("expected RetryCount 1 after requeue, got %d", requeued.RetryCount)
	}
}
