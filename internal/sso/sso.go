package sso

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
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

// GenerateOIDCStateForDomain generates a state parameter that encodes the domainID
// so the callback handler can identify the tenant without a server-side session.
func GenerateOIDCStateForDomain(domainID string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate OIDC state: %w", err)
	}
	raw := domainID + "|" + hex.EncodeToString(b)
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// ParseOIDCStateDomain extracts the domainID encoded by GenerateOIDCStateForDomain.
func ParseOIDCStateDomain(state string) (string, error) {
	domainID, _, err := ParseOIDCStateFields(state)
	return domainID, err
}

// GenerateOIDCStateWithPKCE generates a state that encodes both the domainID and a
// PKCE code_verifier (RFC 7636). The code_verifier is returned separately so the
// caller can compute the code_challenge for the authorization request.
func GenerateOIDCStateWithPKCE(domainID string) (state, codeVerifier string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	codeVerifier = base64.RawURLEncoding.EncodeToString(b)
	raw := domainID + "|" + codeVerifier
	state = base64.RawURLEncoding.EncodeToString([]byte(raw))
	return state, codeVerifier, nil
}

// ParseOIDCStateFields decodes a state produced by either GenerateOIDCStateForDomain
// or GenerateOIDCStateWithPKCE and returns the domainID and optional codeVerifier.
// When the state was created without PKCE, codeVerifier is empty.
func ParseOIDCStateFields(state string) (domainID, codeVerifier string, err error) {
	b, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", "", fmt.Errorf("invalid OIDC state")
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid OIDC state format")
	}
	domainID = parts[0]
	if len(parts) == 2 {
		codeVerifier = parts[1]
	}
	return domainID, codeVerifier, nil
}

// PKCEChallenge computes the S256 code_challenge from a code_verifier per RFC 7636.
// code_challenge = BASE64URL(SHA256(ASCII(code_verifier)))
func PKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// samlXMLResponse holds the minimal subset of a SAML Response needed to extract NameID.
type samlXMLResponse struct {
	Assertion samlXMLAssertion `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
}

type samlXMLAssertion struct {
	Subject samlXMLSubject `xml:"urn:oasis:names:tc:SAML:2.0:assertion Subject"`
}

type samlXMLSubject struct {
	NameID samlXMLNameID `xml:"urn:oasis:names:tc:SAML:2.0:assertion NameID"`
}

type samlXMLNameID struct {
	Value string `xml:",chardata"`
}

// ParseSAMLNameID extracts the NameID value from a SAML Response XML document.
// The NameID is expected to contain the user's email address.
func ParseSAMLNameID(xmlData []byte) (string, error) {
	var resp samlXMLResponse
	if err := xml.Unmarshal(xmlData, &resp); err != nil {
		return "", fmt.Errorf("parse SAML response: %w", err)
	}
	email := strings.TrimSpace(resp.Assertion.Subject.NameID.Value)
	if email == "" {
		return "", fmt.Errorf("SAML NameID is empty")
	}
	return email, nil
}

// oidcTokenPayload holds the fields we care about in an OIDC ID token payload.
type oidcTokenPayload struct {
	Email string `json:"email"`
	Sub   string `json:"sub"`
}

// ParseIDTokenEmail decodes an OIDC ID token (JWT) and returns the email claim.
// It does NOT verify the signature — signature verification requires JWKS and
// is left to a future hardening task.
func ParseIDTokenEmail(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid ID token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode ID token payload: %w", err)
	}
	var claims oidcTokenPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse ID token claims: %w", err)
	}
	if claims.Email == "" {
		return "", fmt.Errorf("ID token missing email claim")
	}
	return claims.Email, nil
}
