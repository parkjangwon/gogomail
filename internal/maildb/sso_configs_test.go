package maildb

import "testing"

func TestGetSSOConfigNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.GetSSOConfig(nil, "domain-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestUpsertSSOConfigNilDB(t *testing.T) {
	r := &Repository{}
	err := r.UpsertSSOConfig(nil, SSOConfig{DomainID: "d1", Provider: "saml", SSOURL: "https://idp/sso"}) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestDeleteSSOConfigNilDB(t *testing.T) {
	r := &Repository{}
	err := r.DeleteSSOConfig(nil, "domain-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestValidateSSOConfigRequiresDomainID(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{Provider: "saml", SSOURL: "https://idp"})
	if err == nil {
		t.Fatal("expected error for missing domain_id")
	}
}

func TestValidateSSOConfigRequiresValidProvider(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{DomainID: "d1", Provider: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestValidateSSOConfigSAMLRequiresSSOURL(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{DomainID: "d1", Provider: "saml"})
	if err == nil {
		t.Fatal("expected error: SAML requires sso_url")
	}
}

func TestValidateSSOConfigOIDCRequiresClientID(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{DomainID: "d1", Provider: "oidc"})
	if err == nil {
		t.Fatal("expected error: OIDC requires client_id")
	}
}

func TestValidateSSOConfigValidSAML(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{DomainID: "d1", Provider: "saml", SSOURL: "https://idp/sso"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSSOConfigValidOIDC(t *testing.T) {
	err := ValidateSSOConfig(SSOConfig{DomainID: "d1", Provider: "oidc", ClientID: "client123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSOConfigSessionTTLSecondsField(t *testing.T) {
	cfg := SSOConfig{
		DomainID:          "d1",
		Provider:          "saml",
		SSOURL:            "https://idp/sso",
		SessionTTLSeconds: 3600,
	}
	if cfg.SessionTTLSeconds != 3600 {
		t.Errorf("SessionTTLSeconds = %d, want 3600", cfg.SessionTTLSeconds)
	}
	cfg2 := SSOConfig{DomainID: "d2", Provider: "oidc", ClientID: "c"}
	if cfg2.SessionTTLSeconds != 0 {
		t.Errorf("default SessionTTLSeconds = %d, want 0", cfg2.SessionTTLSeconds)
	}
}

func TestUpsertSSOConfigNilDBWithSessionTTL(t *testing.T) {
	r := &Repository{}
	err := r.UpsertSSOConfig(nil, SSOConfig{ //nolint:staticcheck
		DomainID:          "d1",
		Provider:          "saml",
		SSOURL:            "https://idp/sso",
		SessionTTLSeconds: 1800,
	})
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}
