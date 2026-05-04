package apimeter

import "testing"

func TestIdentityNormalizeTrimsDimensions(t *testing.T) {
	t.Parallel()

	id := Identity{
		TenantID:    " tenant-1 ",
		CompanyID:   " company-1 ",
		DomainID:    " domain-1 ",
		UserID:      " user-1 ",
		APIKeyID:    " api-key-1 ",
		PrincipalID: " principal-1 ",
		AuthSource:  " bearer ",
	}.Normalize()

	if id.TenantID != "tenant-1" || id.CompanyID != "company-1" || id.DomainID != "domain-1" {
		t.Fatalf("identity dimensions were not trimmed: %+v", id)
	}
	if id.UserID != "user-1" || id.APIKeyID != "api-key-1" || id.PrincipalID != "principal-1" {
		t.Fatalf("identity principals were not trimmed: %+v", id)
	}
	if id.AuthSource != AuthSourceBearer {
		t.Fatalf("AuthSource = %q, want bearer", id.AuthSource)
	}
}

func TestIdentityNormalizeDefaultsAuthSource(t *testing.T) {
	t.Parallel()

	id := Identity{}.Normalize()
	if id.AuthSource != AuthSourceUnknown {
		t.Fatalf("AuthSource = %q, want unknown", id.AuthSource)
	}
}

func TestIdentityNormalizeRejectsUnknownAuthSource(t *testing.T) {
	t.Parallel()

	id := Identity{AuthSource: " mobile-app "}.Normalize()
	if id.AuthSource != AuthSourceUnknown {
		t.Fatalf("AuthSource = %q, want unknown", id.AuthSource)
	}
}

func TestIdentityNormalizeCanonicalizesKnownAuthSource(t *testing.T) {
	t.Parallel()

	id := Identity{AuthSource: " BEARER "}.Normalize()
	if id.AuthSource != AuthSourceBearer {
		t.Fatalf("AuthSource = %q, want bearer", id.AuthSource)
	}
}

func TestIdentityNormalizeDerivesPrincipalID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   Identity
		want string
	}{
		{name: "user", id: Identity{UserID: "user-1"}, want: "user-1"},
		{name: "api key", id: Identity{APIKeyID: "api-key-1"}, want: "api-key-1"},
		{name: "admin token", id: Identity{AuthSource: AuthSourceAdminToken}, want: AuthSourceAdminToken},
		{name: "anonymous", id: Identity{AuthSource: AuthSourceAnonymous}, want: AuthSourceAnonymous},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.id.Normalize().PrincipalID; got != tc.want {
				t.Fatalf("PrincipalID = %q, want %q", got, tc.want)
			}
		})
	}
}
