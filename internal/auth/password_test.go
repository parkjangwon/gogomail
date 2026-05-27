package auth

import (
	"crypto/rand"
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

func TestValidatePasswordHashAcceptsSupportedFormats(t *testing.T) {
	t.Parallel()

	pbkdf2, err := HashPasswordPBKDF2SHA256("secret", []byte("1234567890123456"), 1_000)
	if err != nil {
		t.Fatalf("HashPasswordPBKDF2SHA256 returned error: %v", err)
	}
	sha := sha256.Sum256([]byte("pass"))
	for _, encoded := range []string{
		pbkdf2,
		"sha256:" + hex.EncodeToString(sha[:]),
		"plain:dev-password",
	} {
		if err := ValidatePasswordHash(encoded, true); err != nil {
			t.Fatalf("ValidatePasswordHash(%q) returned error: %v", encoded, err)
		}
	}
}

func TestValidatePasswordHashRejectsUnsafeFormats(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"plain:",
		"plain:pass\nbad",
		"sha256:" + strings.Repeat("a", legacySHA256DigestBytes-1),
		"sha256:" + strings.Repeat("z", legacySHA256DigestBytes),
		PBKDF2SHA256Prefix + "$1000001$" + base64.RawStdEncoding.EncodeToString([]byte("1234567890123456")) + "$" + base64.RawStdEncoding.EncodeToString([]byte("12345678901234567890123456789012")),
		"bcrypt:unsupported",
	}
	for _, encoded := range tests {
		encoded := encoded
		t.Run(encoded, func(t *testing.T) {
			t.Parallel()

			if err := ValidatePasswordHash(encoded, false); err == nil {
				t.Fatalf("ValidatePasswordHash(%q) returned nil", encoded)
			}
		})
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

func TestVerifyPasswordHashResult(t *testing.T) {
	cases := []struct {
		name        string
		password    string
		hash        string
		wantOK      bool
		wantUpgrade bool
	}{
		{"plain match", "secret", "plain:secret", true, true},
		{"plain mismatch", "wrong", "plain:secret", false, false},
		{"sha256 match", "secret", func() string {
			sum := sha256.Sum256([]byte("secret"))
			return "sha256:" + hex.EncodeToString(sum[:])
		}(), true, true},
		{"sha256 mismatch", "wrong", func() string {
			sum := sha256.Sum256([]byte("secret"))
			return "sha256:" + hex.EncodeToString(sum[:])
		}(), false, false},
		{"pbkdf2 no upgrade", "secret", func() string {
			salt := make([]byte, 16)
			rand.Read(salt)
			h, _ := HashPasswordPBKDF2SHA256("secret", salt, 210_000)
			return h
		}(), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, upgrade := VerifyPasswordHashResult(c.password, c.hash)
			if ok != c.wantOK {
				t.Errorf("ok: got %v want %v", ok, c.wantOK)
			}
			if upgrade != c.wantUpgrade {
				t.Errorf("upgrade: got %v want %v", upgrade, c.wantUpgrade)
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
