package maildb

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/apimeter"
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
	ProbeMore    bool // when true the query fetches Limit+1 to detect has_more
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

const listOutboxEventsBaseSQL = `
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
FROM outbox`

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

const apiUsageAggregateProjectionSQL = `
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
  last_seen_at`

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

const listAPIUsageExportBatchesBaseSQL = `
SELECT id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest
FROM api_usage_export_batches`

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
	ProbeMore       bool // when true the query fetches Limit+1 to detect has_more
}

const listDeliveryAttemptsBaseSQL = `
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
FROM delivery_attempts`

const deliveryAttemptStatsBaseSQL = `
SELECT
  count(*)::bigint,
  count(DISTINCT message_id)::bigint,
  count(DISTINCT recipient)::bigint,
  count(*) FILTER (WHERE status = 'delivered')::bigint,
  count(*) FILTER (WHERE status = 'failed')::bigint,
  count(*) FILTER (WHERE status = 'bounced')::bigint,
  count(*) FILTER (WHERE status = 'exhausted')::bigint
FROM delivery_attempts`

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

const listPushNotificationAttemptsBaseSQL = `
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
FROM push_notification_attempts`

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
	Limit     int
	Status    string
	ProbeMore bool // when true the query fetches Limit+1 to detect has_more
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

const listDomainDNSChecksBaseSQL = `
SELECT
  id::text,
  domain_id::text,
  status,
  report,
  checked_at
FROM domain_dns_checks
WHERE domain_id = $1::uuid`

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
	ProbeMore bool // when true the query fetches Limit+1 to detect has_more
}

type UserView struct {
	ID                 string    `json:"id"`
	DomainID           string    `json:"domain_id"`
	Username           string    `json:"username"`
	DisplayName        string    `json:"display_name"`
	RecoveryEmail      string    `json:"recovery_email,omitempty"`
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
	CompanyID          string
	DomainID           string
	Status             string
	PasswordConfigured *bool
	Limit              int
	ProbeMore          bool // when true the query fetches Limit+1 to detect has_more
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
	RecoveryEmail      string `json:"recovery_email,omitempty"`
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

type BulkUpdateUserStatusRequest struct {
	IDs       []string `json:"ids"`
	Status    string   `json:"status"`
	CompanyID string   `json:"company_id,omitempty"`
}

type BulkUpdateUserStatusResult struct {
	Updated []string `json:"updated"`
	Failed  []string `json:"failed"`
}

type DeleteUserRequest struct {
	ID string `json:"id"`
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

type UpdateUserRecoveryEmailRequest struct {
	ID            string `json:"id"`
	RecoveryEmail string `json:"recovery_email"`
}

var validUserRoles = map[string]bool{
	"user":          true,
	"company_admin": true,
	"system_admin":  true,
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

func ValidateUpdateUserRecoveryEmailRequest(req UpdateUserRecoveryEmailRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if _, err := normalizeRecoveryEmail(req.RecoveryEmail); err != nil {
		return err
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
	if _, err := normalizeRecoveryEmail(req.RecoveryEmail); err != nil {
		return err
	}
	if strings.TrimSpace(req.PasswordHash) != "" {
		if err := auth.ValidatePasswordHash(req.PasswordHash, true); err != nil {
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
















