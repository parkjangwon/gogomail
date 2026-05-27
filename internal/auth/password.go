package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const PBKDF2SHA256Prefix = "pbkdf2-sha256"

const (
	maxPasswordHashBytes    = 4096
	maxPBKDF2Iterations     = 1_000_000
	maxPBKDF2SaltBytes      = 64
	maxPBKDF2SaltPartBytes  = 128
	maxPBKDF2KeyBytes       = 64
	maxPBKDF2KeyPartBytes   = 128
	legacySHA256DigestBytes = 64
)

func HashPasswordPBKDF2SHA256(password string, salt []byte, iterations int) (string, error) {
	if iterations <= 0 {
		iterations = 210_000
	}
	if iterations > maxPBKDF2Iterations {
		return "", fmt.Errorf("iterations must be <= %d", maxPBKDF2Iterations)
	}
	if len(salt) < 16 {
		return "", fmt.Errorf("salt must be at least 16 bytes")
	}
	if len(salt) > maxPBKDF2SaltBytes {
		return "", fmt.Errorf("salt must be <= %d bytes", maxPBKDF2SaltBytes)
	}
	key, err := derivePBKDF2SHA256(password, salt, iterations)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		PBKDF2SHA256Prefix,
		strconv.Itoa(iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

func VerifyPasswordHash(password string, encoded string) bool {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return false
	}
	if len(encoded) > maxPasswordHashBytes {
		return false
	}
	if strings.HasPrefix(encoded, "plain:") {
		return subtle.ConstantTimeCompare([]byte(password), []byte(strings.TrimPrefix(encoded, "plain:"))) == 1
	}
	if strings.HasPrefix(encoded, "sha256:") {
		digest := strings.TrimPrefix(encoded, "sha256:")
		if len(digest) != legacySHA256DigestBytes {
			return false
		}
		sum := sha256.Sum256([]byte(password))
		return hmac.Equal([]byte(hex.EncodeToString(sum[:])), []byte(digest))
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != PBKDF2SHA256Prefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > maxPBKDF2Iterations {
		return false
	}
	if len(parts[2]) > maxPBKDF2SaltPartBytes || len(parts[3]) > maxPBKDF2KeyPartBytes {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > maxPBKDF2SaltBytes {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 || len(want) > maxPBKDF2KeyBytes {
		return false
	}
	got, err := derivePBKDF2SHA256(password, salt, iterations)
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}

// ValidatePasswordHash validates the format of a password hash string.
// allowLegacy controls whether deprecated plain: and sha256: formats are accepted.
// Pass allowLegacy=false in production to prevent new credentials using weak hashes.
func ValidatePasswordHash(encoded string, allowLegacy bool) error {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return fmt.Errorf("password_hash is required")
	}
	if strings.ContainsAny(encoded, "\r\n") {
		return fmt.Errorf("password_hash must not contain CR or LF")
	}
	if len(encoded) > maxPasswordHashBytes {
		return fmt.Errorf("password_hash is too long")
	}
	if strings.HasPrefix(encoded, "plain:") {
		if !allowLegacy {
			return fmt.Errorf("plain: password_hash is not allowed in production; use pbkdf2-sha256")
		}
		if strings.TrimPrefix(encoded, "plain:") == "" {
			return fmt.Errorf("plain password_hash must include a password")
		}
		return nil
	}
	if strings.HasPrefix(encoded, "sha256:") {
		if !allowLegacy {
			return fmt.Errorf("sha256: password_hash is not allowed in production; use pbkdf2-sha256")
		}
		digest := strings.TrimPrefix(encoded, "sha256:")
		if len(digest) != legacySHA256DigestBytes {
			return fmt.Errorf("sha256 password_hash digest must be %d hex characters", legacySHA256DigestBytes)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf("sha256 password_hash digest must be hex")
		}
		return nil
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != PBKDF2SHA256Prefix {
		return fmt.Errorf("unsupported password_hash format")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > maxPBKDF2Iterations {
		return fmt.Errorf("password_hash iterations must be between 1 and %d", maxPBKDF2Iterations)
	}
	if len(parts[2]) > maxPBKDF2SaltPartBytes {
		return fmt.Errorf("password_hash salt is too long")
	}
	if len(parts[3]) > maxPBKDF2KeyPartBytes {
		return fmt.Errorf("password_hash key is too long")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > maxPBKDF2SaltBytes {
		return fmt.Errorf("password_hash salt is invalid")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(key) == 0 || len(key) > maxPBKDF2KeyBytes {
		return fmt.Errorf("password_hash key is invalid")
	}
	return nil
}

// GenerateSalt returns n cryptographically random bytes for use as a PBKDF2 salt.
func GenerateSalt(n int) []byte {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("auth: generate salt: %v", err))
	}
	return b
}

// VerifyPasswordHashResult is like VerifyPasswordHash but also returns needsUpgrade=true
// when the hash used a legacy format (plain: or sha256:).
// Callers should re-hash with PBKDF2-SHA256 when needsUpgrade is true.
func VerifyPasswordHashResult(password string, encoded string) (verified bool, needsUpgrade bool) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return false, false
	}
	isLegacy := strings.HasPrefix(encoded, "plain:") || strings.HasPrefix(encoded, "sha256:")
	ok := VerifyPasswordHash(password, encoded)
	if !ok {
		return false, false
	}
	return true, isLegacy
}

func derivePBKDF2SHA256(password string, salt []byte, iterations int) ([]byte, error) {
	return pbkdf2KeySHA256(password, salt, iterations, 32)
}
