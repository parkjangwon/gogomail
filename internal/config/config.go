package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                         string
	HTTPAddr                            string
	HTTPReadTimeout                     time.Duration
	HTTPWriteTimeout                    time.Duration
	HTTPIdleTimeout                     time.Duration
	HTTPReadHeaderTimeout               time.Duration
	HTTPMaxHeaderBytes                  int
	SMTPAddr                            string
	InboundSMTPAddr                     string
	InboundTrustedRelays                []string
	IMAPAddr                            string
	IMAPTLSCertFile                     string
	IMAPTLSKeyFile                      string
	IMAPAllowInsecureAuth               bool
	IMAPMaxConnections                  int
	IMAPReadTimeout                     time.Duration
	IMAPWriteTimeout                    time.Duration
	IMAPIdleTimeout                     time.Duration
	IMAPNotifyConsumerGroup             string
	IMAPNotifyConsumerName              string
	IMAPNotifyConsumerCount             int
	IMAPNotifyConsumerBlock             time.Duration
	IMAPNotifyConsumerClaimIdle         time.Duration
	IMAPNotifyConsumerMaxDeliveries     int64
	IMAPNotifyConsumerDeadLetterStream  string
	WellKnownCalDAVURL                  string
	WellKnownCardDAVURL                 string
	POP3Addr                            string
	POP3SAddr                           string
	POP3TLSCertFile                     string
	POP3TLSKeyFile                      string
	POP3MaxConnections                  int
	POP3IdleTimeout                     time.Duration
	CalDAVAddr                          string
	CalDAVAllowInsecureAuth             bool
	CalDAVTrustForwardedProto           bool
	CalDAVTrustedProxies                []string
	CalDAVScheduling                    bool
	CardDAVAddr                         string
	CardDAVAllowInsecureAuth            bool
	CardDAVTrustForwardedProto          bool
	CardDAVTrustedProxies               []string
	WebDAVAddr                          string
	WebDAVDepthInfinityEnabled          bool
	LDAPAddr                            string
	LDAPSAddr                           string
	LDAPTLSCertFile                     string
	LDAPTLSKeyFile                      string
	LDAPCompanyID                       string
	LDAPBaseDomain                      string
	LDAPReferralURLs                    []string
	SCIMToken                           string
	SCIMDefaultDomainID                 string
	SubmissionAddr                      string
	SubmissionSMTPSAddr                 string
	SubmissionMaxConnections            int
	SubmissionMaxRecipients             int
	SubmissionMaxMessageBytes           int64
	SubmissionAddReceivedHeader         bool
	SubmissionSupportSMTPUTF8           bool
	SubmissionSupportRequireTLS         bool
	SubmissionSupportDSN                bool
	SubmissionSupportBinaryMIME         bool
	SMTPTLSCertFile                     string
	SMTPTLSKeyFile                      string
	SubmissionAllowInsecureAuth         bool
	DatabaseURL                         string
	DBMaxOpenConns                      int
	DBMaxIdleConns                      int
	DBConnMaxLifetime                   time.Duration
	DBConnMaxIdleTime                   time.Duration
	RedisAddr                           string
	RedisPassword                       string
	RedisSentinelAddrs                  []string // GOGOMAIL_REDIS_SENTINEL_ADDRS (comma-separated)
	RedisMasterName                     string   // GOGOMAIL_REDIS_MASTER_NAME
	StorageBackend                      string
	StorageBackendCompatLabels          []string
	StorageS3Endpoint                   string
	StorageS3Region                     string
	StorageS3Bucket                     string
	StorageS3Prefix                     string
	StorageS3AccessKeyID                string
	StorageS3SecretAccessKey            string
	StorageS3SessionToken               string
	StorageS3ForcePathStyle             bool
	StorageS3CACertFile                 string
	StorageS3InsecureSkipVerify         bool
	MigrationDir                        string
	SMTPDomain                          string
	SMTPReadTimeout                     time.Duration
	SMTPWriteTimeout                    time.Duration
	SMTPMaxConnections                  int
	SMTPMaxRecipients                   int
	SMTPMaxMessageBytes                 int64
	SMTPRequireAuth                     bool
	SMTPAddReceivedHeader               bool
	SMTPAuthVerificationEnabled         bool
	SMTPAuthservID                      string
	SMTPDMARCEnforcement                string
	SMTPMaxDKIMVerifications            int
	SMTPSupportSMTPUTF8                 bool
	SMTPSupportRequireTLS               bool
	SMTPSupportDSN                      bool
	SMTPSupportBinaryMIME               bool
	MailstoreRoot                       string
	LocalRecipients                     []string
	DedupBackend                        string
	RateLimitBackend                    string
	BackpressureBackend                 string
	MetricsBackend                      string
	// OTel tracing
	OTelEnabled          bool   // GOGOMAIL_OTEL_ENABLED (default false)
	OTelEndpoint         string // GOGOMAIL_OTEL_ENDPOINT (default "http://localhost:4318")
	OTelServiceName      string // GOGOMAIL_OTEL_SERVICE_NAME (default "gogomail")
	OTelServiceVersion   string // GOGOMAIL_OTEL_SERVICE_VERSION (default "dev")
	LogFormat            string
	MilterEnabled                       bool
	MilterAddr                          string
	MilterTimeout                       time.Duration
	MilterMaxConns                      int
	MilterHealthCheckInterval           time.Duration
	MilterShadowMode                    bool
	DNSBLZones                          []string
	DNSBLTimeout                        time.Duration
	DNSBLPolicy                         string
	AttachmentScanBackend               string
	AttachmentScanClamAVAddr            string
	AttachmentScanMaxConcurrency        int
	AttachmentScanMaxBytes              int64
	AttachmentScanFailureThreshold      int
	AttachmentScanCircuitOpenDuration   time.Duration
	AttachmentScanWebhookURL            string
	AttachmentScanWebhookToken          string
	AttachmentScanTimeout               time.Duration
	AttachmentCleanupInterval           time.Duration
	AttachmentCleanupStaleAge           time.Duration
	AttachmentCleanupBatchSize          int
	AttachmentCleanupRunOnce            bool
	DriveCleanupInterval                time.Duration
	DriveCleanupBatchSize               int
	DriveCleanupRunOnce                 bool
	DAVSyncRetentionInterval            time.Duration
	DAVSyncRetentionCutoffAge           time.Duration
	DAVSyncRetentionBatchSize           int
	DAVSyncRetentionRunOnce             bool
	DAVSyncRetentionDryRun              bool
	DAVSyncRetentionConfirmReady        bool
	DriveShareRateLimitBackend          string
	DriveShareRateLimitPerMinute        int
	MailMutationRateLimitBackend        string
	MailMutationRateLimitPerMinute      int
	DMMasterKey                         string
	PushNotifyBackend                   string
	PushNotifyWebhookURL                string
	PushNotifyWebhookToken              string
	PushNotifyWebhookTimeout            time.Duration
	PushNotifyDeviceLimit               int
	APNsKeyID                           string
	APNsTeamID                          string
	APNsPrivateKey                      string
	APNsBundleID                        string
	WebPushVAPIDPublicKey               string
	WebPushVAPIDPrivateKey              string
	WebPushContactEmail                 string
	WebhookDispatchEnabled              bool
	PushNotifyConsumerGroup             string
	PushNotifyConsumerName              string
	PushNotifyConsumerCount             int
	PushNotifyConsumerBlock             time.Duration
	PushNotifyConsumerClaimIdle         time.Duration
	PushNotifyConsumerMaxDeliveries     int64
	PushNotifyConsumerDeadLetterStream  string
	APIMeteringBackend                  string
	APIMeteringTimeout                  time.Duration
	APIMeteringAggregateBackend         string
	APIMeteringStream                   string
	APIMeteringConsumerGroup            string
	APIMeteringConsumerName             string
	APIMeteringConsumerCount            int
	APIMeteringConsumerBlock            time.Duration
	APIMeteringConsumerClaimIdle        time.Duration
	APIMeteringConsumerMaxDeliveries    int64
	APIMeteringConsumerDeadLetterStream string
	APIUsageRetentionInterval           time.Duration
	APIUsageRetentionCutoffAge          time.Duration
	APIUsageRetentionBatchSize          int
	APIUsageRetentionRunOnce            bool
	APIUsageRetentionDryRun             bool
	APIUsageRetentionConfirmReady       bool
	APIUsageRetentionTenantID           string
	APIUsageRetentionPrincipalID        string
	APIUsageExportManifestSignerBackend string
	APIUsageExportManifestSignerKeyID   string
	APIUsageExportManifestSignerSecret  string
	APIUsageExportSignerPrivateKey      string
	APIUsageExportSignerPublicKey       string
	APIUsageExportSignerURL             string
	APIUsageExportSignerToken           string
	RcptRateLimitPerMinute              int
	OutboxRelayBatchSize                int
	OutboxRelayPollInterval             time.Duration
	OutboxRelayMaxAttempts              int
	OutboxRelayWorkerCount              int // GOGOMAIL_OUTBOX_RELAY_WORKER_COUNT (default 1)
	OutboxRelayShardTotal               int // GOGOMAIL_OUTBOX_RELAY_SHARD_TOTAL (default 1 = no sharding)
	OutboxRelayShardIndex               int // GOGOMAIL_OUTBOX_RELAY_SHARD_INDEX (default 0)
	EventStream                         string
	EventConsumerGroup                  string
	EventConsumerName                   string
	EventConsumerCount                  int
	EventConsumerBlock                  time.Duration
	EventConsumerClaimIdle              time.Duration
	EventConsumerMaxDeliveries          int64
	EventConsumerDeadLetterStream       string
	SearchIndexBackend                  string
	SearchIndexMaxBodyBytes             int64
	SearchIndexConsumerGroup            string
	SearchIndexConsumerName             string
	SearchIndexConsumerCount            int
	SearchIndexConsumerBlock            time.Duration
	SearchIndexConsumerClaimIdle        time.Duration
	SearchIndexConsumerMaxDeliveries    int64
	SearchIndexConsumerDeadLetterStream string
	SearchIndexOpenSearchEndpoint       string
	SearchIndexOpenSearchIndex          string
	SearchIndexOpenSearchUsername       string
	SearchIndexOpenSearchPassword       string
	SearchIndexOpenSearchBootstrap      bool
	SearchIndexOpenSearchTimeout        time.Duration
	SearchIndexOpenSearchKoreanAnalyzer bool
	MailFlowOpenSearchIndex             string
	MailFlowOpenSearchBootstrap         bool
	MailFlowStatsBackend                string
	MessageBodyCacheEntries             int
	MessageBodyCacheTTL                 time.Duration
	DeliveryStream                      string
	DeliveryConsumerGroup               string
	DeliveryConsumerName                string
	DeliveryConsumerCount               int
	DeliveryConsumerBlock               time.Duration
	DeliveryConsumerClaimIdle           time.Duration
	DeliveryConsumerMaxDeliveries       int64
	DeliveryConsumerDeadLetterStream    string
	DeliverySMTPHello                   string
	DeliveryTimeout                     time.Duration
	DeliveryTLSMode                     string
	DeliveryRecipientBatchSize          int
	DeliveryRouteBackend                string
	DeliverySmartHost                   string
	DeliverySmartHostPort               int
	DeliverySmartHostTLSMode            string
	DeliverySmartHostImplicitTLS        bool
	DeliverySmartHostUsername           string
	DeliverySmartHostPassword           string
	DeliverySmartHostIdentity           string
	DeliveryRetryDelays                 []time.Duration
	DeliveryRetryJitterRatio            float64
	DeliveryRetryMaxDelay               time.Duration
	DeliveryThrottleEnabled             bool
	DeliveryThrottleBackend             string
	DeliveryDefaultConcurrency          int
	DeliveryFarmConcurrency             map[string]int
	DeliveryDomainConcurrency           map[string]int
	DeliveryRateLimitEnabled            bool
	DeliveryRateLimitBackend            string // "redis" or "memory" (default "redis")
	DeliveryDefaultRateLimitPerMinute   int
	DeliveryDomainRateLimitPerMinute    map[string]int
	DeliveryDomainBackoffEnabled        bool
	DeliveryDomainBackoffBackend        string
	DeliveryDomainBackoffScope          string
	DeliveryDomainBackoffBaseDelay      time.Duration
	DeliveryDomainBackoffMaxDelay       time.Duration
	DSNPostmaster                       string
	DKIMEnabled                         bool
	AdminToken                          string
	AuthJWTSecret                       string
	AdminMFARequired                    bool
	PublicBaseURL                       string
	AutoPurgeEnabled                    bool
	AutoPurgeInterval                   time.Duration
	AutoPurgeBatchSize                  int

	// Auto backpressure
	AutoBackpressureEnabled       bool
	AutoBackpressureCheckInterval time.Duration
	AutoBackpressureMemWarn       float64
	AutoBackpressureMemDanger     float64
	AutoBackpressureMemCritical   float64
	AutoBackpressureQueueWarn     int64
	AutoBackpressureQueueDanger   int64
	AutoBackpressureQueueCritical int64
	AutoBackpressureInstanceID    string

	// Bulk sender limiter (Submission MTA)
	SubmissionBulkSenderEnabled bool
	SubmissionBulkSenderRate    int
	SubmissionBulkSenderRole    string

	// Delivery circuit breaker
	DeliveryCircuitBreakerEnabled   bool
	DeliveryCircuitBreakerMax       int
	DeliveryCircuitBreakerThreshold int
	DeliveryCircuitBreakerTimeout   time.Duration

	// SMTP latency tracking
	SMTPLatencyTrackingEnabled bool
	SMTPLatencyWindowSize      int

	// Alert dispatcher
	AlertEmailTo    string // GOGOMAIL_ALERT_EMAIL_TO
	AlertEmailFrom  string // GOGOMAIL_ALERT_EMAIL_FROM
	AlertSMTPAddr   string // GOGOMAIL_ALERT_SMTP_ADDR
	AlertWebhookURL string // GOGOMAIL_ALERT_WEBHOOK_URL

	// Farm coordinator
	FarmCoordinatorBackend              string
	FarmCoordinatorNodeID               string
	FarmCoordinatorHeartbeatTTL         time.Duration
	FarmCoordinatorJobVisibilityTimeout time.Duration

	// HTTP CORS
	CORSAllowedOrigins string

	// Metrics HTTP endpoint
	MetricsAddr string
}

func Load() Config {
	cfg := Config{
		Environment:                         envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:                            envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		HTTPReadTimeout:                     durationEnvOrDefault("GOGOMAIL_HTTP_READ_TIMEOUT", 5*time.Minute),
		HTTPWriteTimeout:                    durationEnvOrDefault("GOGOMAIL_HTTP_WRITE_TIMEOUT", 10*time.Minute),
		HTTPIdleTimeout:                     durationEnvOrDefault("GOGOMAIL_HTTP_IDLE_TIMEOUT", 2*time.Minute),
		HTTPReadHeaderTimeout:               durationEnvOrDefault("GOGOMAIL_HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		HTTPMaxHeaderBytes:                  intEnvOrDefault("GOGOMAIL_HTTP_MAX_HEADER_BYTES", 64*1024),
		SMTPAddr:                            envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		InboundSMTPAddr:                     envOrDefault("GOGOMAIL_INBOUND_SMTP_ADDR", ":2526"),
		InboundTrustedRelays:                splitCSV(envOrDefault("GOGOMAIL_INBOUND_TRUSTED_RELAYS", "127.0.0.1/32,::1/128")),
		IMAPAddr:                            envOrDefault("GOGOMAIL_IMAP_ADDR", ":1143"),
		IMAPTLSCertFile:                     envOrDefault("GOGOMAIL_IMAP_TLS_CERT_FILE", ""),
		IMAPTLSKeyFile:                      envOrDefault("GOGOMAIL_IMAP_TLS_KEY_FILE", ""),
		IMAPAllowInsecureAuth:               boolEnvOrDefault("GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		IMAPMaxConnections:                  intEnvOrDefault("GOGOMAIL_IMAP_MAX_CONNECTIONS", 5000),
		IMAPReadTimeout:                     durationEnvOrDefault("GOGOMAIL_IMAP_READ_TIMEOUT", 5*time.Minute),
		IMAPWriteTimeout:                    durationEnvOrDefault("GOGOMAIL_IMAP_WRITE_TIMEOUT", 30*time.Second),
		IMAPIdleTimeout:                     durationEnvOrDefault("GOGOMAIL_IMAP_IDLE_TIMEOUT", 30*time.Minute),
		IMAPNotifyConsumerGroup:             envOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_GROUP", "gogomail.imap-gateway"),
		IMAPNotifyConsumerName:              nodeScopedEnvOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_NAME", "imap-gateway-1"),
		IMAPNotifyConsumerCount:             intEnvOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_COUNT", 50),
		IMAPNotifyConsumerBlock:             durationEnvOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_BLOCK", time.Second),
		IMAPNotifyConsumerClaimIdle:         durationEnvOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		IMAPNotifyConsumerMaxDeliveries:     int64EnvOrDefault("GOGOMAIL_IMAP_NOTIFY_CONSUMER_MAX_DELIVERIES", 10),
		IMAPNotifyConsumerDeadLetterStream:  strings.TrimSpace(os.Getenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_DEAD_LETTER_STREAM")),
		WellKnownCalDAVURL:                  envOrDefault("GOGOMAIL_WELL_KNOWN_CALDAV_URL", ""),
		WellKnownCardDAVURL:                 envOrDefault("GOGOMAIL_WELL_KNOWN_CARDDAV_URL", ""),
		POP3Addr:                            envOrDefault("GOGOMAIL_POP3_ADDR", ":1110"),
		POP3SAddr:                           envOrDefault("GOGOMAIL_POP3S_ADDR", ""),
		POP3TLSCertFile:                     envOrDefault("GOGOMAIL_POP3_TLS_CERT_FILE", ""),
		POP3TLSKeyFile:                      envOrDefault("GOGOMAIL_POP3_TLS_KEY_FILE", ""),
		POP3MaxConnections:                  intEnvOrDefault("GOGOMAIL_POP3_MAX_CONNECTIONS", 2000),
		POP3IdleTimeout:                     durationEnvOrDefault("GOGOMAIL_POP3_IDLE_TIMEOUT", 10*time.Minute),
		CalDAVAddr:                          envOrDefault("GOGOMAIL_CALDAV_ADDR", ":8081"),
		CalDAVAllowInsecureAuth:             boolEnvOrDefault("GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		CalDAVTrustForwardedProto:           boolEnvOrDefault("GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO", false),
		CalDAVTrustedProxies:                splitCSV(os.Getenv("GOGOMAIL_CALDAV_TRUSTED_PROXIES")),
		CalDAVScheduling:                    boolEnvOrDefault("GOGOMAIL_CALDAV_SCHEDULING", false),
		CardDAVAddr:                         envOrDefault("GOGOMAIL_CARDDAV_ADDR", ":8082"),
		CardDAVAllowInsecureAuth:            boolEnvOrDefault("GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		CardDAVTrustForwardedProto:          boolEnvOrDefault("GOGOMAIL_CARDDAV_TRUST_FORWARDED_PROTO", false),
		CardDAVTrustedProxies:               splitCSV(os.Getenv("GOGOMAIL_CARDDAV_TRUSTED_PROXIES")),
		WebDAVAddr:                          envOrDefault("GOGOMAIL_WEBDAV_ADDR", ":8083"),
		WebDAVDepthInfinityEnabled:          boolEnvOrDefault("GOGOMAIL_WEBDAV_DEPTH_INFINITY_ENABLED", false),
		LDAPAddr:                            envOrDefault("GOGOMAIL_LDAP_ADDR", ":389"),
		LDAPSAddr:                           envOrDefault("GOGOMAIL_LDAPS_ADDR", ""),
		LDAPTLSCertFile:                     envOrDefault("GOGOMAIL_LDAP_TLS_CERT_FILE", ""),
		LDAPTLSKeyFile:                      envOrDefault("GOGOMAIL_LDAP_TLS_KEY_FILE", ""),
		LDAPCompanyID:                       envOrDefault("GOGOMAIL_LDAP_COMPANY_ID", ""),
		LDAPBaseDomain:                      envOrDefault("GOGOMAIL_LDAP_BASE_DOMAIN", ""),
		LDAPReferralURLs:                    splitLDAPReferralURLs(os.Getenv("GOGOMAIL_LDAP_REFERRAL_URLS")),
		SCIMToken:                           envOrDefault("GOGOMAIL_SCIM_TOKEN", ""),
		SCIMDefaultDomainID:                 envOrDefault("GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID", ""),
		SubmissionAddr:                      envOrDefault("GOGOMAIL_SUBMISSION_ADDR", ":2587"),
		SubmissionSMTPSAddr:                 envOrDefault("GOGOMAIL_SUBMISSION_SMTPS_ADDR", ""),
		SubmissionMaxConnections:            intEnvOrDefault("GOGOMAIL_SUBMISSION_MAX_CONNECTIONS", 5000),
		SubmissionMaxRecipients:             intEnvOrDefault("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS", 100),
		SubmissionMaxMessageBytes:           int64EnvOrDefault("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", 25*1024*1024),
		SubmissionAddReceivedHeader:         boolEnvOrDefault("GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER", true),
		SubmissionSupportSMTPUTF8:           boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8", false),
		SubmissionSupportRequireTLS:         boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS", false),
		SubmissionSupportDSN:                boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_DSN", false),
		SubmissionSupportBinaryMIME:         boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME", false),
		SMTPTLSCertFile:                     envOrDefault("GOGOMAIL_SMTP_TLS_CERT_FILE", ""),
		SMTPTLSKeyFile:                      envOrDefault("GOGOMAIL_SMTP_TLS_KEY_FILE", ""),
		SubmissionAllowInsecureAuth:         boolEnvOrDefault("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		DatabaseURL:                         envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		DBMaxOpenConns:                      intEnvOrDefault("GOGOMAIL_DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:                      intEnvOrDefault("GOGOMAIL_DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:                   durationEnvOrDefault("GOGOMAIL_DB_CONN_MAX_LIFETIME", 30*time.Minute),
		DBConnMaxIdleTime:                   durationEnvOrDefault("GOGOMAIL_DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		RedisAddr:                           envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		RedisPassword:                       envOrDefault("GOGOMAIL_REDIS_PASSWORD", ""),
		RedisSentinelAddrs:                  splitCSV(os.Getenv("GOGOMAIL_REDIS_SENTINEL_ADDRS")),
		RedisMasterName:                     envOrDefault("GOGOMAIL_REDIS_MASTER_NAME", "mymaster"),
		StorageBackend:                      envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
		StorageBackendCompatLabels:          splitCSV(os.Getenv("GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS")),
		StorageS3Endpoint:                   envOrDefault("GOGOMAIL_STORAGE_S3_ENDPOINT", ""),
		StorageS3Region:                     envOrDefault("GOGOMAIL_STORAGE_S3_REGION", "us-east-1"),
		StorageS3Bucket:                     envOrDefault("GOGOMAIL_STORAGE_S3_BUCKET", ""),
		StorageS3Prefix:                     envOrDefault("GOGOMAIL_STORAGE_S3_PREFIX", ""),
		StorageS3AccessKeyID:                os.Getenv("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID"),
		StorageS3SecretAccessKey:            os.Getenv("GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY"),
		StorageS3SessionToken:               os.Getenv("GOGOMAIL_STORAGE_S3_SESSION_TOKEN"),
		StorageS3ForcePathStyle:             boolEnvOrDefault("GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE", false),
		StorageS3CACertFile:                 envOrDefault("GOGOMAIL_STORAGE_S3_CA_CERT_FILE", ""),
		StorageS3InsecureSkipVerify:         boolEnvOrDefault("GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY", false),
		MigrationDir:                        envOrDefault("GOGOMAIL_MIGRATION_DIR", "migrations"),
		SMTPDomain:                          envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost"),
		SMTPReadTimeout:                     durationEnvOrDefault("GOGOMAIL_SMTP_READ_TIMEOUT", 30*time.Second),
		SMTPWriteTimeout:                    durationEnvOrDefault("GOGOMAIL_SMTP_WRITE_TIMEOUT", 30*time.Second),
		SMTPMaxConnections:                  intEnvOrDefault("GOGOMAIL_SMTP_MAX_CONNECTIONS", 10000),
		SMTPMaxRecipients:                   intEnvOrDefault("GOGOMAIL_SMTP_MAX_RECIPIENTS", 100),
		SMTPMaxMessageBytes:                 int64EnvOrDefault("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", 25*1024*1024),
		SMTPRequireAuth:                     boolEnvOrDefault("GOGOMAIL_SMTP_REQUIRE_AUTH", false),
		SMTPAddReceivedHeader:               boolEnvOrDefault("GOGOMAIL_SMTP_ADD_RECEIVED_HEADER", true),
		SMTPAuthVerificationEnabled:         boolEnvOrDefault("GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED", false),
		SMTPAuthservID:                      envOrDefault("GOGOMAIL_SMTP_AUTHSERV_ID", envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost")),
		SMTPDMARCEnforcement:                envOrDefault("GOGOMAIL_SMTP_DMARC_ENFORCEMENT", "reject"),
		SMTPMaxDKIMVerifications:            intEnvOrDefault("GOGOMAIL_SMTP_MAX_DKIM_VERIFICATIONS", 8),
		SMTPSupportSMTPUTF8:                 boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", false),
		SMTPSupportRequireTLS:               boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", false),
		SMTPSupportDSN:                      boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_DSN", false),
		SMTPSupportBinaryMIME:               boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", false),
		MailstoreRoot:                       envOrDefault("GOGOMAIL_MAILSTORE_ROOT", envOrDefault("GOGOMAIL_STORAGE_ROOT", "var/mailstore")),
		LocalRecipients:                     splitCSV(os.Getenv("GOGOMAIL_LOCAL_RECIPIENTS")),
		DedupBackend:                        envOrDefault("GOGOMAIL_DEDUP_BACKEND", "none"),
		RateLimitBackend:                    envOrDefault("GOGOMAIL_RATELIMIT_BACKEND", "none"),
		BackpressureBackend:                 envOrDefault("GOGOMAIL_BACKPRESSURE_BACKEND", "none"),
		MetricsBackend:                      envOrDefault("GOGOMAIL_METRICS_BACKEND", "none"),
		OTelEnabled:                         boolEnvOrDefault("GOGOMAIL_OTEL_ENABLED", false),
		OTelEndpoint:                        envOrDefault("GOGOMAIL_OTEL_ENDPOINT", "http://localhost:4318"),
		OTelServiceName:                     envOrDefault("GOGOMAIL_OTEL_SERVICE_NAME", "gogomail"),
		OTelServiceVersion:                  envOrDefault("GOGOMAIL_OTEL_SERVICE_VERSION", "dev"),
		LogFormat:                           envOrDefault("GOGOMAIL_LOG_FORMAT", "json"),
		MilterEnabled:                       boolEnvOrDefault("GOGOMAIL_MILTER_ENABLED", false),
		MilterAddr:                          envOrDefault("GOGOMAIL_MILTER_ADDR", "127.0.0.1:7357"),
		MilterTimeout:                       durationEnvOrDefault("GOGOMAIL_MILTER_TIMEOUT", 30*time.Second),
		MilterMaxConns:                      intEnvOrDefault("GOGOMAIL_MILTER_MAX_CONNS", 10),
		MilterHealthCheckInterval:           durationEnvOrDefault("GOGOMAIL_MILTER_HEALTH_CHECK_INTERVAL", 30*time.Second),
		MilterShadowMode:                    boolEnvOrDefault("GOGOMAIL_MILTER_SHADOW_MODE", false),
		DNSBLZones:                          splitCSV(os.Getenv("GOGOMAIL_DNSBL_ZONES")),
		DNSBLTimeout:                        durationEnvOrDefault("GOGOMAIL_DNSBL_TIMEOUT", 5*time.Second),
		DNSBLPolicy:                         envOrDefault("GOGOMAIL_DNSBL_POLICY", "reject"),
		AttachmentScanBackend:               envOrDefault("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", "none"),
		AttachmentScanClamAVAddr:            envOrDefault("GOGOMAIL_ATTACHMENT_SCAN_CLAMAV_ADDR", "127.0.0.1:3310"),
		AttachmentScanMaxConcurrency:        intEnvOrDefault("GOGOMAIL_ATTACHMENT_SCAN_MAX_CONCURRENCY", 4),
		AttachmentScanMaxBytes:              int64EnvOrDefault("GOGOMAIL_ATTACHMENT_SCAN_MAX_BYTES", 25<<20),
		AttachmentScanFailureThreshold:      intEnvOrDefault("GOGOMAIL_ATTACHMENT_SCAN_FAILURE_THRESHOLD", 3),
		AttachmentScanCircuitOpenDuration:   durationEnvOrDefault("GOGOMAIL_ATTACHMENT_SCAN_CIRCUIT_OPEN_DURATION", 30*time.Second),
		AttachmentScanWebhookURL:            envOrDefault("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", ""),
		AttachmentScanWebhookToken:          os.Getenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN"),
		AttachmentScanTimeout:               durationEnvOrDefault("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT", 2*time.Second),
		AttachmentCleanupInterval:           durationEnvOrDefault("GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL", time.Hour),
		AttachmentCleanupStaleAge:           durationEnvOrDefault("GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE", 24*time.Hour),
		AttachmentCleanupBatchSize:          intEnvOrDefault("GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE", 100),
		AttachmentCleanupRunOnce:            boolEnvOrDefault("GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE", false),
		DriveCleanupInterval:                durationEnvOrDefault("GOGOMAIL_DRIVE_CLEANUP_INTERVAL", 15*time.Minute),
		DriveCleanupBatchSize:               intEnvOrDefault("GOGOMAIL_DRIVE_CLEANUP_BATCH_SIZE", 100),
		DriveCleanupRunOnce:                 boolEnvOrDefault("GOGOMAIL_DRIVE_CLEANUP_RUN_ONCE", false),
		DAVSyncRetentionInterval:            durationEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL", 24*time.Hour),
		DAVSyncRetentionCutoffAge:           durationEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE", 90*24*time.Hour),
		DAVSyncRetentionBatchSize:           intEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE", 1000),
		DAVSyncRetentionRunOnce:             boolEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_RUN_ONCE", false),
		DAVSyncRetentionDryRun:              boolEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN", true),
		DAVSyncRetentionConfirmReady:        boolEnvOrDefault("GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY", false),
		DriveShareRateLimitBackend:          envOrDefault("GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND", "none"),
		DriveShareRateLimitPerMinute:        intEnvOrDefault("GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE", 120),
		MailMutationRateLimitBackend:        envOrDefault("GOGOMAIL_MAIL_MUTATION_RATELIMIT_BACKEND", "none"),
		MailMutationRateLimitPerMinute:      intEnvOrDefault("GOGOMAIL_MAIL_MUTATION_RATELIMIT_PER_MINUTE", 300),
		DMMasterKey:                         os.Getenv("GOGOMAIL_DM_MASTER_KEY"),
		PushNotifyBackend:                   envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "none"),
		PushNotifyWebhookURL:                envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", ""),
		PushNotifyWebhookToken:              os.Getenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN"),
		PushNotifyWebhookTimeout:            durationEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT", 2*time.Second),
		PushNotifyDeviceLimit:               intEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT", 200),
		APNsKeyID:                           os.Getenv("GOGOMAIL_APNS_KEY_ID"),
		APNsTeamID:                          os.Getenv("GOGOMAIL_APNS_TEAM_ID"),
		APNsPrivateKey:                      os.Getenv("GOGOMAIL_APNS_PRIVATE_KEY"),
		APNsBundleID:                        os.Getenv("GOGOMAIL_APNS_BUNDLE_ID"),
		WebPushVAPIDPublicKey:               os.Getenv("GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY"),
		WebPushVAPIDPrivateKey:              os.Getenv("GOGOMAIL_WEBPUSH_VAPID_PRIVATE_KEY"),
		WebPushContactEmail:                 os.Getenv("GOGOMAIL_WEBPUSH_CONTACT_EMAIL"),
		WebhookDispatchEnabled:              boolEnvOrDefault("GOGOMAIL_WEBHOOK_DISPATCH_ENABLED", true),
		PushNotifyConsumerGroup:             envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_GROUP", "gogomail.push-notification-worker"),
		PushNotifyConsumerName:              nodeScopedEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_NAME", "push-notification-worker-1"),
		PushNotifyConsumerCount:             intEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_COUNT", 50),
		PushNotifyConsumerBlock:             durationEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_BLOCK", time.Second),
		PushNotifyConsumerClaimIdle:         durationEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		PushNotifyConsumerMaxDeliveries:     int64EnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_MAX_DELIVERIES", 10),
		PushNotifyConsumerDeadLetterStream:  strings.TrimSpace(os.Getenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_DEAD_LETTER_STREAM")),
		APIMeteringBackend:                  envOrDefault("GOGOMAIL_API_METERING_BACKEND", "none"),
		APIMeteringTimeout:                  durationEnvOrDefault("GOGOMAIL_API_METERING_TIMEOUT", 100*time.Millisecond),
		APIMeteringAggregateBackend:         envOrDefault("GOGOMAIL_API_METERING_AGGREGATE_BACKEND", "disabled"),
		APIMeteringStream:                   envOrDefault("GOGOMAIL_API_METERING_STREAM", "api.event"),
		APIMeteringConsumerGroup:            envOrDefault("GOGOMAIL_API_METERING_CONSUMER_GROUP", "gogomail.api-metering-worker"),
		APIMeteringConsumerName:             nodeScopedEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_NAME", "api-metering-worker-1"),
		APIMeteringConsumerCount:            intEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_COUNT", 100),
		APIMeteringConsumerBlock:            durationEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_BLOCK", time.Second),
		APIMeteringConsumerClaimIdle:        durationEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		APIMeteringConsumerMaxDeliveries:    int64EnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_MAX_DELIVERIES", 10),
		APIMeteringConsumerDeadLetterStream: strings.TrimSpace(os.Getenv("GOGOMAIL_API_METERING_CONSUMER_DEAD_LETTER_STREAM")),
		APIUsageRetentionInterval:           durationEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_INTERVAL", 24*time.Hour),
		APIUsageRetentionCutoffAge:          durationEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE", 90*24*time.Hour),
		APIUsageRetentionBatchSize:          intEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE", 1000),
		APIUsageRetentionRunOnce:            boolEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_RUN_ONCE", false),
		APIUsageRetentionDryRun:             boolEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_DRY_RUN", true),
		APIUsageRetentionConfirmReady:       boolEnvOrDefault("GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY", false),
		APIUsageRetentionTenantID:           envOrDefault("GOGOMAIL_API_USAGE_RETENTION_TENANT_ID", ""),
		APIUsageRetentionPrincipalID:        envOrDefault("GOGOMAIL_API_USAGE_RETENTION_PRINCIPAL_ID", ""),
		APIUsageExportManifestSignerBackend: envOrDefault("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND", "disabled"),
		APIUsageExportManifestSignerKeyID:   envOrDefault("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID", ""),
		APIUsageExportManifestSignerSecret:  os.Getenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET"),
		APIUsageExportSignerPrivateKey:      os.Getenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PRIVATE_KEY"),
		APIUsageExportSignerPublicKey:       os.Getenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY"),
		APIUsageExportSignerURL:             envOrDefault("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_URL", ""),
		APIUsageExportSignerToken:           os.Getenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN"),
		RcptRateLimitPerMinute:              intEnvOrDefault("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", 60),
		OutboxRelayBatchSize:                intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", 100),
		OutboxRelayPollInterval:             durationEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", time.Second),
		OutboxRelayMaxAttempts:              intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", 10),
		OutboxRelayWorkerCount:              intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_WORKER_COUNT", 1),
		OutboxRelayShardTotal:               intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_SHARD_TOTAL", 1),
		OutboxRelayShardIndex:               intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_SHARD_INDEX", 0),
		EventStream:                         envOrDefault("GOGOMAIL_EVENT_STREAM", "mail.event"),
		EventConsumerGroup:                  envOrDefault("GOGOMAIL_EVENT_CONSUMER_GROUP", "gogomail.event-worker"),
		EventConsumerName:                   nodeScopedEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_NAME", "event-worker-1"),
		EventConsumerCount:                  intEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_COUNT", 100),
		EventConsumerBlock:                  durationEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_BLOCK", time.Second),
		EventConsumerClaimIdle:              durationEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		EventConsumerMaxDeliveries:          int64EnvOrDefault("GOGOMAIL_EVENT_CONSUMER_MAX_DELIVERIES", 10),
		EventConsumerDeadLetterStream:       strings.TrimSpace(os.Getenv("GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM")),
		SearchIndexBackend:                  envOrDefault("GOGOMAIL_SEARCH_INDEX_BACKEND", "disabled"),
		SearchIndexMaxBodyBytes:             int64EnvOrDefault("GOGOMAIL_SEARCH_INDEX_MAX_BODY_BYTES", 1024*1024),
		SearchIndexConsumerGroup:            envOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_GROUP", "gogomail.search-index-worker"),
		SearchIndexConsumerName:             nodeScopedEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_NAME", "search-index-worker-1"),
		SearchIndexConsumerCount:            intEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_COUNT", 50),
		SearchIndexConsumerBlock:            durationEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_BLOCK", time.Second),
		SearchIndexConsumerClaimIdle:        durationEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		SearchIndexConsumerMaxDeliveries:    int64EnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_MAX_DELIVERIES", 10),
		SearchIndexConsumerDeadLetterStream: strings.TrimSpace(os.Getenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_DEAD_LETTER_STREAM")),
		SearchIndexOpenSearchEndpoint:       envOrDefault("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT", ""),
		SearchIndexOpenSearchIndex:          envOrDefault("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX", "gogomail-messages"),
		SearchIndexOpenSearchUsername:       envOrDefault("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_USERNAME", ""),
		SearchIndexOpenSearchPassword:       os.Getenv("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_PASSWORD"),
		SearchIndexOpenSearchBootstrap:      boolEnvOrDefault("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP", false),
		SearchIndexOpenSearchTimeout:        durationEnvOrDefault("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT", 10*time.Second),
		SearchIndexOpenSearchKoreanAnalyzer: boolEnvOrDefault("GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER", false),
		MailFlowOpenSearchIndex:             envOrDefault("GOGOMAIL_MAIL_FLOW_OPENSEARCH_INDEX", "mail_flow"),
		MailFlowOpenSearchBootstrap:         boolEnvOrDefault("GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP", false),
		MailFlowStatsBackend:                envOrDefault("GOGOMAIL_MAIL_FLOW_STATS_BACKEND", "auto"),
		MessageBodyCacheEntries:             intEnvOrDefault("GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES", 256),
		MessageBodyCacheTTL:                 durationEnvOrDefault("GOGOMAIL_MESSAGE_BODY_CACHE_TTL", 5*time.Minute),
		DeliveryStream:                      envOrDefault("GOGOMAIL_DELIVERY_STREAM", "mail.outbound.general"),
		DeliveryConsumerGroup:               envOrDefault("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "gogomail.delivery-worker"),
		DeliveryConsumerName:                nodeScopedEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-worker-1"),
		DeliveryConsumerCount:               intEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_COUNT", 50),
		DeliveryConsumerBlock:               durationEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", time.Second),
		DeliveryConsumerClaimIdle:           durationEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", 5*time.Minute),
		DeliveryConsumerMaxDeliveries:       int64EnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_MAX_DELIVERIES", 10),
		DeliveryConsumerDeadLetterStream:    strings.TrimSpace(os.Getenv("GOGOMAIL_DELIVERY_CONSUMER_DEAD_LETTER_STREAM")),
		DeliverySMTPHello:                   envOrDefault("GOGOMAIL_DELIVERY_SMTP_HELLO", "localhost"),
		DeliveryTimeout:                     durationEnvOrDefault("GOGOMAIL_DELIVERY_TIMEOUT", 30*time.Second),
		DeliveryTLSMode:                     envOrDefault("GOGOMAIL_DELIVERY_TLS_MODE", "opportunistic"),
		DeliveryRecipientBatchSize:          intEnvOrDefault("GOGOMAIL_DELIVERY_RECIPIENT_BATCH_SIZE", 100),
		DeliveryRouteBackend:                envOrDefault("GOGOMAIL_DELIVERY_ROUTE_BACKEND", "env"),
		DeliverySmartHost:                   envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST", ""),
		DeliverySmartHostPort:               intEnvOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_PORT", 0),
		DeliverySmartHostTLSMode:            envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE", ""),
		DeliverySmartHostImplicitTLS:        boolEnvOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS", false),
		DeliverySmartHostUsername:           envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_USERNAME", ""),
		DeliverySmartHostPassword:           envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD", ""),
		DeliverySmartHostIdentity:           envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_IDENTITY", ""),
		DeliveryRetryDelays:                 durationCSVEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_DELAYS", []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}),
		DeliveryRetryJitterRatio:            floatEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO", 0.20),
		DeliveryRetryMaxDelay:               durationEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY", 24*time.Hour),
		DeliveryThrottleEnabled:             boolEnvOrDefault("GOGOMAIL_DELIVERY_THROTTLE_ENABLED", false),
		DeliveryThrottleBackend:             envOrDefault("GOGOMAIL_DELIVERY_THROTTLE_BACKEND", "redis"),
		DeliveryDefaultConcurrency:          intEnvOrDefault("GOGOMAIL_DELIVERY_DEFAULT_CONCURRENCY", 0),
		DeliveryFarmConcurrency:             intMapEnvOrDefault("GOGOMAIL_DELIVERY_FARM_CONCURRENCY", nil),
		DeliveryDomainConcurrency:           intMapEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_CONCURRENCY", nil),
		DeliveryRateLimitEnabled:            boolEnvOrDefault("GOGOMAIL_DELIVERY_RATE_LIMIT_ENABLED", true),
		DeliveryRateLimitBackend:            envOrDefault("GOGOMAIL_DELIVERY_RATE_LIMIT_BACKEND", "redis"),
		DeliveryDefaultRateLimitPerMinute:   intEnvOrDefault("GOGOMAIL_DELIVERY_DEFAULT_RATE_LIMIT_PER_MINUTE", 60),
		DeliveryDomainRateLimitPerMinute:    intMapEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_RATE_LIMIT_PER_MINUTE", nil),
		DeliveryDomainBackoffEnabled:        boolEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_ENABLED", false),
		DeliveryDomainBackoffBackend:        envOrDefault("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BACKEND", "redis"),
		DeliveryDomainBackoffScope:          envOrDefault("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_SCOPE", "domain"),
		DeliveryDomainBackoffBaseDelay:      durationEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BASE_DELAY", time.Minute),
		DeliveryDomainBackoffMaxDelay:       durationEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_MAX_DELAY", time.Hour),
		DSNPostmaster:                       envOrDefault("GOGOMAIL_DSN_POSTMASTER", ""),
		DKIMEnabled:                         boolEnvOrDefault("GOGOMAIL_DKIM_ENABLED", false),
		AdminToken:                          envOrDefault("GOGOMAIL_ADMIN_TOKEN", ""),
		AuthJWTSecret:                       envOrDefault("GOGOMAIL_AUTH_JWT_SECRET", ""),
		AdminMFARequired:                    boolEnvOrDefault("GOGOMAIL_ADMIN_MFA_REQUIRED", false),
		PublicBaseURL:                       envOrDefault("GOGOMAIL_PUBLIC_BASE_URL", ""),
		AutoPurgeEnabled:                    boolEnvOrDefault("GOGOMAIL_AUTO_PURGE_ENABLED", false),
		AutoPurgeInterval:                   durationEnvOrDefault("GOGOMAIL_AUTO_PURGE_INTERVAL", 24*time.Hour),
		AutoPurgeBatchSize:                  intEnvOrDefault("GOGOMAIL_AUTO_PURGE_BATCH_SIZE", 1000),

		AutoBackpressureEnabled:       boolEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_ENABLED", false),
		AutoBackpressureCheckInterval: durationEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_CHECK_INTERVAL", 5*time.Second),
		AutoBackpressureMemWarn:       floatEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_MEM_WARN", 0.70),
		AutoBackpressureMemDanger:     floatEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_MEM_DANGER", 0.85),
		AutoBackpressureMemCritical:   floatEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_MEM_CRITICAL", 0.95),
		AutoBackpressureQueueWarn:     int64EnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_QUEUE_WARN", 10000),
		AutoBackpressureQueueDanger:   int64EnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_QUEUE_DANGER", 50000),
		AutoBackpressureInstanceID:    nodeScopedEnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_INSTANCE_ID", ""),
		AutoBackpressureQueueCritical: int64EnvOrDefault("GOGOMAIL_AUTO_BACKPRESSURE_QUEUE_CRITICAL", 100000),

		SubmissionBulkSenderEnabled: boolEnvOrDefault("GOGOMAIL_SUBMISSION_BULK_SENDER_ENABLED", false),
		SubmissionBulkSenderRate:    intEnvOrDefault("GOGOMAIL_SUBMISSION_BULK_SENDER_RATE", 100),
		SubmissionBulkSenderRole:    envOrDefault("GOGOMAIL_SUBMISSION_BULK_SENDER_ROLE", "bulk_user"),

		DeliveryCircuitBreakerEnabled:   boolEnvOrDefault("GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_ENABLED", false),
		DeliveryCircuitBreakerMax:       intEnvOrDefault("GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_MAX", 10),
		DeliveryCircuitBreakerThreshold: intEnvOrDefault("GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_THRESHOLD", 5),
		DeliveryCircuitBreakerTimeout:   durationEnvOrDefault("GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_TIMEOUT", 30*time.Second),

		SMTPLatencyTrackingEnabled: boolEnvOrDefault("GOGOMAIL_SMTP_LATENCY_TRACKING_ENABLED", false),
		SMTPLatencyWindowSize:      intEnvOrDefault("GOGOMAIL_SMTP_LATENCY_WINDOW_SIZE", 1000),

		AlertEmailTo:    os.Getenv("GOGOMAIL_ALERT_EMAIL_TO"),
		AlertEmailFrom:  envOrDefault("GOGOMAIL_ALERT_EMAIL_FROM", "alerts@localhost"),
		AlertSMTPAddr:   envOrDefault("GOGOMAIL_ALERT_SMTP_ADDR", "localhost:25"),
		AlertWebhookURL: os.Getenv("GOGOMAIL_ALERT_WEBHOOK_URL"),

		FarmCoordinatorBackend:              envOrDefault("GOGOMAIL_FARM_COORDINATOR_BACKEND", "noop"),
		FarmCoordinatorNodeID:               nodeScopedEnvOrDefault("GOGOMAIL_FARM_COORDINATOR_NODE_ID", ""),
		FarmCoordinatorHeartbeatTTL:         durationEnvOrDefault("GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL", 30*time.Second),
		FarmCoordinatorJobVisibilityTimeout: durationEnvOrDefault("GOGOMAIL_FARM_COORDINATOR_JOB_VISIBILITY_TIMEOUT", 5*time.Minute),
		CORSAllowedOrigins:                  os.Getenv("GOGOMAIL_CORS_ALLOWED_ORIGINS"),
		MetricsAddr:                         envOrDefault("GOGOMAIL_METRICS_ADDR", ":9090"),
	}
	if cfg.EventConsumerDeadLetterStream == "" {
		cfg.EventConsumerDeadLetterStream = cfg.EventStream + ".dead"
	}
	if cfg.IMAPNotifyConsumerDeadLetterStream == "" {
		cfg.IMAPNotifyConsumerDeadLetterStream = cfg.EventStream + ".dead"
	}
	if cfg.SearchIndexConsumerDeadLetterStream == "" {
		cfg.SearchIndexConsumerDeadLetterStream = cfg.EventStream + ".dead"
	}
	if cfg.PushNotifyConsumerDeadLetterStream == "" {
		cfg.PushNotifyConsumerDeadLetterStream = cfg.EventStream + ".dead"
	}
	if cfg.APIMeteringConsumerDeadLetterStream == "" {
		cfg.APIMeteringConsumerDeadLetterStream = cfg.APIMeteringStream + ".dead"
	}
	if cfg.DeliveryConsumerDeadLetterStream == "" {
		cfg.DeliveryConsumerDeadLetterStream = cfg.DeliveryStream + ".dead"
	}
	return cfg
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func nodeScopedEnvOrDefault(key string, fallback string) string {
	value := envOrDefault(key, fallback)
	if !strings.Contains(value, "{hostname}") &&
		!strings.Contains(value, "${HOSTNAME}") &&
		!strings.Contains(value, "$HOSTNAME") {
		return value
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown-host"
	}
	value = strings.ReplaceAll(value, "{hostname}", hostname)
	value = strings.ReplaceAll(value, "${HOSTNAME}", hostname)
	value = strings.ReplaceAll(value, "$HOSTNAME", hostname)
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func splitLDAPReferralURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func intEnvOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64EnvOrDefault(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func defaultSubmissionAllowInsecureAuth() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("GOGOMAIL_ENV")), "production")
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatEnvOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationCSVEnvOrDefault(key string, fallback []time.Duration) []time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]time.Duration(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	durations := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		parsed, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return append([]time.Duration(nil), fallback...)
		}
		durations = append(durations, parsed)
	}
	if len(durations) == 0 {
		return append([]time.Duration(nil), fallback...)
	}
	return durations
}

func intMapEnvOrDefault(key string, fallback map[string]int) map[string]int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return copyStringIntMap(fallback)
	}
	result := make(map[string]int)
	for _, part := range strings.Split(value, ",") {
		name, rawLimit, ok := strings.Cut(part, "=")
		if !ok {
			return copyStringIntMap(fallback)
		}
		name = strings.ToLower(strings.TrimSpace(name))
		limit, err := strconv.Atoi(strings.TrimSpace(rawLimit))
		if name == "" || err != nil || limit <= 0 {
			return copyStringIntMap(fallback)
		}
		result[name] = limit
	}
	return result
}

func copyStringIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
