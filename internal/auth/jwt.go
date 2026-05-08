package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RevocationChecker lets the TokenManager validate session_version on every request.
type RevocationChecker interface {
	SessionVersionFor(ctx context.Context, userID string) (int64, error)
}

// SessionRevoker increments a user's session_version, invalidating all existing tokens.
type SessionRevoker interface {
	IncrementSessionVersion(ctx context.Context, userID string) (int64, error)
}

const (
	maxJWTTokenBytes            = 8192
	maxJWTHeaderSegmentBytes    = 1024
	maxJWTPayloadSegmentBytes   = 4096
	maxJWTSignatureSegmentBytes = 512
	maxJWTIdentityBytes         = 200
)

type Claims struct {
	Subject        string    `json:"sub"`
	UserID         string    `json:"user_id"`
	DomainID       string    `json:"domain_id"`
	Role           string    `json:"role"`
	SessionVersion int64     `json:"session_ver,omitempty"`
	MFAVerified    bool      `json:"mfa_verified,omitempty"`
	Expires        time.Time `json:"-"`
	Expiry         int64     `json:"exp"`
	IssuedAt       int64     `json:"iat"`
}

type TokenManager struct {
	secret  []byte
	now     func() time.Time
	checker RevocationChecker
}

func (m *TokenManager) SetRevocationChecker(c RevocationChecker) {
	m.checker = c
}

// VerifyFull validates the token signature and expiry, then checks session_version
// against the RevocationChecker if one is configured. Use this on every authenticated
// request so that RevokeAllSessions takes effect immediately.
func (m *TokenManager) VerifyFull(ctx context.Context, token string) (Claims, error) {
	claims, err := m.Verify(token)
	if err != nil {
		return Claims{}, err
	}
	if m.checker != nil {
		ver, err := m.checker.SessionVersionFor(ctx, claims.UserID)
		if err != nil {
			return Claims{}, fmt.Errorf("session check: %w", err)
		}
		if claims.SessionVersion < ver {
			return Claims{}, fmt.Errorf("session revoked")
		}
	}
	return claims, nil
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
	var err error
	claims.UserID, err = normalizeJWTIdentity(claims.UserID)
	if err != nil {
		return "", err
	}
	claims.Subject, err = normalizeJWTIdentity(claims.Subject)
	if err != nil {
		return "", err
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
	token = strings.TrimSpace(token)
	if len(token) > maxJWTTokenBytes {
		return Claims{}, fmt.Errorf("jwt token is too long")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid jwt format")
	}
	if err := validateJWTSegments(parts); err != nil {
		return Claims{}, err
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return Claims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	if header.Algorithm != "HS256" {
		return Claims{}, fmt.Errorf("unsupported jwt algorithm")
	}
	if header.Type != "" && !strings.EqualFold(header.Type, "JWT") {
		return Claims{}, fmt.Errorf("unsupported jwt type")
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
	claims.UserID, err = normalizeJWTIdentity(claims.UserID)
	if err != nil {
		return Claims{}, err
	}
	claims.Subject, err = normalizeJWTIdentity(claims.Subject)
	if err != nil {
		return Claims{}, err
	}
	if claims.UserID == "" {
		claims.UserID = claims.Subject
	}
	if claims.UserID == "" {
		return Claims{}, fmt.Errorf("jwt missing user_id")
	}
	if claims.IssuedAt > 0 && claims.IssuedAt > m.now().UTC().Add(time.Minute).Unix() {
		return Claims{}, fmt.Errorf("jwt issued_at is in the future")
	}
	if claims.Expiry <= m.now().UTC().Unix() {
		return Claims{}, fmt.Errorf("jwt expired")
	}
	claims.Expires = time.Unix(claims.Expiry, 0).UTC()
	return claims, nil
}

func validateJWTSegments(parts []string) error {
	if len(parts[0]) > maxJWTHeaderSegmentBytes {
		return fmt.Errorf("jwt header is too long")
	}
	if len(parts[1]) > maxJWTPayloadSegmentBytes {
		return fmt.Errorf("jwt payload is too long")
	}
	if len(parts[2]) > maxJWTSignatureSegmentBytes {
		return fmt.Errorf("jwt signature is too long")
	}
	return nil
}

func normalizeJWTIdentity(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("jwt identity must not contain CR or LF")
	}
	if len(value) > maxJWTIdentityBytes {
		return "", fmt.Errorf("jwt identity is too long")
	}
	return value, nil
}

func (m *TokenManager) sign(input string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// MFAMode represents the multi-factor authentication enforcement level.
type MFAMode string

const (
	MFAModeDisabled  MFAMode = "disabled"
	MFAModeOptional  MFAMode = "optional"
	MFAModeRequired  MFAMode = "required"
)

func (m MFAMode) IsValid() bool {
	switch m {
	case MFAModeDisabled, MFAModeOptional, MFAModeRequired:
		return true
	}
	return false
}
