package drive

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestValidateCreateShareLinkRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req, tokenDigest, err := ValidateCreateShareLinkRequest(CreateShareLinkRequest{
		UserID:     " user-1 ",
		NodeID:     " node-1 ",
		Permission: " Download ",
		ExpiresAt:  now.Add(time.Hour),
		Token:      strings.Repeat("a", 40),
	}, now)
	if err != nil {
		t.Fatalf("ValidateCreateShareLinkRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.Permission != ShareLinkPermissionDownload || req.Token != strings.Repeat("a", 40) {
		t.Fatalf("request = %+v, want normalized fields", req)
	}
	if !strings.Contains(tokenDigest, ":") {
		t.Fatalf("token digest = %q, want hash/suffix pair", tokenDigest)
	}

	defaulted, _, err := ValidateCreateShareLinkRequest(CreateShareLinkRequest{
		UserID: "user-1",
		NodeID: "node-1",
		Token:  strings.Repeat("b", 40),
	}, now)
	if err != nil {
		t.Fatalf("ValidateCreateShareLinkRequest default returned error: %v", err)
	}
	if defaulted.Permission != ShareLinkPermissionView || !defaulted.ExpiresAt.Equal(now.Add(DefaultShareLinkTTL)) {
		t.Fatalf("defaulted request = %+v, want view/default expiry", defaulted)
	}
}

func TestValidateCreateShareLinkRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	tests := []CreateShareLinkRequest{
		{NodeID: "node-1", Token: strings.Repeat("a", 40), ExpiresAt: now.Add(time.Hour)},
		{UserID: "user-1", NodeID: "node\n1", Token: strings.Repeat("a", 40), ExpiresAt: now.Add(time.Hour)},
		{UserID: "user-1", NodeID: "node-1", Permission: "edit", Token: strings.Repeat("a", 40), ExpiresAt: now.Add(time.Hour)},
		{UserID: "user-1", NodeID: "node-1", Token: "short", ExpiresAt: now.Add(time.Hour)},
		{UserID: "user-1", NodeID: "node-1", Token: strings.Repeat("a", 40) + "\n", ExpiresAt: now.Add(time.Hour)},
		{UserID: "user-1", NodeID: "node-1", Token: strings.Repeat("a", 40), ExpiresAt: now},
		{UserID: "user-1", NodeID: "node-1", Token: strings.Repeat("a", 40), ExpiresAt: now.Add(MaxShareLinkTTL + time.Second)},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCreateShareLinkRequest(tc, now); err == nil {
				t.Fatalf("ValidateCreateShareLinkRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateListShareLinksRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListShareLinksRequest(ListShareLinksRequest{
		UserID: " user-1 ",
		NodeID: " node-1 ",
		Status: " Revoked ",
		Limit:  500,
	})
	if err != nil {
		t.Fatalf("ValidateListShareLinksRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.Status != ShareLinkStatusRevoked || req.Limit != 200 {
		t.Fatalf("request = %+v, want normalized list request", req)
	}
}

func TestShareLinkRepositoryAndServiceRequireDependencies(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	if _, err := repo.CreateShareLink(context.Background(), CreateShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CreateShareLink err = %v, want database handle rejection", err)
	}
	if _, err := repo.ListShareLinks(context.Background(), ListShareLinksRequest{}); err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("ListShareLinks err = %v, want database handle rejection", err)
	}
	if _, err := repo.RevokeShareLink(context.Background(), RevokeShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("RevokeShareLink err = %v, want database handle rejection", err)
	}
	if _, err := (*Service)(nil).CreateShareLink(context.Background(), CreateShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service CreateShareLink err = %v, want repository rejection", err)
	}
	if _, err := (*Service)(nil).ListShareLinks(context.Background(), ListShareLinksRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service ListShareLinks err = %v, want repository rejection", err)
	}
	if _, err := (*Service)(nil).RevokeShareLink(context.Background(), RevokeShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service RevokeShareLink err = %v, want repository rejection", err)
	}
}
