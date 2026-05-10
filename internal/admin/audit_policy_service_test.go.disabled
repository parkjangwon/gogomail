package admin

import (
	"context"
	"testing"
)

type mockAuditPolicyRepository struct {
	policies map[string]*AuditPolicyConfig
}

func (m *mockAuditPolicyRepository) GetPolicy(ctx context.Context, companyID string) (*AuditPolicyConfig, error) {
	if policy, ok := m.policies[companyID]; ok {
		return policy, nil
	}
	return nil, ErrNotFound
}

func (m *mockAuditPolicyRepository) SavePolicy(ctx context.Context, policy *AuditPolicyConfig) error {
	m.policies[policy.CompanyID] = policy
	return nil
}

func (m *mockAuditPolicyRepository) DeletePolicy(ctx context.Context, companyID string) error {
	delete(m.policies, companyID)
	return nil
}

func newMockAuditPolicyRepository() *mockAuditPolicyRepository {
	return &mockAuditPolicyRepository{
		policies: make(map[string]*AuditPolicyConfig),
	}
}

func TestAuditPolicyServiceGetPolicy(t *testing.T) {
	repo := newMockAuditPolicyRepository()
	service := NewAuditPolicyService(repo)
	ctx := context.Background()

	// Create a policy
	policy := &AuditPolicyConfig{
		CompanyID:           "company-1",
		AuditLevel:          "level_2",
		AuditAdminActions:   true,
		AuditSecurityEvents: true,
	}
	repo.SavePolicy(ctx, policy)

	tests := []struct {
		name      string
		companyID string
		shouldErr bool
	}{
		{
			name:      "get existing policy",
			companyID: "company-1",
			shouldErr: false,
		},
		{
			name:      "get nonexistent policy",
			companyID: "company-2",
			shouldErr: true,
		},
		{
			name:      "empty companyID",
			companyID: "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := service.GetPolicy(ctx, tt.companyID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetPolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && policy == nil {
				t.Error("GetPolicy() returned nil policy")
			}
		})
	}
}

func TestAuditPolicyServiceSetPolicy(t *testing.T) {
	repo := newMockAuditPolicyRepository()
	service := NewAuditPolicyService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		companyID string
		level     string
		shouldErr bool
	}{
		{
			name:      "set level 1 policy",
			companyID: "company-1",
			level:     "level_1",
			shouldErr: false,
		},
		{
			name:      "set level 2 policy",
			companyID: "company-2",
			level:     "level_2",
			shouldErr: false,
		},
		{
			name:      "set level 3 policy",
			companyID: "company-3",
			level:     "level_3",
			shouldErr: false,
		},
		{
			name:      "invalid level",
			companyID: "company-4",
			level:     "invalid",
			shouldErr: true,
		},
		{
			name:      "empty companyID",
			companyID: "",
			level:     "level_1",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SetPolicy(ctx, tt.companyID, tt.level)
			if (err != nil) != tt.shouldErr {
				t.Errorf("SetPolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestAuditPolicyServiceValidatePolicy(t *testing.T) {
	repo := newMockAuditPolicyRepository()
	service := NewAuditPolicyService(repo)

	tests := []struct {
		name      string
		level     string
		shouldErr bool
	}{
		{
			name:      "valid level 1",
			level:     "level_1",
			shouldErr: false,
		},
		{
			name:      "valid level 2",
			level:     "level_2",
			shouldErr: false,
		},
		{
			name:      "valid level 3",
			level:     "level_3",
			shouldErr: false,
		},
		{
			name:      "invalid level 0",
			level:     "level_0",
			shouldErr: true,
		},
		{
			name:      "invalid level 4",
			level:     "level_4",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePolicy(tt.level)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ValidatePolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestAuditPolicyServiceIsLevelEnabled(t *testing.T) {
	repo := newMockAuditPolicyRepository()
	service := NewAuditPolicyService(repo)
	ctx := context.Background()

	// Create policies at different levels
	repo.SavePolicy(ctx, &AuditPolicyConfig{
		CompanyID:  "level1",
		AuditLevel: "level_1",
	})
	repo.SavePolicy(ctx, &AuditPolicyConfig{
		CompanyID:  "level3",
		AuditLevel: "level_3",
	})

	tests := []struct {
		name       string
		companyID  string
		checkLevel string
		expected   bool
	}{
		{
			name:       "level 1 company checks level 1",
			companyID:  "level1",
			checkLevel: "level_1",
			expected:   true,
		},
		{
			name:       "level 1 company checks level 2",
			companyID:  "level1",
			checkLevel: "level_2",
			expected:   false,
		},
		{
			name:       "level 3 company checks all levels",
			companyID:  "level3",
			checkLevel: "level_2",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IsLevelEnabled(ctx, tt.companyID, tt.checkLevel)
			if result != tt.expected {
				t.Errorf("IsLevelEnabled() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestAuditPolicyServiceDeletePolicy(t *testing.T) {
	repo := newMockAuditPolicyRepository()
	service := NewAuditPolicyService(repo)
	ctx := context.Background()

	// Create a policy
	repo.SavePolicy(ctx, &AuditPolicyConfig{
		CompanyID:  "company-1",
		AuditLevel: "level_2",
	})

	tests := []struct {
		name      string
		companyID string
		shouldErr bool
	}{
		{
			name:      "delete existing policy",
			companyID: "company-1",
			shouldErr: false,
		},
		{
			name:      "delete nonexistent policy",
			companyID: "company-2",
			shouldErr: false,
		},
		{
			name:      "empty companyID",
			companyID: "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.DeletePolicy(ctx, tt.companyID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("DeletePolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
