package idprovider

import "testing"

func TestRedactSecretsReplacesSetSecretsWithIndicator(t *testing.T) {
	cfg := &Config{
		DomainID:     "d1",
		ProviderType: "ldap",
		Settings: map[string]interface{}{
			"host":          "ldap.example.com",
			"bind_password": "s3cret",
			"client_secret": "", // empty -> omitted
		},
	}
	got := RedactSecrets(cfg)

	if got.Settings["host"] != "ldap.example.com" {
		t.Fatalf("non-secret field mutated: %v", got.Settings["host"])
	}
	if got.Settings["bind_password"] != SecretSetIndicator {
		t.Fatalf("bind_password not redacted to indicator: %v", got.Settings["bind_password"])
	}
	if _, present := got.Settings["client_secret"]; present {
		t.Fatalf("empty secret should be omitted, got %v", got.Settings["client_secret"])
	}
	// Original must be untouched (no plaintext leak via aliasing).
	if cfg.Settings["bind_password"] != "s3cret" {
		t.Fatalf("original config was mutated: %v", cfg.Settings["bind_password"])
	}
}

func TestRedactSecretsNilSafe(t *testing.T) {
	if RedactSecrets(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestMergeSecretsPreservesStoredOnBlankOrIndicator(t *testing.T) {
	stored := &Config{Settings: map[string]interface{}{
		"bind_password": "stored-pw",
		"client_secret": "stored-cs",
		"dsn":           "stored-dsn",
	}}
	incoming := &Config{Settings: map[string]interface{}{
		"bind_password": "",                 // blank -> keep stored
		"client_secret": SecretSetIndicator, // sentinel -> keep stored
		// dsn absent -> keep stored
	}}

	MergeSecrets(incoming, stored)

	if incoming.Settings["bind_password"] != "stored-pw" {
		t.Fatalf("blank secret should preserve stored, got %v", incoming.Settings["bind_password"])
	}
	if incoming.Settings["client_secret"] != "stored-cs" {
		t.Fatalf("sentinel secret should preserve stored, got %v", incoming.Settings["client_secret"])
	}
	if incoming.Settings["dsn"] != "stored-dsn" {
		t.Fatalf("absent secret should preserve stored, got %v", incoming.Settings["dsn"])
	}
}

func TestMergeSecretsAcceptsNewValue(t *testing.T) {
	stored := &Config{Settings: map[string]interface{}{"bind_password": "old-pw"}}
	incoming := &Config{Settings: map[string]interface{}{"bind_password": "new-pw"}}

	MergeSecrets(incoming, stored)

	if incoming.Settings["bind_password"] != "new-pw" {
		t.Fatalf("new secret should win, got %v", incoming.Settings["bind_password"])
	}
}

func TestMergeSecretsDropsSentinelWhenNoStored(t *testing.T) {
	incoming := &Config{Settings: map[string]interface{}{"client_secret": SecretSetIndicator}}

	MergeSecrets(incoming, nil)

	if _, present := incoming.Settings["client_secret"]; present {
		t.Fatalf("sentinel with no stored secret should be dropped, got %v", incoming.Settings["client_secret"])
	}
}

func TestMergeSecretsNilIncomingNoPanic(t *testing.T) {
	MergeSecrets(nil, &Config{Settings: map[string]interface{}{"dsn": "x"}})
}
