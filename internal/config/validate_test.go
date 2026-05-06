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

func TestValidateRejectsProductionInsecureIMAPAuth(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.IMAPAllowInsecureAuth = true
	cfg.CalDAVAllowInsecureAuth = false
	cfg.CardDAVAllowInsecureAuth = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production insecure IMAP auth rejection")
	}
}

func TestValidateRejectsProductionInsecureCalDAVAuth(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.IMAPAllowInsecureAuth = false
	cfg.CalDAVAllowInsecureAuth = true
	cfg.CardDAVAllowInsecureAuth = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production insecure CalDAV auth rejection")
	}
}

func TestValidateRejectsProductionInsecureCardDAVAuth(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.IMAPAllowInsecureAuth = false
	cfg.CalDAVAllowInsecureAuth = false
	cfg.CardDAVAllowInsecureAuth = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production insecure CardDAV auth rejection")
	}
}

func TestValidateRejectsUnknownEnvironment(t *testing.T) {
	for _, env := range []string{"prod", "staging", ""} {
		env := env
		t.Run(env, func(t *testing.T) {
			cfg := Load()
			cfg.Environment = env
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unknown environment rejection")
			}
		})
	}
}

func TestValidateAcceptsKnownEnvironmentValues(t *testing.T) {
	for _, env := range []string{"development", " test ", "Production"} {
		env := env
		t.Run(env, func(t *testing.T) {
			cfg := Load()
			cfg.Environment = env
			cfg.SubmissionAllowInsecureAuth = false
			cfg.IMAPAllowInsecureAuth = false
			cfg.CalDAVAllowInsecureAuth = false
			cfg.CardDAVAllowInsecureAuth = false
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
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

func TestValidateRejectsUnknownStorageBackend(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "swift"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown storage backend rejection")
	}
}

func TestValidateAcceptsStorageBackendCompatLabels(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Endpoint = "http://localhost:9000"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	cfg.StorageBackendCompatLabels = []string{" local ", "MINIO"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsUnknownStorageBackendCompatLabel(t *testing.T) {
	cfg := Load()
	cfg.StorageBackendCompatLabels = []string{"swift"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown storage backend compatibility label rejection")
	}
}

func TestValidateRejectsUnsafeLocalStorageRoot(t *testing.T) {
	for _, root := range []string{"", "   ", "var/mailstore\nbad"} {
		root := root
		t.Run(root, func(t *testing.T) {
			cfg := Load()
			cfg.StorageBackend = "local"
			cfg.MailstoreRoot = root
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unsafe local storage root rejection")
			}
		})
	}
}

func TestValidateAcceptsS3StorageBackend(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Endpoint = "http://localhost:9000"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "minioadmin"
	cfg.StorageS3SecretAccessKey = "minioadmin"
	cfg.StorageS3ForcePathStyle = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresExplicitS3EndpointInProduction(t *testing.T) {
	cfg := Load()
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.IMAPAllowInsecureAuth = false
	cfg.CalDAVAllowInsecureAuth = false
	cfg.CardDAVAllowInsecureAuth = false
	cfg.StorageBackend = "s3"
	cfg.StorageS3Endpoint = ""
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail-prod"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "GOGOMAIL_STORAGE_S3_ENDPOINT is required in production") {
		t.Fatalf("Validate() error = %v, want production S3 endpoint rejection", err)
	}

	cfg.StorageS3Endpoint = "https://s3.us-east-1.amazonaws.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error with explicit production S3 endpoint = %v", err)
	}
}

func TestValidateAcceptsMinIOStorageBackend(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "minio"
	cfg.StorageS3Endpoint = "http://localhost:9000"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "minioadmin"
	cfg.StorageS3SecretAccessKey = "minioadmin"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsIncompleteS3StorageBackend(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want incomplete S3 config rejection")
	}
}

func TestValidateRejectsS3CredentialWhitespace(t *testing.T) {
	for _, backend := range []string{"s3", "minio"} {
		backend := backend
		for _, tt := range []struct {
			name   string
			mutate func(*Config)
		}{
			{name: "access key leading space", mutate: func(cfg *Config) { cfg.StorageS3AccessKeyID = " access" }},
			{name: "access key trailing space", mutate: func(cfg *Config) { cfg.StorageS3AccessKeyID = "access " }},
			{name: "access key tab", mutate: func(cfg *Config) { cfg.StorageS3AccessKeyID = "access\tkey" }},
			{name: "secret leading space", mutate: func(cfg *Config) { cfg.StorageS3SecretAccessKey = " secret" }},
			{name: "secret trailing space", mutate: func(cfg *Config) { cfg.StorageS3SecretAccessKey = "secret " }},
			{name: "secret tab", mutate: func(cfg *Config) { cfg.StorageS3SecretAccessKey = "secret\tvalue" }},
			{name: "session leading space", mutate: func(cfg *Config) { cfg.StorageS3SessionToken = " token" }},
			{name: "session trailing space", mutate: func(cfg *Config) { cfg.StorageS3SessionToken = "token " }},
			{name: "session tab", mutate: func(cfg *Config) { cfg.StorageS3SessionToken = "token\tvalue" }},
		} {
			tt := tt
			t.Run(backend+" "+tt.name, func(t *testing.T) {
				cfg := Load()
				cfg.StorageBackend = backend
				cfg.StorageS3Endpoint = "http://localhost:9000"
				cfg.StorageS3Region = "us-east-1"
				cfg.StorageS3Bucket = "gogomail"
				cfg.StorageS3AccessKeyID = "access"
				cfg.StorageS3SecretAccessKey = "secret"
				tt.mutate(&cfg)
				if err := cfg.Validate(); err == nil {
					t.Fatal("Validate() error = nil, want S3 credential whitespace rejection")
				}
			})
		}
	}
}

func TestValidateRejectsUnsafeS3BucketName(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "GoGoMail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe S3 bucket rejection")
	}
}

func TestValidateRejectsAmbiguousS3Endpoint(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Endpoint = "http://localhost:9000/proxy//s3"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want ambiguous S3 endpoint rejection")
	}
}

func TestValidateRejectsReservedS3BucketName(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail--x-s3"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want reserved S3 bucket rejection")
	}
}

func TestValidateRejectsUnsafeS3Region(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Region = "US-EAST-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe S3 region rejection")
	}
}

func TestValidateRejectsUnsafeS3Prefix(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "s3"
	cfg.StorageS3Region = "us-east-1"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3Prefix = "mail//objects"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe S3 prefix rejection")
	}
}

func TestValidateRejectsMinIOWithoutEndpoint(t *testing.T) {
	cfg := Load()
	cfg.StorageBackend = "minio"
	cfg.StorageS3Bucket = "gogomail"
	cfg.StorageS3AccessKeyID = "access"
	cfg.StorageS3SecretAccessKey = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want MinIO endpoint rejection")
	}
}

func TestValidateRejectsUnknownRedisFeatureBackends(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "dedup", mutate: func(cfg *Config) { cfg.DedupBackend = "redsi" }},
		{name: "rate limit", mutate: func(cfg *Config) { cfg.RateLimitBackend = "redsi" }},
		{name: "drive share rate limit", mutate: func(cfg *Config) { cfg.DriveShareRateLimitBackend = "redsi" }},
		{name: "backpressure", mutate: func(cfg *Config) { cfg.BackpressureBackend = "redsi" }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unknown redis feature backend rejection")
			}
		})
	}
}

func TestValidateRejectsNonpositiveRelayOperationalLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "rcpt rate limit", mutate: func(cfg *Config) { cfg.RcptRateLimitPerMinute = 0 }},
		{name: "drive share rate limit", mutate: func(cfg *Config) { cfg.DriveShareRateLimitPerMinute = 0 }},
		{name: "outbox batch size", mutate: func(cfg *Config) { cfg.OutboxRelayBatchSize = 0 }},
		{name: "outbox poll interval", mutate: func(cfg *Config) { cfg.OutboxRelayPollInterval = -time.Second }},
		{name: "outbox max attempts", mutate: func(cfg *Config) { cfg.OutboxRelayMaxAttempts = 0 }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want relay operational limit rejection")
			}
		})
	}
}

func TestValidateAcceptsRedisFeatureBackends(t *testing.T) {
	cfg := Load()
	cfg.DedupBackend = "redis"
	cfg.RateLimitBackend = " redis "
	cfg.BackpressureBackend = "Redis"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidListenerAddresses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "http empty", mutate: func(cfg *Config) { cfg.HTTPAddr = "" }},
		{name: "smtp missing port", mutate: func(cfg *Config) { cfg.SMTPAddr = "localhost" }},
		{name: "inbound nonnumeric port", mutate: func(cfg *Config) { cfg.InboundSMTPAddr = "127.0.0.1:notaport" }},
		{name: "imap empty", mutate: func(cfg *Config) { cfg.IMAPAddr = "" }},
		{name: "imap newline", mutate: func(cfg *Config) { cfg.IMAPAddr = ":1143\nbad" }},
		{name: "caldav missing port", mutate: func(cfg *Config) { cfg.CalDAVAddr = "localhost" }},
		{name: "caldav newline", mutate: func(cfg *Config) { cfg.CalDAVAddr = ":8081\nbad" }},
		{name: "carddav missing port", mutate: func(cfg *Config) { cfg.CardDAVAddr = "localhost" }},
		{name: "carddav newline", mutate: func(cfg *Config) { cfg.CardDAVAddr = ":8082\nbad" }},
		{name: "imap tls cert newline", mutate: func(cfg *Config) { cfg.IMAPTLSCertFile = "cert.pem\nbad" }},
		{name: "submission port too high", mutate: func(cfg *Config) { cfg.SubmissionAddr = "127.0.0.1:70000" }},
		{name: "smtps optional invalid", mutate: func(cfg *Config) { cfg.SubmissionSMTPSAddr = "bad" }},
		{name: "newline", mutate: func(cfg *Config) { cfg.HTTPAddr = ":8080\nbad" }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want listener address rejection")
			}
		})
	}
}

func TestValidateAcceptsListenerAddressForms(t *testing.T) {
	cfg := Load()
	cfg.HTTPAddr = "[::1]:8080"
	cfg.SMTPAddr = ":2525"
	cfg.InboundSMTPAddr = "127.0.0.1:2526"
	cfg.IMAPAddr = "localhost:1143"
	cfg.CalDAVAddr = "localhost:8081"
	cfg.IMAPTLSCertFile = "imap-cert.pem"
	cfg.IMAPTLSKeyFile = "imap-key.pem"
	cfg.SubmissionAddr = "localhost:2587"
	cfg.SubmissionSMTPSAddr = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	cfg.SubmissionSMTPSAddr = "[::1]:465"
	cfg.SMTPTLSCertFile = "cert.pem"
	cfg.SMTPTLSKeyFile = "key.pem"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error with SMTPS addr = %v", err)
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
			cfg.IMAPAllowInsecureAuth = false
			cfg.CalDAVAllowInsecureAuth = false
			cfg.CardDAVAllowInsecureAuth = false
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
	cfg.IMAPAllowInsecureAuth = false
	cfg.CalDAVAllowInsecureAuth = false
	cfg.CardDAVAllowInsecureAuth = false
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

func TestValidateRejectsInvalidDriveCleanupConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "nonpositive interval", mutate: func(cfg *Config) { cfg.DriveCleanupInterval = 0 }},
		{name: "nonpositive batch size", mutate: func(cfg *Config) { cfg.DriveCleanupBatchSize = 0 }},
		{name: "oversized batch size", mutate: func(cfg *Config) { cfg.DriveCleanupBatchSize = maxDriveCleanupBatchSize + 1 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid drive cleanup config rejection")
			}
		})
	}
}

func TestValidateRejectsInvalidDAVSyncRetentionSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "nonpositive interval", mutate: func(cfg *Config) { cfg.DAVSyncRetentionInterval = 0 }},
		{name: "nonpositive cutoff age", mutate: func(cfg *Config) { cfg.DAVSyncRetentionCutoffAge = 0 }},
		{name: "nonpositive batch size", mutate: func(cfg *Config) { cfg.DAVSyncRetentionBatchSize = 0 }},
		{name: "oversized batch size", mutate: func(cfg *Config) { cfg.DAVSyncRetentionBatchSize = maxDAVSyncRetentionBatchSize + 1 }},
		{name: "destructive without confirm", mutate: func(cfg *Config) {
			cfg.DAVSyncRetentionDryRun = false
			cfg.DAVSyncRetentionConfirmReady = false
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid DAV sync retention config rejection")
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
		{name: "max deliveries", mutate: func(cfg *Config) { cfg.APIMeteringConsumerMaxDeliveries = -1 }},
		{name: "dead-letter stream newline", mutate: func(cfg *Config) { cfg.APIMeteringConsumerDeadLetterStream = "api.event\nbad" }},
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

func TestValidateRejectsInvalidAPIUsageRetentionSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "interval", mutate: func(cfg *Config) { cfg.APIUsageRetentionInterval = 0 }},
		{name: "cutoff age", mutate: func(cfg *Config) { cfg.APIUsageRetentionCutoffAge = 0 }},
		{name: "batch size zero", mutate: func(cfg *Config) { cfg.APIUsageRetentionBatchSize = 0 }},
		{name: "batch size too large", mutate: func(cfg *Config) { cfg.APIUsageRetentionBatchSize = 10001 }},
		{name: "destructive without confirm", mutate: func(cfg *Config) {
			cfg.APIUsageRetentionDryRun = false
			cfg.APIUsageRetentionConfirmReady = false
		}},
		{name: "destructive without production signer", mutate: func(cfg *Config) {
			cfg.APIUsageRetentionDryRun = false
			cfg.APIUsageRetentionConfirmReady = true
		}},
		{name: "tenant newline", mutate: func(cfg *Config) { cfg.APIUsageRetentionTenantID = "tenant\nbad" }},
		{name: "principal too large", mutate: func(cfg *Config) { cfg.APIUsageRetentionPrincipalID = strings.Repeat("p", 1025) }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want api usage retention setting rejection")
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
		{name: "event max deliveries", mutate: func(cfg *Config) { cfg.EventConsumerMaxDeliveries = -1 }},
		{name: "event dead-letter stream newline", mutate: func(cfg *Config) { cfg.EventConsumerDeadLetterStream = "mail.event\nbad" }},
		{name: "imap notify count", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerCount = 0 }},
		{name: "imap notify block", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerBlock = 0 }},
		{name: "imap notify max deliveries", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerMaxDeliveries = -1 }},
		{name: "imap notify dead-letter stream newline", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerDeadLetterStream = "mail.event\nbad" }},
		{name: "delivery count", mutate: func(cfg *Config) { cfg.DeliveryConsumerCount = 0 }},
		{name: "delivery block", mutate: func(cfg *Config) { cfg.DeliveryConsumerBlock = 0 }},
		{name: "delivery max deliveries", mutate: func(cfg *Config) { cfg.DeliveryConsumerMaxDeliveries = -1 }},
		{name: "delivery dead-letter stream newline", mutate: func(cfg *Config) { cfg.DeliveryConsumerDeadLetterStream = "delivery.event\nbad" }},
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

func TestValidateRejectsUnsafeRedisConsumerIdentifiers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "event stream blank", mutate: func(cfg *Config) { cfg.EventStream = " " }},
		{name: "event stream newline", mutate: func(cfg *Config) { cfg.EventStream = "mail.event\nbad" }},
		{name: "event group blank", mutate: func(cfg *Config) { cfg.EventConsumerGroup = "" }},
		{name: "event consumer newline", mutate: func(cfg *Config) { cfg.EventConsumerName = "worker\nbad" }},
		{name: "imap notify group blank", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerGroup = " " }},
		{name: "imap notify consumer newline", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerName = "imap\nbad" }},
		{name: "search group blank", mutate: func(cfg *Config) { cfg.SearchIndexConsumerGroup = " " }},
		{name: "search consumer newline", mutate: func(cfg *Config) { cfg.SearchIndexConsumerName = "search\nbad" }},
		{name: "api stream blank", mutate: func(cfg *Config) { cfg.APIMeteringStream = "" }},
		{name: "api group newline", mutate: func(cfg *Config) { cfg.APIMeteringConsumerGroup = "api\nbad" }},
		{name: "api consumer oversized", mutate: func(cfg *Config) { cfg.APIMeteringConsumerName = strings.Repeat("a", 1025) }},
		{name: "push group blank", mutate: func(cfg *Config) { cfg.PushNotifyConsumerGroup = "" }},
		{name: "push consumer newline", mutate: func(cfg *Config) { cfg.PushNotifyConsumerName = "push\nbad" }},
		{name: "delivery stream blank", mutate: func(cfg *Config) { cfg.DeliveryStream = "" }},
		{name: "delivery group newline", mutate: func(cfg *Config) { cfg.DeliveryConsumerGroup = "delivery\nbad" }},
		{name: "delivery consumer oversized", mutate: func(cfg *Config) { cfg.DeliveryConsumerName = strings.Repeat("d", 1025) }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unsafe Redis consumer identifier rejection")
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
		{name: "imap notify", mutate: func(cfg *Config) { cfg.IMAPNotifyConsumerClaimIdle = -time.Second }},
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

func TestValidateRejectsInvalidConsumerDeadLetterSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "search max deliveries", mutate: func(cfg *Config) { cfg.SearchIndexConsumerMaxDeliveries = -1 }},
		{name: "search dead-letter stream newline", mutate: func(cfg *Config) { cfg.SearchIndexConsumerDeadLetterStream = "search.event\nbad" }},
		{name: "push max deliveries", mutate: func(cfg *Config) { cfg.PushNotifyConsumerMaxDeliveries = -1 }},
		{name: "push dead-letter stream newline", mutate: func(cfg *Config) { cfg.PushNotifyConsumerDeadLetterStream = "push.event\nbad" }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want consumer dead-letter setting rejection")
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

func TestValidateRejectsInvalidDeliveryRetryDelays(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "empty delays", mutate: func(cfg *Config) { cfg.DeliveryRetryDelays = nil }},
		{name: "zero delay", mutate: func(cfg *Config) { cfg.DeliveryRetryDelays = []time.Duration{time.Minute, 0} }},
		{name: "negative delay", mutate: func(cfg *Config) { cfg.DeliveryRetryDelays = []time.Duration{-time.Second} }},
		{name: "nonpositive max delay", mutate: func(cfg *Config) { cfg.DeliveryRetryMaxDelay = 0 }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid delivery retry delay rejection")
			}
		})
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
		{name: "http read", mutate: func(cfg *Config) { cfg.HTTPReadTimeout = 0 }},
		{name: "http write", mutate: func(cfg *Config) { cfg.HTTPWriteTimeout = -time.Second }},
		{name: "http idle", mutate: func(cfg *Config) { cfg.HTTPIdleTimeout = 0 }},
		{name: "http read header", mutate: func(cfg *Config) { cfg.HTTPReadHeaderTimeout = 0 }},
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

func TestValidateRejectsUnsafeHTTPMaxHeaderBytes(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{name: "too small", value: minHTTPMaxHeaderBytes - 1},
		{name: "too large", value: maxHTTPMaxHeaderBytes + 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			cfg.HTTPMaxHeaderBytes = tt.value
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want max header bytes rejection")
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
