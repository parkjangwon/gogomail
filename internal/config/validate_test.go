package config

import (
	"testing"
	"time"
)

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

func TestValidateRejectsUnknownPushNotifyBackend(t *testing.T) {
	cfg := Load()
	cfg.PushNotifyBackend = "fcm-direct"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown push notification backend rejection")
	}
}

func TestValidateRejectsNonpositivePushNotificationConsumerSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "count", mutate: func(cfg *Config) { cfg.PushNotifyConsumerCount = 0 }},
		{name: "block", mutate: func(cfg *Config) { cfg.PushNotifyConsumerBlock = 0 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want push notification consumer setting rejection")
			}
		})
	}
}

func TestValidateRejectsUnknownAPIMeteringAggregateBackend(t *testing.T) {
	cfg := Load()
	cfg.APIMeteringAggregateBackend = "warehouse-ish"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown api metering aggregate backend rejection")
	}
}

func TestValidateRejectsNonpositiveAPIMeteringConsumerSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "count", mutate: func(cfg *Config) { cfg.APIMeteringConsumerCount = 0 }},
		{name: "block", mutate: func(cfg *Config) { cfg.APIMeteringConsumerBlock = 0 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want api metering consumer setting rejection")
			}
		})
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

func TestValidateRejectsSMTPSWithoutTLSFiles(t *testing.T) {
	cfg := Load()
	cfg.SubmissionSMTPSAddr = ":2465"
	cfg.SMTPTLSCertFile = ""
	cfg.SMTPTLSKeyFile = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want SMTPS TLS file rejection")
	}
}

func TestValidateRejectsNonpositiveTimeouts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "smtp read", mutate: func(cfg *Config) { cfg.SMTPReadTimeout = 0 }},
		{name: "smtp write", mutate: func(cfg *Config) { cfg.SMTPWriteTimeout = -time.Second }},
		{name: "delivery", mutate: func(cfg *Config) { cfg.DeliveryTimeout = 0 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want timeout rejection")
			}
		})
	}
}

func TestValidateRejectsNonpositiveDKIMVerificationLimit(t *testing.T) {
	cfg := Load()
	cfg.SMTPMaxDKIMVerifications = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want DKIM verification limit rejection")
	}
}

func TestValidateAcceptsDefaultConfig(t *testing.T) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
