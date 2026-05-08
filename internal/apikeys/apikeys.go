package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

const keyPrefix = "gm_"
const keyEntropyBytes = 32
const defaultTTL = 30 * 24 * time.Hour

func GenerateKey() (string, error) {
	b := make([]byte, keyEntropyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key entropy: %w", err)
	}
	return keyPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func VerifyKeyFormat(key string) bool {
	if !strings.HasPrefix(key, keyPrefix) {
		return false
	}
	if len(key) < len(keyPrefix)+8 {
		return false
	}
	return true
}

func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func CheckCIDR(ip net.IP, allowed []*net.IPNet) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, n := range allowed {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func IsKeyExpired(createdAt time.Time, now time.Time) bool {
	return now.Sub(createdAt) > defaultTTL
}
