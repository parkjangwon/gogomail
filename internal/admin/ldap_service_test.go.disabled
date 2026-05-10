package admin

import (
	"context"
	"errors"
	"testing"
)

var ErrNotFound = errors.New("not found")

type mockLDAPProvider struct {
	syncResult *SyncResult
	syncErr    error
}

func (m *mockLDAPProvider) Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error) {
	return nil, nil
}

func (m *mockLDAPProvider) GetUser(ctx context.Context, userID string) (*ProviderUser, error) {
	return nil, nil
}

func (m *mockLDAPProvider) ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	return nil, 0, nil
}

func (m *mockLDAPProvider) SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error) {
	return m.syncResult, m.syncErr
}

func (m *mockLDAPProvider) Validate(ctx context.Context) error {
	return nil
}

type mockLDAPServiceRepository struct {
	syncJobs map[string]*SyncJob
	syncLogs []*SyncLog
}

func (m *mockLDAPServiceRepository) CreateSyncJob(ctx context.Context, job *SyncJob) error {
	m.syncJobs[job.ID] = job
	return nil
}

func (m *mockLDAPServiceRepository) GetSyncJob(ctx context.Context, jobID string) (*SyncJob, error) {
	if job, ok := m.syncJobs[jobID]; ok {
		return job, nil
	}
	return nil, ErrNotFound
}

func (m *mockLDAPServiceRepository) UpdateSyncJob(ctx context.Context, job *SyncJob) error {
	m.syncJobs[job.ID] = job
	return nil
}

func (m *mockLDAPServiceRepository) ListSyncJobs(ctx context.Context, limit, offset int) ([]*SyncJob, int64, error) {
	var jobs []*SyncJob
	for _, job := range m.syncJobs {
		jobs = append(jobs, job)
	}
	return jobs, int64(len(jobs)), nil
}

func (m *mockLDAPServiceRepository) CreateSyncLog(ctx context.Context, log *SyncLog) error {
	m.syncLogs = append(m.syncLogs, log)
	return nil
}

func (m *mockLDAPServiceRepository) ListSyncLogs(ctx context.Context, jobID string, limit, offset int) ([]*SyncLog, int64, error) {
	var logs []*SyncLog
	for _, log := range m.syncLogs {
		if log.JobID == jobID {
			logs = append(logs, log)
		}
	}
	return logs, int64(len(logs)), nil
}

func TestLDAPServiceTriggerSync(t *testing.T) {
	repo := &mockLDAPServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockLDAPProvider{
		syncResult: &SyncResult{
			Created: 10,
			Updated: 5,
			Deleted: 2,
			Failed:  0,
		},
	}

	service := NewLDAPService(repo, provider, nil)
	ctx := context.Background()

	tests := []struct {
		name      string
		incremental bool
		shouldErr bool
	}{
		{
			name:        "trigger full sync",
			incremental: false,
			shouldErr:   false,
		},
		{
			name:        "trigger incremental sync",
			incremental: true,
			shouldErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID, err := service.TriggerSync(ctx, tt.incremental)
			if (err != nil) != tt.shouldErr {
				t.Errorf("TriggerSync() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && jobID == "" {
				t.Error("TriggerSync() returned empty jobID")
			}
		})
	}
}

func TestLDAPServiceGetSyncStatus(t *testing.T) {
	repo := &mockLDAPServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockLDAPProvider{
		syncResult: &SyncResult{
			Created: 10,
		},
	}

	service := NewLDAPService(repo, provider, nil)
	ctx := context.Background()

	// Create a sync job
	jobID, _ := service.TriggerSync(ctx, false)

	tests := []struct {
		name      string
		jobID     string
		shouldErr bool
	}{
		{
			name:      "get existing job status",
			jobID:     jobID,
			shouldErr: false,
		},
		{
			name:      "get nonexistent job",
			jobID:     "nonexistent",
			shouldErr: true,
		},
		{
			name:      "empty jobID",
			jobID:     "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := service.GetSyncStatus(ctx, tt.jobID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetSyncStatus() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && job == nil {
				t.Error("GetSyncStatus() returned nil job")
			}
		})
	}
}

func TestLDAPServiceListSyncHistory(t *testing.T) {
	repo := &mockLDAPServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockLDAPProvider{
		syncResult: &SyncResult{Created: 5},
	}

	service := NewLDAPService(repo, provider, nil)
	ctx := context.Background()

	// Create multiple sync jobs
	for i := 0; i < 3; i++ {
		service.TriggerSync(ctx, false)
	}

	tests := []struct {
		name      string
		limit     int
		offset    int
		shouldErr bool
	}{
		{
			name:      "list all sync history",
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
		{
			name:      "list with offset",
			limit:     2,
			offset:    1,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, count, err := service.ListSyncHistory(ctx, tt.limit, tt.offset)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ListSyncHistory() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && count < 0 {
				t.Errorf("ListSyncHistory() returned negative count")
			}
			if err == nil && len(jobs) == 0 && count > 0 {
				t.Error("ListSyncHistory() count mismatch")
			}
		})
	}
}

func TestLDAPServiceGetSyncLogs(t *testing.T) {
	repo := &mockLDAPServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockLDAPProvider{
		syncResult: &SyncResult{Created: 5},
	}

	service := NewLDAPService(repo, provider, nil)
	ctx := context.Background()

	// Create a sync job
	jobID, _ := service.TriggerSync(ctx, false)

	tests := []struct {
		name      string
		jobID     string
		limit     int
		offset    int
		shouldErr bool
	}{
		{
			name:      "get logs for existing job",
			jobID:     jobID,
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
		{
			name:      "empty jobID",
			jobID:     "",
			limit:     10,
			offset:    0,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, count, err := service.GetSyncLogs(ctx, tt.jobID, tt.limit, tt.offset)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetSyncLogs() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && count < 0 {
				t.Errorf("GetSyncLogs() returned negative count")
			}
			if err == nil && len(logs) == 0 && count == 0 {
				// OK - no logs created yet
			}
		})
	}
}
