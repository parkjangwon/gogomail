package authmfa

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func GenerateSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

func GenerateTOTP(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}

	counter := uint64(t.Unix() / 30)

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	hmac := hmacSHA1(key, buf[:])
	offset := hmac[len(hmac)-1] & 0x0f

	code := binary.BigEndian.Uint32(hmac[offset:offset+4]) & 0x7fffffff
	code = code % 1000000

	return fmt.Sprintf("%06d", code), nil
}

func VerifyTOTP(secret string, code string, t time.Time) bool {
	if len(code) != 6 {
		return false
	}

	for i := -2; i <= 2; i++ {
		windowTime := t.Add(time.Duration(i) * time.Minute)
		expected, err := GenerateTOTP(secret, windowTime)
		if err != nil {
			continue
		}
		if expected == code {
			return true
		}
	}

	return false
}

func GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, 0, count)
	seen := make(map[string]bool)

	for len(codes) < count {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}

	return codes, nil
}

func VerifyRecoveryCode(codes []string, code string) ([]string, bool) {
	for i, c := range codes {
		if c == code {
			remaining := append([]string{}, codes[:i]...)
			remaining = append(remaining, codes[i+1:]...)
			return remaining, true
		}
	}
	return codes, false
}

func generateRecoveryCode() (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var code strings.Builder
	for i := 0; i < 10; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		code.WriteByte(chars[n.Int64()])
	}
	return code.String(), nil
}

func hmacSHA1(key, data []byte) []byte {
	blockSize := 64
	if len(key) > blockSize {
		h := sha1.Sum(key)
		key = h[:]
	}

	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	copy(ipad, key)
	copy(opad, key)

	for i := range ipad {
		ipad[i] ^= 0x36
		opad[i] ^= 0x5c
	}

	h := sha1.New()
	h.Write(ipad)
	h.Write(data)
	inner := h.Sum(nil)

	h = sha1.New()
	h.Write(opad)
	h.Write(inner)
	return h.Sum(nil)
}
