package sso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// oidcFullClaims holds all standard OIDC ID token claims used for verification.
type oidcFullClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Email    string `json:"email"`
}

// VerifyAndParseIDToken validates an OIDC ID token's HMAC-SHA256 signature (when
// clientSecret is non-empty), verifies standard claims (exp, iat, aud), and
// returns the email address.
//
// Signature verification: only alg=HS256 with a clientSecret is supported.
// Tokens using RS256 (the OIDC norm) require JWKS-based verification; pass
// clientSecret="" to skip signature verification in those cases while still
// enforcing exp/iat/aud claims.
func VerifyAndParseIDToken(idToken, clientSecret, clientID string, now time.Time) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid ID token format")
	}

	// Decode and validate header.
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode ID token header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return "", fmt.Errorf("parse ID token header: %w", err)
	}

	// Verify HMAC-SHA256 signature when clientSecret is provided.
	if clientSecret != "" {
		if header.Alg != "HS256" {
			return "", fmt.Errorf("unsupported ID token algorithm %q: only HS256 supported with client secret", header.Alg)
		}
		signingInput := parts[0] + "." + parts[1]
		mac := hmac.New(sha256.New, []byte(clientSecret))
		mac.Write([]byte(signingInput)) //nolint:errcheck
		expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
			return "", fmt.Errorf("ID token signature verification failed")
		}
	}

	// Decode payload.
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode ID token payload: %w", err)
	}
	var claims oidcFullClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return "", fmt.Errorf("parse ID token claims: %w", err)
	}

	// Validate standard claims.
	if claims.Expiry == 0 || now.Unix() >= claims.Expiry {
		return "", fmt.Errorf("ID token expired or missing exp claim")
	}
	if claims.IssuedAt > 0 && now.Unix() < claims.IssuedAt-60 {
		return "", fmt.Errorf("ID token issued in the future")
	}
	if clientID != "" && claims.Audience != clientID {
		return "", fmt.Errorf("ID token audience mismatch: got %q, want %q", claims.Audience, clientID)
	}
	if claims.Email == "" {
		return "", fmt.Errorf("ID token missing email claim")
	}
	return claims.Email, nil
}

// SAMLMaxResponseBytes is the maximum allowed size of a SAML response payload.
// This prevents resource exhaustion from oversized XML documents.
const SAMLMaxResponseBytes = 512 * 1024
