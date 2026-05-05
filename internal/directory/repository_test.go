package directory

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeResolvePrincipalRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeResolvePrincipalRequest(ResolvePrincipalRequest{
		ID:         " user-1 ",
		Kind:       " USER ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeResolvePrincipalRequest returned error: %v", err)
	}
	if got.ID != "user-1" || got.Kind != PrincipalKindUser || !got.ActiveOnly {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizePrincipalKindRejectsUnsupportedKinds(t *testing.T) {
	t.Parallel()

	if _, err := NormalizePrincipalKind("calendar"); err == nil {
		t.Fatal("NormalizePrincipalKind accepted unsupported kind")
	}
}

func TestNormalizePrincipalKindAcceptsOrganizationPrincipals(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		" Organization ": PrincipalKindOrganization,
		" GROUP ":        PrincipalKindGroup,
		" Resource ":     PrincipalKindResource,
	}
	for value, want := range tests {
		value, want := value, want
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizePrincipalKind(value)
			if err != nil {
				t.Fatalf("NormalizePrincipalKind returned error: %v", err)
			}
			if got != want {
				t.Fatalf("kind = %q, want %q", got, want)
			}
		})
	}
}

func TestNormalizePrincipalIDRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"user\n1",
		strings.Repeat("x", MaxPrincipalIDBytes+1),
	}
	for _, value := range tests {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizePrincipalID(value); err == nil {
				t.Fatalf("NormalizePrincipalID(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestNormalizeResolveAliasRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeResolveAliasRequest(ResolveAliasRequest{
		Address:    " Ops@Example.COM ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeResolveAliasRequest returned error: %v", err)
	}
	if got.Address != "ops@example.com" || !got.ActiveOnly {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeResolveAliasRequestRejectsInvalidAddresses(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"not an address",
		"ops@example.com\nbcc@example.net",
		strings.Repeat("local", 90) + "@example.com",
	}
	for _, address := range tests {
		address := address
		t.Run(address, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeResolveAliasRequest(ResolveAliasRequest{Address: address}); err == nil {
				t.Fatalf("NormalizeResolveAliasRequest(%q) error = nil, want rejection", address)
			}
		})
	}
}

func TestRepositoryResolvePrincipalRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolvePrincipal(context.Background(), ResolvePrincipalRequest{ID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryResolveAliasRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolveAlias(context.Background(), ResolveAliasRequest{Address: "ops@example.com"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}
