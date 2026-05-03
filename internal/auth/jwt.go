package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	Subject  string    `json:"sub"`
	UserID   string    `json:"user_id"`
	DomainID string    `json:"domain_id"`
	Role     string    `json:"role"`
	Expires  time.Time `json:"-"`
	Expiry   int64     `json:"exp"`
	IssuedAt int64     `json:"iat"`
}

type TokenManager struct {
	secret []byte
	now    func() time.Time
}

func NewTokenManager(secret string) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	return &TokenManager{secret: []byte(secret), now: time.Now}, nil
}

func (m *TokenManager) Sign(claims Claims, ttl time.Duration) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", fmt.Errorf("token manager is not configured")
	}
	now := m.now().UTC()
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if claims.UserID == "" && claims.Subject != "" {
		claims.UserID = claims.Subject
	}
	if claims.Subject == "" {
		claims.Subject = claims.UserID
	}
	if claims.Subject == "" {
		return "", fmt.Errorf("user_id is required")
	}
	claims.IssuedAt = now.Unix()
	claims.Expiry = now.Add(ttl).Unix()

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerRaw)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signingInput := encodedHeader + "." + encodedPayload
	signature := m.sign(signingInput)
	return signingInput + "." + signature, nil
}

func (m *TokenManager) Verify(token string) (Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return Claims{}, fmt.Errorf("token manager is not configured")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid jwt format")
	}
	signingInput := parts[0] + "." + parts[1]
	expected := m.sign(signingInput)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, fmt.Errorf("invalid jwt signature")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode jwt payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return Claims{}, fmt.Errorf("decode jwt claims: %w", err)
	}
	if claims.UserID == "" {
		claims.UserID = claims.Subject
	}
	if claims.UserID == "" {
		return Claims{}, fmt.Errorf("jwt missing user_id")
	}
	if claims.Expiry <= m.now().UTC().Unix() {
		return Claims{}, fmt.Errorf("jwt expired")
	}
	claims.Expires = time.Unix(claims.Expiry, 0).UTC()
	return claims, nil
}

func (m *TokenManager) sign(input string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
