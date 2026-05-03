//go:build go1.25

package auth

import (
	"crypto/pbkdf2"
	"crypto/sha256"
)

func pbkdf2KeySHA256(password string, salt []byte, iterations int, keyLength int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
}
