package smtpd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisFarmCoordinatorOptions configures the Redis-backed farm coordinator.
type RedisFarmCoordinatorOptions struct {
	// NodeHeartbeatTTL is how long a node registration remains valid without a heartbeat.
	NodeHeartbeatTTL time.Duration
	// JobVisibilityTimeout is how long a dequeued job remains assigned before being requeued.
	JobVisibilityTimeout time.Duration
	// KeyPrefix is the Redis key namespace prefix.
	KeyPrefix string
}

func (o *RedisFarmCoordinatorOptions) setDefaults() {
	if o.NodeHeartbeatTTL <= 0 {
		o.NodeHeartbeatTTL = 30 * time.Second
	}
	if o.JobVisibilityTimeout <= 0 {
		o.JobVisibilityTimeout = 5 * time.Minute
	}
	if strings.TrimSpace(o.KeyPrefix) == "" {
		o.KeyPrefix = "gogomail:farm"
	}
}

// RedisFarmCoordinator is a Redis-backed implementation of FarmCoordinator.
type RedisFarmCoordinator struct {
	client *redis.Client
	opts   RedisFarmCoordinatorOptions
}

// NewRedisFarmCoordinator creates a new Redis-backed farm coordinator.
func NewRedisFarmCoordinator(client *redis.Client, opts RedisFarmCoordinatorOptions) *RedisFarmCoordinator {
	opts.setDefaults()
	return &RedisFarmCoordinator{client: client, opts: opts}
}

func (r *RedisFarmCoordinator) nodeKey(nodeID string) string {
	return r.opts.KeyPrefix + ":node:" + nodeID
}

func (r *RedisFarmCoordinator) pendingQueueKey() string {
	return r.opts.KeyPrefix + ":jobs:pending"
}

func (r *RedisFarmCoordinator) jobKey(jobID string) string {
	return r.opts.KeyPrefix + ":job:" + jobID
}

func (r *RedisFarmCoordinator) processingKey(nodeID string) string {
	return r.opts.KeyPrefix + ":jobs:processing:" + nodeID
}

// RegisterNode registers a node and sets its heartbeat TTL.
func (r *RedisFarmCoordinator) RegisterNode(ctx context.Context, node FarmNode) error {
	key := r.nodeKey(node.NodeID)
	upSince := node.UpSince
	if upSince.IsZero() {
		upSince = time.Now().UTC()
	}
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, key,
		"status", node.Status,
		"active_deliveries", strconv.Itoa(node.ActiveDeliveries),
		"up_since", upSince.Format(time.RFC3339),
		"last_heartbeat", time.Now().UTC().Format(time.RFC3339),
		"node_id", node.NodeID,
	)
	pipe.Expire(ctx, key, r.opts.NodeHeartbeatTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("register farm node %s: %w", node.NodeID, err)
	}
	return nil
}

// UnregisterNode removes a node from the farm.
func (r *RedisFarmCoordinator) UnregisterNode(ctx context.Context, nodeID string) error {
	if err := r.client.Del(ctx, r.nodeKey(nodeID)).Err(); err != nil {
		return fmt.Errorf("unregister farm node %s: %w", nodeID, err)
	}
	return nil
}

// GetHealthyNodes returns all nodes that still have a valid TTL (i.e., have sent a heartbeat recently).
func (r *RedisFarmCoordinator) GetHealthyNodes(ctx context.Context) ([]FarmNode, error) {
	pattern := r.opts.KeyPrefix + ":node:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("get healthy farm nodes: %w", err)
	}

	nodes := make([]FarmNode, 0, len(keys))
	for _, key := range keys {
		// Check TTL — only include nodes with remaining TTL
		ttl, err := r.client.TTL(ctx, key).Result()
		if err != nil || ttl <= 0 {
			continue
		}
		fields, err := r.client.HGetAll(ctx, key).Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		node := farmNodeFromHash(fields)
		if node.Status == "" {
			node.Status = "healthy"
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func farmNodeFromHash(fields map[string]string) FarmNode {
	node := FarmNode{}
	node.NodeID = fields["node_id"]
	node.Status = fields["status"]
	if v, ok := fields["active_deliveries"]; ok {
		n, _ := strconv.Atoi(v)
		node.ActiveDeliveries = n
	}
	if v, ok := fields["up_since"]; ok {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			node.UpSince = t
		}
	}
	if v, ok := fields["last_heartbeat"]; ok {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			node.LastHeartbeat = t
		}
	}
	return node
}

// UpdateNodeStatus updates the status field of a node and refreshes its TTL.
func (r *RedisFarmCoordinator) UpdateNodeStatus(ctx context.Context, nodeID string, status string) error {
	key := r.nodeKey(nodeID)
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, key, "status", status)
	pipe.Expire(ctx, key, r.opts.NodeHeartbeatTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update farm node %s status: %w", nodeID, err)
	}
	return nil
}

// RecordHeartbeat refreshes a node's TTL to keep it alive.
func (r *RedisFarmCoordinator) RecordHeartbeat(ctx context.Context, nodeID string) error {
	key := r.nodeKey(nodeID)
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, key, "last_heartbeat", time.Now().UTC().Format(time.RFC3339))
	pipe.Expire(ctx, key, r.opts.NodeHeartbeatTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("record heartbeat for farm node %s: %w", nodeID, err)
	}
	return nil
}

// EnqueueDelivery adds a job to the pending queue using a priority score.
// Score = priority * 1e9 + UnixNano (lower score = processed first, but higher priority = processed first,
// so we negate priority contribution: score = (10-priority)*1e9 + UnixNano).
func (r *RedisFarmCoordinator) EnqueueDelivery(ctx context.Context, job DeliveryJob) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode delivery job %s: %w", job.JobID, err)
	}

	// Store job hash
	if err := r.client.Set(ctx, r.jobKey(job.JobID), string(jobJSON), 0).Err(); err != nil {
		return fmt.Errorf("store delivery job %s: %w", job.JobID, err)
	}

	// Score: lower priority number → higher score → processed later.
	// We want priority 10 = earliest, so score = (10-priority)*1e9 + UnixNano
	score := float64((10-job.Priority)*1_000_000_000) + float64(time.Now().UnixNano()%1_000_000_000)
	if err := r.client.ZAdd(ctx, r.pendingQueueKey(), redis.Z{
		Score:  score,
		Member: job.JobID,
	}).Err(); err != nil {
		return fmt.Errorf("enqueue delivery job %s: %w", job.JobID, err)
	}
	return nil
}

// dequeueScript atomically pops a job from the ZSET and marks it as assigned.
// KEYS[1] = pending queue, KEYS[2] = job hash key prefix (not used, we do separate calls), KEYS[3] = processing set
// We use a simpler approach: ZPOPMIN + HSET in a pipeline (not fully atomic, but correct enough).
var dequeueScript = redis.NewScript(`
local jobs = redis.call('ZPOPMIN', KEYS[1], 1)
if #jobs == 0 then
    return nil
end
local jobID = jobs[1]
redis.call('SADD', KEYS[2], jobID)
return jobID
`)

// DequeueDelivery dequeues a job for the given node, optionally filtered by domain.
func (r *RedisFarmCoordinator) DequeueDelivery(ctx context.Context, nodeID string, domainFilter string) (*DeliveryJob, error) {
	pendingKey := r.pendingQueueKey()
	processingKey := r.processingKey(nodeID)

	if domainFilter == "" {
		// Use Lua script for atomic pop + assign
		result, err := dequeueScript.Run(ctx, r.client, []string{pendingKey, processingKey}).Result()
		if err == redis.Nil || result == nil {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("dequeue delivery job: %w", err)
		}
		jobID, ok := result.(string)
		if !ok || jobID == "" {
			return nil, nil
		}
		return r.loadAndAssignJob(ctx, jobID, nodeID)
	}

	// Domain filter: scan the queue and find first matching job
	// This is O(n) but acceptable for filtered dequeue
	members, err := r.client.ZRange(ctx, pendingKey, 0, 99).Result()
	if err != nil {
		return nil, fmt.Errorf("scan delivery queue: %w", err)
	}
	for _, jobID := range members {
		jobData, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
		if err == redis.Nil {
			// Job data gone, remove from queue
			r.client.ZRem(ctx, pendingKey, jobID)
			continue
		}
		if err != nil {
			continue
		}
		var job DeliveryJob
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			continue
		}
		if strings.EqualFold(job.RecipientDomain, domainFilter) {
			// Remove from queue atomically
			removed, err := r.client.ZRem(ctx, pendingKey, jobID).Result()
			if err != nil || removed == 0 {
				continue
			}
			if err := r.client.SAdd(ctx, processingKey, jobID).Err(); err != nil {
				// Best effort
			}
			job.AssignedToNode = nodeID
			jobJSON, _ := json.Marshal(job)
			r.client.Set(ctx, r.jobKey(jobID), string(jobJSON), 0)
			return &job, nil
		}
	}
	return nil, nil
}

func (r *RedisFarmCoordinator) loadAndAssignJob(ctx context.Context, jobID string, nodeID string) (*DeliveryJob, error) {
	jobData, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load delivery job %s: %w", jobID, err)
	}
	var job DeliveryJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("decode delivery job %s: %w", jobID, err)
	}
	job.AssignedToNode = nodeID
	// Persist updated assignment
	jobJSON, _ := json.Marshal(job)
	r.client.Set(ctx, r.jobKey(jobID), string(jobJSON), 0)
	return &job, nil
}

// UpdateJobStatus updates the status of a job.
func (r *RedisFarmCoordinator) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	jobData, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load job %s for status update: %w", jobID, err)
	}
	var job DeliveryJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return fmt.Errorf("decode job %s for status update: %w", jobID, err)
	}
	if errorMsg != "" {
		job.LastErrorMessage = errorMsg
	}
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode job %s after status update: %w", jobID, err)
	}
	if err := r.client.Set(ctx, r.jobKey(jobID), string(jobJSON), 0).Err(); err != nil {
		return fmt.Errorf("persist job %s status update: %w", jobID, err)
	}
	return nil
}

// AcknowledgeJob removes a job from the processing set and deletes the job data.
func (r *RedisFarmCoordinator) AcknowledgeJob(ctx context.Context, jobID string, nodeID string) error {
	pipe := r.client.TxPipeline()
	pipe.SRem(ctx, r.processingKey(nodeID), jobID)
	pipe.Del(ctx, r.jobKey(jobID))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("acknowledge job %s from node %s: %w", jobID, nodeID, err)
	}
	return nil
}

// RequeueJob puts a job back into the pending queue after a delay.
func (r *RedisFarmCoordinator) RequeueJob(ctx context.Context, jobID string, retryAfter time.Duration) error {
	jobData, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load job %s for requeue: %w", jobID, err)
	}
	var job DeliveryJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return fmt.Errorf("decode job %s for requeue: %w", jobID, err)
	}
	job.RetryCount++
	job.NextRetryAt = time.Now().UTC().Add(retryAfter)
	job.AssignedToNode = ""
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode job %s for requeue: %w", jobID, err)
	}

	// Score based on retry time (future timestamp)
	score := float64(job.NextRetryAt.UnixNano())
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, r.jobKey(jobID), string(jobJSON), 0)
	pipe.ZAdd(ctx, r.pendingQueueKey(), redis.Z{Score: score, Member: jobID})
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("requeue job %s: %w", jobID, err)
	}
	return nil
}

// GetJobStatus returns the current state of a job.
func (r *RedisFarmCoordinator) GetJobStatus(ctx context.Context, jobID string) (*DeliveryJob, error) {
	jobData, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job %s status: %w", jobID, err)
	}
	var job DeliveryJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("decode job %s status: %w", jobID, err)
	}
	return &job, nil
}

// GetQueueStats returns statistics about the farm queue.
func (r *RedisFarmCoordinator) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	pendingCount, err := r.client.ZCard(ctx, r.pendingQueueKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("get queue stats: %w", err)
	}
	nodeCount := 0
	nodes, err := r.GetHealthyNodes(ctx)
	if err == nil {
		nodeCount = len(nodes)
	}
	return map[string]interface{}{
		"queue_length": pendingCount,
		"mode":         "redis",
		"healthy_nodes": nodeCount,
	}, nil
}
