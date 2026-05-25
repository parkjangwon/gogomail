package webauthn_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/webauthn"
)

// TestNewServiceMissingRPID ensures that NewService returns an error when RPID is empty.
func TestNewServiceMissingRPID(t *testing.T) {
	_, err := webauthn.NewService(webauthn.Config{
		RPDisplayName: "Test",
		RPID:          "",
		RPOrigins:     []string{"https://example.com"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty RPID, got nil")
	}
	if !strings.Contains(err.Error(), "RPID") {
		t.Fatalf("expected RPID in error message, got: %s", err.Error())
	}
}

// TestServiceBeginRegistrationReturnsJSON ensures that BeginRegistration returns valid JSON
// with the expected publicKey field.
// Requires a DB — skip in -short mode.
func TestServiceBeginRegistrationReturnsJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	db := openTestDB(t)
	store := webauthn.NewStore(db)
	svc, err := webauthn.NewService(webauthn.Config{
		RPDisplayName: "GoGoMail Test",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost"},
	}, store)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()
	optionsJSON, err := svc.BeginRegistration(ctx, "user-id-1", "testuser", "Test User")
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(optionsJSON, &parsed); err != nil {
		t.Fatalf("BeginRegistration returned invalid JSON: %v", err)
	}
	if _, ok := parsed["publicKey"]; !ok {
		t.Fatalf("expected 'publicKey' in options JSON, got keys: %v", keys(parsed))
	}
}

// TestStoreCredentialRoundtrip saves and retrieves a credential.
// Requires a DB — skip in -short mode.
func TestStoreCredentialRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	db := openTestDB(t)
	store := webauthn.NewStore(db)
	ctx := context.Background()
	userID := mustCreateTestUser(t, db)

	cred := &webauthn.Credential{
		UserID:       userID,
		CredentialID: []byte("test-credential-id-bytes"),
		PublicKey:    []byte("fake-public-key-cbor"),
		AAGUID:       "",
		SignCount:    0,
		Name:         "My Security Key",
	}

	if err := store.SaveCredential(ctx, userID, cred); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	creds, err := store.GetCredentials(ctx, userID)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	got := creds[0]
	if got.Name != "My Security Key" {
		t.Errorf("Name mismatch: got %q", got.Name)
	}
	if string(got.CredentialID) != "test-credential-id-bytes" {
		t.Errorf("CredentialID mismatch")
	}
}

// TestChallengeGetAndDelete ensures challenges can be retrieved exactly once.
// Requires a DB — skip in -short mode.
func TestChallengeGetAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	db := openTestDB(t)
	store := webauthn.NewStore(db)
	ctx := context.Background()
	userID := mustCreateTestUser(t, db)

	challenge := []byte("random-challenge-bytes-xyz")
	if err := store.SaveChallenge(ctx, userID, "registration", challenge); err != nil {
		t.Fatalf("SaveChallenge: %v", err)
	}

	// First retrieval should succeed.
	got, err := store.GetAndDeleteChallenge(ctx, userID, "registration")
	if err != nil {
		t.Fatalf("GetAndDeleteChallenge (first): %v", err)
	}
	if string(got) != string(challenge) {
		t.Errorf("challenge mismatch: got %q, want %q", got, challenge)
	}

	// Second retrieval should fail (already deleted).
	_, err = store.GetAndDeleteChallenge(ctx, userID, "registration")
	if err == nil {
		t.Fatal("expected error on second GetAndDeleteChallenge, got nil")
	}
}

func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
