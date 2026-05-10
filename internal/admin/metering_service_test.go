package admin

import (
	"context"
	"testing"
	"time"
)

type mockMeteringRepository struct {
	usageRecords map[string]int64
	quotas       map[string]int64
}

func (m *mockMeteringRepository) RecordAPICall(ctx context.Context, userID string) error {
	m.usageRecords[userID]++
	return nil
}

func (m *mockMeteringRepository) GetUsage(ctx context.Context, userID string, period time.Time) (int64, error) {
	if count, ok := m.usageRecords[userID]; ok {
		return count, nil
	}
	return 0, nil
}

func (m *mockMeteringRepository) SetQuota(ctx context.Context, userID string, quota int64) error {
	m.quotas[userID] = quota
	return nil
}

func (m *mockMeteringRepository) GetQuota(ctx context.Context, userID string) (int64, error) {
	if quota, ok := m.quotas[userID]; ok {
		return quota, nil
	}
	return 0, ErrNotFound
}

func (m *mockMeteringRepository) ResetUsage(ctx context.Context, userID string) error {
	m.usageRecords[userID] = 0
	return nil
}

func newMockMeteringRepository() *mockMeteringRepository {
	return &mockMeteringRepository{
		usageRecords: make(map[string]int64),
		quotas:       make(map[string]int64),
	}
}

func TestMeteringServiceRecordAPICall(t *testing.T) {
	repo := newMockMeteringRepository()
	service := NewMeteringService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		shouldErr bool
	}{
		{
			name:      "record valid call",
			userID:    "user1",
			shouldErr: false,
		},
		{
			name:      "missing userID",
			userID:    "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RecordAPICall(ctx, tt.userID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("RecordAPICall() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestMeteringServiceCheckRateLimit(t *testing.T) {
	repo := newMockMeteringRepository()
	service := NewMeteringService(repo)
	ctx := context.Background()

	// Set quota for user
	repo.SetQuota(ctx, "user1", 100) // 100 calls per period

	tests := []struct {
		name         string
		userID       string
		callsToMake  int
		quota        int64
		expectLimited bool
	}{
		{
			name:         "within quota",
			userID:       "user1",
			callsToMake:  50,
			quota:        100,
			expectLimited: false,
		},
		{
			name:         "at quota limit",
			userID:       "user2",
			callsToMake:  100,
			quota:        100,
			expectLimited: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.SetQuota(ctx, tt.userID, tt.quota)

			// Make API calls up to the limit
			for i := int64(0); i < int64(tt.callsToMake); i++ {
				repo.RecordAPICall(ctx, tt.userID)
			}

			allowed, err := service.CheckRateLimit(ctx, tt.userID)
			if err != nil && !tt.expectLimited {
				t.Errorf("CheckRateLimit() error = %v", err)
			}
			if allowed && tt.expectLimited {
				t.Errorf("CheckRateLimit() should have been limited")
			}
		})
	}
}

func TestMeteringServiceGetUsage(t *testing.T) {
	repo := newMockMeteringRepository()
	service := NewMeteringService(repo)
	ctx := context.Background()

	// Record some calls
	repo.RecordAPICall(ctx, "user1")
	repo.RecordAPICall(ctx, "user1")
	repo.RecordAPICall(ctx, "user1")

	tests := []struct {
		name      string
		userID    string
		shouldErr bool
	}{
		{
			name:      "get usage for user with calls",
			userID:    "user1",
			shouldErr: false,
		},
		{
			name:      "get usage for user without calls",
			userID:    "user2",
			shouldErr: false,
		},
		{
			name:      "missing userID",
			userID:    "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, err := service.GetUsage(ctx, tt.userID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetUsage() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && usage < 0 {
				t.Errorf("GetUsage() returned negative count")
			}
		})
	}
}

func TestMeteringServiceSetQuota(t *testing.T) {
	repo := newMockMeteringRepository()
	service := NewMeteringService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		quota     int64
		shouldErr bool
	}{
		{
			name:      "set valid quota",
			userID:    "user1",
			quota:     1000,
			shouldErr: false,
		},
		{
			name:      "zero quota",
			userID:    "user2",
			quota:     0,
			shouldErr: true,
		},
		{
			name:      "missing userID",
			userID:    "",
			quota:     500,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SetQuota(ctx, tt.userID, tt.quota)
			if (err != nil) != tt.shouldErr {
				t.Errorf("SetQuota() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestMeteringServiceResetUsage(t *testing.T) {
	repo := newMockMeteringRepository()
	service := NewMeteringService(repo)
	ctx := context.Background()

	// Record calls
	repo.RecordAPICall(ctx, "user1")
	repo.RecordAPICall(ctx, "user1")

	// Reset usage
	err := service.ResetUsage(ctx, "user1")
	if err != nil {
		t.Errorf("ResetUsage() error = %v", err)
	}

	// Check usage is 0
	usage, _ := service.GetUsage(ctx, "user1")
	if usage != 0 {
		t.Errorf("ResetUsage() failed, usage = %d, expected 0", usage)
	}
}
