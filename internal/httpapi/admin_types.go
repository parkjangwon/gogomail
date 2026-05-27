package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/storage"
	"github.com/redis/go-redis/v9"
)

type adminRouteConfig struct {
	routeCounters          *delivery.RouteCounters
	storageCapabilities    *storage.BackendCapabilities
	configNotifier         configstore.Notifier
	tokenMgr               *auth.TokenManager
	environment            string
	adminMFAStore          MFAStore
	adminMFARequired       bool
	configResolver         ConfigResolver
	dlqReader              eventstream.DLQReader
	systemEmailSender      mailservice.SystemEmailSender
	publicBaseURL          string
	bgTracker              *BackgroundTracker
	adminBootstrapEmail    string
	adminBootstrapPassword string
	redisLoginClient       *redis.Client
}

// AdminRouteOption configures optional capabilities for RegisterAdminRoutes.
type AdminRouteOption func(*adminRouteConfig)

// WithRouteCounters enables the GET /admin/v1/delivery-routes/counters endpoint.
func WithRouteCounters(c *delivery.RouteCounters) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.routeCounters = c }
}

func WithStorageCapabilities(capabilities storage.BackendCapabilities) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.storageCapabilities = &capabilities }
}

// WithConfigNotifier wires a configstore.Notifier into the SSE config-stream
// endpoint so that config change events are pushed to connected admin clients.
func WithConfigNotifier(n configstore.Notifier) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.configNotifier = n }
}

// WithTokenManager enables JWT-based admin authentication in addition to the
// static admin token. Users with role company_admin or system_admin may log in
// and receive a signed JWT that is accepted by all admin routes.
func WithTokenManager(tm *auth.TokenManager) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.tokenMgr = tm }
}

func WithEnvironment(environment string) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.environment = strings.TrimSpace(environment) }
}

func WithAdminMFAStore(s MFAStore) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.adminMFAStore = s }
}

func WithAdminMFARequired(required bool) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.adminMFARequired = required }
}

func WithAdminConfigResolver(r ConfigResolver) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.configResolver = r }
}

// WithDLQReader enables the GET /admin/v1/dlq and DELETE /admin/v1/dlq/{id}
// endpoints for operator visibility into dead-letter streams.
func WithDLQReader(r eventstream.DLQReader) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.dlqReader = r }
}

// WithRedisLoginLimiter enables a distributed Redis-backed rate limiter for
// the admin login endpoint. Falls back to in-memory when not configured.
func WithRedisLoginLimiter(client *redis.Client) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.redisLoginClient = client }
}

func WithSystemEmailSender(sender mailservice.SystemEmailSender, publicBaseURL string) AdminRouteOption {
	return func(cfg *adminRouteConfig) {
		cfg.systemEmailSender = sender
		cfg.publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	}
}

// WithBackgroundTracker wires a BackgroundTracker that the admin handlers use
// to track fire-and-forget goroutines (invite/welcome email sends). Without it
// such goroutines run unsupervised and may be dropped on graceful shutdown.
func WithBackgroundTracker(t *BackgroundTracker) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.bgTracker = t }
}

// WithAdminBootstrap sets the bootstrap admin credentials used to seed a
// system admin on first startup. Leave both empty to disable bootstrap login.
func WithAdminBootstrap(email, password string) AdminRouteOption {
	return func(cfg *adminRouteConfig) {
		cfg.adminBootstrapEmail = strings.TrimSpace(email)
		cfg.adminBootstrapPassword = password
	}
}

type adminContextKey struct{}
type requestIDContextKey struct{}

const companyDomainSettingsDefaultsKey = "domain_settings_defaults"

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, strings.TrimSpace(requestID))
}

func RequestIDAttr(ctx context.Context) slog.Attr {
	return slog.String("request_id", RequestIDFromContext(ctx))
}

func RequestContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs := make([]slog.Attr, 0, 5)
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if claims, ok := ctx.Value(adminContextKey{}).(auth.Claims); ok {
		if claims.UserID != "" {
			attrs = append(attrs, slog.String("user_id", claims.UserID), slog.String("actor_id", claims.UserID))
		}
		if claims.CompanyID != "" {
			attrs = append(attrs, slog.String("company_id", claims.CompanyID), slog.String("tenant_id", claims.CompanyID))
		}
		if claims.DomainID != "" {
			attrs = append(attrs, slog.String("domain_id", claims.DomainID))
		}
	}
	return attrs
}

type adminIMAPUIDBackfillItem struct {
	MessageID string `json:"message_id"`
	MailboxID string `json:"mailbox_id"`
	UID       uint32 `json:"uid"`
	ModSeq    uint64 `json:"modseq"`
}

type AdminBackpressureService interface {
	GetBackpressure(ctx context.Context) (backpressure.State, error)
	UpdateBackpressure(ctx context.Context, req backpressure.StateUpdate) (backpressure.State, error)
}

type adminConsoleCapabilitiesEnvelope struct {
	AdminConsoleCapabilities adminConsoleCapabilities `json:"admin_console_capabilities"`
}

type adminConsoleCapabilities struct {
	ContractVersion string                              `json:"contract_version"`
	Modules         map[string]string                   `json:"modules"`
	Limits          adminConsoleLimits                  `json:"limits"`
	Tenancy         adminConsoleTenancyCapabilities     `json:"tenancy"`
	Operations      adminConsoleOperationCapabilities   `json:"operations"`
	Security        adminConsoleSecurityCapabilities    `json:"security"`
	Integrations    adminConsoleIntegrationCapabilities `json:"integrations"`
	Storage         storage.BackendCapabilities         `json:"storage"`
}

type adminConsoleLimits struct {
	MaxListLimit                 int `json:"max_list_limit"`
	MaxAttachmentCleanupLimit    int `json:"max_attachment_cleanup_limit"`
	MaxAPIUsageRetentionRunLimit int `json:"max_api_usage_retention_run_limit"`
}

type adminConsoleTenancyCapabilities struct {
	Companies      bool `json:"companies"`
	Domains        bool `json:"domains"`
	Users          bool `json:"users"`
	Quotas         bool `json:"quotas"`
	DomainPolicies bool `json:"domain_policies"`
	DNSChecks      bool `json:"dns_checks"`
	DKIMKeys       bool `json:"dkim_keys"`
}

type adminConsoleOperationCapabilities struct {
	QueueStats                bool `json:"queue_stats"`
	OutboxEvents              bool `json:"outbox_events"`
	AuditLogs                 bool `json:"audit_logs"`
	AuditIntegrity            bool `json:"audit_integrity"`
	Backpressure              bool `json:"backpressure"`
	AttachmentCleanup         bool `json:"attachment_cleanup"`
	AttachmentUploadSession   bool `json:"attachment_upload_sessions"`
	DirectoryPrincipals       bool `json:"directory_principals"`
	DirectoryAliases          bool `json:"directory_aliases"`
	DirectoryDelegations      bool `json:"directory_delegations"`
	DirectoryGroupMemberships bool `json:"directory_group_memberships"`
	DriveUploadSessions       bool `json:"drive_upload_sessions"`
	DriveNodes                bool `json:"drive_nodes"`
	DriveNodeDetail           bool `json:"drive_node_detail"`
	DriveUsageSummary         bool `json:"drive_usage_summary"`
	DriveUploadCleanup        bool `json:"drive_upload_cleanup"`
	DriveCleanupFailures      bool `json:"drive_cleanup_failures"`
	DriveCleanupFailureRetry  bool `json:"drive_cleanup_failure_retry"`
	QuotaReconciliation       bool `json:"quota_reconciliation"`
	DeliveryAttempts          bool `json:"delivery_attempts"`
	DeliveryRoutes            bool `json:"delivery_routes"`
	TrustedRelays             bool `json:"trusted_relays"`
	SuppressionList           bool `json:"suppression_list"`
	PushNotificationTriage    bool `json:"push_notification_triage"`
	APIUsage                  bool `json:"api_usage"`
	APIUsageExport            bool `json:"api_usage_export"`
	DAVSyncRetention          bool `json:"dav_sync_retention"`
	IMAPUIDBackfill           bool `json:"imap_uid_backfill"`
}

type adminConsoleSecurityCapabilities struct {
	AdminTokenHeader     bool `json:"admin_token_header"`
	BearerToken          bool `json:"bearer_token"`
	RejectsAmbiguousAuth bool `json:"rejects_ambiguous_auth"`
	NoStoreJSON          bool `json:"no_store_json"`
}

type adminConsoleIntegrationCapabilities struct {
	LDAPRead         string `json:"ldap_read"`
	LDAPSync         string `json:"ldap_sync"`
	OrganizationSync string `json:"organization_sync"`
}

func currentAdminConsoleCapabilities(storageCapabilities storage.BackendCapabilities) adminConsoleCapabilities {
	return adminConsoleCapabilities{
		ContractVersion: BackendContractVersion,
		Modules: map[string]string{
			"mail":  "available",
			"admin": "available",
			"drive": "available",
		},
		Limits: adminConsoleLimits{
			MaxListLimit:                 maildb.MessageListMaxLimit,
			MaxAttachmentCleanupLimit:    maildb.AttachmentCleanupMaxLimit,
			MaxAPIUsageRetentionRunLimit: maildb.APIUsageLedgerRetentionMaxLimit,
		},
		Tenancy: adminConsoleTenancyCapabilities{
			Companies:      true,
			Domains:        true,
			Users:          true,
			Quotas:         true,
			DomainPolicies: true,
			DNSChecks:      true,
			DKIMKeys:       true,
		},
		Operations: adminConsoleOperationCapabilities{
			QueueStats:                true,
			OutboxEvents:              true,
			AuditLogs:                 true,
			AuditIntegrity:            true,
			Backpressure:              true,
			AttachmentCleanup:         true,
			AttachmentUploadSession:   true,
			DirectoryPrincipals:       true,
			DirectoryAliases:          true,
			DirectoryDelegations:      true,
			DirectoryGroupMemberships: true,
			DriveUploadSessions:       true,
			DriveNodes:                true,
			DriveNodeDetail:           true,
			DriveUsageSummary:         true,
			DriveUploadCleanup:        true,
			DriveCleanupFailures:      true,
			DriveCleanupFailureRetry:  true,
			QuotaReconciliation:       true,
			DeliveryAttempts:          true,
			DeliveryRoutes:            true,
			TrustedRelays:             true,
			SuppressionList:           true,
			PushNotificationTriage:    true,
			APIUsage:                  true,
			APIUsageExport:            true,
			DAVSyncRetention:          true,
			IMAPUIDBackfill:           true,
		},
		Security: adminConsoleSecurityCapabilities{
			AdminTokenHeader:     true,
			BearerToken:          true,
			RejectsAmbiguousAuth: true,
			NoStoreJSON:          true,
		},
		Integrations: adminConsoleIntegrationCapabilities{
			LDAPRead:         "available",
			LDAPSync:         "unavailable",
			OrganizationSync: "unavailable",
		},
		Storage: storageCapabilities,
	}
}

func storageCapabilitiesFromRouteConfig(cfg adminRouteConfig) storage.BackendCapabilities {
	if cfg.storageCapabilities != nil {
		return *cfg.storageCapabilities
	}
	activeLabels := []string{"local"}
	supportsLocalNFS, supportsMinIO, supportsAWSCompatible := storage.SupportMatrixForLabels(activeLabels)
	return storage.BackendCapabilities{
		ContractVersion:       BackendContractVersion,
		ConfiguredBackend:     "local",
		BackendClass:          "local",
		ActiveLabels:          activeLabels,
		Operations:            []string{"put", "get", "get_range", "stat", "copy", "move", "list", "delete"},
		LocalFilesystem:       true,
		S3Compatible:          false,
		PathStyleAddressing:   false,
		CompatLabelsEnabled:   false,
		ReadinessProbe:        true,
		SecretsRedacted:       true,
		SupportsBackendSwitch: true,
		SupportsLocalNFS:      supportsLocalNFS,
		SupportsMinIO:         supportsMinIO,
		SupportsAWSCompatible: supportsAWSCompatible,
		RequiresByteMigration: true,
	}
}

type adminAttachmentCleanupRunRequest struct {
	Before string `json:"before"`
	Limit  int    `json:"limit,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

type adminDriveCleanupFailureRetryRunRequest struct {
	UserID string `json:"user_id,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type adminAPIUsageLedgerRetentionRunRequest struct {
	Cutoff       string `json:"cutoff"`
	TenantID     string `json:"tenant_id,omitempty"`
	PrincipalID  string `json:"principal_id,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
	ConfirmReady bool   `json:"confirm_ready,omitempty"`
}

type adminDAVSyncRetentionRunRequest struct {
	Cutoff       string `json:"cutoff"`
	Limit        int    `json:"limit,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
	ConfirmReady bool   `json:"confirm_ready,omitempty"`
}

type adminConfigSetRequest struct {
	Value   json.RawMessage `json:"value"`
	Locked  bool            `json:"locked"`
	Version int64           `json:"version,omitempty"`
}

type adminConfigPropagateRequest struct {
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value"`
	Locked bool            `json:"locked"`
}

var defaultDomainMCPPolicy = json.RawMessage(`{"enabled":false,"allow_bypass_mode":false,"allow_user_access_keys":false,"allowed_scopes":[],"force_generated_mail_notice":false,"external_recipient_confirmation":"basic","public_drive_share_confirmation":"basic","audit_level":"full"}`)
