package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyPasswordHashPBKDF2SHA256(t *testing.T) {
	t.Parallel()

	encoded, err := HashPasswordPBKDF2SHA256("secret", []byte("1234567890123456"), 1_000)
	if err != nil {
		t.Fatalf("HashPasswordPBKDF2SHA256 returned error: %v", err)
	}
	if !VerifyPasswordHash("secret", encoded) {
		t.Fatal("VerifyPasswordHash rejected correct password")
	}
	if VerifyPasswordHash("wrong", encoded) {
		t.Fatal("VerifyPasswordHash accepted wrong password")
	}
}

func TestVerifyPasswordHashSupportsExplicitPlainDevPrefix(t *testing.T) {
	t.Parallel()

	if !VerifyPasswordHash("pass", "plain:pass") {
		t.Fatal("VerifyPasswordHash rejected explicit plain dev hash")
	}
	if VerifyPasswordHash("pass", "pass") {
		t.Fatal("VerifyPasswordHash accepted unprefixed plaintext")
	}
}

func TestVerifyPasswordHashSupportsLegacySHA256(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("pass"))
	encoded := "sha256:" + hex.EncodeToString(sum[:])
	if !VerifyPasswordHash("pass", encoded) {
		t.Fatal("VerifyPasswordHash rejected sha256 hash")
	}
}
