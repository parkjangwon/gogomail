package config

import (
	"strings"
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

func TestValidateRejectsInvalidPushNotifyWebhookConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "missing url", mutate: func(cfg *Config) {
			cfg.PushNotifyBackend = "webhook"
			cfg.PushNotifyWebhookURL = ""
		}},
		{name: "bad url", mutate: func(cfg *Config) {
			cfg.PushNotifyBackend = "webhook"
			cfg.PushNotifyWebhookURL = "mailto:push@example.com"
		}},
		{name: "nonpositive timeout", mutate: func(cfg *Config) {
			cfg.PushNotifyWebhookTimeout = 0
		}},
		{name: "token newline", mutate: func(cfg *Config) {
			cfg.PushNotifyBackend = "webhook"
			cfg.PushNotifyWebhookURL = "http://push.example/send"
			cfg.PushNotifyWebhookToken = "bad\ntoken"
		}},
		{name: "token too long", mutate: func(cfg *Config) {
			cfg.PushNotifyBackend = "webhook"
			cfg.PushNotifyWebhookURL = "http://push.example/send"
			cfg.PushNotifyWebhookToken = strings.Repeat("t", maxWebhookTokenBytes+1)
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid push webhook config rejection")
			}
		})
	}
}

func TestValidateRejectsHTTPWebhooksInProduction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "attachment scanner", mutate: func(cfg *Config) {
			cfg.AttachmentScanBackend = "webhook"
			cfg.AttachmentScanWebhookURL = "http://scanner.example/scan"
		}},
		{name: "push notification", mutate: func(cfg *Config) {
			cfg.PushNotifyBackend = "webhook"
			cfg.PushNotifyWebhookURL = "http://push.example/send"
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			cfg.Environment = "production"
			cfg.SubmissionAllowInsecureAuth = false
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want production http webhook rejection")
			}
		})
	}
}

func TestValidateAcceptsHTTPSWebhooksInProduction(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.AttachmentScanBackend = "webhook"
	cfg.AttachmentScanWebhookURL = "https://scanner.example/scan"
	cfg.PushNotifyBackend = "webhook"
	cfg.PushNotifyWebhookURL = "https://push.example/send"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidAttachmentScanConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "unknown backend", mutate: func(cfg *Config) { cfg.AttachmentScanBackend = "clamd" }},
		{name: "missing webhook url", mutate: func(cfg *Config) {
			cfg.AttachmentScanBackend = "webhook"
			cfg.AttachmentScanWebhookURL = ""
		}},
		{name: "bad webhook url", mutate: func(cfg *Config) {
			cfg.AttachmentScanBackend = "webhook"
			cfg.AttachmentScanWebhookURL = "ftp://scanner.example/scan"
		}},
		{name: "nonpositive timeout", mutate: func(cfg *Config) { cfg.AttachmentScanTimeout = 0 }},
		{name: "token newline", mutate: func(cfg *Config) {
			cfg.AttachmentScanBackend = "webhook"
			cfg.AttachmentScanWebhookURL = "http://scanner.example/scan"
			cfg.AttachmentScanWebhookToken = "bad\ntoken"
		}},
		{name: "token too long", mutate: func(cfg *Config) {
			cfg.AttachmentScanBackend = "webhook"
			cfg.AttachmentScanWebhookURL = "http://scanner.example/scan"
			cfg.AttachmentScanWebhookToken = strings.Repeat("t", maxWebhookTokenBytes+1)
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid attachment scan config rejection")
			}
		})
	}
}

func TestValidateRejectsInvalidAttachmentCleanupConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "nonpositive interval", mutate: func(cfg *Config) { cfg.AttachmentCleanupInterval = 0 }},
		{name: "nonpositive stale age", mutate: func(cfg *Config) { cfg.AttachmentCleanupStaleAge = 0 }},
		{name: "nonpositive batch size", mutate: func(cfg *Config) { cfg.AttachmentCleanupBatchSize = 0 }},
		{name: "oversized batch size", mutate: func(cfg *Config) { cfg.AttachmentCleanupBatchSize = 1001 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid attachment cleanup config rejection")
			}
		})
	}
}

func TestValidateRejectsNonpositivePushNotificationConsumerSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "count", mutate: func(cfg *Config) { cfg.PushNotifyConsumerCount = 0 }},
		{name: "block", mutate: func(cfg *Config) { cfg.PushNotifyConsumerBlock = 0 }},
		{name: "device limit zero", mutate: func(cfg *Config) { cfg.PushNotifyDeviceLimit = 0 }},
		{name: "device limit too large", mutate: func(cfg *Config) { cfg.PushNotifyDeviceLimit = 201 }},
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

func TestValidateRejectsNonpositiveEventAndDeliveryConsumerSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "event count", mutate: func(cfg *Config) { cfg.EventConsumerCount = 0 }},
		{name: "event block", mutate: func(cfg *Config) { cfg.EventConsumerBlock = 0 }},
		{name: "delivery count", mutate: func(cfg *Config) { cfg.DeliveryConsumerCount = 0 }},
		{name: "delivery block", mutate: func(cfg *Config) { cfg.DeliveryConsumerBlock = 0 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want event or delivery consumer setting rejection")
			}
		})
	}
}

func TestValidateRejectsNegativeConsumerClaimIdle(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "event", mutate: func(cfg *Config) { cfg.EventConsumerClaimIdle = -time.Second }},
		{name: "search index", mutate: func(cfg *Config) { cfg.SearchIndexConsumerClaimIdle = -time.Second }},
		{name: "api metering", mutate: func(cfg *Config) { cfg.APIMeteringConsumerClaimIdle = -time.Second }},
		{name: "push notification", mutate: func(cfg *Config) { cfg.PushNotifyConsumerClaimIdle = -time.Second }},
		{name: "delivery", mutate: func(cfg *Config) { cfg.DeliveryConsumerClaimIdle = -time.Second }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want negative claim idle rejection")
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
