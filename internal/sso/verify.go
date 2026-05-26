package sso

import (
	gocrypto "crypto"
	_ "crypto/sha256" // register SHA256 hash
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// oidcRequestTimeout is the timeout for outbound OIDC discovery and JWKS fetches.
const oidcRequestTimeout = 15 * time.Second

// oidcHTTPClient is used for all OIDC discovery and JWKS fetches.
// 15-second timeout matches the per-request context timeout already set by callers.
var oidcHTTPClient = &http.Client{
	Timeout: oidcRequestTimeout,
}

// oidcFullClaims holds all standard OIDC ID token claims used for verification.
type oidcFullClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Email    string `json:"email"`
}

// jwksCache caches JWKS responses keyed by JWKS URI.
// Entries expire after jwksCacheTTL.
var (
	jwksCache    sync.Map // key: jwksURI (string) → *jwksCacheEntry
	jwksCacheTTL = 10 * time.Minute
)

type jwksCacheEntry struct {
	keys      []jwkKey
	fetchedAt time.Time
}

// jwkKey represents a single JWK (JSON Web Key) for RSA keys.
type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"` // Base64url-encoded modulus
	E   string `json:"e"` // Base64url-encoded exponent
}

type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

// oidcDiscoveryMini is a minimal OIDC discovery document for JWKS lookup.
type oidcDiscoveryMini struct {
	JWKSURI string `json:"jwks_uri"`
}

// VerifyAndParseIDToken validates an OIDC ID token and returns the email address.
//
// Signature verification behaviour:
//   - clientSecret != "": HS256 HMAC verification with the shared secret.
//   - clientSecret == "" and alg == "RS256": JWKS-based RSA verification using
//     the issuer's discovery document to locate the JWKS endpoint.
//   - clientSecret == "" and alg != "RS256": rejected — only RS256 is permitted
//     when no client secret is configured. alg=none is never accepted.
//
// In all cases the standard claims (exp, iat, aud, email) are validated.
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
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return "", fmt.Errorf("parse ID token header: %w", err)
	}

	// Decode payload first so we can access the issuer for RS256 discovery.
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode ID token payload: %w", err)
	}
	var claims oidcFullClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return "", fmt.Errorf("parse ID token claims: %w", err)
	}

	switch {
	case clientSecret != "":
		// HS256 path: verify HMAC-SHA256 signature with shared client secret.
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

	case header.Alg == "RS256":
		// RS256 path: verify RSA-SHA256 signature via JWKS discovery.
		if claims.Issuer == "" {
			return "", fmt.Errorf("ID token missing issuer claim (required for RS256 verification)")
		}
		if err := verifyRS256JWT(parts, header.Kid, claims.Issuer); err != nil {
			return "", fmt.Errorf("RS256 ID token signature verification failed: %w", err)
		}

	default:
		// No clientSecret and non-RS256 alg: reject. alg=none and other
		// non-RS256 algorithms are never permitted without a client secret.
		return "", fmt.Errorf("unsupported ID token algorithm %q: only RS256 is accepted without a client secret", header.Alg)
	}

	// Validate standard claims.
	if claims.Expiry == 0 || now.Unix() >= claims.Expiry {
		return "", fmt.Errorf("ID token expired or missing exp claim")
	}
	if claims.IssuedAt > 0 && now.Unix() < claims.IssuedAt-60 {
		return "", fmt.Errorf("ID token issued in the future")
	}
	if clientID != "" {
		if claims.Audience != clientID {
			return "", fmt.Errorf("ID token audience mismatch: got %q, want %q", claims.Audience, clientID)
		}
	} else if claims.Audience != "" {
		// clientID not configured but token carries an audience claim: reject to
		// prevent accepting tokens intended for a different relying party.
		return "", fmt.Errorf("ID token has audience claim %q but no clientID is configured for validation", claims.Audience)
	}
	if claims.Email == "" {
		return "", fmt.Errorf("ID token missing email claim")
	}
	return claims.Email, nil
}

// verifyRS256JWT verifies the RSA-SHA256 signature of a JWT against the IdP's
// published JWKS. parts must be the three base64url segments of the JWT.
// kid is the key ID from the JWT header (may be empty).
// issuer is used to construct the OIDC discovery URL.
func verifyRS256JWT(parts []string, kid, issuer string) error {
	jwksURI, err := fetchJWKSURI(issuer)
	if err != nil {
		return fmt.Errorf("fetch JWKS URI for issuer %q: %w", issuer, err)
	}

	pub, err := fetchRSAPublicKey(jwksURI, kid)
	if err != nil {
		// Invalidate cache and retry once in case the key was rotated.
		jwksCache.Delete(jwksURI)
		pub, err = fetchRSAPublicKey(jwksURI, kid)
		if err != nil {
			return fmt.Errorf("fetch RSA public key (kid=%q): %w", kid, err)
		}
	}

	// Verify RSA-SHA256 signature.
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decode JWT signature: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(pub, gocrypto.SHA256, digest[:], sigBytes); err != nil {
		return fmt.Errorf("RSA signature mismatch: %w", err)
	}
	return nil
}

// fetchJWKSURI retrieves the jwks_uri from the OIDC discovery document at
// issuer + "/.well-known/openid-configuration".
func fetchJWKSURI(issuer string) (string, error) {
	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	ctx, cancel := context.WithTimeout(context.Background(), oidcRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("build discovery request: %w", err)
	}
	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discovery document returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read discovery document: %w", err)
	}
	var doc oidcDiscoveryMini
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("parse discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return "", fmt.Errorf("discovery document missing jwks_uri")
	}
	return doc.JWKSURI, nil
}

// fetchRSAPublicKey retrieves the RSA public key matching kid from the JWKS
// endpoint at jwksURI, using a 10-minute in-process cache.
func fetchRSAPublicKey(jwksURI, kid string) (*rsa.PublicKey, error) {
	keys, err := getJWKSKeys(jwksURI)
	if err != nil {
		return nil, err
	}
	return selectRSAKey(keys, kid)
}

// getJWKSKeys returns the cached JWKS keys, fetching them if the cache is
// empty or stale.
func getJWKSKeys(jwksURI string) ([]jwkKey, error) {
	now := time.Now()
	if v, ok := jwksCache.Load(jwksURI); ok {
		entry := v.(*jwksCacheEntry)
		if now.Sub(entry.fetchedAt) < jwksCacheTTL {
			return entry.keys, nil
		}
	}

	// Fetch fresh JWKS.
	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), oidcRequestTimeout)
	defer fetchCancel()
	jwksReq, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("build JWKS request: %w", err)
	}
	resp, err := oidcHTTPClient.Do(jwksReq)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read JWKS: %w", err)
	}
	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("parse JWKS: %w", err)
	}

	entry := &jwksCacheEntry{keys: set.Keys, fetchedAt: now}
	jwksCache.Store(jwksURI, entry)
	return set.Keys, nil
}

// selectRSAKey finds the RSA key matching kid in the JWKS key set.
// When kid is empty, the first RSA key is returned.
func selectRSAKey(keys []jwkKey, kid string) (*rsa.PublicKey, error) {
	for _, k := range keys {
		if k.Kty != "RSA" {
			continue
		}
		if kid != "" && k.Kid != kid {
			continue
		}
		return decodeRSAPublicKey(k)
	}
	if kid != "" {
		return nil, fmt.Errorf("no RSA key with kid=%q found in JWKS", kid)
	}
	return nil, fmt.Errorf("no RSA key found in JWKS")
}

// decodeRSAPublicKey converts a JWK RSA entry into an *rsa.PublicKey.
func decodeRSAPublicKey(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode JWK modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode JWK exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	eInt := 0
	for _, b := range eBytes {
		eInt = eInt<<8 | int(b)
	}
	if eInt == 0 {
		return nil, fmt.Errorf("invalid JWK exponent (zero)")
	}
	return &rsa.PublicKey{N: n, E: eInt}, nil
}

// SAMLMaxResponseBytes is the maximum allowed size of a SAML response payload.
// This prevents resource exhaustion from oversized XML documents.
const SAMLMaxResponseBytes = 512 * 1024
