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

	xml, err := req.MarshalXML()
	if err != nil {
		t.Fatalf("MarshalXML error: %v", err)
	}
	if len(xml) == 0 {
		t.Fatal("MarshalXML returned empty xml")
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
