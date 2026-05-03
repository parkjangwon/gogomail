package config

import "testing"

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
