package smtpd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FarmNode represents an SMTP server in a server farm.
type FarmNode struct {
	NodeID           string    // unique node identifier (e.g., hostname-instance-1)
	LastHeartbeat    time.Time // last time this node reported health
	ActiveDeliveries int       // current deliveries being processed
	UpSince          time.Time // when this node started
	Status           string    // "healthy", "degraded", "offline"
}

// DeliveryJob represents a message delivery task in the farm queue.
type DeliveryJob struct {
	JobID              string    `json:"job_id"`              // unique delivery job identifier
	RecipientDomain    string    `json:"recipient_domain"`   // destination domain
	StoragePath        string    `json:"storage_path"`       // path to stored message file
	EnvelopeFrom       string    `json:"envelope_from"`      // SMTP from address
	RecipientAddress   string    `json:"recipient_address"`  // SMTP to address
	Priority           int       `json:"priority"`           // delivery priority (1-10, 10=highest)
	RetryCount         int       `json:"retry_count"`        // number of delivery attempts
	MaxRetries         int       `json:"max_retries"`        // max allowed retries
	CreatedAt          time.Time `json:"created_at"`         // when job was created
	NextRetryAt        time.Time `json:"next_retry_at"`      // when next retry should occur
	LastErrorMessage   string    `json:"last_error_message"` // reason for last failure
	AssignedToNode     string    `json:"assigned_to_node"`   // node currently processing this job
}

// FarmCoordinator provides farm-level coordination for distributed SMTP delivery.
// Implementations might use Redis, etcd, or other distributed systems.
type FarmCoordinator interface {
	// Node management
	RegisterNode(ctx context.Context, node FarmNode) error
	UnregisterNode(ctx context.Context, nodeID string) error
	GetHealthyNodes(ctx context.Context) ([]FarmNode, error)
	UpdateNodeStatus(ctx context.Context, nodeID string, status string) error
	RecordHeartbeat(ctx context.Context, nodeID string) error

	// Job queue operations
	EnqueueDelivery(ctx context.Context, job DeliveryJob) error
	DequeueDelivery(ctx context.Context, nodeID string, domainFilter string) (*DeliveryJob, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error
	AcknowledgeJob(ctx context.Context, jobID string, nodeID string) error
	RequeueJob(ctx context.Context, jobID string, retryAfter time.Duration) error
	GetJobStatus(ctx context.Context, jobID string) (*DeliveryJob, error)

	// Queue stats
	GetQueueStats(ctx context.Context) (map[string]interface{}, error)
}

// NoOpFarmCoordinator is a stub implementation that does nothing (for single-node deployments).
type NoOpFarmCoordinator struct {
	queue []*DeliveryJob
	mu    sync.RWMutex
}

func NewNoOpFarmCoordinator() *NoOpFarmCoordinator {
	return &NoOpFarmCoordinator{
		queue: make([]*DeliveryJob, 0),
	}
}

func (n *NoOpFarmCoordinator) RegisterNode(ctx context.Context, node FarmNode) error {
	return nil
}

func (n *NoOpFarmCoordinator) UnregisterNode(ctx context.Context, nodeID string) error {
	return nil
}

func (n *NoOpFarmCoordinator) GetHealthyNodes(ctx context.Context) ([]FarmNode, error) {
	return []FarmNode{}, nil
}

func (n *NoOpFarmCoordinator) UpdateNodeStatus(ctx context.Context, nodeID string, status string) error {
	return nil
}

func (n *NoOpFarmCoordinator) RecordHeartbeat(ctx context.Context, nodeID string) error {
	return nil
}

func (n *NoOpFarmCoordinator) EnqueueDelivery(ctx context.Context, job DeliveryJob) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.queue = append(n.queue, &job)
	return nil
}

func (n *NoOpFarmCoordinator) DequeueDelivery(ctx context.Context, nodeID string, domainFilter string) (*DeliveryJob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.queue) == 0 {
		return nil, nil
	}
	job := n.queue[0]
	n.queue = n.queue[1:]
	job.AssignedToNode = nodeID
	return job, nil
}

func (n *NoOpFarmCoordinator) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	return nil
}

func (n *NoOpFarmCoordinator) AcknowledgeJob(ctx context.Context, jobID string, nodeID string) error {
	return nil
}

func (n *NoOpFarmCoordinator) RequeueJob(ctx context.Context, jobID string, retryAfter time.Duration) error {
	return nil
}

func (n *NoOpFarmCoordinator) GetJobStatus(ctx context.Context, jobID string) (*DeliveryJob, error) {
	return nil, nil
}

func (n *NoOpFarmCoordinator) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return map[string]interface{}{
		"queue_length": len(n.queue),
		"mode":         "noop",
	}, nil
}

// DeliveryJobCodec handles JSON encoding/decoding of delivery jobs.
func EncodeDeliveryJob(job *DeliveryJob) (string, error) {
	data, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("marshal delivery job: %w", err)
	}
	return string(data), nil
}

func DecodeDeliveryJob(data string) (*DeliveryJob, error) {
	var job DeliveryJob
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("unmarshal delivery job: %w", err)
	}
	return &job, nil
}
