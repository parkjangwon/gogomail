package admin

import (
	"context"
	"fmt"
	"time"
)

// RDBMSService manages external RDBMS sync operations
type RDBMSService struct {
	repo     LDAPServiceRepository
	provider IdentityProvider
}

// NewRDBMSService creates a new RDBMS service
func NewRDBMSService(repo LDAPServiceRepository, provider IdentityProvider) *RDBMSService {
	return &RDBMSService{
		repo:     repo,
		provider: provider,
	}
}

// TriggerSync triggers a new RDBMS sync job
func (rs *RDBMSService) TriggerSync(ctx context.Context, incremental bool) (string, error) {
	jobID := fmt.Sprintf("rdbms-sync-%d", time.Now().UnixNano())

	job := &SyncJob{
		ID:          jobID,
		Status:      "pending",
		Incremental: incremental,
		StartTime:   time.Now(),
	}

	if err := rs.repo.CreateSyncJob(ctx, job); err != nil {
		return "", err
	}

	// Execute sync asynchronously
	go rs.executeSync(context.Background(), jobID, incremental)

	return jobID, nil
}

// executeSync performs the actual sync operation
func (rs *RDBMSService) executeSync(ctx context.Context, jobID string, incremental bool) {
	job, err := rs.repo.GetSyncJob(ctx, jobID)
	if err != nil {
		return
	}

	job.Status = "running"
	rs.repo.UpdateSyncJob(ctx, job)

	// Call the provider to sync users
	syncResult, err := rs.provider.SyncUsers(ctx, incremental)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.EndTime = time.Now()
		rs.repo.UpdateSyncJob(ctx, job)

		rs.logSync(ctx, jobID, "error", fmt.Sprintf("Sync failed: %v", err))
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

	rs.repo.UpdateSyncJob(ctx, job)

	rs.logSync(ctx, jobID, "info", fmt.Sprintf(
		"Sync completed: created=%d, updated=%d, deleted=%d, failed=%d",
		syncResult.Created, syncResult.Updated, syncResult.Deleted, syncResult.Failed,
	))
}

// GetSyncStatus returns the status of a sync job
func (rs *RDBMSService) GetSyncStatus(ctx context.Context, jobID string) (*SyncJob, error) {
	if jobID == "" {
		return nil, fmt.Errorf("%w: jobID", ErrMissingRequiredField)
	}

	return rs.repo.GetSyncJob(ctx, jobID)
}

// ListSyncHistory lists all sync jobs
func (rs *RDBMSService) ListSyncHistory(ctx context.Context, limit, offset int) ([]*SyncJob, int64, error) {
	if limit == 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	return rs.repo.ListSyncJobs(ctx, limit, offset)
}

// GetSyncLogs gets logs for a specific sync job
func (rs *RDBMSService) GetSyncLogs(ctx context.Context, jobID string, limit, offset int) ([]*SyncLog, int64, error) {
	if jobID == "" {
		return nil, 0, fmt.Errorf("%w: jobID", ErrMissingRequiredField)
	}

	if limit == 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	return rs.repo.ListSyncLogs(ctx, jobID, limit, offset)
}

// ValidateConnection validates the RDBMS connection
func (rs *RDBMSService) ValidateConnection(ctx context.Context) error {
	return rs.provider.Validate(ctx)
}

// logSync creates a sync log entry
func (rs *RDBMSService) logSync(ctx context.Context, jobID, level, message string) {
	if rs.repo == nil {
		return
	}

	log := &SyncLog{
		ID:        fmt.Sprintf("rdbms-log-%d", time.Now().UnixNano()),
		JobID:     jobID,
		Message:   message,
		Level:     level,
		CreatedAt: time.Now(),
	}

	rs.repo.CreateSyncLog(ctx, log)
}
