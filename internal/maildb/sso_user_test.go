package maildb

import "testing"

func TestGetUserByEmailNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.GetUserByEmail(nil, "user@example.com") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestJITCreateSSOUserNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.JITCreateSSOUser(nil, "user@example.com", "domain-1", "") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestJITCreateSSOUserInvalidEmail(t *testing.T) {
	r := &Repository{}
	_, err := r.JITCreateSSOUser(nil, "not-an-email", "domain-1", "") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for missing @ in email")
	}
}
