package sso

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func GenerateSAMLRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate SAML ID: %w", err)
	}
	return "_" + hex.EncodeToString(b), nil
}

type AuthnRequest struct {
	ID           string
	IssueInstant string
	Destination  string
	Issuer       string
	ACSURL       string
}

func (r *AuthnRequest) MarshalXML() ([]byte, error) {
	xml := fmt.Sprintf(`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" `+
		`xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" `+
		`ID="%s" Version="2.0" IssueInstant="%s" Destination="%s">`+
		`<saml:Issuer>%s</saml:Issuer>`+
		`<samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" AllowCreate="true"/>`+
		`</samlp:AuthnRequest>`,
		r.ID, r.IssueInstant, r.Destination, r.Issuer)
	return []byte(xml), nil
}

func GenerateOIDCState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate OIDC state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func VerifyOIDCState(expected, received string) bool {
	if expected == "" || received == "" {
		return false
	}
	return subtleConstantTimeCompare(expected, received)
}

func subtleConstantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	v := 0
	for i := 0; i < len(a); i++ {
		v |= int(a[i] ^ b[i])
	}
	return v == 0
}

type OIDCDiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func ParseOIDCDiscovery(data []byte) (*OIDCDiscoveryDocument, error) {
	var doc OIDCDiscoveryDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse OIDC discovery: %w", err)
	}
	if doc.Issuer == "" {
		return nil, fmt.Errorf("missing issuer in discovery document")
	}
	return &doc, nil
}

type IDToken struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
}

func (t *IDToken) Verify(issuer, clientID string, now time.Time) error {
	if t.Issuer != issuer {
		return fmt.Errorf("invalid issuer: %s", t.Issuer)
	}
	if t.Audience != clientID {
		return fmt.Errorf("invalid audience: %s", t.Audience)
	}
	if now.Unix() > t.Expiry {
		return fmt.Errorf("token expired")
	}
	if now.Unix() < t.IssuedAt {
		return fmt.Errorf("token issued in the future")
	}
	return nil
}

func HashNonce(nonce string) string {
	h := sha256.Sum256([]byte(nonce))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
