package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const PBKDF2SHA256Prefix = "pbkdf2-sha256"

func HashPasswordPBKDF2SHA256(password string, salt []byte, iterations int) (string, error) {
	if iterations <= 0 {
		iterations = 210_000
	}
	if len(salt) < 16 {
		return "", fmt.Errorf("salt must be at least 16 bytes")
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
	if strings.HasPrefix(encoded, "plain:") {
		return subtle.ConstantTimeCompare([]byte(password), []byte(strings.TrimPrefix(encoded, "plain:"))) == 1
	}
	if strings.HasPrefix(encoded, "sha256:") {
		sum := sha256.Sum256([]byte(password))
		return hmac.Equal([]byte(hex.EncodeToString(sum[:])), []byte(strings.TrimPrefix(encoded, "sha256:")))
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != PBKDF2SHA256Prefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := derivePBKDF2SHA256(password, salt, iterations)
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}

func derivePBKDF2SHA256(password string, salt []byte, iterations int) ([]byte, error) {
	return pbkdf2KeySHA256(password, salt, iterations, 32)
}
