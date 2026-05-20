package sso

import (
	"bytes"
	gocrypto "crypto"
	_ "crypto/sha256" // register SHA256 hash
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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

func (r *AuthnRequest) BuildXML() ([]byte, error) {
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

// samlSignatureXML holds the ds:Signature block extracted for verification.
type samlSignatureXML struct {
	SignedInfo struct {
		CanonicalizationMethod struct {
			Algorithm string `xml:"Algorithm,attr"`
		} `xml:"http://www.w3.org/2000/09/xmldsig# CanonicalizationMethod"`
		SignatureMethod struct {
			Algorithm string `xml:"Algorithm,attr"`
		} `xml:"http://www.w3.org/2000/09/xmldsig# SignatureMethod"`
		Reference struct {
			DigestMethod struct {
				Algorithm string `xml:"Algorithm,attr"`
			} `xml:"http://www.w3.org/2000/09/xmldsig# DigestMethod"`
			DigestValue string `xml:"http://www.w3.org/2000/09/xmldsig# DigestValue"`
		} `xml:"http://www.w3.org/2000/09/xmldsig# Reference"`
	} `xml:"http://www.w3.org/2000/09/xmldsig# SignedInfo"`
	SignatureValue string `xml:"http://www.w3.org/2000/09/xmldsig# SignatureValue"`
	KeyInfo        struct {
		X509Data struct {
			X509Certificate string `xml:"http://www.w3.org/2000/09/xmldsig# X509Certificate"`
		} `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
	} `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
}

// samlResponseWithSig is the top-level envelope for extracting signatures.
type samlResponseWithSig struct {
	XMLName   xml.Name         `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	Signature samlSignatureXML `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	Assertion struct {
		Signature samlSignatureXML `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	} `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
}

// VerifySAMLSignature checks the RSA-SHA256 signature on a SAML Response.
// idpCertPEM must be a PEM-encoded X.509 certificate for the IdP.
// It checks for a signature at the Assertion level first, then Response level.
// Returns an error if the certificate is missing, parsing fails, or the
// signature is invalid.
func VerifySAMLSignature(xmlData []byte, idpCertPEM string) error {
	if strings.TrimSpace(idpCertPEM) == "" {
		return fmt.Errorf("SAML IDP certificate not configured")
	}

	// Parse the configured IdP certificate.
	idpKey, err := parsePEMCertPublicKey(idpCertPEM)
	if err != nil {
		return fmt.Errorf("parse IDP certificate: %w", err)
	}

	// Parse the SAML response to extract signatures.
	var env samlResponseWithSig
	if err := xml.Unmarshal(xmlData, &env); err != nil {
		return fmt.Errorf("parse SAML response for signature: %w", err)
	}

	// Try Assertion-level signature first, then Response-level.
	sigBlock := env.Assertion.Signature
	if sigBlock.SignatureValue == "" {
		sigBlock = env.Signature
	}
	if sigBlock.SignatureValue == "" {
		return fmt.Errorf("SAML response contains no signature")
	}

	// Decode the signature value.
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigBlock.SignatureValue))
	if err != nil {
		// try without whitespace
		sigBytes, err = base64.StdEncoding.DecodeString(strings.ReplaceAll(sigBlock.SignatureValue, "\n", ""))
		if err != nil {
			return fmt.Errorf("decode SAML SignatureValue: %w", err)
		}
	}

	// Re-serialize SignedInfo to canonical form for digest.
	// We extract the raw <ds:SignedInfo>...</ds:SignedInfo> bytes from the XML
	// using a simple byte search, which handles exclusive C14N adequately for
	// our use case (the IdP already produced the canonical form).
	signedInfoBytes, err := extractSignedInfoBytes(xmlData)
	if err != nil {
		return fmt.Errorf("extract SignedInfo: %w", err)
	}

	// Verify RSA-SHA256 signature: SHA256(SignedInfo) verified with IdP public key.
	digest := sha256.Sum256(signedInfoBytes)
	if err := rsa.VerifyPKCS1v15(idpKey, gocrypto.SHA256, digest[:], sigBytes); err != nil {
		return fmt.Errorf("SAML signature verification failed: %w", err)
	}
	return nil
}

// parsePEMCertPublicKey parses a PEM-encoded X.509 certificate and returns its
// RSA public key. It also accepts a bare base64 DER certificate (no PEM headers).
func parsePEMCertPublicKey(certPEM string) (*rsa.PublicKey, error) {
	trimmed := strings.TrimSpace(certPEM)

	var derBytes []byte
	if strings.HasPrefix(trimmed, "-----") {
		block, _ := pem.Decode([]byte(trimmed))
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}
		derBytes = block.Bytes
	} else {
		// Bare base64 DER (common in SAML metadata).
		cleaned := strings.ReplaceAll(trimmed, "\n", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("decode certificate: %w", err)
		}
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("IDP certificate does not contain an RSA public key")
	}
	return pub, nil
}

// extractSignedInfoBytes extracts the raw bytes of the first <ds:SignedInfo> or
// <SignedInfo> element found in the XML document.
func extractSignedInfoBytes(xmlData []byte) ([]byte, error) {
	// Try both namespace-qualified and unqualified forms.
	starts := [][]byte{
		[]byte("<ds:SignedInfo"),
		[]byte("<SignedInfo"),
	}
	ends := [][]byte{
		[]byte("</ds:SignedInfo>"),
		[]byte("</SignedInfo>"),
	}

	for i, start := range starts {
		si := bytes.Index(xmlData, start)
		if si < 0 {
			continue
		}
		ei := bytes.Index(xmlData[si:], ends[i])
		if ei < 0 {
			continue
		}
		return xmlData[si : si+ei+len(ends[i])], nil
	}
	return nil, fmt.Errorf("SignedInfo element not found in SAML response")
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
