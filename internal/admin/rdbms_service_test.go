package admin

import (
	"context"
	"testing"
)

type mockRDBMSProvider struct {
	syncResult *SyncResult
	syncErr    error
}

func (m *mockRDBMSProvider) Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error) {
	return nil, nil
}

func (m *mockRDBMSProvider) GetUser(ctx context.Context, userID string) (*ProviderUser, error) {
	return nil, nil
}

func (m *mockRDBMSProvider) ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	return nil, 0, nil
}

func (m *mockRDBMSProvider) SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error) {
	return m.syncResult, m.syncErr
}

func (m *mockRDBMSProvider) Validate(ctx context.Context) error {
	return nil
}

type mockRDBMSServiceRepository struct {
	syncJobs map[string]*SyncJob
	syncLogs []*SyncLog
}

func (m *mockRDBMSServiceRepository) CreateSyncJob(ctx context.Context, job *SyncJob) error {
	m.syncJobs[job.ID] = job
	return nil
}

func (m *mockRDBMSServiceRepository) GetSyncJob(ctx context.Context, jobID string) (*SyncJob, error) {
	if job, ok := m.syncJobs[jobID]; ok {
		return job, nil
	}
	return nil, ErrNotFound
}

func (m *mockRDBMSServiceRepository) UpdateSyncJob(ctx context.Context, job *SyncJob) error {
	m.syncJobs[job.ID] = job
	return nil
}

func (m *mockRDBMSServiceRepository) ListSyncJobs(ctx context.Context, limit, offset int) ([]*SyncJob, int64, error) {
	var jobs []*SyncJob
	for _, job := range m.syncJobs {
		jobs = append(jobs, job)
	}
	return jobs, int64(len(jobs)), nil
}

func (m *mockRDBMSServiceRepository) CreateSyncLog(ctx context.Context, log *SyncLog) error {
	m.syncLogs = append(m.syncLogs, log)
	return nil
}

func (m *mockRDBMSServiceRepository) ListSyncLogs(ctx context.Context, jobID string, limit, offset int) ([]*SyncLog, int64, error) {
	var logs []*SyncLog
	for _, log := range m.syncLogs {
		if log.JobID == jobID {
			logs = append(logs, log)
		}
	}
	return logs, int64(len(logs)), nil
}

func TestRDBMSServiceTriggerSync(t *testing.T) {
	repo := &mockRDBMSServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockRDBMSProvider{
		syncResult: &SyncResult{
			Created: 25,
			Updated: 10,
			Deleted: 3,
			Failed:  0,
		},
	}

	service := NewRDBMSService(repo, provider)
	ctx := context.Background()

	tests := []struct {
		name        string
		incremental bool
		shouldErr   bool
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

func TestRDBMSServiceGetSyncStatus(t *testing.T) {
	repo := &mockRDBMSServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockRDBMSProvider{
		syncResult: &SyncResult{Created: 20},
	}

	service := NewRDBMSService(repo, provider)
	ctx := context.Background()

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

func TestRDBMSServiceListSyncHistory(t *testing.T) {
	repo := &mockRDBMSServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockRDBMSProvider{
		syncResult: &SyncResult{Created: 15},
	}

	service := NewRDBMSService(repo, provider)
	ctx := context.Background()

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

func TestRDBMSServiceGetSyncLogs(t *testing.T) {
	repo := &mockRDBMSServiceRepository{
		syncJobs: make(map[string]*SyncJob),
		syncLogs: []*SyncLog{},
	}
	provider := &mockRDBMSProvider{
		syncResult: &SyncResult{Created: 18},
	}

	service := NewRDBMSService(repo, provider)
	ctx := context.Background()

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
				// OK - no logs yet
			}
		})
	}
}

func TestRDBMSServiceValidateConnection(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	service := NewRDBMSService(
		&mockRDBMSServiceRepository{
			syncJobs: make(map[string]*SyncJob),
		},
		provider,
	)
	ctx := context.Background()

	err := service.ValidateConnection(ctx)
	if err != nil {
		t.Errorf("ValidateConnection() error = %v", err)
	}
}
