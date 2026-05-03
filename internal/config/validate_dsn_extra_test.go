package config

import "testing"

func TestValidateRejectsInvalidDSNPostmaster(t *testing.T) {
	cfg := Load()
	cfg.DSNPostmaster = "not an address"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid DSN postmaster error")
	}
}

func TestValidateAcceptsNamedDSNPostmaster(t *testing.T) {
	cfg := Load()
	cfg.DSNPostmaster = "Mail Delivery Subsystem <bounces@example.com>"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
