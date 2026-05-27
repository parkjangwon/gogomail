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

	"github.com/golang-jwt/jwt/v5"
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

// Claims holds the parsed, validated fields of a GoGoMail JWT.
type Claims struct {
	Subject        string    `json:"sub"`
	UserID         string    `json:"user_id"`
	DomainID       string    `json:"domain_id"`
	CompanyID      string    `json:"company_id,omitempty"`
	Role           string    `json:"role"`
	SessionVersion int64     `json:"session_ver,omitempty"`
	TokenType      string    `json:"token_type,omitempty"`
	MFAVerified    bool      `json:"mfa_verified,omitempty"`
	Expires        time.Time `json:"-"`
	Expiry         int64     `json:"exp"`
	IssuedAt       int64     `json:"iat"`
}

// jwtInternalClaims is the wire format used with golang-jwt/jwt/v5.
type jwtInternalClaims struct {
	UserID         string `json:"user_id"`
	DomainID       string `json:"domain_id"`
	CompanyID      string `json:"company_id,omitempty"`
	Role           string `json:"role"`
	SessionVersion int64  `json:"session_ver,omitempty"`
	TokenType      string `json:"token_type,omitempty"`
	MFAVerified    bool   `json:"mfa_verified,omitempty"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies GoGoMail JWTs.
type TokenManager struct {
	secret  []byte
	now     func() time.Time
	checker RevocationChecker
}

func (m *TokenManager) SetRevocationChecker(c RevocationChecker) {
	m.checker = c
}

// VerifyFull validates signature + expiry, then checks session_version against the
// RevocationChecker if one is configured.
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
	if len([]byte(secret)) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes")
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

	internal := jwtInternalClaims{
		UserID:         claims.UserID,
		DomainID:       claims.DomainID,
		CompanyID:      claims.CompanyID,
		Role:           claims.Role,
		SessionVersion: claims.SessionVersion,
		TokenType:      claims.TokenType,
		MFAVerified:    claims.MFAVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   claims.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, internal)
	return token.SignedString(m.secret)
}

func (m *TokenManager) Verify(tokenString string) (Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return Claims{}, fmt.Errorf("token manager is not configured")
	}
	tokenString = strings.TrimSpace(tokenString)
	if len(tokenString) > maxJWTTokenBytes {
		return Claims{}, fmt.Errorf("jwt token is too long")
	}

	// Segment-size checks before any decoding.
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid jwt format")
	}
	if err := validateJWTSegments(parts); err != nil {
		return Claims{}, err
	}

	// Decode header to check typ only. Algorithm is validated by golang-jwt's
	// key function below; duplicating the check here would be redundant.
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	var header struct {
		Type string `json:"typ"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return Claims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	if header.Type != "" && !strings.EqualFold(header.Type, "JWT") {
		return Claims{}, fmt.Errorf("unsupported jwt type")
	}

	// Decode payload to check issued-at before handing off to golang-jwt.
	// golang-jwt v5 does not enforce iat-in-future by default.
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode jwt payload: %w", err)
	}
	var rawClaims struct {
		IssuedAt int64 `json:"iat"`
	}
	if err := json.Unmarshal(payloadRaw, &rawClaims); err != nil {
		return Claims{}, fmt.Errorf("decode jwt claims: %w", err)
	}
	if rawClaims.IssuedAt > 0 && rawClaims.IssuedAt > m.now().UTC().Add(time.Minute).Unix() {
		return Claims{}, fmt.Errorf("jwt issued_at is in the future")
	}

	// Use golang-jwt for signature + expiry validation with the mocked clock.
	var internal jwtInternalClaims
	token, err := jwt.ParseWithClaims(tokenString, &internal, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unsupported jwt algorithm: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithTimeFunc(m.now), jwt.WithLeeway(0))
	if err != nil {
		return Claims{}, fmt.Errorf("invalid jwt: %w", err)
	}
	if !token.Valid {
		return Claims{}, fmt.Errorf("invalid jwt")
	}

	userID, err := normalizeJWTIdentity(internal.UserID)
	if err != nil {
		return Claims{}, err
	}
	subject, err := normalizeJWTIdentity(internal.Subject)
	if err != nil {
		return Claims{}, err
	}
	if userID == "" {
		userID = subject
	}
	if userID == "" {
		return Claims{}, fmt.Errorf("jwt missing user_id")
	}

	expiry := int64(0)
	var expires time.Time
	if internal.ExpiresAt != nil {
		expiry = internal.ExpiresAt.Unix()
		expires = internal.ExpiresAt.Time
	}
	issuedAt := int64(0)
	if internal.IssuedAt != nil {
		issuedAt = internal.IssuedAt.Unix()
	}

	return Claims{
		Subject:        subject,
		UserID:         userID,
		DomainID:       internal.DomainID,
		CompanyID:      internal.CompanyID,
		Role:           internal.Role,
		SessionVersion: internal.SessionVersion,
		TokenType:      internal.TokenType,
		MFAVerified:    internal.MFAVerified,
		Expires:        expires,
		Expiry:         expiry,
		IssuedAt:       issuedAt,
	}, nil
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

// sign computes the HMAC-SHA256 signature for a JWT signing input.
// Kept for test helpers that craft raw tokens.
func (m *TokenManager) sign(input string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// MFAMode represents the multi-factor authentication enforcement level.
type MFAMode string

const (
	MFAModeDisabled MFAMode = "disabled"
	MFAModeOptional MFAMode = "optional"
	MFAModeRequired MFAMode = "required"
)

func (m MFAMode) IsValid() bool {
	switch m {
	case MFAModeDisabled, MFAModeOptional, MFAModeRequired:
		return true
	}
	return false
}
