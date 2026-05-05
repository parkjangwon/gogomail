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

func TestRepositoryResolvePrincipalRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolvePrincipal(context.Background(), ResolvePrincipalRequest{ID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}
