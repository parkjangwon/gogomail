package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsUnknownSmartHostTLSMode(t *testing.T) {
	cfg := Load()
	cfg.DeliverySmartHost = "smtp.relay.example.net"
	cfg.DeliverySmartHostTLSMode = "tls-ish"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want smart-host TLS mode rejection")
	}
}

func TestValidateRejectsSmartHostOptionsWithoutHost(t *testing.T) {
	cfg := Load()
	cfg.DeliverySmartHost = ""
	cfg.DeliverySmartHostUsername = "relay-user"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing smart-host rejection")
	}
}

func TestValidateRejectsInvalidSmartHostPort(t *testing.T) {
	cfg := Load()
	cfg.DeliverySmartHost = "smtp.relay.example.net"
	cfg.DeliverySmartHostPort = 70000

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid smart-host port rejection")
	}
}

func TestValidateRejectsConflictingImplicitSmartHostTLS(t *testing.T) {
	cfg := Load()
	cfg.DeliverySmartHost = "smtp.relay.example.net"
	cfg.DeliverySmartHostImplicitTLS = true
	cfg.DeliverySmartHostTLSMode = "disable"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want implicit TLS conflict rejection")
	}
}

func TestValidateRejectsUnsafeSmartHostCredentials(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "password without username", edit: func(cfg *Config) { cfg.DeliverySmartHostPassword = "secret" }},
		{name: "username newline", edit: func(cfg *Config) { cfg.DeliverySmartHostUsername = "relay\nbad" }},
		{name: "username oversized", edit: func(cfg *Config) {
			cfg.DeliverySmartHostUsername = strings.Repeat("u", maxDeliverySmartHostCredentialBytes+1)
		}},
		{name: "password newline", edit: func(cfg *Config) {
			cfg.DeliverySmartHostUsername = "relay"
			cfg.DeliverySmartHostPassword = "secret\nbad"
		}},
		{name: "password oversized", edit: func(cfg *Config) {
			cfg.DeliverySmartHostUsername = "relay"
			cfg.DeliverySmartHostPassword = strings.Repeat("p", maxDeliverySmartHostCredentialBytes+1)
		}},
		{name: "identity newline", edit: func(cfg *Config) { cfg.DeliverySmartHostIdentity = "tenant\nbad" }},
		{name: "identity oversized", edit: func(cfg *Config) {
			cfg.DeliverySmartHostIdentity = strings.Repeat("i", maxDeliverySmartHostCredentialBytes+1)
		}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			cfg.DeliverySmartHost = "smtp.relay.example.net"
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unsafe smart-host credential rejection")
			}
		})
	}
}
