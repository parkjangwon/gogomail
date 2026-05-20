package sso

import (
	gocrypto "crypto"
	_ "crypto/sha256"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// buildRS256IDToken creates a JWT signed with the given RSA private key.
// kid is embedded in the header. claims must be a JSON-serialisable struct.
func buildRS256IDToken(t *testing.T, key *rsa.PrivateKey, kid string, claims any) string {
	t.Helper()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	payloadJSON, _ := json.Marshal(claims)

	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigInput := h + "." + p

	digest := sha256.Sum256([]byte(sigInput))
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, key, gocrypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return sigInput + "." + base64.RawURLEncoding.EncodeToString(sigBytes)
}

// buildJWKS builds a JWKS JSON document for a single RSA public key.
func buildJWKS(pub *rsa.PublicKey, kid string) []byte {
	nB64 := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eBytes := make([]byte, 4)
	eBytes[0] = byte(pub.E >> 24)
	eBytes[1] = byte(pub.E >> 16)
	eBytes[2] = byte(pub.E >> 8)
	eBytes[3] = byte(pub.E)
	// Trim leading zero bytes (per JWK spec).
	i := 0
	for i < len(eBytes)-1 && eBytes[i] == 0 {
		i++
	}
	eB64 := base64.RawURLEncoding.EncodeToString(eBytes[i:])

	return []byte(fmt.Sprintf(
		`{"keys":[{"kty":"RSA","kid":%q,"alg":"RS256","use":"sig","n":%q,"e":%q}]}`,
		kid, nB64, eB64,
	))
}

// startOIDCTestServer starts a httptest.Server that serves:
//   - GET /.well-known/openid-configuration → discovery doc pointing to /jwks
//   - GET /jwks → JWKS with the given key
func startOIDCTestServer(t *testing.T, pub *rsa.PublicKey, kid string) *httptest.Server {
	t.Helper()
	jwks := buildJWKS(pub, kid)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			issuer := "http://" + r.Host
			discovery := fmt.Sprintf(`{"issuer":%q,"jwks_uri":%q}`, issuer, issuer+"/jwks")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(discovery)) //nolint:errcheck
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			w.Write(jwks) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestVerifyAndParseIDTokenRS256Valid(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key-1"

	srv := startOIDCTestServer(t, &key.PublicKey, kid)
	defer srv.Close()

	// Clear cache to avoid interference between tests.
	jwksCache.Delete(srv.URL + "/jwks")

	issuer := srv.URL
	const exp2099 = 4070908800
	token := buildRS256IDToken(t, key, kid, map[string]any{
		"iss":   issuer,
		"sub":   "sub-rs256",
		"aud":   "client-abc",
		"email": "rs256user@example.com",
		"exp":   exp2099,
		"iat":   time.Now().Unix(),
	})

	email, err := VerifyAndParseIDToken(token, "", "client-abc", time.Now())
	if err != nil {
		t.Fatalf("VerifyAndParseIDToken RS256: %v", err)
	}
	if email != "rs256user@example.com" {
		t.Errorf("email = %q, want rs256user@example.com", email)
	}
}

func TestVerifyAndParseIDTokenRS256TamperedSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key-tamper"

	srv := startOIDCTestServer(t, &key.PublicKey, kid)
	defer srv.Close()
	jwksCache.Delete(srv.URL + "/jwks")

	issuer := srv.URL
	const exp2099 = 4070908800
	token := buildRS256IDToken(t, key, kid, map[string]any{
		"iss":   issuer,
		"sub":   "sub-tamper",
		"aud":   "client-abc",
		"email": "tamper@example.com",
		"exp":   exp2099,
		"iat":   time.Now().Unix(),
	})

	// Tamper: replace the payload with a different email.
	parts := strings.Split(token, ".")
	maliciousPayload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"iss":%q,"sub":"sub-attacker","aud":"client-abc","email":"admin@example.com","exp":%d}`, issuer, exp2099),
	))
	tamperedToken := parts[0] + "." + maliciousPayload + "." + parts[2]

	_, err = VerifyAndParseIDToken(tamperedToken, "", "client-abc", time.Now())
	if err == nil {
		t.Fatal("expected error for tampered RS256 token, got nil")
	}
	if !strings.Contains(err.Error(), "RS256") && !strings.Contains(err.Error(), "signature") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestVerifyAndParseIDTokenRS256WrongKey(t *testing.T) {
	// Signing key and served key are different.
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key-wrong"

	// Server serves wrongKey's public key.
	srv := startOIDCTestServer(t, &wrongKey.PublicKey, kid)
	defer srv.Close()
	jwksCache.Delete(srv.URL + "/jwks")

	issuer := srv.URL
	const exp2099 = 4070908800
	// Token is signed with signingKey, not wrongKey.
	token := buildRS256IDToken(t, signingKey, kid, map[string]any{
		"iss":   issuer,
		"sub":   "sub-wrong",
		"aud":   "client-abc",
		"email": "wrongkey@example.com",
		"exp":   exp2099,
		"iat":   time.Now().Unix(),
	})

	_, err = VerifyAndParseIDToken(token, "", "client-abc", time.Now())
	if err == nil {
		t.Fatal("expected error for RS256 token signed with wrong key")
	}
}

func TestVerifyAndParseIDTokenHS256StillWorks(t *testing.T) {
	const exp2099 = 4070908800
	secret := "my-client-secret"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"sub":"sub123","email":"hs256@example.com","iss":"https://idp.example.com","aud":"client123","exp":%d}`, exp2099),
	))
	sigInput := header + "." + payload
	sig := testHMACSHA256B64([]byte(secret), []byte(sigInput))
	token := sigInput + "." + sig

	email, err := VerifyAndParseIDToken(token, secret, "client123", time.Now())
	if err != nil {
		t.Fatalf("HS256 token verification: %v", err)
	}
	if email != "hs256@example.com" {
		t.Errorf("email = %q, want hs256@example.com", email)
	}
}

// testHMACSHA256B64 computes HMAC-SHA256 and returns base64url-encoded result.
func testHMACSHA256B64(key, data []byte) string {
	var ipad, opad [64]byte
	k := key
	if len(k) > 64 {
		s := sha256.Sum256(k)
		k = s[:]
	}
	copy(ipad[:], k)
	copy(opad[:], k)
	for i := range ipad {
		ipad[i] ^= 0x36
		opad[i] ^= 0x5c
	}
	inner := sha256.New()
	inner.Write(ipad[:])
	inner.Write(data)
	innerSum := inner.Sum(nil)
	outer := sha256.New()
	outer.Write(opad[:])
	outer.Write(innerSum)
	return base64.RawURLEncoding.EncodeToString(outer.Sum(nil))
}

func TestDecodeRSAPublicKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := buildJWKS(&key.PublicKey, "k1")
	var set jwkSet
	if err := json.Unmarshal(jwks, &set); err != nil {
		t.Fatalf("unmarshal JWKS: %v", err)
	}
	pub, err := decodeRSAPublicKey(set.Keys[0])
	if err != nil {
		t.Fatalf("decodeRSAPublicKey: %v", err)
	}
	if pub.E != key.PublicKey.E {
		t.Errorf("E = %d, want %d", pub.E, key.PublicKey.E)
	}
	if pub.N.Cmp(key.PublicKey.N) != 0 {
		t.Error("N mismatch")
	}
}

func TestSelectRSAKeyByKid(t *testing.T) {
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)
	_ = key2

	n1B64 := base64.RawURLEncoding.EncodeToString(key1.PublicKey.N.Bytes())
	eBytes := []byte{0x01, 0x00, 0x01} // 65537
	e1B64 := base64.RawURLEncoding.EncodeToString(eBytes)

	keys := []jwkKey{
		{Kid: "k1", Kty: "RSA", N: n1B64, E: e1B64},
	}

	pub, err := selectRSAKey(keys, "k1")
	if err != nil {
		t.Fatalf("selectRSAKey: %v", err)
	}
	if pub.N.Cmp(key1.PublicKey.N) != 0 {
		t.Error("wrong key selected")
	}
}

func TestSelectRSAKeyMissingKid(t *testing.T) {
	keys := []jwkKey{
		{Kid: "k1", Kty: "RSA", N: "abc", E: "AQAB"},
	}
	_, err := selectRSAKey(keys, "nonexistent-kid")
	if err == nil {
		t.Fatal("expected error for missing kid")
	}
}

// Ensure big.Int.Bytes() compatibility: test with a known modulus.
func TestBigIntModulusRoundTrip(t *testing.T) {
	n := new(big.Int)
	n.SetString("123456789012345678901234567890123456789012345678901234567890", 10)
	b64 := base64.RawURLEncoding.EncodeToString(n.Bytes())
	decoded, _ := base64.RawURLEncoding.DecodeString(b64)
	n2 := new(big.Int).SetBytes(decoded)
	if n.Cmp(n2) != 0 {
		t.Error("big.Int round-trip failed")
	}
}
