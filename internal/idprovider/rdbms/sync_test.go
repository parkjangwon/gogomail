package rdbms

import (
	"context"
	"testing"
	"time"
)

func TestSyncUsersNotConfigured(t *testing.T) {
	p := New(nil)

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncUsers(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
}

func TestSyncUsersNotConnected(t *testing.T) {
	p := New(&Config{
		ConnectionString: "postgres://localhost/test",
		UserQuery:        "SELECT id, name FROM users",
	})

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncUsers(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for disconnected provider")
	}
}

func TestSyncUsersMissingDomainID(t *testing.T) {
	p := New(&Config{
		ConnectionString: "postgres://localhost/test",
		UserQuery:        "SELECT id, name FROM users",
	})
	p.db = nil

	req := SyncRequest{
		DomainID: "",
	}

	_, err := p.SyncUsers(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing domain id")
	}
}

func TestSyncUsersNotImplemented(t *testing.T) {
	cfg := &Config{
		ConnectionString: "postgres://localhost/test",
		UserQuery:        "SELECT id, name FROM users",
		MaxPoolSize:      5,
	}
	p := New(cfg)

	// Test that NotImplemented error is returned when not configured
	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncUsers(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for not connected provider")
	}
}

func TestSyncGroupsNotConfigured(t *testing.T) {
	p := New(nil)

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncGroups(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
}

func TestSyncGroupsNotConnected(t *testing.T) {
	p := New(&Config{
		ConnectionString: "postgres://localhost/test",
		GroupQuery:       "SELECT id, name FROM groups",
	})

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncGroups(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for disconnected provider")
	}
}

func TestSyncMembershipsNotConfigured(t *testing.T) {
	p := New(nil)

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncMemberships(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
}

func TestSyncMembershipsNotConnected(t *testing.T) {
	p := New(&Config{
		ConnectionString: "postgres://localhost/test",
		UserQuery:        "SELECT id, name FROM users",
		GroupQuery:       "SELECT id, name FROM groups",
	})

	req := SyncRequest{
		DomainID: "domain-1",
	}

	_, err := p.SyncMemberships(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for disconnected provider")
	}
}

func TestSyncResultInitialization(t *testing.T) {
	result := SyncResult{
		LastSyncTime: time.Now(),
	}

	if result.UsersCreated != 0 {
		t.Fatal("expected UsersCreated to be 0")
	}
	if result.UsersUpdated != 0 {
		t.Fatal("expected UsersUpdated to be 0")
	}
	if result.UsersDeleted != 0 {
		t.Fatal("expected UsersDeleted to be 0")
	}
	if result.ConflictCount != 0 {
		t.Fatal("expected ConflictCount to be 0")
	}
	if result.ErrorCount != 0 {
		t.Fatal("expected ErrorCount to be 0")
	}
	if result.LastSyncTime.IsZero() {
		t.Fatal("expected LastSyncTime to be set")
	}
}

func TestSyncRequestInitialization(t *testing.T) {
	now := time.Now()
	req := SyncRequest{
		DomainID:  "domain-1",
		Query:     "SELECT * FROM users",
		Limit:     100,
		Timestamp: now,
	}

	if req.DomainID != "domain-1" {
		t.Fatal("expected DomainID to be set")
	}
	if req.Query != "SELECT * FROM users" {
		t.Fatal("expected Query to be set")
	}
	if req.Limit != 100 {
		t.Fatal("expected Limit to be 100")
	}
	if req.Timestamp != now {
		t.Fatal("expected Timestamp to be set")
	}
}
