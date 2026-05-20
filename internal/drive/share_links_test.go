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

func TestValidateResolveShareLinkRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req, tokenHash, err := ValidateResolveShareLinkRequest(ResolveShareLinkRequest{
		Token: strings.Repeat("a", 40),
		Now:   now,
	})
	if err != nil {
		t.Fatalf("ValidateResolveShareLinkRequest returned error: %v", err)
	}
	if req.Token != strings.Repeat("a", 40) || !req.Now.Equal(now) {
		t.Fatalf("request = %+v, want token/now preserved", req)
	}
	if len(tokenHash) != 64 {
		t.Fatalf("token hash length = %d, want 64", len(tokenHash))
	}
}

func TestValidateResolveShareLinkRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, token := range []string{
		"short",
		strings.Repeat("a", MaxShareLinkTokenBytes+1),
		strings.Repeat("a", 40) + "\n",
	} {
		token := token
		t.Run(token[:min(len(token), 8)], func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateResolveShareLinkRequest(ResolveShareLinkRequest{Token: token}); err == nil {
				t.Fatalf("ValidateResolveShareLinkRequest(%q) error = nil, want rejection", token)
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

func TestListShareLinksQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListShareLinksQuery(ListShareLinksRequest{
		UserID: "user-1",
		NodeID: "node-1",
		Status: ShareLinkStatusActive,
		Limit:  25,
	})
	for _, want := range []string{
		"FROM drive_share_links",
		"WHERE user_id = $1::uuid",
		"AND status = $2",
		"AND node_id = $3::uuid",
		"ORDER BY updated_at DESC, id DESC",
		"LIMIT $4",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list share links query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF($2, '') IS NULL",
		"OR node_id",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list share links query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args len = %d, want 4", len(args))
	}
	if args[0] != "user-1" || args[1] != ShareLinkStatusActive || args[2] != "node-1" || args[3] != 25 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildListShareLinksQuery(ListShareLinksRequest{UserID: "user-1", Status: ShareLinkStatusRevoked, Limit: 50})
	if strings.Contains(query, "node_id = $") {
		t.Fatalf("node-agnostic share link query unexpectedly includes node filter:\n%s", query)
	}
	if len(args) != 3 || args[0] != "user-1" || args[1] != ShareLinkStatusRevoked || args[2] != 50 {
		t.Fatalf("unfiltered args = %#v", args)
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
	if _, err := (*Service)(nil).ResolveShareLink(context.Background(), ResolveShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service ResolveShareLink err = %v, want repository rejection", err)
	}
	if _, err := (*Service)(nil).OpenSharedFile(context.Background(), ResolveShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service OpenSharedFile err = %v, want repository rejection", err)
	}
	if _, err := (*Service)(nil).StatSharedFile(context.Background(), ResolveShareLinkRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("service StatSharedFile err = %v, want repository rejection", err)
	}
}

func TestValidateCreateShareLinkRequestHashesPassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req, _, err := ValidateCreateShareLinkRequest(CreateShareLinkRequest{
		UserID:    "user-1",
		NodeID:    "node-1",
		Token:     strings.Repeat("a", 40),
		Password:  "open sesame",
		ExpiresAt: now.Add(time.Hour),
	}, now)
	if err != nil {
		t.Fatalf("ValidateCreateShareLinkRequest returned error: %v", err)
	}
	if req.Password == "" || req.Password == "open sesame" {
		t.Fatalf("password = %q, want stored hash", req.Password)
	}
	if !strings.HasPrefix(req.Password, "pbkdf2-sha256$") {
		t.Fatalf("password hash = %q, want pbkdf2-sha256", req.Password)
	}
}

func TestValidateCreateShareLinkRequestRejectsUnsafePassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	for _, password := range []string{strings.Repeat("a", MaxShareLinkPasswordBytes+1), "line\nbreak"} {
		password := password
		t.Run(password[:1], func(t *testing.T) {
			t.Parallel()
			_, _, err := ValidateCreateShareLinkRequest(CreateShareLinkRequest{
				UserID:    "user-1",
				NodeID:    "node-1",
				Token:     strings.Repeat("a", 40),
				Password:  password,
				ExpiresAt: now.Add(time.Hour),
			}, now)
			if err == nil {
				t.Fatal("ValidateCreateShareLinkRequest error = nil, want rejection")
			}
		})
	}
}
