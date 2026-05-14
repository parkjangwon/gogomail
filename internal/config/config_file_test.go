package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLoadFileAppliesStorageOverlay(t *testing.T) {
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "local")
	t.Setenv("GOGOMAIL_STORAGE_S3_ENDPOINT", "http://env-minio:9000")
	t.Setenv("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", "env-access")
	t.Setenv("GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY", "env-secret")

	path := writeYAMLConfig(t, `
environment: test
storage_backend: s3
storage_backend_compat_labels: local,nfs
storage_s3_endpoint: https://s3.us-east-1.amazonaws.com
storage_s3_region: us-east-1
storage_s3_bucket: gogomail-prod
storage_s3_prefix: mail
storage_s3_access_key_id: file-access
storage_s3_secret_access_key: file-secret
storage_s3_session_token: file-session
storage_s3_force_path_style: true
storage_s3_ca_cert_file: /etc/gogomail/s3-ca.pem
storage_s3_insecure_skip_verify: true
caldav_trust_forwarded_proto: false
carddav_trust_forwarded_proto: false
caldav_trusted_proxies:
  - 127.0.0.1/8
  - ::1
carddav_trusted_proxies:
  - 198.51.100.0/24
  - 2001:db8::/32
mailstore_root: /srv/gogomail
local_recipients:
  - admin@example.com
  - user@example.com
delivery_farm_concurrency:
  general: 50
  bulk: 10
delivery_domain_concurrency: example.com=5,example.net=3
delivery_domain_backoff_enabled: true
delivery_domain_backoff_base_delay: 2m
delivery_domain_backoff_max_delay: 30m
attachment_scan_timeout: 3s
imap_max_connections: 512
imap_read_timeout: 45s
imap_write_timeout: 15s
imap_idle_timeout: 35m
pop3s_addr: :1995
pop3_max_connections: 384
ldap_addr: :1389
ldaps_addr: :1636
ldap_tls_cert_file: /etc/gogomail/ldap.crt
ldap_tls_key_file: /etc/gogomail/ldap.key
ldap_company_id: company-1
ldap_base_domain: dc=example,dc=com
ldap_referral_urls:
  - ldap://directory.example.net/dc=example,dc=net
smtp_max_connections: 1024
submission_max_connections: 256
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.Environment != "test" || cfg.StorageBackend != "s3" {
		t.Fatalf("basic overlay = env:%q backend:%q", cfg.Environment, cfg.StorageBackend)
	}
	if cfg.StorageS3Endpoint != "https://s3.us-east-1.amazonaws.com" || cfg.StorageS3AccessKeyID != "file-access" || cfg.StorageS3SecretAccessKey != "file-secret" {
		t.Fatalf("S3 overlay endpoint/access/secret = %q/%q/%q", cfg.StorageS3Endpoint, cfg.StorageS3AccessKeyID, cfg.StorageS3SecretAccessKey)
	}
	if !cfg.StorageS3ForcePathStyle || !cfg.StorageS3InsecureSkipVerify || cfg.StorageS3CACertFile != "/etc/gogomail/s3-ca.pem" {
		t.Fatalf("S3 TLS/path overlay = force:%v insecure:%v ca:%q", cfg.StorageS3ForcePathStyle, cfg.StorageS3InsecureSkipVerify, cfg.StorageS3CACertFile)
	}
	if cfg.CalDAVTrustForwardedProto {
		t.Fatal("CalDAVTrustForwardedProto = true, want false")
	}
	if cfg.CardDAVTrustForwardedProto {
		t.Fatal("CardDAVTrustForwardedProto = true, want false")
	}
	if len(cfg.CalDAVTrustedProxies) != 2 || cfg.CalDAVTrustedProxies[0] != "127.0.0.1/8" || cfg.CalDAVTrustedProxies[1] != "::1" {
		t.Fatalf("CalDAVTrustedProxies = %#v, want [127.0.0.1/8 ::1]", cfg.CalDAVTrustedProxies)
	}
	if len(cfg.CardDAVTrustedProxies) != 2 || cfg.CardDAVTrustedProxies[0] != "198.51.100.0/24" || cfg.CardDAVTrustedProxies[1] != "2001:db8::/32" {
		t.Fatalf("CardDAVTrustedProxies = %#v, want [198.51.100.0/24 2001:db8::/32]", cfg.CardDAVTrustedProxies)
	}
	if len(cfg.StorageBackendCompatLabels) != 2 || cfg.StorageBackendCompatLabels[0] != "local" || cfg.StorageBackendCompatLabels[1] != "nfs" {
		t.Fatalf("compat labels = %#v", cfg.StorageBackendCompatLabels)
	}
	if len(cfg.LocalRecipients) != 2 || cfg.LocalRecipients[0] != "admin@example.com" || cfg.LocalRecipients[1] != "user@example.com" {
		t.Fatalf("local recipients = %#v", cfg.LocalRecipients)
	}
	if cfg.DeliveryFarmConcurrency["general"] != 50 || cfg.DeliveryFarmConcurrency["bulk"] != 10 {
		t.Fatalf("farm concurrency = %#v", cfg.DeliveryFarmConcurrency)
	}
	if cfg.DeliveryDomainConcurrency["example.com"] != 5 || cfg.DeliveryDomainConcurrency["example.net"] != 3 {
		t.Fatalf("domain concurrency = %#v", cfg.DeliveryDomainConcurrency)
	}
	if !cfg.DeliveryDomainBackoffEnabled {
		t.Fatal("DeliveryDomainBackoffEnabled = false, want true")
	}
	if cfg.DeliveryDomainBackoffBaseDelay != 2*time.Minute || cfg.DeliveryDomainBackoffMaxDelay != 30*time.Minute {
		t.Fatalf("DeliveryDomainBackoff delays = %s/%s, want 2m/30m", cfg.DeliveryDomainBackoffBaseDelay, cfg.DeliveryDomainBackoffMaxDelay)
	}
	if cfg.AttachmentScanTimeout != 3*time.Second {
		t.Fatalf("AttachmentScanTimeout = %s, want 3s", cfg.AttachmentScanTimeout)
	}
	if cfg.IMAPMaxConnections != 512 {
		t.Fatalf("IMAPMaxConnections = %d, want 512", cfg.IMAPMaxConnections)
	}
	if cfg.IMAPReadTimeout != 45*time.Second {
		t.Fatalf("IMAPReadTimeout = %s, want 45s", cfg.IMAPReadTimeout)
	}
	if cfg.IMAPWriteTimeout != 15*time.Second {
		t.Fatalf("IMAPWriteTimeout = %s, want 15s", cfg.IMAPWriteTimeout)
	}
	if cfg.IMAPIdleTimeout != 35*time.Minute {
		t.Fatalf("IMAPIdleTimeout = %s, want 35m", cfg.IMAPIdleTimeout)
	}
	if cfg.POP3MaxConnections != 384 {
		t.Fatalf("POP3MaxConnections = %d, want 384", cfg.POP3MaxConnections)
	}
	if cfg.POP3SAddr != ":1995" {
		t.Fatalf("POP3SAddr = %q, want :1995", cfg.POP3SAddr)
	}
	if cfg.LDAPAddr != ":1389" || cfg.LDAPSAddr != ":1636" || cfg.LDAPTLSCertFile != "/etc/gogomail/ldap.crt" || cfg.LDAPTLSKeyFile != "/etc/gogomail/ldap.key" {
		t.Fatalf("LDAP overlay = addr:%q ldaps:%q tls:%q/%q", cfg.LDAPAddr, cfg.LDAPSAddr, cfg.LDAPTLSCertFile, cfg.LDAPTLSKeyFile)
	}
	if cfg.LDAPCompanyID != "company-1" || cfg.LDAPBaseDomain != "dc=example,dc=com" {
		t.Fatalf("LDAP scope overlay = company:%q base:%q", cfg.LDAPCompanyID, cfg.LDAPBaseDomain)
	}
	if len(cfg.LDAPReferralURLs) != 1 || cfg.LDAPReferralURLs[0] != "ldap://directory.example.net/dc=example,dc=net" {
		t.Fatalf("LDAP referral overlay = %#v", cfg.LDAPReferralURLs)
	}
	if cfg.SMTPMaxConnections != 1024 {
		t.Fatalf("SMTPMaxConnections = %d, want 1024", cfg.SMTPMaxConnections)
	}
	if cfg.SubmissionMaxConnections != 256 {
		t.Fatalf("SubmissionMaxConnections = %d, want 256", cfg.SubmissionMaxConnections)
	}
}

func TestLoadFileAcceptsStorageRootAlias(t *testing.T) {
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "/env/mailstore")

	path := writeYAMLConfig(t, `
storage_backend: nfs
storage_root: /mnt/gogomail-storage
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.StorageBackend != "nfs" || cfg.MailstoreRoot != "/mnt/gogomail-storage" {
		t.Fatalf("storage alias overlay = backend:%q root:%q", cfg.StorageBackend, cfg.MailstoreRoot)
	}
}

func TestLoadFileRejectsUnsupportedKey(t *testing.T) {
	path := writeYAMLConfig(t, "storage_backend: local\nsurprise: true\n")

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), `unsupported key "surprise"`) {
		t.Fatalf("LoadFile error = %v, want unsupported key rejection", err)
	}
}

func TestLoadFileRejectsInvalidTypes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "string bool", raw: "storage_s3_force_path_style: yes-please\n"},
		{name: "bad duration", raw: "attachment_scan_timeout: soon\n"},
		{name: "bad map", raw: "delivery_domain_concurrency: example.com:not-number\n"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAMLConfig(t, tt.raw)
			if _, err := LoadFile(path); err == nil {
				t.Fatal("LoadFile error = nil, want invalid config file rejection")
			}
		})
	}
}

func TestLoadFileRejectsMalformedYAML(t *testing.T) {
	path := writeYAMLConfig(t, "storage_backend: [")

	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("LoadFile error = %v, want parse failure", err)
	}
}

func TestLoadFileParsesExampleConfig(t *testing.T) {
	cfg, err := LoadFile("../../configs/config.example.yaml")
	if err != nil {
		t.Fatalf("LoadFile(config.example.yaml) returned error: %v", err)
	}
	if cfg.StorageBackend != "local" || cfg.DatabaseURL == "" || cfg.RedisAddr == "" {
		t.Fatalf("example config core fields = backend:%q db:%q redis:%q", cfg.StorageBackend, cfg.DatabaseURL, cfg.RedisAddr)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("example config Validate() error = %v", err)
	}
}

func TestLoadFileParsesStorageProfileConfigs(t *testing.T) {
	tests := []struct {
		path        string
		backend     string
		endpoint    string
		region      string
		bucket      string
		prefix      string
		accessKeyID string
		secretKey   string
		root        string
		compat      []string
		pathStyle   bool
		environment string
	}{
		{path: "../../configs/storage.local.yaml", backend: "local"},
		{path: "../../configs/storage.nfs.yaml", backend: "nfs", root: "/mnt/gogomail-storage", compat: []string{"local"}},
		{path: "../../configs/storage.minio.yaml", backend: "minio", endpoint: "http://localhost:19000", region: "us-east-1", bucket: "gogomail", accessKeyID: "gogomail", secretKey: "gogomail123", pathStyle: true},
		{path: "../../configs/storage.s3.yaml", backend: "s3", endpoint: "https://s3.us-east-1.amazonaws.com", region: "us-east-1", bucket: "gogomail-prod", prefix: "mail", accessKeyID: "CHANGE_ME_ACCESS_KEY_ID", secretKey: "CHANGE_ME_SECRET_ACCESS_KEY", environment: "production"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			cfg, err := LoadFile(tt.path)
			if err != nil {
				t.Fatalf("LoadFile(%s) returned error: %v", tt.path, err)
			}
			if cfg.StorageBackend != tt.backend {
				t.Fatalf("StorageBackend = %q, want %q", cfg.StorageBackend, tt.backend)
			}
			if cfg.StorageS3Endpoint != tt.endpoint {
				t.Fatalf("StorageS3Endpoint = %q, want %q", cfg.StorageS3Endpoint, tt.endpoint)
			}
			if tt.region != "" && cfg.StorageS3Region != tt.region {
				t.Fatalf("StorageS3Region = %q, want %q", cfg.StorageS3Region, tt.region)
			}
			if cfg.StorageS3Bucket != tt.bucket {
				t.Fatalf("StorageS3Bucket = %q, want %q", cfg.StorageS3Bucket, tt.bucket)
			}
			if cfg.StorageS3Prefix != tt.prefix {
				t.Fatalf("StorageS3Prefix = %q, want %q", cfg.StorageS3Prefix, tt.prefix)
			}
			if cfg.StorageS3AccessKeyID != tt.accessKeyID {
				t.Fatalf("StorageS3AccessKeyID = %q, want %q", cfg.StorageS3AccessKeyID, tt.accessKeyID)
			}
			if cfg.StorageS3SecretAccessKey != tt.secretKey {
				t.Fatalf("StorageS3SecretAccessKey = %q, want %q", cfg.StorageS3SecretAccessKey, tt.secretKey)
			}
			if tt.root != "" && cfg.MailstoreRoot != tt.root {
				t.Fatalf("MailstoreRoot = %q, want %q", cfg.MailstoreRoot, tt.root)
			}
			if !slices.Equal(cfg.StorageBackendCompatLabels, tt.compat) {
				t.Fatalf("StorageBackendCompatLabels = %#v, want %#v", cfg.StorageBackendCompatLabels, tt.compat)
			}
			if cfg.StorageS3ForcePathStyle != tt.pathStyle {
				t.Fatalf("StorageS3ForcePathStyle = %v, want %v", cfg.StorageS3ForcePathStyle, tt.pathStyle)
			}
			if tt.environment != "" && cfg.Environment != tt.environment {
				t.Fatalf("Environment = %q, want %q", cfg.Environment, tt.environment)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("%s Validate() error = %v", tt.path, err)
			}
		})
	}
}

func writeYAMLConfig(t *testing.T, raw string) string {
	t.Helper()
	path := t.TempDir() + "/gogomail.yaml"
	if err := os.WriteFile(path, []byte(strings.TrimLeft(raw, "\n")), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
