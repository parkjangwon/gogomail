package config

import "testing"

func TestValidateRejectsProductionInsecureSubmissionAuth(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production insecure auth rejection")
	}
}

func TestValidateRejectsUnknownMetricsBackend(t *testing.T) {
	cfg := Load()
	cfg.MetricsBackend = "promish"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown metrics backend rejection")
	}
}

func TestValidateRejectsThrottleWithoutLimits(t *testing.T) {
	cfg := Load()
	cfg.DeliveryThrottleEnabled = true
	cfg.DeliveryDefaultConcurrency = 0
	cfg.DeliveryFarmConcurrency = nil
	cfg.DeliveryDomainConcurrency = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing throttle limits rejection")
	}
}

func TestValidateAcceptsDefaultConfig(t *testing.T) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
