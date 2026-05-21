package maildb

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestAuthenticateLDAPNilDB(t *testing.T) {
	r := &Repository{}
	ok, err := r.AuthenticateLDAP(context.Background(), "user@example.com", "pass")
	if err != nil {
		t.Fatalf("AuthenticateLDAP with nil db should not error, got %v", err)
	}
	if ok {
		t.Fatal("AuthenticateLDAP with nil db should return false, not panic")
	}
}

func TestAuthenticateLDAPUsesTypedUserIDFastPath(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("ldap_auth.go")
	if err != nil {
		t.Fatalf("read ldap_auth.go: %v", err)
	}
	if !strings.Contains(string(source), "u.id = $3::uuid") {
		t.Fatalf("AuthenticateLDAP query does not use typed user id predicate")
	}
	if strings.Contains(string(source), "u.id::text =") {
		t.Fatalf("AuthenticateLDAP query casts indexed user id column")
	}
}
