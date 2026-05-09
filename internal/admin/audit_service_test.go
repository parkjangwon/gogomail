package admin

import (
	"context"
	"testing"
)

func TestLogAdminAction(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)
	ctx := context.Background()

	tests := []struct {
		name        string
		adminID     string
		companyID   string
		action      string
		resourceType string
		shouldErr   bool
	}{
		{
			name:        "valid action log",
			adminID:     "admin-1",
			companyID:   "company-1",
			action:      "user.create",
			resourceType: "user",
			shouldErr:   false,
		},
		{
			name:      "missing adminID",
			adminID:   "",
			companyID: "company-1",
			action:    "user.create",
			shouldErr: true,
		},
		{
			name:      "missing companyID",
			adminID:   "admin-1",
			companyID: "",
			action:    "user.create",
			shouldErr: true,
		},
		{
			name:        "missing action",
			adminID:     "admin-1",
			companyID:   "company-1",
			action:      "",
			resourceType: "user",
			shouldErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.LogAdminAction(ctx, tt.adminID, tt.companyID, tt.action, tt.resourceType, "resource-1", nil)
			if (err != nil) != tt.shouldErr {
				t.Errorf("LogAdminAction() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestLogSecurityEvent(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		adminID   string
		companyID string
		event     string
		shouldErr bool
	}{
		{
			name:      "valid security event",
			adminID:   "admin-1",
			companyID: "company-1",
			event:     "permission_granted",
			shouldErr: false,
		},
		{
			name:      "missing adminID",
			adminID:   "",
			companyID: "company-1",
			event:     "permission_granted",
			shouldErr: true,
		},
		{
			name:      "missing event",
			adminID:   "admin-1",
			companyID: "company-1",
			event:     "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.LogSecurityEvent(ctx, tt.adminID, tt.companyID, tt.event, nil)
			if (err != nil) != tt.shouldErr {
				t.Errorf("LogSecurityEvent() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestLogLoginAttempt(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		companyID string
		success   bool
		shouldErr bool
	}{
		{
			name:      "successful login",
			userID:    "user-1",
			companyID: "company-1",
			success:   true,
			shouldErr: false,
		},
		{
			name:      "failed login",
			userID:    "user-1",
			companyID: "company-1",
			success:   false,
			shouldErr: false,
		},
		{
			name:      "missing userID",
			userID:    "",
			companyID: "company-1",
			success:   true,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.LogLoginAttempt(ctx, tt.userID, tt.companyID, "127.0.0.1", "Mozilla", tt.success, "")
			if (err != nil) != tt.shouldErr {
				t.Errorf("LogLoginAttempt() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestQueryAuditLogs(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)
	ctx := context.Background()

	// Create test logs
	svc.LogAdminAction(ctx, "admin-1", "company-1", "user.create", "user", "user-1", nil)
	svc.LogAdminAction(ctx, "admin-1", "company-1", "role.assign", "user", "user-2", nil)
	svc.LogSecurityEvent(ctx, "admin-2", "company-1", "permission_granted", nil)

	tests := []struct {
		name      string
		companyID string
		action    string
		shouldErr bool
	}{
		{
			name:      "query all company logs",
			companyID: "company-1",
			shouldErr: false,
		},
		{
			name:      "missing companyID",
			companyID: "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &AuditLogFilter{
				CompanyID: tt.companyID,
				Limit:     10,
			}
			_, _, err := svc.QueryAuditLogs(ctx, filter)
			if (err != nil) != tt.shouldErr {
				t.Errorf("QueryAuditLogs() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestMaskEmail(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)

	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "valid email",
			email: "user@example.com",
			want:  "u***@example.com",
		},
		{
			name:  "short email",
			email: "ab@example.com",
			want:  "a*@example.com",
		},
		{
			name:  "empty email",
			email: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.MaskEmail(tt.email)
			if got != tt.want {
				t.Errorf("MaskEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyRetentionPolicy(t *testing.T) {
	repo := newMockRepository()
	svc := NewAuditService(repo)
	ctx := context.Background()

	// Create some logs
	svc.LogAdminAction(ctx, "admin-1", "company-1", "user.create", "user", "user-1", nil)

	tests := []struct {
		name          string
		companyID     string
		retentionDays int
		shouldErr     bool
	}{
		{
			name:          "cleanup old logs",
			companyID:     "company-1",
			retentionDays: 90,
			shouldErr:     false,
		},
		{
			name:          "missing companyID",
			companyID:     "",
			retentionDays: 90,
			shouldErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ApplyRetentionPolicy(ctx, tt.companyID, tt.retentionDays)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ApplyRetentionPolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
