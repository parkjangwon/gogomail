package admin

import (
	"context"
	"fmt"
	"time"
)

// SyncJob represents an LDAP sync job
type SyncJob struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Incremental bool     `json:"incremental"`
	Created    int64     `json:"created"`
	Updated    int64     `json:"updated"`
	Deleted    int64     `json:"deleted"`
	Failed     int64     `json:"failed"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time,omitempty"`
	Error      string    `json:"error,omitempty"`
	SyncToken  string    `json:"sync_token,omitempty"`
}

// SyncLog represents a sync operation log entry
type SyncLog struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	CreatedAt time.Time `json:"created_at"`
}

// LDAPServiceRepository interface for LDAP sync operations
type LDAPServiceRepository interface {
	CreateSyncJob(ctx context.Context, job *SyncJob) error
	GetSyncJob(ctx context.Context, jobID string) (*SyncJob, error)
	UpdateSyncJob(ctx context.Context, job *SyncJob) error
	ListSyncJobs(ctx context.Context, limit, offset int) ([]*SyncJob, int64, error)
	CreateSyncLog(ctx context.Context, log *SyncLog) error
	ListSyncLogs(ctx context.Context, jobID string, limit, offset int) ([]*SyncLog, int64, error)
}

// LDAPService manages LDAP sync operations
type LDAPService struct {
	repo     LDAPServiceRepository
	provider IdentityProvider
}

// NewLDAPService creates a new LDAP service
func NewLDAPService(repo LDAPServiceRepository, provider IdentityProvider, _ *AuditService) *LDAPService {
	return &LDAPService{
		repo:     repo,
		provider: provider,
	}
}

// TriggerSync triggers a new LDAP sync job
func (ls *LDAPService) TriggerSync(ctx context.Context, incremental bool) (string, error) {
	jobID := fmt.Sprintf("sync-%d", time.Now().UnixNano())

	job := &SyncJob{
		ID:          jobID,
		Status:      "pending",
		Incremental: incremental,
		StartTime:   time.Now(),
	}

	if err := ls.repo.CreateSyncJob(ctx, job); err != nil {
		return "", err
	}

	// Execute sync asynchronously would happen here
	// For now, execute synchronously and update job
	go ls.executeSync(context.Background(), jobID, incremental)

	return jobID, nil
}

// executeSync performs the actual sync operation
func (ls *LDAPService) executeSync(ctx context.Context, jobID string, incremental bool) {
	job, err := ls.repo.GetSyncJob(ctx, jobID)
	if err != nil {
		return
	}

	job.Status = "running"
	ls.repo.UpdateSyncJob(ctx, job)

	// Call the LDAP provider to sync users
	syncResult, err := ls.provider.SyncUsers(ctx, incremental)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.EndTime = time.Now()
		ls.repo.UpdateSyncJob(ctx, job)

		ls.logSync(ctx, jobID, "error", fmt.Sprintf("Sync failed: %v", err))
		return
	}

	// Update job with results
	job.Status = "completed"
	job.Created = int64(syncResult.Created)
	job.Updated = int64(syncResult.Updated)
	job.Deleted = int64(syncResult.Deleted)
	job.Failed = int64(syncResult.Failed)
	job.EndTime = time.Now()
	job.SyncToken = syncResult.LastToken

	ls.repo.UpdateSyncJob(ctx, job)

	ls.logSync(ctx, jobID, "info", fmt.Sprintf(
		"Sync completed: created=%d, updated=%d, deleted=%d, failed=%d",
		syncResult.Created, syncResult.Updated, syncResult.Deleted, syncResult.Failed,
	))
}

// GetSyncStatus returns the status of a sync job
func (ls *LDAPService) GetSyncStatus(ctx context.Context, jobID string) (*SyncJob, error) {
	if jobID == "" {
		return nil, fmt.Errorf("%w: jobID", ErrMissingRequiredField)
	}

	return ls.repo.GetSyncJob(ctx, jobID)
}

// ListSyncHistory lists all sync jobs
func (ls *LDAPService) ListSyncHistory(ctx context.Context, limit, offset int) ([]*SyncJob, int64, error) {
	if limit == 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	return ls.repo.ListSyncJobs(ctx, limit, offset)
}

// GetSyncLogs gets logs for a specific sync job
func (ls *LDAPService) GetSyncLogs(ctx context.Context, jobID string, limit, offset int) ([]*SyncLog, int64, error) {
	if jobID == "" {
		return nil, 0, fmt.Errorf("%w: jobID", ErrMissingRequiredField)
	}

	if limit == 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	return ls.repo.ListSyncLogs(ctx, jobID, limit, offset)
}

// logSync creates a sync log entry
func (ls *LDAPService) logSync(ctx context.Context, jobID, level, message string) {
	if ls.repo == nil {
		return
	}

	log := &SyncLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		JobID:     jobID,
		Message:   message,
		Level:     level,
		CreatedAt: time.Now(),
	}

	ls.repo.CreateSyncLog(ctx, log)
}
