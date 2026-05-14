package ldap

import (
	"context"
	"testing"
	"time"
)

func TestSyncUsersValidation(t *testing.T) {
	provider := New(nil)

	// Test missing domain_id
	_, err := provider.SyncUsers(context.Background(), SyncRequest{})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}

	// Test with valid domain_id but no config
	_, err = provider.SyncUsers(context.Background(), SyncRequest{DomainID: "test-domain"})
	if err == nil {
		t.Errorf("Expected error when provider not configured, got nil")
	}
}

func TestSyncGroupsValidation(t *testing.T) {
	provider := New(nil)

	// Test missing domain_id
	_, err := provider.SyncGroups(context.Background(), SyncRequest{})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}

	// Test with valid domain_id but no config
	_, err = provider.SyncGroups(context.Background(), SyncRequest{DomainID: "test-domain"})
	if err == nil {
		t.Errorf("Expected error when provider not configured, got nil")
	}
}

func TestSyncMembershipsValidation(t *testing.T) {
	provider := New(nil)

	// Test missing domain_id
	_, err := provider.SyncMemberships(context.Background(), SyncRequest{})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}

	// Test with valid domain_id but no config
	_, err = provider.SyncMemberships(context.Background(), SyncRequest{DomainID: "test-domain"})
	if err == nil {
		t.Errorf("Expected error when provider not configured, got nil")
	}
}

func TestSyncRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     SyncRequest
		wantErr bool
	}{
		{
			name:    "empty request",
			req:     SyncRequest{},
			wantErr: true,
		},
		{
			name: "valid request with domain_id but no config",
			req: SyncRequest{
				DomainID: "test-domain",
				TargetDN: "ou=users,dc=example,dc=com",
				Filter:   "(objectClass=person)",
				Limit:    100,
			},
			wantErr: true, // Error because no config provided
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := New(nil)
			_, err := provider.SyncUsers(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncUsers error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncResultTimestamp(t *testing.T) {
	provider := New(&Config{
		Host:      "ldap.example.com",
		Port:      389,
		BaseDN:    "dc=example,dc=com",
		BindDN:    "cn=admin,dc=example,dc=com",
		BindPass:  "password",
		UsersDN:   "ou=users,dc=example,dc=com",
		GroupsDN:  "ou=groups,dc=example,dc=com",
		UserAttr:  "uid",
		GroupAttr: "cn",
	})

	before := time.Now()
	result, _ := provider.SyncUsers(context.Background(), SyncRequest{DomainID: "test-domain"})
	after := time.Now()

	if result.LastSyncTime.Before(before) || result.LastSyncTime.After(after.Add(time.Second)) {
		t.Errorf("LastSyncTime %v not in expected range [%v, %v]", result.LastSyncTime, before, after)
	}
}
