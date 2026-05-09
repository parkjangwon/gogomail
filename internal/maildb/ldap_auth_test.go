package maildb

import (
	"context"
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
