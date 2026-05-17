package sso

import (
	"testing"
	"time"
)

func TestGenerateSAMLRequestID(t *testing.T) {
	id, err := GenerateSAMLRequestID()
	if err != nil {
		t.Fatalf("GenerateSAMLRequestID error: %v", err)
	}
	if id == "" {
		t.Fatal("GenerateSAMLRequestID returned empty id")
	}
	if len(id) < 16 {
		t.Fatalf("id too short: %d", len(id))
	}
}

func TestSAMLAuthnRequest(t *testing.T) {
	req := AuthnRequest{
		ID:           "_test123",
		IssueInstant: time.Now().UTC().Format(time.RFC3339),
		Destination:  "https://idp.example.com/sso",
		Issuer:       "https://sp.example.com",
		ACSURL:       "https://sp.example.com/acs",
	}

	xml, err := req.BuildXML()
	if err != nil {
		t.Fatalf("BuildXML error: %v", err)
	}
	if len(xml) == 0 {
		t.Fatal("BuildXML returned empty xml")
	}
	xmlStr := string(xml)
	if !contains(xmlStr, "AuthnRequest") {
		t.Fatal("xml missing AuthnRequest element")
	}
	if !contains(xmlStr, "_test123") {
		t.Fatal("xml missing ID attribute")
	}
}

func TestOIDCStateGeneration(t *testing.T) {
	state, err := GenerateOIDCState()
	if err != nil {
		t.Fatalf("GenerateOIDCState error: %v", err)
	}
	if state == "" {
		t.Fatal("GenerateOIDCState returned empty state")
	}
	if len(state) < 16 {
		t.Fatalf("state too short: %d", len(state))
	}
}

func TestOIDCVerifyState(t *testing.T) {
	state, _ := GenerateOIDCState()
	if !VerifyOIDCState(state, state) {
		t.Fatal("VerifyOIDCState should return true for matching states")
	}
	if VerifyOIDCState(state, "different") {
		t.Fatal("VerifyOIDCState should return false for non-matching states")
	}
}

func TestParseOIDCDiscovery(t *testing.T) {
	jsonData := `{
		"issuer": "https://idp.example.com",
		"authorization_endpoint": "https://idp.example.com/auth",
		"token_endpoint": "https://idp.example.com/token",
		"userinfo_endpoint": "https://idp.example.com/userinfo",
		"jwks_uri": "https://idp.example.com/jwks"
	}`

	doc, err := ParseOIDCDiscovery([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseOIDCDiscovery error: %v", err)
	}
	if doc.Issuer != "https://idp.example.com" {
		t.Fatalf("Issuer = %s, want https://idp.example.com", doc.Issuer)
	}
	if doc.AuthorizationEndpoint != "https://idp.example.com/auth" {
		t.Fatalf("AuthEndpoint = %s", doc.AuthorizationEndpoint)
	}
}

func TestGenerateOIDCStateForDomain(t *testing.T) {
	state, err := GenerateOIDCStateForDomain("my-domain-id")
	if err != nil {
		t.Fatalf("GenerateOIDCStateForDomain: %v", err)
	}
	domainID, err := ParseOIDCStateDomain(state)
	if err != nil {
		t.Fatalf("ParseOIDCStateDomain: %v", err)
	}
	if domainID != "my-domain-id" {
		t.Errorf("domainID = %q, want my-domain-id", domainID)
	}
}

func TestParseOIDCStateDomainInvalid(t *testing.T) {
	if _, err := ParseOIDCStateDomain("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestGenerateOIDCStateWithPKCE(t *testing.T) {
	state, verifier, err := GenerateOIDCStateWithPKCE("dom-pkce")
	if err != nil {
		t.Fatalf("GenerateOIDCStateWithPKCE: %v", err)
	}
	if verifier == "" {
		t.Fatal("expected non-empty code_verifier")
	}
	domainID, gotVerifier, err := ParseOIDCStateFields(state)
	if err != nil {
		t.Fatalf("ParseOIDCStateFields: %v", err)
	}
	if domainID != "dom-pkce" {
		t.Errorf("domainID = %q, want dom-pkce", domainID)
	}
	if gotVerifier != verifier {
		t.Errorf("code_verifier round-trip mismatch")
	}
}

func TestPKCEChallenge(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := PKCEChallenge(verifier)
	if got != want {
		t.Errorf("PKCEChallenge = %q, want %q", got, want)
	}
}

func TestPKCEChallengeNonEmpty(t *testing.T) {
	challenge := PKCEChallenge("some-verifier-string")
	if challenge == "" {
		t.Fatal("expected non-empty PKCE challenge")
	}
	if len(challenge) < 32 {
		t.Errorf("challenge too short: %d", len(challenge))
	}
}

func TestParseSAMLNameID(t *testing.T) {
	xml := []byte(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">` +
		`<saml:Assertion>` +
		`<saml:Subject><saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">alice@example.com</saml:NameID></saml:Subject>` +
		`</saml:Assertion>` +
		`</samlp:Response>`)
	email, err := ParseSAMLNameID(xml)
	if err != nil {
		t.Fatalf("ParseSAMLNameID: %v", err)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", email)
	}
}

func TestParseSAMLNameIDEmpty(t *testing.T) {
	xml := []byte(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">` +
		`<saml:Assertion><saml:Subject><saml:NameID></saml:NameID></saml:Subject></saml:Assertion>` +
		`</samlp:Response>`)
	if _, err := ParseSAMLNameID(xml); err == nil {
		t.Fatal("expected error for empty NameID")
	}
}

func TestParseIDTokenEmail(t *testing.T) {
	// payload: {"sub":"sub123","email":"user@example.com"}
	payload := "eyJzdWIiOiJzdWIxMjMiLCJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20ifQ"
	token := "eyJhbGciOiJIUzI1NiJ9." + payload + ".fakesig"
	email, err := ParseIDTokenEmail(token)
	if err != nil {
		t.Fatalf("ParseIDTokenEmail: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", email)
	}
}

func TestParseIDTokenEmailMissingClaim(t *testing.T) {
	// payload: {"sub":"sub123"} — no email field
	payload := "eyJzdWIiOiJzdWIxMjMifQ"
	token := "eyJhbGciOiJIUzI1NiJ9." + payload + ".fakesig"
	if _, err := ParseIDTokenEmail(token); err == nil {
		t.Fatal("expected error for missing email claim")
	}
}

func TestParseIDTokenEmailInvalidFormat(t *testing.T) {
	if _, err := ParseIDTokenEmail("notajwt"); err == nil {
		t.Fatal("expected error for non-JWT string")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
