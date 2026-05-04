package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
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

func TestHashPasswordPBKDF2SHA256RejectsUnsafeCostAndSalt(t *testing.T) {
	t.Parallel()

	if _, err := HashPasswordPBKDF2SHA256("secret", []byte("1234567890123456"), maxPBKDF2Iterations+1); err == nil {
		t.Fatal("HashPasswordPBKDF2SHA256 accepted excessive iterations")
	}
	if _, err := HashPasswordPBKDF2SHA256("secret", []byte(strings.Repeat("s", maxPBKDF2SaltBytes+1)), 1_000); err == nil {
		t.Fatal("HashPasswordPBKDF2SHA256 accepted oversized salt")
	}
}

func TestVerifyPasswordHashRejectsUnsafePBKDF2Metadata(t *testing.T) {
	t.Parallel()

	validSalt := base64.RawStdEncoding.EncodeToString([]byte("1234567890123456"))
	validKey := base64.RawStdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))

	for _, tc := range []struct {
		name    string
		encoded string
	}{
		{
			name:    "whole_hash",
			encoded: strings.Repeat("x", maxPasswordHashBytes+1),
		},
		{
			name:    "iterations",
			encoded: PBKDF2SHA256Prefix + "$" + "1000001" + "$" + validSalt + "$" + validKey,
		},
		{
			name:    "salt_part",
			encoded: PBKDF2SHA256Prefix + "$1000$" + strings.Repeat("a", maxPBKDF2SaltPartBytes+1) + "$" + validKey,
		},
		{
			name:    "key_part",
			encoded: PBKDF2SHA256Prefix + "$1000$" + validSalt + "$" + strings.Repeat("b", maxPBKDF2KeyPartBytes+1),
		},
		{
			name:    "decoded_salt",
			encoded: PBKDF2SHA256Prefix + "$1000$" + base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("s", maxPBKDF2SaltBytes+1))) + "$" + validKey,
		},
		{
			name:    "decoded_key",
			encoded: PBKDF2SHA256Prefix + "$1000$" + validSalt + "$" + base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("k", maxPBKDF2KeyBytes+1))),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if VerifyPasswordHash("secret", tc.encoded) {
				t.Fatalf("VerifyPasswordHash accepted unsafe metadata for %s", tc.name)
			}
		})
	}
}

func TestVerifyPasswordHashRejectsMalformedLegacySHA256(t *testing.T) {
	t.Parallel()

	if VerifyPasswordHash("pass", "sha256:"+strings.Repeat("a", legacySHA256DigestBytes+1)) {
		t.Fatal("VerifyPasswordHash accepted oversized legacy sha256 digest")
	}
	if VerifyPasswordHash("pass", "sha256:"+strings.Repeat("a", legacySHA256DigestBytes-1)) {
		t.Fatal("VerifyPasswordHash accepted short legacy sha256 digest")
	}
}
