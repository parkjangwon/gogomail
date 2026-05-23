package maildb

import (
	"os"
	"strings"
	"testing"
)

func TestAuthUsernameLookupsUsePreNormalizedParameter(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"user_auth.go", "ldap_auth.go", "submission.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(source), "lower($1)") {
			t.Fatalf("%s still lowercases the username bind parameter in SQL", path)
		}
		if !strings.Contains(string(source), "lower(u.username) = $1") {
			t.Fatalf("%s does not use the normalized username expression predicate", path)
		}
	}
}

func TestAuthUsernameLookupIndexMigration(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("../../migrations/0151_user_username_lookup_index.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"idx_users_local_active_username_lower",
		"ON users (lower(username), id)",
		"WHERE status = 'active' AND auth_source = 'local'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}
