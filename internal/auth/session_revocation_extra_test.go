package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRevocationChecker struct {
	version int64
	err     error
}

func (s *stubRevocationChecker) SessionVersionFor(_ context.Context, _ string) (int64, error) {
	return s.version, s.err
}

func TestVerifyFull_NoChecker_Passes(t *testing.T) {
	mgr, _ := NewTokenManager("test-secret-32-bytes-long-enough!")
	token, _ := mgr.Sign(Claims{UserID: "u1"}, time.Minute)

	claims, err := mgr.VerifyFull(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != "u1" {
		t.Fatalf("expected user u1, got %s", claims.UserID)
	}
}

func TestVerifyFull_VersionMatches_Passes(t *testing.T) {
	mgr, _ := NewTokenManager("test-secret-32-bytes-long-enough!")
	mgr.SetRevocationChecker(&stubRevocationChecker{version: 3})

	token, _ := mgr.Sign(Claims{UserID: "u1", SessionVersion: 3}, time.Minute)
	if _, err := mgr.VerifyFull(context.Background(), token); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestVerifyFull_VersionAhead_Passes(t *testing.T) {
	mgr, _ := NewTokenManager("test-secret-32-bytes-long-enough!")
	mgr.SetRevocationChecker(&stubRevocationChecker{version: 2})

	// token issued after an increment — session_ver == current version
	token, _ := mgr.Sign(Claims{UserID: "u1", SessionVersion: 2}, time.Minute)
	if _, err := mgr.VerifyFull(context.Background(), token); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestVerifyFull_OldToken_Rejected(t *testing.T) {
	mgr, _ := NewTokenManager("test-secret-32-bytes-long-enough!")
	mgr.SetRevocationChecker(&stubRevocationChecker{version: 1})

	// old token has session_ver=0 (omitted), user revoked all sessions
	token, _ := mgr.Sign(Claims{UserID: "u1", SessionVersion: 0}, time.Minute)
	_, err := mgr.VerifyFull(context.Background(), token)
	if err == nil {
		t.Fatal("expected rejection of old token")
	}
	if err.Error() != "session revoked" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyFull_CheckerError_Propagates(t *testing.T) {
	mgr, _ := NewTokenManager("test-secret-32-bytes-long-enough!")
	mgr.SetRevocationChecker(&stubRevocationChecker{err: errors.New("db down")})

	token, _ := mgr.Sign(Claims{UserID: "u1"}, time.Minute)
	_, err := mgr.VerifyFull(context.Background(), token)
	if err == nil || err.Error() != "session check: db down" {
		t.Fatalf("expected session check error, got %v", err)
	}
}
