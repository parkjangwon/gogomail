package admin

import (
	"context"
	"testing"
	"time"
)

type mockMailLogRepository struct {
	logs map[string]*MailLogEntry
}

func (m *mockMailLogRepository) CreateMailLog(ctx context.Context, log *MailLogEntry) error {
	m.logs[log.ID] = log
	return nil
}

func (m *mockMailLogRepository) GetMailLog(ctx context.Context, logID string) (*MailLogEntry, error) {
	if log, ok := m.logs[logID]; ok {
		return log, nil
	}
	return nil, ErrNotFound
}

func (m *mockMailLogRepository) QueryMailLogs(ctx context.Context, query *MailLogQuery) ([]*MailLogEntry, int64, error) {
	var results []*MailLogEntry
	for _, log := range m.logs {
		if query.UserID != "" && log.UserID != query.UserID {
			continue
		}
		if query.Action != "" && log.Action != query.Action {
			continue
		}
		if !query.StartTime.IsZero() && log.Timestamp.Before(query.StartTime) {
			continue
		}
		if !query.EndTime.IsZero() && log.Timestamp.After(query.EndTime) {
			continue
		}
		results = append(results, log)
	}
	count := int64(len(results))
	return results, count, nil
}

func (m *mockMailLogRepository) DeleteMailLogsBefore(ctx context.Context, timestamp time.Time) (int64, error) {
	deleted := int64(0)
	for id, log := range m.logs {
		if log.Timestamp.Before(timestamp) {
			delete(m.logs, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockMailLogRepository) CountMailLogsByAction(ctx context.Context, action string) (int64, error) {
	count := int64(0)
	for _, log := range m.logs {
		if log.Action == action {
			count++
		}
	}
	return count, nil
}

func newMockMailLogRepository() *mockMailLogRepository {
	return &mockMailLogRepository{
		logs: make(map[string]*MailLogEntry),
	}
}

func TestMailLogServiceLogOperation(t *testing.T) {
	repo := newMockMailLogRepository()
	service := NewMailLogService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		action    string
		messageID string
		details   map[string]string
		shouldErr bool
	}{
		{
			name:      "log send operation",
			userID:    "user1",
			action:    "send",
			messageID: "msg1",
			details:   map[string]string{"to": "user2@example.com"},
			shouldErr: false,
		},
		{
			name:      "log receive operation",
			userID:    "user1",
			action:    "receive",
			messageID: "msg2",
			details:   map[string]string{"from": "user3@example.com"},
			shouldErr: false,
		},
		{
			name:      "missing userID",
			userID:    "",
			action:    "send",
			messageID: "msg3",
			shouldErr: true,
		},
		{
			name:      "missing action",
			userID:    "user1",
			action:    "",
			messageID: "msg4",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logID, err := service.LogOperation(ctx, tt.userID, tt.action, tt.messageID, tt.details)
			if (err != nil) != tt.shouldErr {
				t.Errorf("LogOperation() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && logID == "" {
				t.Error("LogOperation() returned empty logID")
			}
		})
	}
}

func TestMailLogServiceQueryLogs(t *testing.T) {
	repo := newMockMailLogRepository()
	service := NewMailLogService(repo)
	ctx := context.Background()

	// Create test logs
	service.LogOperation(ctx, "user1", "send", "msg1", map[string]string{})
	service.LogOperation(ctx, "user1", "receive", "msg2", map[string]string{})
	service.LogOperation(ctx, "user2", "send", "msg3", map[string]string{})

	tests := []struct {
		name      string
		userID    string
		action    string
		limit     int
		offset    int
		shouldErr bool
	}{
		{
			name:      "query by user",
			userID:    "user1",
			action:    "",
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
		{
			name:      "query by action",
			userID:    "",
			action:    "send",
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
		{
			name:      "query by user and action",
			userID:    "user1",
			action:    "send",
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &MailLogQuery{
				UserID: tt.userID,
				Action: tt.action,
				Limit:  tt.limit,
				Offset: tt.offset,
			}
			logs, count, err := service.QueryLogs(ctx, query)
			if (err != nil) != tt.shouldErr {
				t.Errorf("QueryLogs() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && count < 0 {
				t.Errorf("QueryLogs() returned negative count")
			}
			if err == nil && len(logs) == 0 && count > 0 {
				t.Error("QueryLogs() count mismatch")
			}
		})
	}
}

func TestMailLogServiceApplyRetentionPolicy(t *testing.T) {
	repo := newMockMailLogRepository()
	service := NewMailLogService(repo)
	ctx := context.Background()

	// Create logs with different timestamps
	now := time.Now()
	oldTime := now.AddDate(0, -13, 0) // 13 months ago (beyond 12 month retention)
	recentTime := now.AddDate(0, -1, 0) // 1 month ago (within 12 month retention)

	// Create old log (should be deleted)
	repo.CreateMailLog(ctx, &MailLogEntry{
		ID:        "old-log",
		UserID:    "user1",
		Action:    "send",
		MessageID: "msg1",
		Timestamp: oldTime,
	})

	// Create recent log (should be kept)
	repo.CreateMailLog(ctx, &MailLogEntry{
		ID:        "recent-log",
		UserID:    "user1",
		Action:    "send",
		MessageID: "msg2",
		Timestamp: recentTime,
	})

	// Apply retention policy (12 months)
	deleted, err := service.ApplyRetentionPolicy(ctx, 12)
	if err != nil {
		t.Errorf("ApplyRetentionPolicy() error = %v", err)
	}
	if deleted == 0 {
		t.Error("ApplyRetentionPolicy() should have deleted old logs")
	}

	// Verify old log was deleted
	_, err = repo.GetMailLog(ctx, "old-log")
	if err == nil {
		t.Error("Old log should have been deleted")
	}

	// Verify recent log was kept
	_, err = repo.GetMailLog(ctx, "recent-log")
	if err != nil {
		t.Errorf("Recent log should have been kept")
	}
}

func TestMailLogServiceGetStatistics(t *testing.T) {
	repo := newMockMailLogRepository()
	service := NewMailLogService(repo)
	ctx := context.Background()

	// Create test logs
	service.LogOperation(ctx, "user1", "send", "msg1", map[string]string{})
	service.LogOperation(ctx, "user1", "send", "msg2", map[string]string{})
	service.LogOperation(ctx, "user1", "receive", "msg3", map[string]string{})

	tests := []struct {
		name      string
		action    string
		shouldErr bool
	}{
		{
			name:      "get send count",
			action:    "send",
			shouldErr: false,
		},
		{
			name:      "get receive count",
			action:    "receive",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := service.GetActionCount(ctx, tt.action)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetActionCount() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && count < 0 {
				t.Errorf("GetActionCount() returned negative count")
			}
		})
	}
}
