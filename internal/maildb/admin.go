package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/mail"
)

var ErrDeliveryRouteNotFound = errors.New("delivery route not found")

type stringArray []string

func (a stringArray) Value() (driver.Value, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, value := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range value {
			switch r {
			case '\\', '"':
				b.WriteByte('\\')
				b.WriteRune(r)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (a *stringArray) Scan(src any) error {
	switch value := src.(type) {
	case string:
		parsed, err := parsePostgresTextArray(value)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []byte:
		parsed, err := parsePostgresTextArray(string(value))
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	default:
		return fmt.Errorf("unsupported text array source %T", src)
	}
}

func parsePostgresTextArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "{}" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return nil, fmt.Errorf("invalid text array")
	}
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}")
	var values []string
	var b strings.Builder
	inQuote := false
	escaped := false
	for _, r := range raw {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if inQuote && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			values = append(values, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	if inQuote || escaped {
		return nil, fmt.Errorf("invalid quoted text array")
	}
	values = append(values, b.String())
	return values, nil
}

type QueueStat struct {
	Topic                string     `json:"topic"`
	Status               string     `json:"status"`
	Count                int64      `json:"count"`
	ReadyCount           int64      `json:"ready_count"`
	DelayedCount         int64      `json:"delayed_count"`
	StaleProcessingCount int64      `json:"stale_processing_count"`
	OldestReadyAt        *time.Time `json:"oldest_ready_at,omitempty"`
	NextAvailableAt      *time.Time `json:"next_available_at,omitempty"`
}

type OutboxEventListRequest struct {
	Limit        int
	Topic        string
	PartitionKey string
	Status       string
	Since        time.Time
}

type OutboxEventView struct {
	ID           string     `json:"id"`
	Topic        string     `json:"topic"`
	PartitionKey string     `json:"partition_key"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	LastError    string     `json:"last_error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	AvailableAt  time.Time  `json:"available_at"`
	LockedAt     *time.Time `json:"locked_at,omitempty"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

const outboxEventListErrorPreviewBytes = 512

type CompanyView struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	Status                 string    `json:"status"`
	QuotaUsed              int64     `json:"quota_used"`
	QuotaLimit             int64     `json:"quota_limit,omitempty"`
	QuotaRemaining         int64     `json:"quota_remaining"`
	AllocatedDomainQuota   int64     `json:"allocated_domain_quota"`
	AllocatableDomainQuota int64     `json:"allocatable_domain_quota"`
	OverAllocated          bool      `json:"over_allocated"`
	CreatedAt              time.Time `json:"created_at"`
}

type QuotaUsageView struct {
	Scope            string    `json:"scope"`
	ID               string    `json:"id"`
	DomainID         string    `json:"domain_id,omitempty"`
	Name             string    `json:"name"`
	QuotaUsed        int64     `json:"quota_used"`
	QuotaLimit       int64     `json:"quota_limit"`
	QuotaRemaining   int64     `json:"quota_remaining"`
	AllocatedQuota   int64     `json:"allocated_quota"`
	AllocatableQuota int64     `json:"allocatable_quota"`
	UsageRatio       float64   `json:"usage_ratio"`
	AllocationRatio  float64   `json:"allocation_ratio"`
	OverLimit        bool      `json:"over_limit"`
	OverAllocated    bool      `json:"over_allocated"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type QuotaUsageListRequest struct {
	Limit         int
	Scope         string
	DomainID      string
	OverLimit     *bool
	OverAllocated *bool
}

type APIUsageDailyView struct {
	Day              time.Time `json:"day"`
	Method           string    `json:"method"`
	Route            string    `json:"route"`
	Status           int       `json:"status"`
	TenantID         string    `json:"tenant_id,omitempty"`
	CompanyID        string    `json:"company_id,omitempty"`
	DomainID         string    `json:"domain_id,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	APIKeyID         string    `json:"api_key_id,omitempty"`
	PrincipalID      string    `json:"principal_id,omitempty"`
	AuthSource       string    `json:"auth_source,omitempty"`
	RequestCount     int64     `json:"request_count"`
	RequestBytes     int64     `json:"request_bytes"`
	ResponseBytes    int64     `json:"response_bytes"`
	LatencyMSTotal   int64     `json:"latency_ms_total"`
	LatencyMSMax     int64     `json:"latency_ms_max"`
	LatencyMSAverage float64   `json:"latency_ms_average"`
	FirstSeenAt      time.Time `json:"first_seen_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type APIUsageMonthlyView struct {
	Month            time.Time `json:"month"`
	Method           string    `json:"method"`
	Route            string    `json:"route"`
	Status           int       `json:"status"`
	TenantID         string    `json:"tenant_id,omitempty"`
	CompanyID        string    `json:"company_id,omitempty"`
	DomainID         string    `json:"domain_id,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	APIKeyID         string    `json:"api_key_id,omitempty"`
	PrincipalID      string    `json:"principal_id,omitempty"`
	AuthSource       string    `json:"auth_source,omitempty"`
	RequestCount     int64     `json:"request_count"`
	RequestBytes     int64     `json:"request_bytes"`
	ResponseBytes    int64     `json:"response_bytes"`
	LatencyMSTotal   int64     `json:"latency_ms_total"`
	LatencyMSMax     int64     `json:"latency_ms_max"`
	LatencyMSAverage float64   `json:"latency_ms_average"`
	FirstSeenAt      time.Time `json:"first_seen_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type APIUsageAggregateListRequest struct {
	Limit       int
	TenantID    string
	CompanyID   string
	DomainID    string
	UserID      string
	APIKeyID    string
	PrincipalID string
	AuthSource  string
	Method      string
	Route       string
	Status      int
	From        time.Time
	To          time.Time
}

type APIUsageLedgerView struct {
	EventID       string          `json:"event_id"`
	SchemaVersion string          `json:"schema_version"`
	EventTime     time.Time       `json:"event_timestamp"`
	RecordedAt    time.Time       `json:"recorded_at"`
	Method        string          `json:"method"`
	Route         string          `json:"route"`
	Status        int             `json:"status"`
	TenantID      string          `json:"tenant_id,omitempty"`
	CompanyID     string          `json:"company_id,omitempty"`
	DomainID      string          `json:"domain_id,omitempty"`
	UserID        string          `json:"user_id,omitempty"`
	APIKeyID      string          `json:"api_key_id,omitempty"`
	PrincipalID   string          `json:"principal_id,omitempty"`
	AuthSource    string          `json:"auth_source,omitempty"`
	RequestCount  int64           `json:"request_count"`
	RequestBytes  int64           `json:"request_bytes"`
	ResponseBytes int64           `json:"response_bytes"`
	LatencyMS     int64           `json:"latency_ms"`
	Payload       json.RawMessage `json:"payload"`
}

type APIUsageLedgerListRequest struct {
	Limit       int
	TenantID    string
	PrincipalID string
	From        time.Time
	To          time.Time
}

const APIUsageLedgerNoLimit = -1

type APIUsageLedgerRetentionRequest struct {
	Cutoff      time.Time
	TenantID    string
	PrincipalID string
}

type APIUsageLedgerRetentionRunRequest struct {
	Cutoff       time.Time
	TenantID     string
	PrincipalID  string
	Limit        int
	DryRun       bool
	ConfirmReady bool
}

type APIUsageLedgerRetentionRunListRequest struct {
	Limit       int
	TenantID    string
	PrincipalID string
	CreatedFrom time.Time
	CreatedTo   time.Time
}

type APIUsageLedgerStatsView struct {
	EventCount       int64      `json:"event_count"`
	RequestCount     int64      `json:"request_count"`
	RequestBytes     int64      `json:"request_bytes"`
	ResponseBytes    int64      `json:"response_bytes"`
	LatencyMSTotal   int64      `json:"latency_ms_total"`
	LatencyMSMax     int64      `json:"latency_ms_max"`
	FirstEventAt     *time.Time `json:"first_event_at,omitempty"`
	LastEventAt      *time.Time `json:"last_event_at,omitempty"`
	LatencyMSAverage float64    `json:"latency_ms_average"`
}

type APIUsageLedgerRetentionReadinessView struct {
	Cutoff                         time.Time  `json:"cutoff"`
	TenantID                       string     `json:"tenant_id,omitempty"`
	PrincipalID                    string     `json:"principal_id,omitempty"`
	CandidateEventCount            int64      `json:"candidate_event_count"`
	CandidateRequestCount          int64      `json:"candidate_request_count"`
	CandidateRequestBytes          int64      `json:"candidate_request_bytes"`
	CandidateResponseBytes         int64      `json:"candidate_response_bytes"`
	CandidateLatencyMSTotal        int64      `json:"candidate_latency_ms_total"`
	CandidateLatencyMSMax          int64      `json:"candidate_latency_ms_max"`
	FirstCandidateEventAt          *time.Time `json:"first_candidate_event_at,omitempty"`
	LastCandidateEventAt           *time.Time `json:"last_candidate_event_at,omitempty"`
	LatestCandidateRecordedAt      *time.Time `json:"latest_candidate_recorded_at,omitempty"`
	CoveringExportBatchID          string     `json:"covering_export_batch_id,omitempty"`
	CoveringExportBatchCompletedAt *time.Time `json:"covering_export_batch_completed_at,omitempty"`
	CoveringExportBatchWindowStart *time.Time `json:"covering_export_batch_window_start,omitempty"`
	CoveringExportBatchWindowEnd   *time.Time `json:"covering_export_batch_window_end,omitempty"`
	CoveringExportBatchEventCount  int64      `json:"covering_export_batch_event_count,omitempty"`
	CoveringArtifactCount          int64      `json:"covering_artifact_count"`
	CoveringArtifactEventCount     int64      `json:"covering_artifact_event_count"`
	CoveringManifestDigestCount    int64      `json:"covering_manifest_digest_count"`
	CoveringManifestSignatureCount int64      `json:"covering_manifest_signature_count"`
	Ready                          bool       `json:"ready"`
	BlockingReasons                []string   `json:"blocking_reasons"`
}

type APIUsageLedgerRetentionRunView struct {
	ID             string                               `json:"id"`
	CreatedAt      time.Time                            `json:"created_at"`
	Cutoff         time.Time                            `json:"cutoff"`
	TenantID       string                               `json:"tenant_id,omitempty"`
	PrincipalID    string                               `json:"principal_id,omitempty"`
	Limit          int                                  `json:"limit"`
	DryRun         bool                                 `json:"dry_run"`
	ConfirmReady   bool                                 `json:"confirm_ready"`
	Ready          bool                                 `json:"ready"`
	CandidateCount int64                                `json:"candidate_count"`
	LimitedCount   int64                                `json:"limited_count"`
	DeletedCount   int64                                `json:"deleted_count"`
	Readiness      APIUsageLedgerRetentionReadinessView `json:"readiness"`
}

const (
	APIUsageLedgerRetentionDefaultLimit = 1000
	APIUsageLedgerRetentionMaxLimit     = 10000
)

type APIUsageExportBatchView struct {
	ID             string          `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Status         string          `json:"status"`
	ExportFormat   string          `json:"export_format"`
	TenantID       string          `json:"tenant_id,omitempty"`
	PrincipalID    string          `json:"principal_id,omitempty"`
	WindowStart    *time.Time      `json:"window_start,omitempty"`
	WindowEnd      *time.Time      `json:"window_end,omitempty"`
	EventCount     int64           `json:"event_count"`
	RequestCount   int64           `json:"request_count"`
	RequestBytes   int64           `json:"request_bytes"`
	ResponseBytes  int64           `json:"response_bytes"`
	LatencyMSTotal int64           `json:"latency_ms_total"`
	LatencyMSMax   int64           `json:"latency_ms_max"`
	FirstEventAt   *time.Time      `json:"first_event_at,omitempty"`
	LastEventAt    *time.Time      `json:"last_event_at,omitempty"`
	Manifest       json.RawMessage `json:"manifest"`
}

type APIUsageExportBatchListRequest struct {
	Limit       int
	TenantID    string
	PrincipalID string
	Status      string
	From        time.Time
	To          time.Time
}

type APIUsageExportArtifactView struct {
	ID             string          `json:"id"`
	BatchID        string          `json:"batch_id"`
	CreatedAt      time.Time       `json:"created_at"`
	StorageBackend string          `json:"storage_backend"`
	ObjectKey      string          `json:"object_key"`
	ContentType    string          `json:"content_type"`
	ByteCount      int64           `json:"byte_count"`
	SHA256Hex      string          `json:"sha256_hex"`
	EventCount     int64           `json:"event_count"`
	Metadata       json.RawMessage `json:"metadata"`
}

type APIUsageExportArtifactVerificationView struct {
	BatchID           string `json:"batch_id"`
	ArtifactID        string `json:"artifact_id"`
	ObjectKey         string `json:"object_key"`
	ExpectedByteCount int64  `json:"expected_byte_count"`
	ActualByteCount   int64  `json:"actual_byte_count"`
	ExpectedSHA256Hex string `json:"expected_sha256_hex"`
	ActualSHA256Hex   string `json:"actual_sha256_hex"`
	Valid             bool   `json:"valid"`
}

type APIUsageExportManifestDigestView struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batch_id"`
	CreatedAt       time.Time       `json:"created_at"`
	SchemaVersion   string          `json:"schema_version"`
	DigestAlgorithm string          `json:"digest_algorithm"`
	DigestHex       string          `json:"digest_hex"`
	Manifest        json.RawMessage `json:"manifest"`
}

type APIUsageExportManifestDigestVerificationView struct {
	BatchID           string          `json:"batch_id"`
	DigestID          string          `json:"digest_id"`
	SchemaVersion     string          `json:"schema_version"`
	DigestAlgorithm   string          `json:"digest_algorithm"`
	ExpectedDigestHex string          `json:"expected_digest_hex"`
	ActualDigestHex   string          `json:"actual_digest_hex"`
	Valid             bool            `json:"valid"`
	CanonicalManifest json.RawMessage `json:"canonical_manifest"`
}

type APIUsageExportManifestSignatureView struct {
	ID                 string          `json:"id"`
	DigestID           string          `json:"digest_id"`
	BatchID            string          `json:"batch_id"`
	CreatedAt          time.Time       `json:"created_at"`
	SignerBackend      string          `json:"signer_backend"`
	KeyID              string          `json:"key_id"`
	SignatureAlgorithm string          `json:"signature_algorithm"`
	SignedDigestHex    string          `json:"signed_digest_hex"`
	SignatureHex       string          `json:"signature_hex"`
	Metadata           json.RawMessage `json:"metadata"`
}

type APIUsageExportManifestSignatureVerificationView struct {
	BatchID            string `json:"batch_id"`
	DigestID           string `json:"digest_id"`
	SignatureID        string `json:"signature_id"`
	SignerBackend      string `json:"signer_backend"`
	KeyID              string `json:"key_id"`
	SignatureAlgorithm string `json:"signature_algorithm"`
	SignedDigestHex    string `json:"signed_digest_hex"`
	ExpectedDigestHex  string `json:"expected_digest_hex"`
	Valid              bool   `json:"valid"`
}

type APIUsageExportHandoffView struct {
	BatchID                       string                                           `json:"batch_id"`
	BatchStatus                   string                                           `json:"batch_status"`
	BatchCompleted                bool                                             `json:"batch_completed"`
	EventCount                    int64                                            `json:"event_count"`
	ArtifactCount                 int64                                            `json:"artifact_count"`
	ArtifactEventCount            int64                                            `json:"artifact_event_count"`
	ArtifactByteCount             int64                                            `json:"artifact_byte_count"`
	EventsCovered                 bool                                             `json:"events_covered"`
	ManifestDigestCount           int64                                            `json:"manifest_digest_count"`
	LatestManifestDigestID        string                                           `json:"latest_manifest_digest_id,omitempty"`
	LatestManifestDigestHex       string                                           `json:"latest_manifest_digest_hex,omitempty"`
	LatestManifestDigestAt        *time.Time                                       `json:"latest_manifest_digest_at,omitempty"`
	LatestDigestSignatureCount    int64                                            `json:"latest_digest_signature_count"`
	LatestSignatureID             string                                           `json:"latest_signature_id,omitempty"`
	LatestSignatureSigner         string                                           `json:"latest_signature_signer,omitempty"`
	LatestSignatureKeyID          string                                           `json:"latest_signature_key_id,omitempty"`
	LatestSignatureAt             *time.Time                                       `json:"latest_signature_at,omitempty"`
	Ready                         bool                                             `json:"ready"`
	ReadinessGrade                string                                           `json:"readiness_grade"`
	BillingReady                  bool                                             `json:"billing_ready"`
	MissingRequirements           []string                                         `json:"missing_requirements,omitempty"`
	BillingBlockingReasons        []string                                         `json:"billing_blocking_reasons,omitempty"`
	DeepVerification              bool                                             `json:"deep_verification"`
	DeepReady                     bool                                             `json:"deep_ready"`
	VerifiedBillingReady          bool                                             `json:"verified_billing_ready"`
	DeepBlockingReasons           []string                                         `json:"deep_blocking_reasons,omitempty"`
	DeepVerificationErrors        []string                                         `json:"deep_verification_errors,omitempty"`
	ManifestArtifactCoverageValid *bool                                            `json:"manifest_artifact_coverage_valid,omitempty"`
	ArtifactVerifications         []APIUsageExportArtifactVerificationView         `json:"artifact_verifications,omitempty"`
	ManifestDigestVerification    *APIUsageExportManifestDigestVerificationView    `json:"manifest_digest_verification,omitempty"`
	ManifestSignatureVerification *APIUsageExportManifestSignatureVerificationView `json:"manifest_signature_verification,omitempty"`
}

type APIUsageExportCapabilityView struct {
	ExportFormat                                string   `json:"export_format"`
	ArtifactContentType                         string   `json:"artifact_content_type"`
	ManifestDigestAlgorithm                     string   `json:"manifest_digest_algorithm"`
	SignerBackend                               string   `json:"signer_backend"`
	SignerConfigured                            bool     `json:"signer_configured"`
	SignerKeyID                                 string   `json:"signer_key_id,omitempty"`
	VerifierConfigured                          bool     `json:"verifier_configured"`
	ProductionSignatureReady                    bool     `json:"production_signature_ready"`
	BillingReadySupported                       bool     `json:"billing_ready_supported"`
	VerifiedBillingReadySupported               bool     `json:"verified_billing_ready_supported"`
	RetentionRunsSupported                      bool     `json:"retention_runs_supported"`
	RetentionWorkerSupported                    bool     `json:"retention_worker_supported"`
	RetentionWorkerDestructiveRequiresRemoteKey bool     `json:"retention_worker_destructive_requires_remote_key"`
	BlockingReasons                             []string `json:"blocking_reasons,omitempty"`
}

type CreateAPIUsageExportManifestSignatureRequest struct {
	BatchID       string                           `json:"-"`
	DigestID      string                           `json:"-"`
	SignerBackend string                           `json:"signer_backend"`
	Signature     apimeter.ExportManifestSignature `json:"-"`
	Metadata      json.RawMessage                  `json:"metadata,omitempty"`
}

type CreateAPIUsageExportArtifactRequest struct {
	BatchID        string          `json:"-"`
	StorageBackend string          `json:"storage_backend"`
	ObjectKey      string          `json:"object_key"`
	ContentType    string          `json:"content_type"`
	ByteCount      int64           `json:"byte_count"`
	SHA256Hex      string          `json:"sha256_hex"`
	EventCount     int64           `json:"event_count"`
	Metadata       json.RawMessage `json:"metadata"`
}

type WriteAPIUsageExportArtifactRequest struct {
	ObjectKey      string          `json:"object_key,omitempty"`
	StorageBackend string          `json:"storage_backend,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type DeliveryAttemptView struct {
	ID                string    `json:"id"`
	MessageID         string    `json:"message_id"`
	RFCMessageID      string    `json:"rfc_message_id"`
	Farm              string    `json:"farm"`
	Sender            string    `json:"sender,omitempty"`
	Recipient         string    `json:"recipient"`
	RecipientDomain   string    `json:"recipient_domain"`
	Status            string    `json:"status"`
	EnhancedStatus    string    `json:"enhanced_status,omitempty"`
	ErrorMessage      string    `json:"error_message"`
	DSNReturn         string    `json:"dsn_return,omitempty"`
	DSNEnvelopeID     string    `json:"dsn_envelope_id,omitempty"`
	DSNNotify         []string  `json:"dsn_notify,omitempty"`
	OriginalRecipient string    `json:"original_recipient,omitempty"`
	AttemptedAt       time.Time `json:"attempted_at"`
}

type DeliveryAttemptListRequest struct {
	Limit           int
	Status          string
	RecipientDomain string
	MessageID       string
	Farm            string
	Sender          string
	Since           time.Time
}

type DeliveryAttemptStatsRequest struct {
	Status          string
	RecipientDomain string
	MessageID       string
	Farm            string
	Sender          string
	Since           time.Time
}

type DeliveryAttemptStatsView struct {
	TotalAttempts    int64 `json:"total_attempts"`
	UniqueMessages   int64 `json:"unique_messages"`
	UniqueRecipients int64 `json:"unique_recipients"`
	Delivered        int64 `json:"delivered"`
	Failed           int64 `json:"failed"`
	Bounced          int64 `json:"bounced"`
	Exhausted        int64 `json:"exhausted"`
}

type deliveryAttemptScanner interface {
	Scan(dest ...any) error
}

func scanDeliveryAttempt(scanner deliveryAttemptScanner, attempt *DeliveryAttemptView) error {
	var dsnNotify json.RawMessage
	if err := scanner.Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.RFCMessageID,
		&attempt.Farm,
		&attempt.Sender,
		&attempt.Recipient,
		&attempt.RecipientDomain,
		&attempt.Status,
		&attempt.EnhancedStatus,
		&attempt.ErrorMessage,
		&attempt.DSNReturn,
		&attempt.DSNEnvelopeID,
		&dsnNotify,
		&attempt.OriginalRecipient,
		&attempt.AttemptedAt,
	); err != nil {
		return err
	}
	if len(dsnNotify) > 0 {
		if err := json.Unmarshal(dsnNotify, &attempt.DSNNotify); err != nil {
			return fmt.Errorf("decode delivery attempt dsn notify: %w", err)
		}
	}
	return nil
}

type ExhaustedAttemptListRequest struct {
	Limit           int
	RecipientDomain string
	MessageID       string
	Farm            string
	Sender          string
	Since           time.Time
}

type PushNotificationAttemptView struct {
	ID                string    `json:"id"`
	MessageID         string    `json:"message_id"`
	RFCMessageID      string    `json:"rfc_message_id"`
	CompanyID         string    `json:"company_id,omitempty"`
	DomainID          string    `json:"domain_id,omitempty"`
	UserID            string    `json:"user_id"`
	Recipient         string    `json:"recipient"`
	Subject           string    `json:"subject"`
	DeviceID          string    `json:"device_id,omitempty"`
	Platform          string    `json:"platform"`
	TokenSuffix       string    `json:"token_suffix,omitempty"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message"`
	ProviderMessageID string    `json:"provider_message_id,omitempty"`
	ProviderStatus    string    `json:"provider_status,omitempty"`
	AttemptedAt       time.Time `json:"attempted_at"`
}

type PushNotificationAttemptListRequest struct {
	Limit             int
	MessageID         string
	Status            string
	UserID            string
	Platform          string
	DeviceID          string
	ProviderStatus    string
	ProviderMessageID string
	Since             time.Time
}

type UpdatePushNotificationOutcomeRequest struct {
	AttemptID         string `json:"-"`
	Status            string `json:"status"`
	ErrorMessage      string `json:"error_message,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	ProviderStatus    string `json:"provider_status,omitempty"`
}

type PushNotificationStatsRequest struct {
	MessageID string
	UserID    string
	Platform  string
	DeviceID  string
	Since     time.Time
}

type PushNotificationStatsView struct {
	ActiveDevices int64 `json:"active_devices"`
	TotalAttempts int64 `json:"total_attempts"`
	Candidate     int64 `json:"candidate"`
	Queued        int64 `json:"queued"`
	Delivered     int64 `json:"delivered"`
	Failed        int64 `json:"failed"`
	InvalidToken  int64 `json:"invalid_token"`
}

const (
	maxPushNotificationFilterBytes     = 1024
	maxDeliveryRouteCredentialBytes    = 4096
	maxDeliveryRouteDescriptionBytes   = 512
	maxDeliveryRouteOperationalIDBytes = 1024
)

type SuppressionEntry struct {
	ID              string    `json:"id"`
	DomainID        string    `json:"domain_id"`
	Email           string    `json:"email"`
	Reason          string    `json:"reason"`
	SourceMessageID string    `json:"source_message_id"`
	CreatedAt       time.Time `json:"created_at"`
}

type SuppressionEntryListRequest struct {
	Limit    int
	DomainID string
	Email    string
	Reason   string
}

type DomainStatsView struct {
	DomainID          string `json:"domain_id"`
	ActiveUsers       int64  `json:"active_users"`
	TotalUsers        int64  `json:"total_users"`
	ActiveMessages    int64  `json:"active_messages"`
	InboundMessages   int64  `json:"inbound_messages"`
	OutboundMessages  int64  `json:"outbound_messages"`
	StorageUsedBytes  int64  `json:"storage_used_bytes"`
	StorageLimitBytes int64  `json:"storage_limit_bytes"`
	Delivered24h      int64  `json:"delivered_24h"`
	Failed24h         int64  `json:"failed_24h"`
	SuppressionCount  int64  `json:"suppression_count"`
}

type CompanyListRequest struct {
	Limit  int
	Status string
}

type CreateCompanyRequest struct {
	Name       string `json:"name"`
	QuotaLimit int64  `json:"quota_limit"`
}

type TrustedRelayView struct {
	ID          string    `json:"id"`
	CIDR        string    `json:"cidr"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type TrustedRelayListRequest struct {
	Limit       int
	CIDR        string
	Description string
}

type DeliveryRouteView struct {
	ID            string    `json:"id"`
	DomainPattern string    `json:"domain_pattern"`
	Farm          string    `json:"farm"`
	Hosts         []string  `json:"hosts"`
	Port          int       `json:"port"`
	TLSMode       string    `json:"tls_mode"`
	ImplicitTLS   bool      `json:"implicit_tls"`
	SMTPHello     string    `json:"smtp_hello"`
	PoolName      string    `json:"pool_name"`
	AuthIdentity  string    `json:"auth_identity,omitempty"`
	AuthUsername  string    `json:"auth_username,omitempty"`
	AuthPassword  string    `json:"-"`
	Status        string    `json:"status"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeliveryRouteListRequest struct {
	Limit         int
	Status        string
	Farm          string
	DomainPattern string
}

type DeliveryRouteResolveView struct {
	Domain  string             `json:"domain"`
	Matched bool               `json:"matched"`
	Route   *DeliveryRouteView `json:"route,omitempty"`
}

type DomainDNSCheckView struct {
	ID        string                `json:"id"`
	DomainID  string                `json:"domain_id"`
	Status    string                `json:"status"`
	Report    dnscheck.DomainReport `json:"report"`
	CheckedAt time.Time             `json:"checked_at"`
}

type DomainDNSCheckListRequest struct {
	DomainID string
	Limit    int
	Status   string
	Since    time.Time
}

type DomainView struct {
	ID                   string     `json:"id"`
	CompanyID            string     `json:"company_id"`
	CompanyName          string     `json:"company_name"`
	Name                 string     `json:"name"`
	NameACE              string     `json:"name_ace"`
	Status               string     `json:"status"`
	QuotaUsed            int64      `json:"quota_used"`
	QuotaLimit           int64      `json:"quota_limit"`
	QuotaRemaining       int64      `json:"quota_remaining"`
	DefaultUserQuota     int64      `json:"default_user_quota,omitempty"`
	AllocatedUserQuota   int64      `json:"allocated_user_quota"`
	AllocatableUserQuota int64      `json:"allocatable_user_quota"`
	OverAllocated        bool       `json:"over_allocated"`
	LastDNSCheckStatus   string     `json:"last_dns_check_status,omitempty"`
	LastDNSCheckedAt     *time.Time `json:"last_dns_checked_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type DomainListRequest struct {
	Limit     int
	CompanyID string
	Status    string
	DNSStatus string
}

type UserView struct {
	ID                 string    `json:"id"`
	DomainID           string    `json:"domain_id"`
	Username           string    `json:"username"`
	DisplayName        string    `json:"display_name"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	PasswordConfigured bool      `json:"password_configured"`
	MustChangePassword bool      `json:"must_change_password"`
	QuotaUsed          int64     `json:"quota_used"`
	QuotaLimit         int64     `json:"quota_limit,omitempty"`
	QuotaRemaining     int64     `json:"quota_remaining"`
	QuotaSource        string    `json:"quota_source"`
	CreatedAt          time.Time `json:"created_at"`
}

type UserListRequest struct {
	DomainID           string
	Status             string
	PasswordConfigured *bool
	Limit              int
}

type DomainPolicyView struct {
	DomainID                string    `json:"domain_id"`
	InboundMode             string    `json:"inbound_mode"`
	OutboundMode            string    `json:"outbound_mode"`
	MaxRecipientsPerMessage int       `json:"max_recipients_per_message,omitempty"`
	MaxMessageBytes         int64     `json:"max_message_bytes,omitempty"`
	MaxAttachmentBytes      int64     `json:"max_attachment_bytes,omitempty"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type UpdateDomainStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateDomainQuotaRequest struct {
	ID               string `json:"id"`
	QuotaLimit       int64  `json:"quota_limit"`
	DefaultUserQuota *int64 `json:"default_user_quota,omitempty"`
}

type UpdateCompanyQuotaRequest struct {
	ID         string `json:"id"`
	QuotaLimit int64  `json:"quota_limit"`
}

type UpdateCompanyRequest struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	QuotaLimit int64  `json:"quota_limit,omitempty"`
}

type UpdateDomainPolicyRequest struct {
	ID                      string `json:"id"`
	InboundMode             string `json:"inbound_mode"`
	OutboundMode            string `json:"outbound_mode"`
	MaxRecipientsPerMessage int    `json:"max_recipients_per_message,omitempty"`
	MaxMessageBytes         int64  `json:"max_message_bytes,omitempty"`
	MaxAttachmentBytes      int64  `json:"max_attachment_bytes,omitempty"`
}

type CreateDomainRequest struct {
	CompanyID  string `json:"company_id"`
	Name       string `json:"name"`
	NameACE    string `json:"name_ace"`
	QuotaLimit int64  `json:"quota_limit,omitempty"`
}

type CreateUserRequest struct {
	DomainID           string `json:"domain_id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Address            string `json:"address"`
	Password           string `json:"password,omitempty"`      // plain text; hashed by caller before reaching DB
	PasswordHash       string `json:"password_hash,omitempty"` // pre-hashed alternative
	MustChangePassword bool   `json:"must_change_password,omitempty"`
	QuotaLimit         int64  `json:"quota_limit,omitempty"`
}

type UpdateUserStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateUserQuotaRequest struct {
	ID          string `json:"id"`
	QuotaLimit  int64  `json:"quota_limit"`
	QuotaSource string `json:"quota_source,omitempty"`
}

type UpdateUserPasswordHashRequest struct {
	ID           string `json:"id"`
	PasswordHash string `json:"password_hash"`
}

type UpdateUserRoleRequest struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

var validUserRoles = map[string]bool{
	"user":         true,
	"company_admin": true,
	"system_admin": true,
}

func ValidateUpdateUserRoleRequest(req UpdateUserRoleRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if !validUserRoles[req.Role] {
		return fmt.Errorf("invalid role %q: must be user, company_admin, or system_admin", req.Role)
	}
	return nil
}

type CreateTrustedRelayRequest struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description,omitempty"`
}

type CreateDeliveryRouteRequest struct {
	DomainPattern string   `json:"domain_pattern"`
	Farm          string   `json:"farm,omitempty"`
	Hosts         []string `json:"hosts"`
	Port          int      `json:"port,omitempty"`
	TLSMode       string   `json:"tls_mode,omitempty"`
	ImplicitTLS   bool     `json:"implicit_tls,omitempty"`
	SMTPHello     string   `json:"smtp_hello,omitempty"`
	PoolName      string   `json:"pool_name,omitempty"`
	AuthIdentity  string   `json:"auth_identity,omitempty"`
	AuthUsername  string   `json:"auth_username,omitempty"`
	AuthPassword  string   `json:"auth_password,omitempty"`
	Description   string   `json:"description,omitempty"`
}

type UpdateDeliveryRouteStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func ValidateUpdateDomainStatusRequest(req UpdateDomainStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	if !isDomainStatus(normalizeAdminStatus(req.Status)) {
		return fmt.Errorf("unsupported domain status %q", req.Status)
	}
	return nil
}

func ValidateDomainListRequest(req DomainListRequest) error {
	if err := validatePushNotificationFilter("company_id", strings.TrimSpace(req.CompanyID)); err != nil {
		return err
	}
	status := normalizeAdminStatus(req.Status)
	if status != "" && !isDomainStatus(status) {
		return fmt.Errorf("unsupported domain status %q", req.Status)
	}
	dnsStatus := normalizeDNSStatus(req.DNSStatus)
	if dnsStatus != "" && !isDNSStatus(dnsStatus) {
		return fmt.Errorf("unsupported domain dns status %q", req.DNSStatus)
	}
	return nil
}

func isDomainStatus(status string) bool {
	switch status {
	case "active", "suspended", "disabled":
		return true
	default:
		return false
	}
}

func normalizeDNSStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isDNSStatus(status string) bool {
	switch dnscheck.Status(status) {
	case dnscheck.StatusOK, dnscheck.StatusMissing, dnscheck.StatusMismatch, dnscheck.StatusError:
		return true
	default:
		return false
	}
}

func ValidateUpdateDomainQuotaRequest(req UpdateDomainQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	if req.DefaultUserQuota != nil && *req.DefaultUserQuota < 0 {
		return fmt.Errorf("default_user_quota must not be negative")
	}
	return nil
}

func ValidateUpdateCompanyQuotaRequest(req UpdateCompanyQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("company id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func ValidateCompanyListRequest(req CompanyListRequest) error {
	status := normalizeAdminStatus(req.Status)
	if status != "" && !isCompanyStatus(status) {
		return fmt.Errorf("unsupported company status %q", req.Status)
	}
	return nil
}

func isCompanyStatus(status string) bool {
	switch status {
	case "active", "suspended", "disabled":
		return true
	default:
		return false
	}
}

func ValidateUpdateDomainPolicyRequest(req UpdateDomainPolicyRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	if _, err := normalizeDomainPolicyMode(req.InboundMode); err != nil {
		return fmt.Errorf("inbound_mode %w", err)
	}
	if _, err := normalizeDomainPolicyMode(req.OutboundMode); err != nil {
		return fmt.Errorf("outbound_mode %w", err)
	}
	if req.MaxRecipientsPerMessage < 0 {
		return fmt.Errorf("max_recipients_per_message must not be negative")
	}
	if req.MaxMessageBytes < 0 {
		return fmt.Errorf("max_message_bytes must not be negative")
	}
	if req.MaxAttachmentBytes < 0 {
		return fmt.Errorf("max_attachment_bytes must not be negative")
	}
	return nil
}

func normalizeDomainPolicyMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "inherit", nil
	}
	switch value {
	case "inherit", "monitor", "enforce":
		return value, nil
	default:
		return "", fmt.Errorf("must be inherit, monitor, or enforce")
	}
}

func ValidateCreateDomainRequest(req CreateDomainRequest) error {
	if strings.TrimSpace(req.CompanyID) == "" {
		return fmt.Errorf("company_id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !validAdminDomainName(req.Name) {
		return fmt.Errorf("name must be a domain name")
	}
	if strings.TrimSpace(req.NameACE) != "" && !validAdminDomainName(req.NameACE) {
		return fmt.Errorf("name_ace must be a domain name")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func validAdminDomainName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 253 || strings.ContainsAny(name, " \t\r\n/\\") {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

func ValidateCreateUserRequest(req CreateUserRequest) error {
	if strings.TrimSpace(req.DomainID) == "" {
		return fmt.Errorf("domain_id is required")
	}
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if !validAdminUsername(req.Username) {
		return fmt.Errorf("username must be a local account name")
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return fmt.Errorf("display_name is required")
	}
	if strings.TrimSpace(req.Address) == "" {
		return fmt.Errorf("address is required")
	}
	if _, err := mail.NormalizeAddress(req.Address); err != nil {
		return err
	}
	local, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(req.Address)), "@")
	if local != strings.ToLower(strings.TrimSpace(req.Username)) {
		return fmt.Errorf("address local part must match username")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	if strings.TrimSpace(req.PasswordHash) != "" {
		if err := auth.ValidatePasswordHash(req.PasswordHash); err != nil {
			return err
		}
	}
	return nil
}

func validAdminUsername(username string) bool {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 64 || strings.ContainsAny(username, " \t\r\n@/\\") {
		return false
	}
	if strings.HasPrefix(username, ".") || strings.HasSuffix(username, ".") || strings.Contains(username, "..") {
		return false
	}
	return true
}

func ValidateCreateTrustedRelayRequest(req CreateTrustedRelayRequest) error {
	if _, err := normalizeTrustedRelayCIDR(req.CIDR); err != nil {
		return err
	}
	if strings.ContainsAny(req.Description, "\r\n") {
		return fmt.Errorf("description must not contain newlines")
	}
	if len(req.Description) > 512 {
		return fmt.Errorf("description is too long")
	}
	return nil
}

func ValidateTrustedRelayListRequest(req TrustedRelayListRequest) error {
	if strings.TrimSpace(req.CIDR) != "" {
		if _, err := normalizeTrustedRelayCIDR(req.CIDR); err != nil {
			return err
		}
	}
	if err := validatePushNotificationFilter("description", strings.TrimSpace(req.Description)); err != nil {
		return err
	}
	return nil
}

func ValidateCreateDeliveryRouteRequest(req CreateDeliveryRouteRequest) error {
	if _, err := normalizeDeliveryRouteDomainPattern(req.DomainPattern); err != nil {
		return err
	}
	if _, err := normalizeDeliveryRouteHosts(req.Hosts); err != nil {
		return err
	}
	if req.Port != 0 && (req.Port < 1 || req.Port > 65535) {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if _, err := normalizeDeliveryRouteTLSMode(req.TLSMode); err != nil {
		return err
	}
	if req.ImplicitTLS && strings.EqualFold(strings.TrimSpace(req.TLSMode), "disable") {
		return fmt.Errorf("implicit_tls cannot be used with disabled tls_mode")
	}
	if strings.TrimSpace(req.AuthPassword) != "" && strings.TrimSpace(req.AuthUsername) == "" {
		return fmt.Errorf("auth_username is required when auth_password is set")
	}
	for field, value := range map[string]string{
		"farm":        req.Farm,
		"smtp_hello":  req.SMTPHello,
		"pool_name":   req.PoolName,
		"description": req.Description,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain newlines", field)
		}
		if len(strings.TrimSpace(value)) > maxDeliveryRouteOperationalIDBytes {
			return fmt.Errorf("%s is too long", field)
		}
	}
	for field, value := range map[string]string{
		"auth_identity": req.AuthIdentity,
		"auth_username": req.AuthUsername,
		"auth_password": req.AuthPassword,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain newlines", field)
		}
		if len(strings.TrimSpace(value)) > maxDeliveryRouteCredentialBytes {
			return fmt.Errorf("%s is too long", field)
		}
	}
	if len(req.Description) > maxDeliveryRouteDescriptionBytes {
		return fmt.Errorf("description is too long")
	}
	return nil
}

func ValidateUpdateDeliveryRouteStatusRequest(req UpdateDeliveryRouteStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("delivery route id is required")
	}
	if !isDeliveryRouteStatus(strings.ToLower(strings.TrimSpace(req.Status))) {
		return fmt.Errorf("unsupported delivery route status %q", req.Status)
	}
	return nil
}

func ValidateDeliveryRouteListRequest(req DeliveryRouteListRequest) error {
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isDeliveryRouteStatus(status) {
		return fmt.Errorf("unsupported delivery route status %q", req.Status)
	}
	for field, value := range map[string]string{
		"farm":           req.Farm,
		"domain_pattern": req.DomainPattern,
	} {
		if err := validatePushNotificationFilter(field, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return nil
}

func isDeliveryRouteStatus(status string) bool {
	switch status {
	case "active", "disabled":
		return true
	default:
		return false
	}
}

func normalizeDeliveryRouteDomainPattern(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("domain_pattern is required")
	}
	if value == "*" {
		return value, nil
	}
	if strings.HasPrefix(value, "*.") {
		suffix := strings.TrimPrefix(value, "*.")
		if !validAdminDomainName(suffix) {
			return "", fmt.Errorf("domain_pattern wildcard suffix must be a domain name")
		}
		return "*." + suffix, nil
	}
	if !validAdminDomainName(value) {
		return "", fmt.Errorf("domain_pattern must be a domain name, wildcard domain, or *")
	}
	return value, nil
}

func normalizeDeliveryRouteHosts(hosts []string) ([]string, error) {
	normalized := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || strings.ContainsAny(host, " \t\r\n/\\") {
			return nil, fmt.Errorf("hosts must contain DNS names or IP literals")
		}
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			return nil, fmt.Errorf("hosts must not include ports")
		}
		host = strings.Trim(host, "[]")
		if host == "" {
			return nil, fmt.Errorf("hosts must contain DNS names or IP literals")
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("hosts is required")
	}
	return normalized, nil
}

func normalizeDeliveryRouteTLSMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "opportunistic", nil
	}
	switch value {
	case "opportunistic", "require", "disable":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported tls_mode %q", value)
	}
}

func normalizeTrustedRelayCIDR(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("cidr is required")
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", fmt.Errorf("cidr must be an IP address or CIDR prefix")
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32).String(), nil
	}
	return netip.PrefixFrom(addr, 128).String(), nil
}

func (r *Repository) CreateDomain(ctx context.Context, req CreateDomainRequest) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDomainRequest(req); err != nil {
		return DomainView{}, err
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	nameACE := strings.ToLower(strings.TrimSpace(req.NameACE))
	if nameACE == "" {
		nameACE = name
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DomainView{}, fmt.Errorf("begin create domain transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO domains (company_id, name, name_ace, quota_limit)
VALUES ($1, $2, $3, NULLIF($4::bigint, 0))
RETURNING id::text, company_id::text, name, name_ace, status, quota_used, COALESCE(quota_limit, 0), created_at`

	var domain DomainView
	if err := tx.QueryRowContext(ctx, query, strings.TrimSpace(req.CompanyID), name, nameACE, req.QuotaLimit).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.CreatedAt,
	); err != nil {
		return DomainView{}, fmt.Errorf("create domain: %w", err)
	}
	detail, err := domainCreateAuditDetail(domain)
	if err != nil {
		return DomainView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  domain.CompanyID,
		DomainID:   domain.ID,
		Category:   "admin",
		Action:     "domain.create",
		TargetType: "domain",
		TargetID:   domain.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return DomainView{}, fmt.Errorf("record domain create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DomainView{}, fmt.Errorf("commit create domain transaction: %w", err)
	}
	return domain, nil
}

func (r *Repository) CreateUser(ctx context.Context, req CreateUserRequest) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateUserRequest(req); err != nil {
		return UserView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserView{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer tx.Rollback()

	quotaSource := "default"
	if req.QuotaLimit > 0 {
		quotaSource = "custom"
	}

	const insertUser = `
INSERT INTO users (domain_id, username, display_name, password_hash, quota_limit, quota_source, must_change_password)
SELECT
  d.id,
  $2,
  $3,
  NULLIF($4, ''),
  CASE
    WHEN $5::bigint > 0 THEN $5::bigint
    ELSE NULLIF(COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0), 0)
  END,
  $6,
  $7
FROM domains d
WHERE d.id = $1
RETURNING id::text, domain_id::text, username, display_name, role, status, COALESCE(password_hash, '') <> '', must_change_password, quota_used, COALESCE(quota_limit, 0), quota_source, created_at`

	var user UserView
	if err := tx.QueryRowContext(ctx, insertUser, strings.TrimSpace(req.DomainID), strings.TrimSpace(req.Username), strings.TrimSpace(req.DisplayName), strings.TrimSpace(req.PasswordHash), req.QuotaLimit, quotaSource, req.MustChangePassword).Scan(
		&user.ID,
		&user.DomainID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&user.PasswordConfigured,
		&user.MustChangePassword,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.QuotaSource,
		&user.CreatedAt,
	); err != nil {
		return UserView{}, fmt.Errorf("create user: %w", err)
	}
	if err := createPrimaryAddress(ctx, tx, user.ID, user.DomainID, req.Address); err != nil {
		return UserView{}, err
	}
	if err := createSystemFolders(ctx, tx, user.ID); err != nil {
		return UserView{}, err
	}
	var companyID string
	if err := tx.QueryRowContext(ctx, `SELECT company_id::text FROM domains WHERE id = $1`, user.DomainID).Scan(&companyID); err != nil {
		return UserView{}, fmt.Errorf("lookup user company for audit: %w", err)
	}
	detail, err := userCreateAuditDetail(userCreateAuditView{
		User:      user,
		CompanyID: companyID,
		Address:   strings.ToLower(strings.TrimSpace(req.Address)),
	})
	if err != nil {
		return UserView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  companyID,
		DomainID:   user.DomainID,
		UserID:     user.ID,
		Category:   "admin",
		Action:     "user.create",
		TargetType: "user",
		TargetID:   user.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return UserView{}, fmt.Errorf("record user create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("commit create user transaction: %w", err)
	}
	return user, nil
}

func createPrimaryAddress(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, userID string, domainID string, address string) error {
	address = strings.ToLower(strings.TrimSpace(address))
	local, domainACE, ok := strings.Cut(address, "@")
	if !ok || local == "" || domainACE == "" {
		return fmt.Errorf("address must be an email address")
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_addresses (user_id, domain_id, local_part, local_part_ace, domain_ace, address, address_ace, is_primary)
VALUES ($1, $2, $3, $3, $4, $5, $5, true)`, userID, domainID, local, domainACE, address); err != nil {
		return fmt.Errorf("create primary user address: %w", err)
	}
	return nil
}

func createSystemFolders(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, userID string) error {
	folders := []struct {
		name       string
		systemType string
	}{
		{"Inbox", "inbox"},
		{"Drafts", "drafts"},
		{"Sent", "sent"},
		{"Trash", "trash"},
	}
	for i, folder := range folders {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO folders (user_id, name, full_path, type, system_type, order_index)
VALUES ($1, $2, $3, 'system', $4, $5)
ON CONFLICT (user_id, full_path) DO NOTHING`, userID, folder.name, "/"+folder.name, folder.systemType, i); err != nil {
			return fmt.Errorf("create %s folder: %w", folder.systemType, err)
		}
	}
	return nil
}

func ValidateUpdateUserStatusRequest(req UpdateUserStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if !isUserStatus(normalizeAdminStatus(req.Status)) {
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
	return nil
}

func ValidateUserListRequest(req UserListRequest) error {
	status := normalizeAdminStatus(req.Status)
	if status != "" && !isUserStatus(status) {
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
	return nil
}

func isUserStatus(status string) bool {
	switch status {
	case "active", "suspended", "disabled":
		return true
	default:
		return false
	}
}

func ValidateUpdateUserQuotaRequest(req UpdateUserQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	if _, err := normalizeQuotaSource(req.QuotaSource, "custom"); err != nil {
		return err
	}
	return nil
}

func ValidateUpdateUserPasswordHashRequest(req UpdateUserPasswordHashRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if err := auth.ValidatePasswordHash(req.PasswordHash); err != nil {
		return err
	}
	return nil
}

func normalizeQuotaSource(value string, fallback string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	switch value {
	case "default", "custom":
		return value, nil
	default:
		return "", fmt.Errorf("quota_source must be default or custom")
	}
}

func (r *Repository) UpdateDomainStatus(ctx context.Context, req UpdateDomainStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin domain status transaction: %w", err)
	}
	defer tx.Rollback()

	var view domainStatusAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE domains
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, status`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status)).Scan(
		&view.ID,
		&view.CompanyID,
		&view.Name,
		&view.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", req.ID)
		}
		return fmt.Errorf("update domain status: %w", err)
	}
	detail, err := domainStatusAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.ID,
		Category:   "admin",
		Action:     "domain.status_update",
		TargetType: "domain",
		TargetID:   view.ID,
		Result:     view.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record domain status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit domain status transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateDomainQuota(ctx context.Context, req UpdateDomainQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainQuotaRequest(req); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update domain quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view domainQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE domains
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, COALESCE(quota_limit, 0)`, strings.TrimSpace(req.ID), req.QuotaLimit).Scan(
		&view.ID,
		&view.CompanyID,
		&view.Name,
		&view.QuotaLimit,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", req.ID)
		}
		return fmt.Errorf("update domain quota: %w", err)
	}
	if req.DefaultUserQuota != nil {
		defaultQuota := *req.DefaultUserQuota
		view.DefaultUserQuotaSet = true
		view.DefaultUserQuota = defaultQuota
		if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET settings = jsonb_set(settings, '{policy,default_user_quota}', to_jsonb($2::bigint), true),
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), defaultQuota); err != nil {
			return fmt.Errorf("update domain default user quota: %w", err)
		}
		result, err := tx.ExecContext(ctx, `
UPDATE users
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE domain_id = $1
  AND quota_source = 'default'`, strings.TrimSpace(req.ID), defaultQuota)
		if err != nil {
			return fmt.Errorf("apply domain default user quota: %w", err)
		}
		if affected, err := result.RowsAffected(); err == nil {
			view.DefaultUserQuotaUserUpdates = affected
		}
	}
	detail, err := domainQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.ID,
		Category:   "admin",
		Action:     "domain.quota_update",
		TargetType: "domain",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record domain quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update domain quota transaction: %w", err)
	}
	return nil
}

func (r *Repository) ListCompanies(ctx context.Context, req CompanyListRequest) ([]CompanyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateCompanyListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)

	rows, err := r.db.QueryContext(ctx, `
SELECT
  id::text,
  name,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM domains child
    WHERE child.company_id = companies.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_domain_quota,
  created_at
FROM companies
WHERE ($1 = '' OR status = $1)
ORDER BY created_at DESC
LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	defer rows.Close()

	var companies []CompanyView
	for rows.Next() {
		var company CompanyView
		if err := rows.Scan(
			&company.ID,
			&company.Name,
			&company.Status,
			&company.QuotaUsed,
			&company.QuotaLimit,
			&company.AllocatedDomainQuota,
			&company.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}
		company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
		company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
		company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate companies: %w", err)
	}
	return companies, nil
}

func (r *Repository) GetCompany(ctx context.Context, id string) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CompanyView{}, fmt.Errorf("company id is required")
	}

	var company CompanyView
	if err := r.db.QueryRowContext(ctx, `
SELECT
  id::text,
  name,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM domains child
    WHERE child.company_id = companies.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_domain_quota,
  created_at
FROM companies
WHERE id = $1`, id).Scan(
		&company.ID,
		&company.Name,
		&company.Status,
		&company.QuotaUsed,
		&company.QuotaLimit,
		&company.AllocatedDomainQuota,
		&company.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return CompanyView{}, fmt.Errorf("company %q not found", id)
		}
		return CompanyView{}, fmt.Errorf("get company: %w", err)
	}
	company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
	company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
	company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
	return company, nil
}

func (r *Repository) CreateCompany(ctx context.Context, req CreateCompanyRequest) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return CompanyView{}, fmt.Errorf("company name is required")
	}
	if req.QuotaLimit < 0 {
		return CompanyView{}, fmt.Errorf("quota limit must be non-negative")
	}

	var company CompanyView
	if err := r.db.QueryRowContext(ctx, `
INSERT INTO companies (name, status, quota_limit, created_at)
VALUES ($1, 'active', $2, NOW())
RETURNING id::text, name, status, quota_used, COALESCE(quota_limit, 0), 0, created_at
	`, req.Name, req.QuotaLimit).Scan(
		&company.ID,
		&company.Name,
		&company.Status,
		&company.QuotaUsed,
		&company.QuotaLimit,
		&company.AllocatedDomainQuota,
		&company.CreatedAt,
	); err != nil {
		return CompanyView{}, fmt.Errorf("create company: %w", err)
	}
	company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
	company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
	company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
	return company, nil
}

func (r *Repository) UpdateCompanyQuota(ctx context.Context, req UpdateCompanyQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateCompanyQuotaRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view companyQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE companies
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE id = $1
RETURNING id::text, name, status, COALESCE(quota_limit, 0)`, strings.TrimSpace(req.ID), req.QuotaLimit).Scan(
		&view.ID,
		&view.Name,
		&view.Status,
		&view.QuotaLimit,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("company %q not found", req.ID)
		}
		return fmt.Errorf("update company quota: %w", err)
	}
	detail, err := companyQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.ID,
		Category:   "admin",
		Action:     "company.quota_update",
		TargetType: "company",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record company quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company quota transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateDomainPolicy(ctx context.Context, req UpdateDomainPolicyRequest) (DomainPolicyView, error) {
	if r.db == nil {
		return DomainPolicyView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainPolicyRequest(req); err != nil {
		return DomainPolicyView{}, err
	}
	inboundMode, _ := normalizeDomainPolicyMode(req.InboundMode)
	outboundMode, _ := normalizeDomainPolicyMode(req.OutboundMode)
	policy := DomainPolicyView{
		DomainID:                strings.TrimSpace(req.ID),
		InboundMode:             inboundMode,
		OutboundMode:            outboundMode,
		MaxRecipientsPerMessage: req.MaxRecipientsPerMessage,
		MaxMessageBytes:         req.MaxMessageBytes,
		MaxAttachmentBytes:      req.MaxAttachmentBytes,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("marshal domain policy: %w", err)
	}

	const query = `
UPDATE domains
SET settings = jsonb_set(settings, '{policy}', COALESCE(settings->'policy', '{}'::jsonb) || $2::jsonb, true),
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, updated_at`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("begin domain policy transaction: %w", err)
	}
	defer tx.Rollback()

	var auditView domainPolicyAuditView
	if err := tx.QueryRowContext(ctx, query, policy.DomainID, policyJSON).Scan(
		&auditView.ID,
		&auditView.CompanyID,
		&auditView.Name,
		&policy.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainPolicyView{}, fmt.Errorf("domain %q not found", req.ID)
		}
		return DomainPolicyView{}, fmt.Errorf("update domain policy: %w", err)
	}
	auditView.Policy = policy
	detail, err := domainPolicyAuditDetail(auditView)
	if err != nil {
		return DomainPolicyView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  auditView.CompanyID,
		DomainID:   auditView.ID,
		Category:   "admin",
		Action:     "domain.policy_update",
		TargetType: "domain",
		TargetID:   auditView.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return DomainPolicyView{}, fmt.Errorf("record domain policy audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DomainPolicyView{}, fmt.Errorf("commit domain policy transaction: %w", err)
	}
	return policy, nil
}

func (r *Repository) UpdateUserStatus(ctx context.Context, req UpdateUserStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user status transaction: %w", err)
	}
	defer tx.Rollback()

	var view userStatusAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET status = $2,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, u.status`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status)).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user status: %w", err)
	}
	detail, err := userStatusAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.status_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     view.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user status transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateUserQuota(ctx context.Context, req UpdateUserQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserQuotaRequest(req); err != nil {
		return err
	}
	quotaSource, _ := normalizeQuotaSource(req.QuotaSource, "custom")

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view userQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET quota_limit = CASE
      WHEN $3 = 'default' THEN NULLIF(COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0), 0)
      ELSE NULLIF($2::bigint, 0)
    END,
    quota_source = $3,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, COALESCE(u.quota_limit, 0), u.quota_source`, strings.TrimSpace(req.ID), req.QuotaLimit, quotaSource).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.QuotaLimit,
		&view.QuotaSource,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user quota: %w", err)
	}
	detail, err := userQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.quota_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user quota transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateUserPasswordHash(ctx context.Context, req UpdateUserPasswordHashRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserPasswordHashRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user password hash transaction: %w", err)
	}
	defer tx.Rollback()

	var view userCredentialAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET password_hash = $2,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, COALESCE(u.password_hash, '') <> ''`, strings.TrimSpace(req.ID), strings.TrimSpace(req.PasswordHash)).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.PasswordConfigured,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user password hash: %w", err)
	}
	detail, err := userCredentialAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.password_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user password hash audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user password hash transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateUserRole(ctx context.Context, req UpdateUserRoleRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserRoleRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE users SET role = $2, updated_at = now() WHERE id = $1::uuid`, strings.TrimSpace(req.ID), req.Role)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func normalizeAdminStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

type domainStatusAuditView struct {
	ID        string
	CompanyID string
	Name      string
	Status    string
}

func domainStatusAuditDetail(view domainStatusAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":  view.ID,
		"company_id": view.CompanyID,
		"name":       view.Name,
		"status":     view.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain status audit detail: %w", err)
	}
	return detail, nil
}

func domainCreateAuditDetail(domain DomainView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":   domain.ID,
		"company_id":  domain.CompanyID,
		"name":        domain.Name,
		"name_ace":    domain.NameACE,
		"status":      domain.Status,
		"quota_limit": domain.QuotaLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain create audit detail: %w", err)
	}
	return detail, nil
}

type userStatusAuditView struct {
	ID        string
	DomainID  string
	CompanyID string
	Username  string
	Status    string
}

func userStatusAuditDetail(view userStatusAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":    view.ID,
		"domain_id":  view.DomainID,
		"company_id": view.CompanyID,
		"username":   view.Username,
		"status":     view.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user status audit detail: %w", err)
	}
	return detail, nil
}

type userCreateAuditView struct {
	User      UserView
	CompanyID string
	Address   string
}

func userCreateAuditDetail(view userCreateAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":             view.User.ID,
		"domain_id":           view.User.DomainID,
		"company_id":          view.CompanyID,
		"username":            view.User.Username,
		"display_name":        view.User.DisplayName,
		"address":             view.Address,
		"role":                view.User.Role,
		"status":              view.User.Status,
		"password_configured": view.User.PasswordConfigured,
		"quota_limit":         view.User.QuotaLimit,
		"quota_source":        view.User.QuotaSource,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user create audit detail: %w", err)
	}
	return detail, nil
}

type userCredentialAuditView struct {
	ID                 string
	DomainID           string
	CompanyID          string
	Username           string
	PasswordConfigured bool
}

func userCredentialAuditDetail(view userCredentialAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":             view.ID,
		"domain_id":           view.DomainID,
		"company_id":          view.CompanyID,
		"username":            view.Username,
		"password_configured": view.PasswordConfigured,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user credential audit detail: %w", err)
	}
	return detail, nil
}

type companyQuotaAuditView struct {
	ID         string
	Name       string
	Status     string
	QuotaLimit int64
}

func companyQuotaAuditDetail(view companyQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"company_id":  view.ID,
		"name":        view.Name,
		"status":      view.Status,
		"quota_limit": view.QuotaLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal company quota audit detail: %w", err)
	}
	return detail, nil
}

type domainQuotaAuditView struct {
	ID                          string
	CompanyID                   string
	Name                        string
	QuotaLimit                  int64
	DefaultUserQuotaSet         bool
	DefaultUserQuota            int64
	DefaultUserQuotaUserUpdates int64
}

func domainQuotaAuditDetail(view domainQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":                       view.ID,
		"company_id":                      view.CompanyID,
		"name":                            view.Name,
		"quota_limit":                     view.QuotaLimit,
		"default_user_quota_set":          view.DefaultUserQuotaSet,
		"default_user_quota":              view.DefaultUserQuota,
		"default_user_quota_user_updates": view.DefaultUserQuotaUserUpdates,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain quota audit detail: %w", err)
	}
	return detail, nil
}

type userQuotaAuditView struct {
	ID          string
	DomainID    string
	CompanyID   string
	Username    string
	QuotaLimit  int64
	QuotaSource string
}

func userQuotaAuditDetail(view userQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":      view.ID,
		"domain_id":    view.DomainID,
		"company_id":   view.CompanyID,
		"username":     view.Username,
		"quota_limit":  view.QuotaLimit,
		"quota_source": view.QuotaSource,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user quota audit detail: %w", err)
	}
	return detail, nil
}

type domainPolicyAuditView struct {
	ID        string
	CompanyID string
	Name      string
	Policy    DomainPolicyView
}

func domainPolicyAuditDetail(view domainPolicyAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":                  view.ID,
		"company_id":                 view.CompanyID,
		"name":                       view.Name,
		"inbound_mode":               view.Policy.InboundMode,
		"outbound_mode":              view.Policy.OutboundMode,
		"max_recipients_per_message": view.Policy.MaxRecipientsPerMessage,
		"max_message_bytes":          view.Policy.MaxMessageBytes,
		"max_attachment_bytes":       view.Policy.MaxAttachmentBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain policy audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListUsers(ctx context.Context, req UserListRequest) ([]UserView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateUserListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)

	const query = `
SELECT
  id::text,
  domain_id::text,
  username,
  display_name,
  role,
  status,
  COALESCE(password_hash, '') <> '' AS password_configured,
  quota_used,
  COALESCE(quota_limit, 0),
  quota_source,
  created_at
FROM users
WHERE ($1 = '' OR domain_id::text = $1)
  AND ($2 = '' OR status = $2)
  AND ($3::boolean IS NULL OR (COALESCE(password_hash, '') <> '') = $3)
ORDER BY created_at DESC
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(req.DomainID), status, req.PasswordConfigured, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []UserView
	for rows.Next() {
		var user UserView
		if err := rows.Scan(
			&user.ID,
			&user.DomainID,
			&user.Username,
			&user.DisplayName,
			&user.Role,
			&user.Status,
			&user.PasswordConfigured,
			&user.QuotaUsed,
			&user.QuotaLimit,
			&user.QuotaSource,
			&user.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.QuotaRemaining = quotaRemaining(user.QuotaUsed, user.QuotaLimit)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r *Repository) GetUser(ctx context.Context, id string) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return UserView{}, fmt.Errorf("user id is required")
	}

	const query = `
SELECT
  id::text,
  domain_id::text,
  username,
  display_name,
  role,
  status,
  COALESCE(password_hash, '') <> '' AS password_configured,
  quota_used,
  COALESCE(quota_limit, 0),
  quota_source,
  created_at
FROM users
WHERE id = $1
LIMIT 1`

	var user UserView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.DomainID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&user.PasswordConfigured,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.QuotaSource,
		&user.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return UserView{}, fmt.Errorf("user %q not found", id)
		}
		return UserView{}, fmt.Errorf("get user: %w", err)
	}
	user.QuotaRemaining = quotaRemaining(user.QuotaUsed, user.QuotaLimit)
	return user, nil
}

func (r *Repository) ListDomains(ctx context.Context, req DomainListRequest) ([]DomainView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDomainListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)
	dnsStatus := normalizeDNSStatus(req.DNSStatus)

	const query = `
SELECT
  d.id::text,
  d.company_id::text,
  COALESCE(c.name, ''),
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM users child
    WHERE child.domain_id = d.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_user_quota,
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN companies c ON c.id = d.company_id
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC
  LIMIT 1
) latest ON true
WHERE ($1 = '' OR d.company_id::text = $1)
  AND ($2 = '' OR d.status = $2)
  AND ($3 = '' OR COALESCE(latest.status, '') = $3)
ORDER BY d.created_at DESC
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(req.CompanyID), status, dnsStatus, limit)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []DomainView
	for rows.Next() {
		var domain DomainView
		var lastDNSCheckedAt sql.NullTime
		if err := rows.Scan(
			&domain.ID,
			&domain.CompanyID,
			&domain.CompanyName,
			&domain.Name,
			&domain.NameACE,
			&domain.Status,
			&domain.QuotaUsed,
			&domain.QuotaLimit,
			&domain.DefaultUserQuota,
			&domain.AllocatedUserQuota,
			&domain.LastDNSCheckStatus,
			&lastDNSCheckedAt,
			&domain.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		domain.QuotaRemaining = quotaRemaining(domain.QuotaUsed, domain.QuotaLimit)
		domain.AllocatableUserQuota = quotaRemaining(domain.AllocatedUserQuota, domain.QuotaLimit)
		domain.OverAllocated = domain.QuotaLimit > 0 && domain.AllocatedUserQuota > domain.QuotaLimit
		if lastDNSCheckedAt.Valid {
			domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domains: %w", err)
	}
	return domains, nil
}

func (r *Repository) GetDomain(ctx context.Context, id string) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DomainView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT
  d.id::text,
  d.company_id::text,
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM users child
    WHERE child.domain_id = d.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_user_quota,
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC
  LIMIT 1
) latest ON true
WHERE d.id = $1
LIMIT 1`

	var domain DomainView
	var lastDNSCheckedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.DefaultUserQuota,
		&domain.AllocatedUserQuota,
		&domain.LastDNSCheckStatus,
		&lastDNSCheckedAt,
		&domain.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DomainView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainView{}, fmt.Errorf("get domain: %w", err)
	}
	if lastDNSCheckedAt.Valid {
		domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
	}
	domain.QuotaRemaining = quotaRemaining(domain.QuotaUsed, domain.QuotaLimit)
	domain.AllocatableUserQuota = quotaRemaining(domain.AllocatedUserQuota, domain.QuotaLimit)
	domain.OverAllocated = domain.QuotaLimit > 0 && domain.AllocatedUserQuota > domain.QuotaLimit
	return domain, nil
}

func (r *Repository) GetDomainStats(ctx context.Context, id string) (DomainStatsView, error) {
	if r.db == nil {
		return DomainStatsView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DomainStatsView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT
  d.id::text,
  (SELECT COUNT(*) FROM users WHERE domain_id = d.id AND status = 'active'),
  (SELECT COUNT(*) FROM users WHERE domain_id = d.id),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND status = 'active'),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND received_at IS NOT NULL AND sent_at IS NULL AND status = 'active'),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND sent_at IS NOT NULL AND status = 'active'),
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  (SELECT COUNT(*) FROM delivery_attempts da
   JOIN messages m ON m.id = da.message_id
   WHERE m.domain_id = d.id AND da.attempted_at > now() - INTERVAL '24 hours'
     AND da.status = 'delivered'),
  (SELECT COUNT(*) FROM delivery_attempts da
   JOIN messages m ON m.id = da.message_id
   WHERE m.domain_id = d.id AND da.attempted_at > now() - INTERVAL '24 hours'
     AND da.status IN ('failed', 'bounced', 'exhausted')),
  (SELECT COUNT(*) FROM suppression_list WHERE domain_id = d.id)
FROM domains d
WHERE d.id = $1
LIMIT 1`

	var stats DomainStatsView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&stats.DomainID,
		&stats.ActiveUsers,
		&stats.TotalUsers,
		&stats.ActiveMessages,
		&stats.InboundMessages,
		&stats.OutboundMessages,
		&stats.StorageUsedBytes,
		&stats.StorageLimitBytes,
		&stats.Delivered24h,
		&stats.Failed24h,
		&stats.SuppressionCount,
	); err != nil {
		if err == sql.ErrNoRows {
			return DomainStatsView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainStatsView{}, fmt.Errorf("get domain stats: %w", err)
	}
	return stats, nil
}

func ValidateDomainDNSCheckListRequest(req DomainDNSCheckListRequest) error {
	domainID := strings.TrimSpace(req.DomainID)
	if domainID == "" {
		return fmt.Errorf("domain id is required")
	}
	if err := validatePushNotificationFilter("domain_id", domainID); err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isDomainDNSCheckStatus(status) {
		return fmt.Errorf("unsupported domain dns check status %q", req.Status)
	}
	return nil
}

func isDomainDNSCheckStatus(status string) bool {
	switch dnscheck.Status(status) {
	case dnscheck.StatusOK, dnscheck.StatusMissing, dnscheck.StatusMismatch, dnscheck.StatusError:
		return true
	default:
		return false
	}
}

func (r *Repository) ListDomainDNSChecks(ctx context.Context, req DomainDNSCheckListRequest) ([]DomainDNSCheckView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDomainDNSCheckListRequest(req); err != nil {
		return nil, err
	}
	domainID := strings.TrimSpace(req.DomainID)
	limit := normalizeLimit(req.Limit)
	status := strings.ToLower(strings.TrimSpace(req.Status))
	since := sql.NullTime{}
	if !req.Since.IsZero() {
		since = sql.NullTime{Time: req.Since.UTC(), Valid: true}
	}

	const query = `
SELECT
  id::text,
  domain_id::text,
  status,
  report,
  checked_at
FROM domain_dns_checks
WHERE domain_id = $1
  AND ($2 = '' OR status = $2)
  AND ($3::timestamptz IS NULL OR checked_at >= $3)
ORDER BY checked_at DESC
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, domainID, status, since, limit)
	if err != nil {
		return nil, fmt.Errorf("list domain dns checks: %w", err)
	}
	defer rows.Close()

	var checks []DomainDNSCheckView
	for rows.Next() {
		var check DomainDNSCheckView
		var rawReport []byte
		if err := rows.Scan(
			&check.ID,
			&check.DomainID,
			&check.Status,
			&rawReport,
			&check.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain dns check: %w", err)
		}
		if err := json.Unmarshal(rawReport, &check.Report); err != nil {
			return nil, fmt.Errorf("decode domain dns check report: %w", err)
		}
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain dns checks: %w", err)
	}
	return checks, nil
}

func (r *Repository) VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error) {
	if r.db == nil {
		return dnscheck.DomainReport{}, fmt.Errorf("database handle is required")
	}
	domain, err := r.GetDomain(ctx, id)
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	keys, err := r.ListDKIMKeys(ctx, DKIMKeyListRequest{DomainID: id, Limit: 200})
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	expectations := make([]dnscheck.DKIMExpectation, 0, len(keys))
	for _, key := range keys {
		if normalizeAdminStatus(key.Status) != "active" {
			continue
		}
		expectations = append(expectations, dnscheck.DKIMExpectation{
			Selector:     key.Selector,
			PublicKeyDNS: key.PublicKeyDNS,
		})
	}
	name := strings.TrimSpace(domain.NameACE)
	if name == "" {
		name = domain.Name
	}
	report := dnscheck.Verifier{}.VerifyDomain(ctx, name, expectations)
	if err := r.recordDomainDNSCheck(ctx, domain, report); err != nil {
		return dnscheck.DomainReport{}, err
	}
	return report, nil
}

func (r *Repository) recordDomainDNSCheck(ctx context.Context, domain DomainView, report dnscheck.DomainReport) error {
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal domain dns check report: %w", err)
	}
	status := string(report.SummaryStatus())

	var checkID string
	if err := r.db.QueryRowContext(ctx, `
INSERT INTO domain_dns_checks (domain_id, status, report)
VALUES ($1, $2, $3)
RETURNING id::text`, domain.ID, status, reportJSON).Scan(&checkID); err != nil {
		return fmt.Errorf("record domain dns check: %w", err)
	}

	detailJSON, err := json.Marshal(map[string]any{
		"dns_check_id": checkID,
		"domain":       report.Domain,
		"status":       status,
	})
	if err != nil {
		return fmt.Errorf("marshal domain dns check audit detail: %w", err)
	}
	if err := audit.NewPostgresRepository(r.db).Insert(ctx, audit.Log{
		CompanyID:  domain.CompanyID,
		DomainID:   domain.ID,
		Category:   "admin",
		Action:     "domain.dns_check",
		TargetType: "domain",
		TargetID:   domain.ID,
		Result:     status,
		Detail:     detailJSON,
	}); err != nil {
		return fmt.Errorf("record domain dns check audit: %w", err)
	}
	return nil
}

func (r *Repository) ListQueueStats(ctx context.Context) ([]QueueStat, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  topic,
  status,
  count(*)::bigint,
  count(*) FILTER (WHERE status = 'pending' AND available_at <= now())::bigint,
  count(*) FILTER (WHERE status = 'pending' AND available_at > now())::bigint,
  count(*) FILTER (WHERE status = 'processing' AND locked_at < now() - interval '5 minutes')::bigint,
  min(created_at) FILTER (WHERE status = 'pending' AND available_at <= now()),
  min(available_at) FILTER (WHERE status = 'pending' AND available_at > now())
FROM outbox
GROUP BY topic, status
ORDER BY topic, status`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list queue stats: %w", err)
	}
	defer rows.Close()

	var stats []QueueStat
	for rows.Next() {
		var stat QueueStat
		var oldestReadyAt sql.NullTime
		var nextAvailableAt sql.NullTime
		if err := rows.Scan(
			&stat.Topic,
			&stat.Status,
			&stat.Count,
			&stat.ReadyCount,
			&stat.DelayedCount,
			&stat.StaleProcessingCount,
			&oldestReadyAt,
			&nextAvailableAt,
		); err != nil {
			return nil, fmt.Errorf("scan queue stat: %w", err)
		}
		if oldestReadyAt.Valid {
			stat.OldestReadyAt = &oldestReadyAt.Time
		}
		if nextAvailableAt.Valid {
			stat.NextAvailableAt = &nextAvailableAt.Time
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue stats: %w", err)
	}
	return stats, nil
}

func (r *Repository) ListOutboxEvents(ctx context.Context, req OutboxEventListRequest) ([]OutboxEventView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	req.Topic = strings.TrimSpace(req.Topic)
	req.PartitionKey = strings.TrimSpace(req.PartitionKey)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	if req.Status != "" && !allowedOutboxStatus(req.Status) {
		return nil, fmt.Errorf("unsupported outbox status")
	}

	const query = `
SELECT
  id::text,
  topic,
  partition_key,
  status,
  attempts,
  COALESCE(last_error, ''),
  created_at,
  available_at,
  locked_at,
  processed_at
FROM outbox
WHERE (NULLIF($2, '') IS NULL OR topic = $2)
  AND (NULLIF($3, '') IS NULL OR partition_key = $3)
  AND (NULLIF($4, '') IS NULL OR status = $4)
  AND ($5::timestamptz IS NULL OR created_at >= $5::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, req.Limit, req.Topic, req.PartitionKey, req.Status, nullableTime(req.Since))
	if err != nil {
		return nil, fmt.Errorf("list outbox events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEventView
	for rows.Next() {
		var event OutboxEventView
		var lockedAt sql.NullTime
		var processedAt sql.NullTime
		if err := rows.Scan(
			&event.ID,
			&event.Topic,
			&event.PartitionKey,
			&event.Status,
			&event.Attempts,
			&event.LastError,
			&event.CreatedAt,
			&event.AvailableAt,
			&lockedAt,
			&processedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.LastError = truncateUTF8Bytes(event.LastError, outboxEventListErrorPreviewBytes)
		if lockedAt.Valid {
			event.LockedAt = &lockedAt.Time
		}
		if processedAt.Valid {
			event.ProcessedAt = &processedAt.Time
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) GetOutboxEvent(ctx context.Context, id string) (OutboxEventView, error) {
	if r.db == nil {
		return OutboxEventView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return OutboxEventView{}, fmt.Errorf("outbox event id is required")
	}

	const query = `
SELECT
  id::text,
  topic,
  partition_key,
  status,
  attempts,
  COALESCE(last_error, ''),
  created_at,
  available_at,
  locked_at,
  processed_at
FROM outbox
WHERE id = $1`

	var event OutboxEventView
	var lockedAt sql.NullTime
	var processedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Topic,
		&event.PartitionKey,
		&event.Status,
		&event.Attempts,
		&event.LastError,
		&event.CreatedAt,
		&event.AvailableAt,
		&lockedAt,
		&processedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return OutboxEventView{}, fmt.Errorf("outbox event %q not found", id)
		}
		return OutboxEventView{}, fmt.Errorf("get outbox event: %w", err)
	}
	if lockedAt.Valid {
		event.LockedAt = &lockedAt.Time
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}
	return event, nil
}

func allowedOutboxStatus(status string) bool {
	switch status {
	case "pending", "processing", "done", "failed":
		return true
	default:
		return false
	}
}

func (r *Repository) ListQuotaUsage(ctx context.Context, req QuotaUsageListRequest) ([]QuotaUsageView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := normalizeQuotaUsageListRequest(req)
	if err != nil {
		return nil, err
	}

	const query = `
SELECT scope, id, domain_id, name, quota_used, quota_limit, allocated_quota, updated_at
FROM (
  SELECT
    'company' AS scope,
    id::text AS id,
    '' AS domain_id,
    name AS name,
    quota_used,
    quota_limit,
    COALESCE((
      SELECT SUM(child.quota_limit)
      FROM domains child
      WHERE child.company_id = companies.id
        AND child.quota_limit IS NOT NULL
        AND child.quota_limit > 0
    ), 0) AS allocated_quota,
    updated_at
  FROM companies
  WHERE quota_limit IS NOT NULL AND quota_limit > 0
  UNION ALL
  SELECT
    'domain' AS scope,
    id::text AS id,
    id::text AS domain_id,
    name AS name,
    quota_used,
    quota_limit,
    COALESCE((
      SELECT SUM(child.quota_limit)
      FROM users child
      WHERE child.domain_id = domains.id
        AND child.quota_limit IS NOT NULL
        AND child.quota_limit > 0
    ), 0) AS allocated_quota,
    updated_at
  FROM domains
  WHERE quota_limit IS NOT NULL AND quota_limit > 0
  UNION ALL
  SELECT
    'user' AS scope,
    users.id::text AS id,
    users.domain_id::text AS domain_id,
    users.username || '@' || domains.name_ace AS name,
    users.quota_used,
    users.quota_limit,
    0::bigint AS allocated_quota,
    users.updated_at
  FROM users
  JOIN domains ON domains.id = users.domain_id
  WHERE users.quota_limit IS NOT NULL AND users.quota_limit > 0
) usage
WHERE ($2 = '' OR scope = $2)
  AND ($3 = '' OR domain_id = $3)
  AND ($4::bool IS NULL OR (quota_used >= quota_limit) = $4)
  AND ($5::bool IS NULL OR (allocated_quota > quota_limit) = $5)
ORDER BY (quota_used::double precision / quota_limit::double precision) DESC, updated_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, req.Limit, req.Scope, req.DomainID, req.OverLimit, req.OverAllocated)
	if err != nil {
		return nil, fmt.Errorf("list quota usage: %w", err)
	}
	defer rows.Close()

	var usages []QuotaUsageView
	for rows.Next() {
		var usage QuotaUsageView
		if err := rows.Scan(
			&usage.Scope,
			&usage.ID,
			&usage.DomainID,
			&usage.Name,
			&usage.QuotaUsed,
			&usage.QuotaLimit,
			&usage.AllocatedQuota,
			&usage.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota usage: %w", err)
		}
		usage.QuotaRemaining = quotaRemaining(usage.QuotaUsed, usage.QuotaLimit)
		usage.AllocatableQuota = quotaRemaining(usage.AllocatedQuota, usage.QuotaLimit)
		usage.UsageRatio = quotaUsageRatio(usage.QuotaUsed, usage.QuotaLimit)
		usage.AllocationRatio = quotaUsageRatio(usage.AllocatedQuota, usage.QuotaLimit)
		usage.OverLimit = usage.QuotaLimit > 0 && usage.QuotaUsed >= usage.QuotaLimit
		usage.OverAllocated = usage.QuotaLimit > 0 && usage.AllocatedQuota > usage.QuotaLimit
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota usage: %w", err)
	}
	return usages, nil
}

func normalizeQuotaUsageListRequest(req QuotaUsageListRequest) (QuotaUsageListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	if req.Scope != "" {
		switch req.Scope {
		case "company", "domain", "user":
		default:
			return QuotaUsageListRequest{}, fmt.Errorf("unsupported quota usage scope %q", req.Scope)
		}
	}
	var err error
	if req.DomainID, err = normalizeAPIUsageAggregateFilter("domain_id", req.DomainID, false); err != nil {
		return QuotaUsageListRequest{}, err
	}
	return req, nil
}

func (r *Repository) ListAPIUsageDaily(ctx context.Context, req APIUsageAggregateListRequest) ([]APIUsageDailyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeAPIUsageAggregateListRequest(req)
	if err != nil {
		return nil, err
	}

	const query = `
SELECT
  day,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_seen_at,
  last_seen_at
FROM api_usage_daily
WHERE ($2 = '' OR tenant_id = $2)
  AND ($3 = '' OR company_id = $3)
  AND ($4 = '' OR domain_id = $4)
  AND ($5 = '' OR user_id = $5)
  AND ($6 = '' OR api_key_id = $6)
  AND ($7 = '' OR principal_id = $7)
  AND ($8 = '' OR auth_source = $8)
  AND ($9 = '' OR method = $9)
  AND ($10 = '' OR route = $10)
  AND ($11 = 0 OR status = $11)
  AND ($12::timestamptz IS NULL OR day >= $12::timestamptz)
  AND ($13::timestamptz IS NULL OR day < $13::timestamptz)
ORDER BY day DESC, request_count DESC, route, status
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query,
		filters.Limit,
		filters.TenantID,
		filters.CompanyID,
		filters.DomainID,
		filters.UserID,
		filters.APIKeyID,
		filters.PrincipalID,
		filters.AuthSource,
		filters.Method,
		filters.Route,
		filters.Status,
		nullableTime(filters.From),
		nullableTime(filters.To),
	)
	if err != nil {
		return nil, fmt.Errorf("list api usage daily: %w", err)
	}
	defer rows.Close()

	var usages []APIUsageDailyView
	for rows.Next() {
		var usage APIUsageDailyView
		if err := rows.Scan(
			&usage.Day,
			&usage.Method,
			&usage.Route,
			&usage.Status,
			&usage.TenantID,
			&usage.CompanyID,
			&usage.DomainID,
			&usage.UserID,
			&usage.APIKeyID,
			&usage.PrincipalID,
			&usage.AuthSource,
			&usage.RequestCount,
			&usage.RequestBytes,
			&usage.ResponseBytes,
			&usage.LatencyMSTotal,
			&usage.LatencyMSMax,
			&usage.FirstSeenAt,
			&usage.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan api usage daily: %w", err)
		}
		if usage.RequestCount > 0 {
			usage.LatencyMSAverage = float64(usage.LatencyMSTotal) / float64(usage.RequestCount)
		}
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage daily: %w", err)
	}
	return usages, nil
}

func (r *Repository) ListAPIUsageMonthly(ctx context.Context, req APIUsageAggregateListRequest) ([]APIUsageMonthlyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeAPIUsageAggregateListRequest(req)
	if err != nil {
		return nil, err
	}

	const query = `
SELECT
  month,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_seen_at,
  last_seen_at
FROM api_usage_monthly
WHERE ($2 = '' OR tenant_id = $2)
  AND ($3 = '' OR company_id = $3)
  AND ($4 = '' OR domain_id = $4)
  AND ($5 = '' OR user_id = $5)
  AND ($6 = '' OR api_key_id = $6)
  AND ($7 = '' OR principal_id = $7)
  AND ($8 = '' OR auth_source = $8)
  AND ($9 = '' OR method = $9)
  AND ($10 = '' OR route = $10)
  AND ($11 = 0 OR status = $11)
  AND ($12::timestamptz IS NULL OR month >= $12::timestamptz)
  AND ($13::timestamptz IS NULL OR month < $13::timestamptz)
ORDER BY month DESC, request_count DESC, route, status
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query,
		filters.Limit,
		filters.TenantID,
		filters.CompanyID,
		filters.DomainID,
		filters.UserID,
		filters.APIKeyID,
		filters.PrincipalID,
		filters.AuthSource,
		filters.Method,
		filters.Route,
		filters.Status,
		nullableTime(filters.From),
		nullableTime(filters.To),
	)
	if err != nil {
		return nil, fmt.Errorf("list api usage monthly: %w", err)
	}
	defer rows.Close()

	var usages []APIUsageMonthlyView
	for rows.Next() {
		var usage APIUsageMonthlyView
		if err := rows.Scan(
			&usage.Month,
			&usage.Method,
			&usage.Route,
			&usage.Status,
			&usage.TenantID,
			&usage.CompanyID,
			&usage.DomainID,
			&usage.UserID,
			&usage.APIKeyID,
			&usage.PrincipalID,
			&usage.AuthSource,
			&usage.RequestCount,
			&usage.RequestBytes,
			&usage.ResponseBytes,
			&usage.LatencyMSTotal,
			&usage.LatencyMSMax,
			&usage.FirstSeenAt,
			&usage.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan api usage monthly: %w", err)
		}
		if usage.RequestCount > 0 {
			usage.LatencyMSAverage = float64(usage.LatencyMSTotal) / float64(usage.RequestCount)
		}
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage monthly: %w", err)
	}
	return usages, nil
}

func normalizeAPIUsageAggregateListRequest(req APIUsageAggregateListRequest) (APIUsageAggregateListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	var err error
	if req.TenantID, err = normalizeAPIUsageAggregateFilter("tenant_id", req.TenantID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.CompanyID, err = normalizeAPIUsageAggregateFilter("company_id", req.CompanyID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.DomainID, err = normalizeAPIUsageAggregateFilter("domain_id", req.DomainID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.UserID, err = normalizeAPIUsageAggregateFilter("user_id", req.UserID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.APIKeyID, err = normalizeAPIUsageAggregateFilter("api_key_id", req.APIKeyID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.PrincipalID, err = normalizeAPIUsageAggregateFilter("principal_id", req.PrincipalID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.AuthSource, err = normalizeAPIUsageAggregateFilter("auth_source", req.AuthSource, true); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Method, err = normalizeAPIUsageAggregateFilter("method", req.Method, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Route, err = normalizeAPIUsageAggregateFilter("route", req.Route, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Status < 0 || req.Status > 599 || (req.Status > 0 && req.Status < 100) {
		return APIUsageAggregateListRequest{}, fmt.Errorf("status must be an HTTP-like status code")
	}
	if !req.From.IsZero() {
		req.From = req.From.UTC()
	}
	if !req.To.IsZero() {
		req.To = req.To.UTC()
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		return APIUsageAggregateListRequest{}, fmt.Errorf("from must be before to")
	}
	return req, nil
}

func normalizeAPIUsageAggregateFilter(name, value string, lower bool) (string, error) {
	value = strings.TrimSpace(value)
	if lower {
		value = strings.ToLower(value)
	}
	if err := validatePushNotificationFilter(name, value); err != nil {
		return "", err
	}
	return value, nil
}

func (r *Repository) ListAPIUsageLedger(ctx context.Context, req APIUsageLedgerListRequest) ([]APIUsageLedgerView, error) {
	var usages []APIUsageLedgerView
	if err := r.StreamAPIUsageLedger(ctx, req, func(usage APIUsageLedgerView) error {
		usages = append(usages, usage)
		return nil
	}); err != nil {
		return nil, err
	}
	return usages, nil
}

func (r *Repository) StreamAPIUsageLedger(ctx context.Context, req APIUsageLedgerListRequest, yield func(APIUsageLedgerView) error) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if yield == nil {
		return fmt.Errorf("api usage ledger yield function is required")
	}
	limit, unbounded := apiUsageLedgerStreamLimit(req.Limit)

	query := `
SELECT
  event_id,
  schema_version,
  event_timestamp,
  recorded_at,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms,
  payload
FROM api_usage_ledger`
	var conditions []string
	var args []any
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if principalID := strings.TrimSpace(req.PrincipalID); principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	if unbounded {
		query += `
ORDER BY event_timestamp DESC, event_id DESC
`
	} else {
		args = append(args, limit)
		query += fmt.Sprintf(`
ORDER BY event_timestamp DESC, event_id DESC
LIMIT $%d`, len(args))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("stream api usage ledger: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var usage APIUsageLedgerView
		if err := rows.Scan(
			&usage.EventID,
			&usage.SchemaVersion,
			&usage.EventTime,
			&usage.RecordedAt,
			&usage.Method,
			&usage.Route,
			&usage.Status,
			&usage.TenantID,
			&usage.CompanyID,
			&usage.DomainID,
			&usage.UserID,
			&usage.APIKeyID,
			&usage.PrincipalID,
			&usage.AuthSource,
			&usage.RequestCount,
			&usage.RequestBytes,
			&usage.ResponseBytes,
			&usage.LatencyMS,
			&usage.Payload,
		); err != nil {
			return fmt.Errorf("scan api usage ledger: %w", err)
		}
		if err := yield(usage); err != nil {
			return fmt.Errorf("yield api usage ledger: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate api usage ledger: %w", err)
	}
	return nil
}

func apiUsageLedgerStreamLimit(limit int) (int, bool) {
	if limit == APIUsageLedgerNoLimit {
		return 0, true
	}
	return normalizeLimit(limit), false
}

func (r *Repository) GetAPIUsageLedgerStats(ctx context.Context, req APIUsageLedgerListRequest) (APIUsageLedgerStatsView, error) {
	if r.db == nil {
		return APIUsageLedgerStatsView{}, fmt.Errorf("database handle is required")
	}
	query := `
SELECT
  count(*)::bigint,
  COALESCE(sum(request_count), 0)::bigint,
  COALESCE(sum(request_bytes), 0)::bigint,
  COALESCE(sum(response_bytes), 0)::bigint,
  COALESCE(sum(latency_ms), 0)::bigint,
  COALESCE(max(latency_ms), 0)::bigint,
  min(event_timestamp),
  max(event_timestamp)
FROM api_usage_ledger`
	var conditions []string
	var args []any
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if principalID := strings.TrimSpace(req.PrincipalID); principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	var stats APIUsageLedgerStatsView
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.EventCount,
		&stats.RequestCount,
		&stats.RequestBytes,
		&stats.ResponseBytes,
		&stats.LatencyMSTotal,
		&stats.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
	); err != nil {
		return APIUsageLedgerStatsView{}, fmt.Errorf("get api usage ledger stats: %w", err)
	}
	if firstEventAt.Valid {
		stats.FirstEventAt = &firstEventAt.Time
	}
	if lastEventAt.Valid {
		stats.LastEventAt = &lastEventAt.Time
	}
	if stats.RequestCount > 0 {
		stats.LatencyMSAverage = float64(stats.LatencyMSTotal) / float64(stats.RequestCount)
	}
	return stats, nil
}

func (r *Repository) GetAPIUsageLedgerRetentionReadiness(ctx context.Context, req APIUsageLedgerRetentionRequest) (APIUsageLedgerRetentionReadinessView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff must not be in the future")
	}

	view := APIUsageLedgerRetentionReadinessView{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
	}
	query := `
SELECT
  count(*)::bigint,
  COALESCE(sum(request_count), 0)::bigint,
  COALESCE(sum(request_bytes), 0)::bigint,
  COALESCE(sum(response_bytes), 0)::bigint,
  COALESCE(sum(latency_ms), 0)::bigint,
  COALESCE(max(latency_ms), 0)::bigint,
  min(event_timestamp),
  max(event_timestamp),
  max(recorded_at)
FROM api_usage_ledger`
	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	query += "\nWHERE " + strings.Join(conditions, "\n  AND ")

	var firstCandidateAt sql.NullTime
	var lastCandidateAt sql.NullTime
	var latestRecordedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&view.CandidateEventCount,
		&view.CandidateRequestCount,
		&view.CandidateRequestBytes,
		&view.CandidateResponseBytes,
		&view.CandidateLatencyMSTotal,
		&view.CandidateLatencyMSMax,
		&firstCandidateAt,
		&lastCandidateAt,
		&latestRecordedAt,
	); err != nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("get api usage ledger retention candidates: %w", err)
	}
	if firstCandidateAt.Valid {
		view.FirstCandidateEventAt = &firstCandidateAt.Time
	}
	if lastCandidateAt.Valid {
		view.LastCandidateEventAt = &lastCandidateAt.Time
	}
	if latestRecordedAt.Valid {
		view.LatestCandidateRecordedAt = &latestRecordedAt.Time
	}
	if view.CandidateEventCount > 0 && view.FirstCandidateEventAt != nil {
		if err := r.findAPIUsageLedgerRetentionCoveringBatch(ctx, req, view.FirstCandidateEventAt, &view); err != nil {
			return APIUsageLedgerRetentionReadinessView{}, err
		}
	}
	applyAPIUsageLedgerRetentionReadiness(&view)
	return view, nil
}

func NormalizeAPIUsageLedgerRetentionLimit(limit int) int {
	if limit <= 0 {
		return APIUsageLedgerRetentionDefaultLimit
	}
	if limit > APIUsageLedgerRetentionMaxLimit {
		return APIUsageLedgerRetentionMaxLimit
	}
	return limit
}

func (r *Repository) RunAPIUsageLedgerRetention(ctx context.Context, req APIUsageLedgerRetentionRunRequest) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff must not be in the future")
	}
	if req.Limit < 0 {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("limit must not be negative")
	}
	if !req.DryRun && !req.ConfirmReady {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("confirm_ready is required for destructive retention runs")
	}
	limit := NormalizeAPIUsageLedgerRetentionLimit(req.Limit)
	id, err := newAPIUsageLedgerRetentionRunID()
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}

	readiness, err := r.GetAPIUsageLedgerRetentionReadiness(ctx, APIUsageLedgerRetentionRequest{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
	})
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	limited := readiness.CandidateEventCount
	if limited > int64(limit) {
		limited = int64(limit)
	}
	view := APIUsageLedgerRetentionRunView{
		ID:             id,
		Cutoff:         req.Cutoff,
		TenantID:       req.TenantID,
		PrincipalID:    req.PrincipalID,
		Limit:          limit,
		DryRun:         req.DryRun,
		ConfirmReady:   req.ConfirmReady,
		Ready:          readiness.Ready,
		CandidateCount: readiness.CandidateEventCount,
		LimitedCount:   limited,
		Readiness:      readiness,
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("begin api usage ledger retention transaction: %w", err)
	}
	defer tx.Rollback()

	if req.DryRun || !readiness.Ready || limited == 0 {
		if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := tx.Commit(); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
		}
		return view, nil
	}

	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if readiness.CoveringExportBatchCompletedAt != nil {
		args = append(args, readiness.CoveringExportBatchCompletedAt.UTC())
		conditions = append(conditions, fmt.Sprintf("recorded_at <= $%d", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))
	query := fmt.Sprintf(`
DELETE FROM api_usage_ledger
WHERE event_id IN (
  SELECT event_id
  FROM api_usage_ledger
  WHERE %s
  ORDER BY event_timestamp ASC, event_id ASC
  LIMIT %s
)`, strings.Join(conditions, "\n    AND "), limitPlaceholder)

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("run api usage ledger retention: %w", err)
	}
	view.DeletedCount, _ = result.RowsAffected()
	if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
	}
	return view, nil
}

func recordAPIUsageLedgerRetentionRunAudit(ctx context.Context, tx *sql.Tx, view APIUsageLedgerRetentionRunView) error {
	detail, err := apiUsageLedgerRetentionRunAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage.retention_run",
		TargetType: "api_usage_ledger_retention_run",
		TargetID:   view.ID,
		Result:     apiUsageLedgerRetentionRunAuditResult(view),
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record api usage ledger retention run audit: %w", err)
	}
	return nil
}

func apiUsageLedgerRetentionRunAuditResult(view APIUsageLedgerRetentionRunView) string {
	switch {
	case view.DryRun:
		return "dry_run"
	case !view.Ready:
		return "blocked"
	case view.DeletedCount == 0:
		return "no_op"
	default:
		return "completed"
	}
}

func apiUsageLedgerRetentionRunAuditDetail(view APIUsageLedgerRetentionRunView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"run_id":                             view.ID,
		"cutoff":                             view.Cutoff.UTC().Format(time.RFC3339),
		"tenant_id":                          view.TenantID,
		"principal_id":                       view.PrincipalID,
		"limit":                              view.Limit,
		"dry_run":                            view.DryRun,
		"confirm_ready":                      view.ConfirmReady,
		"ready":                              view.Ready,
		"candidate_count":                    view.CandidateCount,
		"limited_count":                      view.LimitedCount,
		"deleted_count":                      view.DeletedCount,
		"blocking_reasons":                   view.Readiness.BlockingReasons,
		"covering_export_batch_id":           view.Readiness.CoveringExportBatchID,
		"covering_artifact_count":            view.Readiness.CoveringArtifactCount,
		"covering_manifest_digest_count":     view.Readiness.CoveringManifestDigestCount,
		"covering_manifest_signature_count":  view.Readiness.CoveringManifestSignatureCount,
		"covering_export_batch_completed_at": optionalTimeStringPtr(view.Readiness.CoveringExportBatchCompletedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage ledger retention run audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageLedgerRetentionRuns(ctx context.Context, req APIUsageLedgerRetentionRunListRequest) ([]APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)

	query := `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs`
	var conditions []string
	var args []any
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.CreatedFrom.IsZero() {
		args = append(args, req.CreatedFrom.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.CreatedTo.IsZero() {
		args = append(args, req.CreatedTo.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage ledger retention runs: %w", err)
	}
	defer rows.Close()

	var runs []APIUsageLedgerRetentionRunView
	for rows.Next() {
		run, err := scanAPIUsageLedgerRetentionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage ledger retention runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) GetAPIUsageLedgerRetentionRun(ctx context.Context, id string) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run id is required")
	}
	const query = `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs
WHERE id = $1`
	run, err := scanAPIUsageLedgerRetentionRun(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run not found")
		}
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("get api usage ledger retention run: %w", err)
	}
	return run, nil
}

type apiUsageLedgerRetentionRunScanner interface {
	Scan(...any) error
}

func scanAPIUsageLedgerRetentionRun(scanner apiUsageLedgerRetentionRunScanner) (APIUsageLedgerRetentionRunView, error) {
	var run APIUsageLedgerRetentionRunView
	var readiness json.RawMessage
	if err := scanner.Scan(
		&run.ID,
		&run.CreatedAt,
		&run.Cutoff,
		&run.TenantID,
		&run.PrincipalID,
		&run.Limit,
		&run.DryRun,
		&run.ConfirmReady,
		&run.Ready,
		&run.CandidateCount,
		&run.LimitedCount,
		&run.DeletedCount,
		&readiness,
	); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("scan api usage ledger retention run: %w", err)
	}
	if len(readiness) > 0 {
		if err := json.Unmarshal(readiness, &run.Readiness); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("decode api usage ledger retention run readiness: %w", err)
		}
	}
	run.CreatedAt = run.CreatedAt.UTC()
	run.Cutoff = run.Cutoff.UTC()
	return run, nil
}

func (r *Repository) insertAPIUsageLedgerRetentionRun(ctx context.Context, execer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, view *APIUsageLedgerRetentionRunView) error {
	readiness, err := json.Marshal(view.Readiness)
	if err != nil {
		return fmt.Errorf("marshal api usage ledger retention readiness: %w", err)
	}
	const query = `
INSERT INTO api_usage_ledger_retention_runs (
  id,
  cutoff,
  tenant_id,
  principal_id,
  limit_count,
  dry_run,
  confirm_ready,
  ready,
  candidate_count,
  limited_count,
  deleted_count,
  readiness
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
RETURNING created_at`
	if err := execer.QueryRowContext(
		ctx,
		query,
		view.ID,
		view.Cutoff,
		view.TenantID,
		view.PrincipalID,
		view.Limit,
		view.DryRun,
		view.ConfirmReady,
		view.Ready,
		view.CandidateCount,
		view.LimitedCount,
		view.DeletedCount,
		string(readiness),
	).Scan(&view.CreatedAt); err != nil {
		return fmt.Errorf("record api usage ledger retention run: %w", err)
	}
	view.CreatedAt = view.CreatedAt.UTC()
	return nil
}

func (r *Repository) findAPIUsageLedgerRetentionCoveringBatch(ctx context.Context, req APIUsageLedgerRetentionRequest, firstCandidateAt *time.Time, view *APIUsageLedgerRetentionReadinessView) error {
	const query = `
SELECT
  b.id,
  b.completed_at,
  b.window_start,
  b.window_end,
  b.event_count,
  COALESCE(a.artifact_count, 0)::bigint,
  COALESCE(a.artifact_event_count, 0)::bigint,
  COALESCE(d.digest_count, 0)::bigint,
  COALESCE(s.signature_count, 0)::bigint
FROM api_usage_export_batches b
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS artifact_count, COALESCE(sum(event_count), 0)::bigint AS artifact_event_count
  FROM api_usage_export_artifacts
  WHERE batch_id = b.id
) a ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS digest_count
  FROM api_usage_export_manifest_digests
  WHERE batch_id = b.id
) d ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS signature_count
  FROM api_usage_export_manifest_signatures
  WHERE batch_id = b.id
) s ON true
WHERE b.status = 'completed'
  AND b.completed_at IS NOT NULL
  AND b.tenant_id = $1
  AND b.principal_id = $2
  AND (b.window_start IS NULL OR b.window_start <= $3)
  AND b.window_end IS NOT NULL
  AND b.window_end >= $4
ORDER BY b.completed_at DESC, b.id DESC
LIMIT 1`
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	err := r.db.QueryRowContext(ctx, query, req.TenantID, req.PrincipalID, firstCandidateAt.UTC(), req.Cutoff).Scan(
		&view.CoveringExportBatchID,
		&completedAt,
		&windowStart,
		&windowEnd,
		&view.CoveringExportBatchEventCount,
		&view.CoveringArtifactCount,
		&view.CoveringArtifactEventCount,
		&view.CoveringManifestDigestCount,
		&view.CoveringManifestSignatureCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get api usage ledger retention covering export batch: %w", err)
	}
	if completedAt.Valid {
		view.CoveringExportBatchCompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		view.CoveringExportBatchWindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		view.CoveringExportBatchWindowEnd = &windowEnd.Time
	}
	return nil
}

func applyAPIUsageLedgerRetentionReadiness(view *APIUsageLedgerRetentionReadinessView) {
	var blocking []string
	if view.CandidateEventCount > 0 {
		if view.CoveringExportBatchID == "" {
			blocking = append(blocking, "covering_export_batch_required")
		}
		if view.CoveringExportBatchCompletedAt != nil && view.LatestCandidateRecordedAt != nil && view.CoveringExportBatchCompletedAt.Before(*view.LatestCandidateRecordedAt) {
			blocking = append(blocking, "covering_export_batch_stale")
		}
		if view.CoveringExportBatchID != "" && (view.CoveringArtifactCount == 0 || view.CoveringArtifactEventCount < view.CoveringExportBatchEventCount) {
			blocking = append(blocking, "covering_export_artifact_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestDigestCount == 0 {
			blocking = append(blocking, "covering_manifest_digest_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestSignatureCount == 0 {
			blocking = append(blocking, "covering_manifest_signature_required")
		}
	}
	view.BlockingReasons = blocking
	view.Ready = len(blocking) == 0
}

func (r *Repository) CreateAPIUsageExportBatch(ctx context.Context, req APIUsageLedgerListRequest) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	stats, err := r.GetAPIUsageLedgerStats(ctx, req)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	id, err := newAPIUsageExportBatchID()
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	manifest, err := json.Marshal(map[string]any{
		"version":      "2026-05-04.api-usage-export.v1",
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
		"from":         optionalTimeString(req.From),
		"to":           optionalTimeString(req.To),
		"format":       "ndjson",
	})
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("marshal api usage export manifest: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("begin api usage export batch transaction: %w", err)
	}
	defer tx.Rollback()

	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	const query = `
INSERT INTO api_usage_export_batches (
  id,
  completed_at,
  status,
  export_format,
  tenant_id,
  principal_id,
  window_start,
  window_end,
  event_count,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_event_at,
  last_event_at,
  manifest
) VALUES ($1, now(), 'completed', 'ndjson', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest`
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		strings.TrimSpace(req.TenantID),
		strings.TrimSpace(req.PrincipalID),
		nullableTime(req.From),
		nullableTime(req.To),
		stats.EventCount,
		stats.RequestCount,
		stats.RequestBytes,
		stats.ResponseBytes,
		stats.LatencyMSTotal,
		stats.LatencyMSMax,
		nullableTimePtr(stats.FirstEventAt),
		nullableTimePtr(stats.LastEventAt),
		manifest,
	).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("create api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	detail, err := apiUsageExportBatchAuditDetail(batch)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.batch_create",
		TargetType: "api_usage_export_batch",
		TargetID:   batch.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("record api usage export batch audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("commit api usage export batch transaction: %w", err)
	}
	return batch, nil
}

func apiUsageExportBatchAuditDetail(batch APIUsageExportBatchView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"batch_id":         batch.ID,
		"tenant_id":        batch.TenantID,
		"principal_id":     batch.PrincipalID,
		"status":           batch.Status,
		"export_format":    batch.ExportFormat,
		"window_start":     optionalTimeStringPtr(batch.WindowStart),
		"window_end":       optionalTimeStringPtr(batch.WindowEnd),
		"event_count":      batch.EventCount,
		"request_count":    batch.RequestCount,
		"request_bytes":    batch.RequestBytes,
		"response_bytes":   batch.ResponseBytes,
		"latency_ms_total": batch.LatencyMSTotal,
		"latency_ms_max":   batch.LatencyMSMax,
		"first_event_at":   optionalTimeStringPtr(batch.FirstEventAt),
		"last_event_at":    optionalTimeStringPtr(batch.LastEventAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export batch audit detail: %w", err)
	}
	return detail, nil
}

func optionalTimeStringPtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func ValidateAPIUsageExportBatchListRequest(req APIUsageExportBatchListRequest) error {
	for field, value := range map[string]string{
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return err
		}
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isAPIUsageExportBatchStatus(status) {
		return fmt.Errorf("unsupported api usage export batch status %q", req.Status)
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		return fmt.Errorf("from must be before to")
	}
	return nil
}

func isAPIUsageExportBatchStatus(status string) bool {
	switch status {
	case "pending", "completed", "failed":
		return true
	default:
		return false
	}
}

func (r *Repository) ListAPIUsageExportBatches(ctx context.Context, req APIUsageExportBatchListRequest) ([]APIUsageExportBatchView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateAPIUsageExportBatchListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	tenantID := strings.TrimSpace(req.TenantID)
	principalID := strings.TrimSpace(req.PrincipalID)
	status := strings.ToLower(strings.TrimSpace(req.Status))
	const query = `
SELECT id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest
FROM api_usage_export_batches
WHERE ($1 = '' OR tenant_id = $1)
  AND ($2 = '' OR principal_id = $2)
  AND ($3 = '' OR status = $3)
  AND ($4::timestamptz IS NULL OR window_start >= $4)
  AND ($5::timestamptz IS NULL OR window_end < $5)
ORDER BY created_at DESC, id DESC
LIMIT $6`
	rows, err := r.db.QueryContext(ctx, query, tenantID, principalID, status, nullableTime(req.From), nullableTime(req.To), limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export batches: %w", err)
	}
	defer rows.Close()

	var batches []APIUsageExportBatchView
	for rows.Next() {
		var batch APIUsageExportBatchView
		var completedAt sql.NullTime
		var windowStart sql.NullTime
		var windowEnd sql.NullTime
		var firstEventAt sql.NullTime
		var lastEventAt sql.NullTime
		if err := rows.Scan(
			&batch.ID,
			&batch.CreatedAt,
			&completedAt,
			&batch.Status,
			&batch.ExportFormat,
			&batch.TenantID,
			&batch.PrincipalID,
			&windowStart,
			&windowEnd,
			&batch.EventCount,
			&batch.RequestCount,
			&batch.RequestBytes,
			&batch.ResponseBytes,
			&batch.LatencyMSTotal,
			&batch.LatencyMSMax,
			&firstEventAt,
			&lastEventAt,
			&batch.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export batch: %w", err)
		}
		applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export batches: %w", err)
	}
	return batches, nil
}

func (r *Repository) GetAPIUsageExportBatch(ctx context.Context, id string) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch id is required")
	}
	const query = `
SELECT id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest
FROM api_usage_export_batches
WHERE id = $1`
	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch not found")
		}
		return APIUsageExportBatchView{}, fmt.Errorf("get api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	return batch, nil
}

func (r *Repository) GetAPIUsageExportHandoff(ctx context.Context, batchID string) (APIUsageExportHandoffView, error) {
	if r.db == nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportHandoffView{}, err
	}
	view := APIUsageExportHandoffView{
		BatchID:        batch.ID,
		BatchStatus:    batch.Status,
		BatchCompleted: batch.Status == "completed" && batch.CompletedAt != nil,
		EventCount:     batch.EventCount,
	}
	const artifactQuery = `
SELECT count(*), coalesce(sum(event_count), 0), coalesce(sum(byte_count), 0)
FROM api_usage_export_artifacts
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, artifactQuery, batch.ID).Scan(
		&view.ArtifactCount,
		&view.ArtifactEventCount,
		&view.ArtifactByteCount,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export artifact handoff stats: %w", err)
	}

	var latestDigestAt sql.NullTime
	const digestQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(digest_hex ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_digests
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, digestQuery, batch.ID).Scan(
		&view.ManifestDigestCount,
		&view.LatestManifestDigestID,
		&view.LatestManifestDigestHex,
		&latestDigestAt,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest digest handoff stats: %w", err)
	}
	if latestDigestAt.Valid {
		view.LatestManifestDigestAt = &latestDigestAt.Time
	}

	if view.LatestManifestDigestID != "" {
		var latestSignatureAt sql.NullTime
		const signatureQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(signer_backend ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(key_id ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2`
		if err := r.db.QueryRowContext(ctx, signatureQuery, batch.ID, view.LatestManifestDigestID).Scan(
			&view.LatestDigestSignatureCount,
			&view.LatestSignatureID,
			&view.LatestSignatureSigner,
			&view.LatestSignatureKeyID,
			&latestSignatureAt,
		); err != nil {
			return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest signature handoff stats: %w", err)
		}
		if latestSignatureAt.Valid {
			view.LatestSignatureAt = &latestSignatureAt.Time
		}
	}

	applyAPIUsageExportHandoffReadiness(&view)
	return view, nil
}

func (r *Repository) CreateAPIUsageExportArtifact(ctx context.Context, req CreateAPIUsageExportArtifactRequest) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportArtifactRequest(&req); err != nil {
		return APIUsageExportArtifactView{}, err
	}
	id, err := newAPIUsageExportArtifactID()
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("begin api usage export artifact transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_artifacts (
  id,
  batch_id,
  storage_backend,
  object_key,
  content_type,
  byte_count,
  sha256_hex,
  event_count,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (batch_id, object_key) DO UPDATE SET
  metadata = EXCLUDED.metadata
WHERE api_usage_export_artifacts.sha256_hex = EXCLUDED.sha256_hex
RETURNING id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata`
	var artifact APIUsageExportArtifactView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.BatchID,
		req.StorageBackend,
		req.ObjectKey,
		req.ContentType,
		req.ByteCount,
		req.SHA256Hex,
		req.EventCount,
		req.Metadata,
	).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("create api usage export artifact: %w", err)
	}
	detail, err := apiUsageExportArtifactAuditDetail(artifact)
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.artifact_create",
		TargetType: "api_usage_export_artifact",
		TargetID:   artifact.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("record api usage export artifact audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("commit api usage export artifact transaction: %w", err)
	}
	return artifact, nil
}

func apiUsageExportArtifactAuditDetail(artifact APIUsageExportArtifactView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"artifact_id":     artifact.ID,
		"batch_id":        artifact.BatchID,
		"storage_backend": artifact.StorageBackend,
		"object_key":      artifact.ObjectKey,
		"content_type":    artifact.ContentType,
		"byte_count":      artifact.ByteCount,
		"sha256_hex":      artifact.SHA256Hex,
		"event_count":     artifact.EventCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export artifact audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, limit, false)
}

func (r *Repository) ListAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAllAPIUsageExportArtifacts(ctx, batchID)
}

func (r *Repository) listAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, 0, true)
}

func (r *Repository) listAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int, unbounded bool) ([]APIUsageExportArtifactView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	query := apiUsageExportArtifactsQuery(unbounded)
	args := []any{batchID}
	if !unbounded {
		limit = normalizeLimit(limit)
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage export artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []APIUsageExportArtifactView
	for rows.Next() {
		var artifact APIUsageExportArtifactView
		if err := rows.Scan(
			&artifact.ID,
			&artifact.BatchID,
			&artifact.CreatedAt,
			&artifact.StorageBackend,
			&artifact.ObjectKey,
			&artifact.ContentType,
			&artifact.ByteCount,
			&artifact.SHA256Hex,
			&artifact.EventCount,
			&artifact.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export artifacts: %w", err)
	}
	return artifacts, nil
}

func apiUsageExportArtifactsQuery(unbounded bool) string {
	query := `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
`
	if !unbounded {
		query += `LIMIT $2`
	}
	return query
}

func (r *Repository) GetAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	artifactID = strings.TrimSpace(artifactID)
	if batchID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("batch_id is required")
	}
	if artifactID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("artifact_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
  AND id = $2`
	var artifact APIUsageExportArtifactView
	if err := r.db.QueryRowContext(ctx, query, batchID, artifactID).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportArtifactView{}, fmt.Errorf("api usage export artifact not found")
		}
		return APIUsageExportArtifactView{}, fmt.Errorf("get api usage export artifact: %w", err)
	}
	return artifact, nil
}

func (r *Repository) CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	artifacts, err := r.listAllAPIUsageExportArtifacts(ctx, batch.ID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	manifest := apiUsageExportManifest(batch, artifacts)
	digest, raw, err := apimeter.DigestExportManifest(manifest)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	id, err := newAPIUsageExportManifestDigestID()
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("begin api usage export manifest digest transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_digests (
  id,
  batch_id,
  schema_version,
  digest_algorithm,
  digest_hex,
  manifest
) VALUES ($1, $2, $3, 'sha256', $4, $5)
ON CONFLICT (batch_id, digest_algorithm, digest_hex) DO UPDATE SET
  manifest = EXCLUDED.manifest
RETURNING id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest`
	var view APIUsageExportManifestDigestView
	if err := tx.QueryRowContext(ctx, query, id, batch.ID, manifest.SchemaVersion, digest, raw).Scan(
		&view.ID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SchemaVersion,
		&view.DigestAlgorithm,
		&view.DigestHex,
		&view.Manifest,
	); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("create api usage export manifest digest: %w", err)
	}
	detail, err := apiUsageExportManifestDigestAuditDetail(view, len(manifest.Artifacts))
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_digest_create",
		TargetType: "api_usage_export_manifest_digest",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("record api usage export manifest digest audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("commit api usage export manifest digest transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestDigestAuditDetail(digest APIUsageExportManifestDigestView, artifactCount int) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"digest_id":        digest.ID,
		"batch_id":         digest.BatchID,
		"schema_version":   digest.SchemaVersion,
		"digest_algorithm": digest.DigestAlgorithm,
		"digest_hex":       digest.DigestHex,
		"manifest_bytes":   len(digest.Manifest),
		"artifact_count":   artifactCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest digest audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, batchID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest digests: %w", err)
	}
	defer rows.Close()

	var digests []APIUsageExportManifestDigestView
	for rows.Next() {
		var digest APIUsageExportManifestDigestView
		if err := rows.Scan(
			&digest.ID,
			&digest.BatchID,
			&digest.CreatedAt,
			&digest.SchemaVersion,
			&digest.DigestAlgorithm,
			&digest.DigestHex,
			&digest.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest digest: %w", err)
		}
		digests = append(digests, digest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest digests: %w", err)
	}
	return digests, nil
}

func (r *Repository) GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("digest_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
  AND id = $2`
	var digest APIUsageExportManifestDigestView
	if err := r.db.QueryRowContext(ctx, query, batchID, digestID).Scan(
		&digest.ID,
		&digest.BatchID,
		&digest.CreatedAt,
		&digest.SchemaVersion,
		&digest.DigestAlgorithm,
		&digest.DigestHex,
		&digest.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestDigestView{}, fmt.Errorf("api usage export manifest digest not found")
		}
		return APIUsageExportManifestDigestView{}, fmt.Errorf("get api usage export manifest digest: %w", err)
	}
	return digest, nil
}

func (r *Repository) VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestVerificationView, error) {
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return apiUsageExportManifestDigestVerification(digest)
}

func apiUsageExportManifestDigestVerification(digest APIUsageExportManifestDigestView) (APIUsageExportManifestDigestVerificationView, error) {
	actual, canonical, err := apimeter.DigestExportManifestJSON(digest.Manifest)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return APIUsageExportManifestDigestVerificationView{
		BatchID:           digest.BatchID,
		DigestID:          digest.ID,
		SchemaVersion:     digest.SchemaVersion,
		DigestAlgorithm:   digest.DigestAlgorithm,
		ExpectedDigestHex: digest.DigestHex,
		ActualDigestHex:   actual,
		Valid:             digest.DigestAlgorithm == "sha256" && digest.DigestHex == actual,
		CanonicalManifest: canonical,
	}, nil
}

func applyAPIUsageExportHandoffReadiness(view *APIUsageExportHandoffView) {
	view.EventsCovered = view.ArtifactEventCount >= view.EventCount
	var missing []string
	if !view.BatchCompleted {
		missing = append(missing, "batch_completed")
	}
	if view.ArtifactCount == 0 {
		missing = append(missing, "export_artifact")
	}
	if !view.EventsCovered {
		missing = append(missing, "event_coverage")
	}
	if view.ManifestDigestCount == 0 || view.LatestManifestDigestID == "" {
		missing = append(missing, "manifest_digest")
	}
	if view.LatestDigestSignatureCount == 0 {
		missing = append(missing, "manifest_signature")
	}
	view.MissingRequirements = missing
	view.Ready = len(missing) == 0
	view.ReadinessGrade = "billing_blocked"
	if !view.Ready {
		view.BillingBlockingReasons = []string{"handoff_not_ready"}
		return
	}
	if apiUsageExportManifestSignerNeedsProductionBackend(view.LatestSignatureSigner) {
		view.ReadinessGrade = "operational"
		view.BillingBlockingReasons = []string{"production_manifest_signer_required"}
		return
	}
	view.ReadinessGrade = "billing_candidate"
	view.BillingReady = true
}

func apiUsageExportManifestSignerNeedsProductionBackend(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "local-hmac", "local-ed25519":
		return true
	default:
		return false
	}
}

func (r *Repository) CreateAPIUsageExportManifestSignature(ctx context.Context, req CreateAPIUsageExportManifestSignatureRequest) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, req.BatchID, req.DigestID)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if digest.DigestHex != req.Signature.SignedDigestHex {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signed_digest_hex must match manifest digest")
	}
	id, err := newAPIUsageExportManifestSignatureID()
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("begin api usage export manifest signature transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_signatures (
  id,
  digest_id,
  batch_id,
  signer_backend,
  key_id,
  signature_algorithm,
  signed_digest_hex,
  signature_hex,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (digest_id, signature_algorithm, key_id, signature_hex) DO UPDATE SET
  metadata = EXCLUDED.metadata
RETURNING id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata`
	var view APIUsageExportManifestSignatureView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.DigestID,
		req.BatchID,
		req.SignerBackend,
		req.Signature.KeyID,
		req.Signature.Algorithm,
		req.Signature.SignedDigestHex,
		req.Signature.SignatureHex,
		req.Metadata,
	).Scan(
		&view.ID,
		&view.DigestID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SignerBackend,
		&view.KeyID,
		&view.SignatureAlgorithm,
		&view.SignedDigestHex,
		&view.SignatureHex,
		&view.Metadata,
	); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("create api usage export manifest signature: %w", err)
	}
	detail, err := apiUsageExportManifestSignatureAuditDetail(view)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_signature_create",
		TargetType: "api_usage_export_manifest_signature",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("record api usage export manifest signature audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("commit api usage export manifest signature transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestSignatureAuditDetail(signature APIUsageExportManifestSignatureView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"signature_id":        signature.ID,
		"digest_id":           signature.DigestID,
		"batch_id":            signature.BatchID,
		"signer_backend":      signature.SignerBackend,
		"key_id":              signature.KeyID,
		"signature_algorithm": signature.SignatureAlgorithm,
		"signed_digest_hex":   signature.SignedDigestHex,
		"signature_hex_len":   len(signature.SignatureHex),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest signature audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return nil, fmt.Errorf("digest_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, batchID, digestID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest signatures: %w", err)
	}
	defer rows.Close()

	var signatures []APIUsageExportManifestSignatureView
	for rows.Next() {
		var signature APIUsageExportManifestSignatureView
		if err := scanAPIUsageExportManifestSignature(rows, &signature); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest signature: %w", err)
		}
		signatures = append(signatures, signature)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest signatures: %w", err)
	}
	return signatures, nil
}

func (r *Repository) GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	signatureID = strings.TrimSpace(signatureID)
	if batchID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("digest_id is required")
	}
	if signatureID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signature_id is required")
	}
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
  AND id = $3`
	var signature APIUsageExportManifestSignatureView
	if err := scanAPIUsageExportManifestSignature(r.db.QueryRowContext(ctx, query, batchID, digestID, signatureID), &signature); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signature not found")
		}
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("get api usage export manifest signature: %w", err)
	}
	return signature, nil
}

type apiUsageExportManifestSignatureScanner interface {
	Scan(dest ...any) error
}

func scanAPIUsageExportManifestSignature(scanner apiUsageExportManifestSignatureScanner, signature *APIUsageExportManifestSignatureView) error {
	return scanner.Scan(
		&signature.ID,
		&signature.DigestID,
		&signature.BatchID,
		&signature.CreatedAt,
		&signature.SignerBackend,
		&signature.KeyID,
		&signature.SignatureAlgorithm,
		&signature.SignedDigestHex,
		&signature.SignatureHex,
		&signature.Metadata,
	)
}

func ValidateCreateAPIUsageExportManifestSignatureRequest(req *CreateAPIUsageExportManifestSignatureRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.DigestID = strings.TrimSpace(req.DigestID)
	req.SignerBackend = strings.TrimSpace(req.SignerBackend)
	req.Signature.Algorithm = strings.TrimSpace(req.Signature.Algorithm)
	req.Signature.KeyID = strings.TrimSpace(req.Signature.KeyID)
	req.Signature.SignedDigestHex = strings.ToLower(strings.TrimSpace(req.Signature.SignedDigestHex))
	req.Signature.SignatureHex = strings.ToLower(strings.TrimSpace(req.Signature.SignatureHex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.DigestID == "" {
		return fmt.Errorf("digest_id is required")
	}
	if req.SignerBackend == "" {
		return fmt.Errorf("signer_backend is required")
	}
	switch req.Signature.Algorithm {
	case apimeter.ExportManifestSignatureAlgorithmHMACSHA256, apimeter.ExportManifestSignatureAlgorithmEd25519:
	default:
		return fmt.Errorf("signature_algorithm must be hmac-sha256 or ed25519")
	}
	if !apiUsageExportManifestSignatureBackendMatchesAlgorithm(req.SignerBackend, req.Signature.Algorithm) {
		return fmt.Errorf("signer_backend %q is not compatible with signature_algorithm %q", req.SignerBackend, req.Signature.Algorithm)
	}
	if req.Signature.KeyID == "" {
		return fmt.Errorf("key_id is required")
	}
	if !isLowerHexSHA256(req.Signature.SignedDigestHex) {
		return fmt.Errorf("signed_digest_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256 && !isLowerHexBytes(req.Signature.SignatureHex, 32) {
		return fmt.Errorf("signature_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519 && !isLowerHexBytes(req.Signature.SignatureHex, 64) {
		return fmt.Errorf("signature_hex must be 128 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func apiUsageExportManifestSignatureBackendMatchesAlgorithm(backend string, algorithm string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "local-hmac":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256
	case "local-ed25519":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519
	default:
		return true
	}
}

func apiUsageExportManifest(batch APIUsageExportBatchView, artifacts []APIUsageExportArtifactView) apimeter.ExportManifest {
	ordered := append([]APIUsageExportArtifactView(nil), artifacts...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	manifest := apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Batch: apimeter.ExportManifestBatch{
			ID:             batch.ID,
			TenantID:       batch.TenantID,
			PrincipalID:    batch.PrincipalID,
			WindowStart:    apimeter.FormatManifestTime(batch.WindowStart),
			WindowEnd:      apimeter.FormatManifestTime(batch.WindowEnd),
			EventCount:     batch.EventCount,
			RequestCount:   batch.RequestCount,
			RequestBytes:   batch.RequestBytes,
			ResponseBytes:  batch.ResponseBytes,
			LatencyMSTotal: batch.LatencyMSTotal,
			LatencyMSMax:   batch.LatencyMSMax,
		},
		Artifacts: make([]apimeter.ExportManifestArtifact, 0, len(ordered)),
	}
	for _, artifact := range ordered {
		manifest.Artifacts = append(manifest.Artifacts, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	return manifest
}

func ValidateCreateAPIUsageExportArtifactRequest(req *CreateAPIUsageExportArtifactRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.StorageBackend = strings.TrimSpace(req.StorageBackend)
	if strings.ContainsAny(req.ObjectKey, "\r\n") {
		return fmt.Errorf("object_key cannot contain line breaks")
	}
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.SHA256Hex = strings.ToLower(strings.TrimSpace(req.SHA256Hex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.StorageBackend == "" {
		req.StorageBackend = "external"
	}
	if req.ObjectKey == "" {
		return fmt.Errorf("object_key is required")
	}
	if req.ContentType == "" {
		req.ContentType = "application/x-ndjson"
	}
	if req.ContentType != "application/x-ndjson" {
		return fmt.Errorf("content_type must be application/x-ndjson")
	}
	if req.ByteCount < 0 {
		return fmt.Errorf("byte_count must be nonnegative")
	}
	if req.EventCount < 0 {
		return fmt.Errorf("event_count must be nonnegative")
	}
	if !isLowerHexSHA256(req.SHA256Hex) {
		return fmt.Errorf("sha256_hex must be 64 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func isLowerHexSHA256(value string) bool {
	return isLowerHexBytes(value, 32)
}

func isLowerHexBytes(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func newAPIUsageExportBatchID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export batch id: %w", err)
	}
	return "api-usage-export-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageLedgerRetentionRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage ledger retention run id: %w", err)
	}
	return "api-usage-retention-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportArtifactID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export artifact id: %w", err)
	}
	return "api-usage-artifact-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestDigestID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest digest id: %w", err)
	}
	return "api-usage-manifest-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestSignatureID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest signature id: %w", err)
	}
	return "api-usage-signature-" + hex.EncodeToString(random[:]), nil
}

func optionalTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func applyExportBatchNullableTimes(batch *APIUsageExportBatchView, completedAt sql.NullTime, windowStart sql.NullTime, windowEnd sql.NullTime, firstEventAt sql.NullTime, lastEventAt sql.NullTime) {
	if completedAt.Valid {
		batch.CompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		batch.WindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		batch.WindowEnd = &windowEnd.Time
	}
	if firstEventAt.Valid {
		batch.FirstEventAt = &firstEventAt.Time
	}
	if lastEventAt.Valid {
		batch.LastEventAt = &lastEventAt.Time
	}
}

func quotaUsageRatio(used int64, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	if used <= 0 {
		return 0
	}
	return float64(used) / float64(limit)
}

func quotaRemaining(used int64, limit int64) int64 {
	if limit <= 0 {
		return 0
	}
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (r *Repository) ListDeliveryAttempts(ctx context.Context, req DeliveryAttemptListRequest) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		Status:          req.Status,
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return nil, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  sender,
  recipient,
  recipient_domain,
  status,
  enhanced_status,
  error_message,
  dsn_return,
  dsn_envelope_id,
  dsn_notify,
  original_recipient,
  attempted_at
FROM delivery_attempts
WHERE (NULLIF($2, '') IS NULL OR status = $2)
  AND ($3::timestamptz IS NULL OR attempted_at >= $3::timestamptz)
  AND (NULLIF($4, '') IS NULL OR recipient_domain = $4)
  AND (NULLIF($5, '') IS NULL OR message_id::text = $5)
  AND (NULLIF($6, '') IS NULL OR farm = $6)
  AND (NULLIF($7, '') IS NULL OR lower(sender) = $7)
ORDER BY attempted_at DESC, id DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, req.Limit, filters.Status, nullableTime(req.Since), filters.RecipientDomain, filters.MessageID, filters.Farm, filters.Sender)
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := scanDeliveryAttempt(rows, &attempt); err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery attempts: %w", err)
	}
	return attempts, nil
}

type deliveryAttemptFilters struct {
	Status          string
	RecipientDomain string
	MessageID       string
	Farm            string
	Sender          string
}

func normalizeDeliveryAttemptFilters(filters deliveryAttemptFilters) (deliveryAttemptFilters, error) {
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Status != "" && !allowedDeliveryAttemptStatus(filters.Status) {
		return deliveryAttemptFilters{}, fmt.Errorf("unsupported delivery attempt status")
	}
	var err error
	if filters.RecipientDomain, err = normalizeDeliveryAttemptTextFilter("recipient_domain", filters.RecipientDomain, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	filters.RecipientDomain = strings.Trim(filters.RecipientDomain, ".")
	if filters.MessageID, err = normalizeDeliveryAttemptTextFilter("message_id", filters.MessageID, false); err != nil {
		return deliveryAttemptFilters{}, err
	}
	if filters.Farm, err = normalizeDeliveryAttemptTextFilter("farm", filters.Farm, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	if filters.Sender, err = normalizeDeliveryAttemptTextFilter("sender", filters.Sender, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	return filters, nil
}

func normalizeDeliveryAttemptTextFilter(name string, value string, lower bool) (string, error) {
	value = strings.TrimSpace(value)
	if lower {
		value = strings.ToLower(value)
	}
	if err := validatePushNotificationFilter(name, value); err != nil {
		return "", err
	}
	return value, nil
}

func allowedDeliveryAttemptStatus(status string) bool {
	switch status {
	case "delivered", "failed", "bounced", "exhausted":
		return true
	default:
		return false
	}
}

func (r *Repository) GetDeliveryAttemptStats(ctx context.Context, req DeliveryAttemptStatsRequest) (DeliveryAttemptStatsView, error) {
	if r.db == nil {
		return DeliveryAttemptStatsView{}, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		Status:          req.Status,
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return DeliveryAttemptStatsView{}, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}

	const query = `
SELECT
  count(*)::bigint,
  count(DISTINCT message_id)::bigint,
  count(DISTINCT recipient)::bigint,
  count(*) FILTER (WHERE status = 'delivered')::bigint,
  count(*) FILTER (WHERE status = 'failed')::bigint,
  count(*) FILTER (WHERE status = 'bounced')::bigint,
  count(*) FILTER (WHERE status = 'exhausted')::bigint
FROM delivery_attempts
WHERE (NULLIF($1, '') IS NULL OR status = $1)
  AND ($2::timestamptz IS NULL OR attempted_at >= $2::timestamptz)
  AND (NULLIF($3, '') IS NULL OR recipient_domain = $3)
  AND (NULLIF($4, '') IS NULL OR message_id::text = $4)
  AND (NULLIF($5, '') IS NULL OR farm = $5)
  AND (NULLIF($6, '') IS NULL OR lower(sender) = $6)`

	var stats DeliveryAttemptStatsView
	if err := r.db.QueryRowContext(ctx, query, filters.Status, nullableTime(req.Since), filters.RecipientDomain, filters.MessageID, filters.Farm, filters.Sender).Scan(
		&stats.TotalAttempts,
		&stats.UniqueMessages,
		&stats.UniqueRecipients,
		&stats.Delivered,
		&stats.Failed,
		&stats.Bounced,
		&stats.Exhausted,
	); err != nil {
		return DeliveryAttemptStatsView{}, fmt.Errorf("get delivery attempt stats: %w", err)
	}
	return stats, nil
}

func (r *Repository) ListExhaustedAttempts(ctx context.Context, req ExhaustedAttemptListRequest) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return nil, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  sender,
  recipient,
  recipient_domain,
  status,
  enhanced_status,
  error_message,
  dsn_return,
  dsn_envelope_id,
  dsn_notify,
  original_recipient,
  attempted_at
FROM delivery_attempts
WHERE status = 'exhausted'
  AND ($2::timestamptz IS NULL OR attempted_at >= $2::timestamptz)
  AND (NULLIF($3, '') IS NULL OR recipient_domain = $3)
  AND (NULLIF($4, '') IS NULL OR message_id::text = $4)
  AND (NULLIF($5, '') IS NULL OR farm = $5)
  AND (NULLIF($6, '') IS NULL OR lower(sender) = $6)
ORDER BY attempted_at DESC, id DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, req.Limit, nullableTime(req.Since), filters.RecipientDomain, filters.MessageID, filters.Farm, filters.Sender)
	if err != nil {
		return nil, fmt.Errorf("list exhausted delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := scanDeliveryAttempt(rows, &attempt); err != nil {
			return nil, fmt.Errorf("scan exhausted delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exhausted delivery attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) ListPushNotificationAttempts(ctx context.Context, req PushNotificationAttemptListRequest) ([]PushNotificationAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	var err error
	req, err = normalizePushNotificationAttemptListRequest(req)
	if err != nil {
		return nil, err
	}

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  user_id::text,
  recipient,
  subject,
  COALESCE(device_id::text, ''),
  platform,
  token_suffix,
  status,
  error_message,
  provider_message_id,
  provider_status,
  attempted_at
FROM push_notification_attempts
WHERE (NULLIF($2, '')::uuid IS NULL OR message_id = NULLIF($2, '')::uuid)
  AND (NULLIF($3, '') IS NULL OR status = $3)
  AND (NULLIF($4, '')::uuid IS NULL OR user_id = NULLIF($4, '')::uuid)
  AND ($5::timestamptz IS NULL OR attempted_at >= $5::timestamptz)
  AND (NULLIF($6, '') IS NULL OR platform = $6)
  AND (NULLIF($7, '')::uuid IS NULL OR device_id = NULLIF($7, '')::uuid)
  AND (NULLIF($8, '') IS NULL OR provider_status = $8)
  AND (NULLIF($9, '') IS NULL OR provider_message_id = $9)
ORDER BY attempted_at DESC, id DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, req.Limit, req.MessageID, req.Status, req.UserID, nullableTime(req.Since), req.Platform, req.DeviceID, req.ProviderStatus, req.ProviderMessageID)
	if err != nil {
		return nil, fmt.Errorf("list push notification attempts: %w", err)
	}
	defer rows.Close()

	var attempts []PushNotificationAttemptView
	for rows.Next() {
		var attempt PushNotificationAttemptView
		if err := rows.Scan(
			&attempt.ID,
			&attempt.MessageID,
			&attempt.RFCMessageID,
			&attempt.CompanyID,
			&attempt.DomainID,
			&attempt.UserID,
			&attempt.Recipient,
			&attempt.Subject,
			&attempt.DeviceID,
			&attempt.Platform,
			&attempt.TokenSuffix,
			&attempt.Status,
			&attempt.ErrorMessage,
			&attempt.ProviderMessageID,
			&attempt.ProviderStatus,
			&attempt.AttemptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan push notification attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate push notification attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) GetPushNotificationAttempt(ctx context.Context, id string) (PushNotificationAttemptView, error) {
	if r.db == nil {
		return PushNotificationAttemptView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if err := validatePushNotificationFilter("attempt_id", id); err != nil {
		return PushNotificationAttemptView{}, err
	}
	if id == "" {
		return PushNotificationAttemptView{}, fmt.Errorf("attempt_id is required")
	}

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  user_id::text,
  recipient,
  subject,
  COALESCE(device_id::text, ''),
  platform,
  token_suffix,
  status,
  error_message,
  provider_message_id,
  provider_status,
  attempted_at
FROM push_notification_attempts
WHERE id = $1::uuid`

	var attempt PushNotificationAttemptView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.RFCMessageID,
		&attempt.CompanyID,
		&attempt.DomainID,
		&attempt.UserID,
		&attempt.Recipient,
		&attempt.Subject,
		&attempt.DeviceID,
		&attempt.Platform,
		&attempt.TokenSuffix,
		&attempt.Status,
		&attempt.ErrorMessage,
		&attempt.ProviderMessageID,
		&attempt.ProviderStatus,
		&attempt.AttemptedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return PushNotificationAttemptView{}, fmt.Errorf("push notification attempt %q not found", id)
		}
		return PushNotificationAttemptView{}, fmt.Errorf("get push notification attempt: %w", err)
	}
	return attempt, nil
}

func normalizePushNotificationAttemptListRequest(req PushNotificationAttemptListRequest) (PushNotificationAttemptListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ProviderStatus = strings.TrimSpace(req.ProviderStatus)
	req.ProviderMessageID = strings.TrimSpace(req.ProviderMessageID)
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	for field, value := range map[string]string{
		"message_id":          req.MessageID,
		"status":              req.Status,
		"user_id":             req.UserID,
		"platform":            req.Platform,
		"device_id":           req.DeviceID,
		"provider_status":     req.ProviderStatus,
		"provider_message_id": req.ProviderMessageID,
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return PushNotificationAttemptListRequest{}, err
		}
	}
	if req.Status != "" && !allowedPushNotificationAttemptStatus(req.Status) {
		return PushNotificationAttemptListRequest{}, fmt.Errorf("unsupported push notification attempt status")
	}
	if req.Platform != "" && !allowedPushPlatform(req.Platform) {
		return PushNotificationAttemptListRequest{}, fmt.Errorf("unsupported push notification platform")
	}
	return req, nil
}

func allowedPushNotificationAttemptStatus(status string) bool {
	switch status {
	case "candidate", "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}

func (r *Repository) UpdatePushNotificationOutcome(ctx context.Context, req UpdatePushNotificationOutcomeRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	normalized, err := normalizeUpdatePushNotificationOutcomeRequest(req)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push notification outcome transaction: %w", err)
	}
	defer tx.Rollback()

	attempt, err := readPushNotificationAttemptForUpdate(ctx, tx, normalized.AttemptID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE push_notification_attempts
SET status = $2,
    error_message = $3,
    provider_message_id = $4,
    provider_status = $5,
    attempted_at = now()
WHERE id = $1::uuid`,
		normalized.AttemptID,
		normalized.Status,
		normalized.ErrorMessage,
		normalized.ProviderMessageID,
		normalized.ProviderStatus,
	); err != nil {
		return fmt.Errorf("update push notification outcome: %w", err)
	}

	invalidTokenDeviceDeleted := false
	if normalized.Status == "invalid_token" && strings.TrimSpace(attempt.DeviceID) != "" {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE push_devices SET status = 'deleted', updated_at = now() WHERE id = $1::uuid AND user_id = $2::uuid`,
			attempt.DeviceID,
			attempt.UserID,
		)
		if err != nil {
			return fmt.Errorf("delete invalid push device: %w", err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected > 0 {
			invalidTokenDeviceDeleted = true
		}
	}

	detail, err := pushNotificationOutcomeAuditDetail(attempt, normalized, invalidTokenDeviceDeleted)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  attempt.CompanyID,
		DomainID:   attempt.DomainID,
		UserID:     attempt.UserID,
		Category:   "admin",
		Action:     "push_notification.outcome_update",
		TargetType: "push_notification_attempt",
		TargetID:   attempt.ID,
		Result:     normalized.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record push notification outcome audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push notification outcome: %w", err)
	}
	return nil
}

func readPushNotificationAttemptForUpdate(ctx context.Context, tx *sql.Tx, id string) (PushNotificationAttemptView, error) {
	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  user_id::text,
  recipient,
  subject,
  COALESCE(device_id::text, ''),
  platform,
  token_suffix,
  status,
  error_message,
  provider_message_id,
  provider_status,
  attempted_at
FROM push_notification_attempts
WHERE id = $1::uuid
FOR UPDATE`

	var attempt PushNotificationAttemptView
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.RFCMessageID,
		&attempt.CompanyID,
		&attempt.DomainID,
		&attempt.UserID,
		&attempt.Recipient,
		&attempt.Subject,
		&attempt.DeviceID,
		&attempt.Platform,
		&attempt.TokenSuffix,
		&attempt.Status,
		&attempt.ErrorMessage,
		&attempt.ProviderMessageID,
		&attempt.ProviderStatus,
		&attempt.AttemptedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PushNotificationAttemptView{}, fmt.Errorf("push notification attempt %q not found", id)
		}
		return PushNotificationAttemptView{}, fmt.Errorf("read push notification attempt for outcome audit: %w", err)
	}
	return attempt, nil
}

func pushNotificationOutcomeAuditDetail(attempt PushNotificationAttemptView, update UpdatePushNotificationOutcomeRequest, invalidTokenDeviceDeleted bool) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"attempt_id":                   attempt.ID,
		"message_id":                   attempt.MessageID,
		"rfc_message_id":               attempt.RFCMessageID,
		"platform":                     attempt.Platform,
		"device_id":                    attempt.DeviceID,
		"previous_status":              attempt.Status,
		"status":                       update.Status,
		"previous_error_message":       attempt.ErrorMessage,
		"error_message":                update.ErrorMessage,
		"previous_provider_message_id": attempt.ProviderMessageID,
		"provider_message_id":          update.ProviderMessageID,
		"previous_provider_status":     attempt.ProviderStatus,
		"provider_status":              update.ProviderStatus,
		"invalid_token_device_deleted": invalidTokenDeviceDeleted,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal push notification outcome audit detail: %w", err)
	}
	return detail, nil
}

func normalizeUpdatePushNotificationOutcomeRequest(req UpdatePushNotificationOutcomeRequest) (UpdatePushNotificationOutcomeRequest, error) {
	req.AttemptID = strings.TrimSpace(req.AttemptID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.ErrorMessage = cleanAdminBoundedText(req.ErrorMessage, 2000)
	req.ProviderMessageID = cleanAdminBoundedText(req.ProviderMessageID, 500)
	req.ProviderStatus = cleanAdminBoundedText(req.ProviderStatus, 500)
	if err := validatePushNotificationFilter("attempt_id", req.AttemptID); err != nil {
		return UpdatePushNotificationOutcomeRequest{}, err
	}
	if req.AttemptID == "" {
		return UpdatePushNotificationOutcomeRequest{}, fmt.Errorf("attempt_id is required")
	}
	if !allowedPushNotificationOutcomeStatus(req.Status) {
		return UpdatePushNotificationOutcomeRequest{}, fmt.Errorf("unsupported push notification outcome status")
	}
	return req, nil
}

func allowedPushNotificationOutcomeStatus(status string) bool {
	switch status {
	case "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}

func (r *Repository) GetPushNotificationStats(ctx context.Context, req PushNotificationStatsRequest) (PushNotificationStatsView, error) {
	if r.db == nil {
		return PushNotificationStatsView{}, fmt.Errorf("database handle is required")
	}
	var err error
	req, err = normalizePushNotificationStatsRequest(req)
	if err != nil {
		return PushNotificationStatsView{}, err
	}

	const query = `
SELECT
  COALESCE((
    SELECT COUNT(*)
    FROM push_devices
    WHERE status = 'active'
      AND (NULLIF($1, '')::uuid IS NULL OR user_id = NULLIF($1, '')::uuid)
      AND (NULLIF($3, '') IS NULL OR platform = NULLIF($3, ''))
      AND (NULLIF($4, '')::uuid IS NULL OR id = NULLIF($4, '')::uuid)
  ), 0),
  COALESCE(COUNT(*), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'candidate'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'queued'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'delivered'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'failed'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'invalid_token'), 0)
FROM push_notification_attempts
WHERE (NULLIF($1, '')::uuid IS NULL OR user_id = NULLIF($1, '')::uuid)
  AND (NULLIF($2, '')::uuid IS NULL OR message_id = NULLIF($2, '')::uuid)
  AND (NULLIF($3, '') IS NULL OR platform = NULLIF($3, ''))
  AND (NULLIF($4, '')::uuid IS NULL OR device_id = NULLIF($4, '')::uuid)
  AND ($5::timestamptz IS NULL OR attempted_at >= $5::timestamptz)`

	var stats PushNotificationStatsView
	if err := r.db.QueryRowContext(ctx, query, req.UserID, req.MessageID, req.Platform, req.DeviceID, nullableTime(req.Since)).Scan(
		&stats.ActiveDevices,
		&stats.TotalAttempts,
		&stats.Candidate,
		&stats.Queued,
		&stats.Delivered,
		&stats.Failed,
		&stats.InvalidToken,
	); err != nil {
		return PushNotificationStatsView{}, fmt.Errorf("get push notification stats: %w", err)
	}
	return stats, nil
}

func normalizePushNotificationStatsRequest(req PushNotificationStatsRequest) (PushNotificationStatsRequest, error) {
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	for field, value := range map[string]string{
		"message_id": req.MessageID,
		"user_id":    req.UserID,
		"platform":   req.Platform,
		"device_id":  req.DeviceID,
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return PushNotificationStatsRequest{}, err
		}
	}
	if req.Platform != "" && !allowedPushPlatform(req.Platform) {
		return PushNotificationStatsRequest{}, fmt.Errorf("unsupported push notification platform")
	}
	return req, nil
}

func validatePushNotificationFilter(field string, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain line breaks", field)
	}
	if len(value) > maxPushNotificationFilterBytes {
		return fmt.Errorf("%s is too long", field)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", field)
	}
	return nil
}

func cleanAdminBoundedText(value string, maxBytes int) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}

func (r *Repository) ListSuppressionEntries(ctx context.Context, req SuppressionEntryListRequest) ([]SuppressionEntry, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateSuppressionEntryListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	const query = `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
WHERE ($1 = '' OR domain_id::text = $1)
  AND ($2 = '' OR email = $2)
  AND ($3 = '' OR reason = $3)
ORDER BY created_at DESC
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(req.DomainID), strings.TrimSpace(req.Email), strings.TrimSpace(req.Reason), limit)
	if err != nil {
		return nil, fmt.Errorf("list suppression entries: %w", err)
	}
	defer rows.Close()

	var entries []SuppressionEntry
	for rows.Next() {
		var entry SuppressionEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.DomainID,
			&entry.Email,
			&entry.Reason,
			&entry.SourceMessageID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suppression entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression entries: %w", err)
	}
	return entries, nil
}

func ValidateSuppressionEntryListRequest(req SuppressionEntryListRequest) error {
	for field, value := range map[string]string{
		"domain_id": req.DomainID,
		"email":     req.Email,
		"reason":    req.Reason,
	} {
		if err := validatePushNotificationFilter(field, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListTrustedRelays(ctx context.Context, req TrustedRelayListRequest) ([]TrustedRelayView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateTrustedRelayListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	cidr := strings.TrimSpace(req.CIDR)
	if cidr != "" {
		normalized, err := normalizeTrustedRelayCIDR(cidr)
		if err != nil {
			return nil, err
		}
		cidr = normalized
	}
	description := strings.TrimSpace(req.Description)

	const query = `
SELECT
  id::text,
  cidr::text,
  description,
  created_at
FROM trusted_relays
WHERE ($1 = '' OR cidr = $1::cidr)
  AND ($2 = '' OR description ILIKE '%' || $2 || '%')
ORDER BY created_at DESC
LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, cidr, description, limit)
	if err != nil {
		return nil, fmt.Errorf("list trusted relays: %w", err)
	}
	defer rows.Close()

	var relays []TrustedRelayView
	for rows.Next() {
		var relay TrustedRelayView
		if err := rows.Scan(&relay.ID, &relay.CIDR, &relay.Description, &relay.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trusted relay: %w", err)
		}
		relays = append(relays, relay)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted relays: %w", err)
	}
	return relays, nil
}

func (r *Repository) CreateTrustedRelay(ctx context.Context, req CreateTrustedRelayRequest) (TrustedRelayView, error) {
	if r.db == nil {
		return TrustedRelayView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateTrustedRelayRequest(req); err != nil {
		return TrustedRelayView{}, err
	}
	cidr, err := normalizeTrustedRelayCIDR(req.CIDR)
	if err != nil {
		return TrustedRelayView{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TrustedRelayView{}, fmt.Errorf("begin trusted relay create transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO trusted_relays (cidr, description)
VALUES ($1, $2)
RETURNING id::text, cidr::text, description, created_at`

	var relay TrustedRelayView
	if err := tx.QueryRowContext(ctx, query, cidr, strings.TrimSpace(req.Description)).Scan(
		&relay.ID,
		&relay.CIDR,
		&relay.Description,
		&relay.CreatedAt,
	); err != nil {
		return TrustedRelayView{}, fmt.Errorf("create trusted relay: %w", err)
	}
	detail, err := trustedRelayAuditDetail(relay)
	if err != nil {
		return TrustedRelayView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "trusted_relay.create",
		TargetType: "trusted_relay",
		TargetID:   relay.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return TrustedRelayView{}, fmt.Errorf("record trusted relay create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TrustedRelayView{}, fmt.Errorf("commit trusted relay create transaction: %w", err)
	}
	return relay, nil
}

func (r *Repository) DeleteTrustedRelay(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("trusted relay id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trusted relay delete transaction: %w", err)
	}
	defer tx.Rollback()

	var relay TrustedRelayView
	if err := tx.QueryRowContext(ctx, `
SELECT id::text, cidr::text, description, created_at
FROM trusted_relays
WHERE id = $1
FOR UPDATE`, id).Scan(&relay.ID, &relay.CIDR, &relay.Description, &relay.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("trusted relay %q not found", id)
		}
		return fmt.Errorf("read trusted relay for deletion: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM trusted_relays WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete trusted relay: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("trusted relay %q not found", id)
	}
	detail, err := trustedRelayAuditDetail(relay)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "trusted_relay.delete",
		TargetType: "trusted_relay",
		TargetID:   relay.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record trusted relay delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trusted relay delete transaction: %w", err)
	}
	return nil
}

func trustedRelayAuditDetail(relay TrustedRelayView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"trusted_relay_id": relay.ID,
		"cidr":             relay.CIDR,
		"description":      relay.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal trusted relay audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListDeliveryRoutes(ctx context.Context, req DeliveryRouteListRequest) ([]DeliveryRouteView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDeliveryRouteListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := strings.ToLower(strings.TrimSpace(req.Status))

	const query = `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes
WHERE ($1 = '' OR status = $1)
  AND ($2 = '' OR farm = $2)
  AND ($3 = '' OR domain_pattern = $3)
ORDER BY created_at DESC
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, status, strings.TrimSpace(req.Farm), strings.TrimSpace(req.DomainPattern), limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery routes: %w", err)
	}
	defer rows.Close()

	var routes []DeliveryRouteView
	for rows.Next() {
		var route DeliveryRouteView
		if err := rows.Scan(
			&route.ID,
			&route.DomainPattern,
			&route.Farm,
			(*stringArray)(&route.Hosts),
			&route.Port,
			&route.TLSMode,
			&route.ImplicitTLS,
			&route.SMTPHello,
			&route.PoolName,
			&route.AuthIdentity,
			&route.AuthUsername,
			&route.Status,
			&route.Description,
			&route.CreatedAt,
			&route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery route: %w", err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery routes: %w", err)
	}
	return routes, nil
}

func (r *Repository) CreateDeliveryRoute(ctx context.Context, req CreateDeliveryRouteRequest) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDeliveryRouteRequest(req); err != nil {
		return DeliveryRouteView{}, err
	}
	domainPattern, err := normalizeDeliveryRouteDomainPattern(req.DomainPattern)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	hosts, err := normalizeDeliveryRouteHosts(req.Hosts)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	tlsMode, err := normalizeDeliveryRouteTLSMode(req.TLSMode)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	port := req.Port
	if port == 0 {
		port = 25
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryRouteView{}, fmt.Errorf("begin delivery route create transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO delivery_routes (
  domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, auth_password,
  description
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at`

	var route DeliveryRouteView
	if err := tx.QueryRowContext(
		ctx,
		query,
		domainPattern,
		strings.TrimSpace(req.Farm),
		stringArray(hosts),
		port,
		tlsMode,
		req.ImplicitTLS,
		strings.TrimSpace(req.SMTPHello),
		strings.TrimSpace(req.PoolName),
		strings.TrimSpace(req.AuthIdentity),
		strings.TrimSpace(req.AuthUsername),
		strings.TrimSpace(req.AuthPassword),
		strings.TrimSpace(req.Description),
	).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("create delivery route: %w", err)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.create",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("record delivery route create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("commit delivery route create transaction: %w", err)
	}
	return route, nil
}

func (r *Repository) UpdateDeliveryRouteStatus(ctx context.Context, req UpdateDeliveryRouteStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDeliveryRouteStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery route status transaction: %w", err)
	}
	defer tx.Rollback()

	var route DeliveryRouteView
	if err := tx.QueryRowContext(ctx, `
UPDATE delivery_routes
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at`, strings.TrimSpace(req.ID), strings.ToLower(strings.TrimSpace(req.Status))).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delivery route %q not found", req.ID)
		}
		return fmt.Errorf("update delivery route status: %w", err)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.status_update",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     route.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record delivery route status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery route status transaction: %w", err)
	}
	return nil
}

func (r *Repository) DeliveryRouteForDomain(ctx context.Context, domain string) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteView{}, fmt.Errorf("domain must be a domain name")
	}

	const query = `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  auth_password,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes
WHERE status = 'active'
  AND (
    domain_pattern = $1
    OR domain_pattern = '*'
    OR (
      left(domain_pattern, 2) = '*.'
      AND right($1, length(domain_pattern) - 1) = substring(domain_pattern from 2)
    )
  )
ORDER BY
  CASE
    WHEN domain_pattern = $1 THEN 0
    WHEN left(domain_pattern, 2) = '*.' THEN 1
    ELSE 2
  END,
  length(domain_pattern) DESC,
  created_at DESC
LIMIT 1`

	var route DeliveryRouteView
	if err := r.db.QueryRowContext(ctx, query, domain).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.AuthPassword,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DeliveryRouteView{}, ErrDeliveryRouteNotFound
		}
		return DeliveryRouteView{}, fmt.Errorf("get delivery route for domain: %w", err)
	}
	return route, nil
}

func (r *Repository) ResolveDeliveryRoute(ctx context.Context, domain string) (DeliveryRouteResolveView, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteResolveView{}, fmt.Errorf("domain must be a domain name")
	}
	route, err := r.DeliveryRouteForDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, ErrDeliveryRouteNotFound) {
			return DeliveryRouteResolveView{Domain: domain, Matched: false}, nil
		}
		return DeliveryRouteResolveView{}, err
	}
	return DeliveryRouteResolveView{Domain: domain, Matched: true, Route: &route}, nil
}

func (r *Repository) DeleteDeliveryRoute(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("delivery route id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery route delete transaction: %w", err)
	}
	defer tx.Rollback()

	var route DeliveryRouteView
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at
FROM delivery_routes
WHERE id = $1
FOR UPDATE`, id).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delivery route %q not found", id)
		}
		return fmt.Errorf("read delivery route for deletion: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM delivery_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete delivery route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("delivery route %q not found", id)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.delete",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record delivery route delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery route delete transaction: %w", err)
	}
	return nil
}

func deliveryRouteAuditDetail(route DeliveryRouteView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delivery_route_id": route.ID,
		"domain_pattern":    route.DomainPattern,
		"farm":              route.Farm,
		"hosts":             route.Hosts,
		"port":              route.Port,
		"tls_mode":          route.TLSMode,
		"implicit_tls":      route.ImplicitTLS,
		"smtp_hello":        route.SMTPHello,
		"pool_name":         route.PoolName,
		"auth_identity":     route.AuthIdentity,
		"auth_username":     route.AuthUsername,
		"status":            route.Status,
		"description":       route.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal delivery route audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) RetryOutbox(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("outbox event id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin outbox retry transaction: %w", err)
	}
	defer tx.Rollback()

	var event OutboxEventView
	var lockedAt sql.NullTime
	var processedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text,
  topic,
  partition_key,
  status,
  attempts,
  COALESCE(last_error, ''),
  created_at,
  available_at,
  locked_at,
  processed_at
FROM outbox
WHERE id = $1
FOR UPDATE`, id).Scan(
		&event.ID,
		&event.Topic,
		&event.PartitionKey,
		&event.Status,
		&event.Attempts,
		&event.LastError,
		&event.CreatedAt,
		&event.AvailableAt,
		&lockedAt,
		&processedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("outbox event %q not found", id)
		}
		return fmt.Errorf("read outbox event for retry: %w", err)
	}
	if lockedAt.Valid {
		event.LockedAt = &lockedAt.Time
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}

	const query = `
UPDATE outbox
SET status = 'pending',
    attempts = 0,
    last_error = NULL,
    locked_at = NULL,
    available_at = now(),
    processed_at = NULL
WHERE id = $1`

	result, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("outbox event %q not found", id)
	}
	detail, err := outboxRetryAuditDetail(event)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "outbox.retry",
		TargetType: "outbox_event",
		TargetID:   event.ID,
		Result:     "retried",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record outbox retry audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit outbox retry transaction: %w", err)
	}
	return nil
}

func outboxRetryAuditDetail(event OutboxEventView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"outbox_event_id":   event.ID,
		"topic":             event.Topic,
		"partition_key":     event.PartitionKey,
		"previous_status":   event.Status,
		"previous_attempts": event.Attempts,
		"previous_last_error": truncateUTF8Bytes(
			event.LastError,
			outboxEventListErrorPreviewBytes,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal outbox retry audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) DeleteSuppressionEntry(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("suppression entry id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin suppression delete transaction: %w", err)
	}
	defer tx.Rollback()

	var entry SuppressionEntry
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
WHERE id = $1
FOR UPDATE`, id).Scan(
		&entry.ID,
		&entry.DomainID,
		&entry.Email,
		&entry.Reason,
		&entry.SourceMessageID,
		&entry.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("suppression entry %q not found", id)
		}
		return fmt.Errorf("read suppression entry for deletion: %w", err)
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM suppression_list WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete suppression entry: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("suppression entry %q not found", id)
	}
	detail, err := suppressionEntryAuditDetail(entry)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		DomainID:   entry.DomainID,
		Category:   "admin",
		Action:     "suppression.delete",
		TargetType: "suppression_entry",
		TargetID:   entry.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record suppression delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit suppression delete transaction: %w", err)
	}
	return nil
}

func suppressionEntryAuditDetail(entry SuppressionEntry) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"suppression_entry_id": entry.ID,
		"domain_id":            entry.DomainID,
		"email":                entry.Email,
		"reason":               entry.Reason,
		"source_message_id":    entry.SourceMessageID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal suppression audit detail: %w", err)
	}
	return detail, nil
}

// ─── Invite tokens ───────────────────────────────────────────────────────────

type InviteToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	DomainID   string     `json:"domain_id"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by,omitempty"`
}

func (r *Repository) CreateInviteToken(ctx context.Context, userID, createdBy string) (InviteToken, error) {
	if r.db == nil {
		return InviteToken{}, fmt.Errorf("database handle is required")
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return InviteToken{}, fmt.Errorf("generate invite token: %w", err)
	}
	token := hex.EncodeToString(rawToken)
	expiresAt := time.Now().Add(72 * time.Hour)

	var it InviteToken
	err := r.db.QueryRowContext(ctx, `
INSERT INTO user_invite_tokens (user_id, domain_id, token, expires_at, created_by)
SELECT u.id, u.domain_id, $2, $3, NULLIF($4, '')::uuid
FROM users u WHERE u.id = $1
RETURNING id::text, user_id::text, domain_id::text, token, expires_at, accepted_at, created_at, COALESCE(created_by::text, '')`,
		userID, token, expiresAt, createdBy,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt, &it.CreatedAt, &it.CreatedBy)
	if err != nil {
		return InviteToken{}, fmt.Errorf("create invite token: %w", err)
	}
	return it, nil
}

func (r *Repository) GetInviteToken(ctx context.Context, token string) (InviteToken, error) {
	if r.db == nil {
		return InviteToken{}, fmt.Errorf("database handle is required")
	}
	var it InviteToken
	err := r.db.QueryRowContext(ctx, `
SELECT id::text, user_id::text, domain_id::text, token, expires_at, accepted_at, created_at, COALESCE(created_by::text, '')
FROM user_invite_tokens WHERE token = $1`, token,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt, &it.CreatedAt, &it.CreatedBy)
	if err == sql.ErrNoRows {
		return InviteToken{}, fmt.Errorf("invite token not found")
	}
	if err != nil {
		return InviteToken{}, fmt.Errorf("get invite token: %w", err)
	}
	return it, nil
}

func (r *Repository) AcceptInviteToken(ctx context.Context, token, passwordHash string) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserView{}, fmt.Errorf("begin accept invite transaction: %w", err)
	}
	defer tx.Rollback()

	var it InviteToken
	err = tx.QueryRowContext(ctx, `
SELECT id::text, user_id::text, domain_id::text, token, expires_at, accepted_at
FROM user_invite_tokens WHERE token = $1 FOR UPDATE`, token,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt)
	if err == sql.ErrNoRows {
		return UserView{}, fmt.Errorf("invite token not found")
	}
	if err != nil {
		return UserView{}, fmt.Errorf("lookup invite token: %w", err)
	}
	if it.AcceptedAt != nil {
		return UserView{}, fmt.Errorf("invite token already accepted")
	}
	if time.Now().After(it.ExpiresAt) {
		return UserView{}, fmt.Errorf("invite token expired")
	}

	var user UserView
	err = tx.QueryRowContext(ctx, `
UPDATE users SET password_hash = $2, must_change_password = false, status = 'active'
WHERE id = $1
RETURNING id::text, domain_id::text, username, display_name, role, status,
          COALESCE(password_hash, '') <> '', must_change_password,
          quota_used, COALESCE(quota_limit, 0), quota_source, created_at`,
		it.UserID, passwordHash,
	).Scan(&user.ID, &user.DomainID, &user.Username, &user.DisplayName,
		&user.Role, &user.Status, &user.PasswordConfigured, &user.MustChangePassword,
		&user.QuotaUsed, &user.QuotaLimit, &user.QuotaSource, &user.CreatedAt)
	if err != nil {
		return UserView{}, fmt.Errorf("set user password: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE user_invite_tokens SET accepted_at = now() WHERE id = $1`, it.ID); err != nil {
		return UserView{}, fmt.Errorf("mark invite accepted: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("commit accept invite: %w", err)
	}
	return user, nil
}

func (r *Repository) UpdateCompany(ctx context.Context, req UpdateCompanyRequest) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(req.ID) == "" {
		return CompanyView{}, fmt.Errorf("id is required")
	}
	if req.Name != "" && strings.TrimSpace(req.Name) == "" {
		return CompanyView{}, fmt.Errorf("name must not be blank")
	}
	if req.QuotaLimit < 0 {
		return CompanyView{}, fmt.Errorf("quota_limit must not be negative")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CompanyView{}, fmt.Errorf("begin update company transaction: %w", err)
	}
	defer tx.Rollback()

	var view CompanyView
	if err := tx.QueryRowContext(ctx, `
UPDATE companies
SET name        = CASE WHEN $2 <> '' THEN $2 ELSE name END,
    quota_limit = CASE WHEN $3::bigint >= 0 THEN NULLIF($3::bigint, 0) ELSE quota_limit END,
    updated_at  = now()
WHERE id = $1
RETURNING id::text, name, status, quota_used, COALESCE(quota_limit, 0), created_at`,
		strings.TrimSpace(req.ID), strings.TrimSpace(req.Name), req.QuotaLimit,
	).Scan(&view.ID, &view.Name, &view.Status, &view.QuotaUsed, &view.QuotaLimit, &view.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CompanyView{}, fmt.Errorf("company %q not found", req.ID)
		}
		return CompanyView{}, fmt.Errorf("update company: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.ID,
		Category:   "admin",
		Action:     "company.update",
		TargetType: "company",
		TargetID:   view.ID,
		Result:     "updated",
	}); err != nil {
		return CompanyView{}, fmt.Errorf("record company update audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CompanyView{}, fmt.Errorf("commit update company transaction: %w", err)
	}
	return view, nil
}

func (r *Repository) DeleteCompany(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete company transaction: %w", err)
	}
	defer tx.Rollback()

	var domainCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM domains WHERE company_id = $1`, id).Scan(&domainCount); err != nil {
		return fmt.Errorf("check company domains: %w", err)
	}
	if domainCount > 0 {
		return fmt.Errorf("cannot delete company with %d domain(s); remove all domains first", domainCount)
	}

	var name string
	if err := tx.QueryRowContext(ctx, `DELETE FROM companies WHERE id = $1 RETURNING name`, id).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("company %q not found", id)
		}
		return fmt.Errorf("delete company: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  id,
		Category:   "admin",
		Action:     "company.delete",
		TargetType: "company",
		TargetID:   id,
		Result:     "deleted",
	}); err != nil {
		return fmt.Errorf("record company delete audit: %w", err)
	}
	return tx.Commit()
}

func (r *Repository) DeleteDomain(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete domain transaction: %w", err)
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE domain_id = $1`, id).Scan(&userCount); err != nil {
		return fmt.Errorf("check domain users: %w", err)
	}
	if userCount > 0 {
		return fmt.Errorf("cannot delete domain with %d user(s); remove all users first", userCount)
	}

	var companyID, name string
	if err := tx.QueryRowContext(ctx, `DELETE FROM domains WHERE id = $1 RETURNING company_id::text, name`, id).Scan(&companyID, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", id)
		}
		return fmt.Errorf("delete domain: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  companyID,
		DomainID:   id,
		Category:   "admin",
		Action:     "domain.delete",
		TargetType: "domain",
		TargetID:   id,
		Result:     "deleted",
	}); err != nil {
		return fmt.Errorf("record domain delete audit: %w", err)
	}
	return tx.Commit()
}
