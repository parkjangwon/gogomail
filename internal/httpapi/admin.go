package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
)

type adminRouteConfig struct {
	routeCounters       *delivery.RouteCounters
	storageCapabilities *storage.BackendCapabilities
	configNotifier      configstore.Notifier
	tokenMgr            *auth.TokenManager
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

type adminContextKey struct{}

func adminClaimsFromCtx(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(adminContextKey{}).(auth.Claims)
	return c, ok
}

func adminJWTOrStaticAuth(token string, tokenMgr *auth.TokenManager, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	return func(w http.ResponseWriter, r *http.Request) {
		// No auth configured: allow all (dev/test mode, same as original adminAuth behaviour).
		if token == "" && tokenMgr == nil {
			if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
				return
			}
			next(w, r)
			return
		}

		got, ok := adminTokenFromRequest(w, r)
		if !ok {
			return
		}
		authorized := false
		if tokenMgr != nil && got != "" {
			if claims, err := tokenMgr.Verify(got); err == nil {
				if claims.Role == "company_admin" || claims.Role == "system_admin" {
					r = r.WithContext(context.WithValue(r.Context(), adminContextKey{}, claims))
					authorized = true
				}
			}
		}
		if !authorized && token != "" && constantTimeTokenEqual(got, token) {
			authorized = true
		}
		if !authorized {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
			return
		}
		next(w, r)
	}
}

func rejectUnknownAPIUsageAggregateQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r,
		"limit",
		"tenant_id",
		"company_id",
		"domain_id",
		"user_id",
		"api_key_id",
		"principal_id",
		"auth_source",
		"method",
		"route",
		"status",
		"from",
		"to",
	)
}

func rejectUnknownAPIUsageLedgerQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageLedgerStatsQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageRetentionReadinessQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "cutoff", "tenant_id", "principal_id")
}

func rejectUnknownAPIUsageRetentionRunListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "created_from", "created_to")
}

func rejectUnknownDAVSyncRetentionRunListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "status", "created_from", "created_to")
}

func rejectUnknownDAVSyncRetentionReadinessQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "cutoff", "limit")
}

func rejectUnknownAPIUsageExportBatchCreateQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageExportBatchListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "status", "from", "to")
}

type AdminService interface {
	ListCompanies(ctx context.Context, req maildb.CompanyListRequest) ([]maildb.CompanyView, error)
	CreateCompany(ctx context.Context, req maildb.CreateCompanyRequest) (maildb.CompanyView, error)
	GetCompany(ctx context.Context, id string) (maildb.CompanyView, error)
	UpdateCompanyQuota(ctx context.Context, req maildb.UpdateCompanyQuotaRequest) error
	UpdateCompany(ctx context.Context, req maildb.UpdateCompanyRequest) (maildb.CompanyView, error)
	DeleteCompany(ctx context.Context, id string) error
	ListDomains(ctx context.Context, req maildb.DomainListRequest) ([]maildb.DomainView, error)
	GetDomain(ctx context.Context, id string) (maildb.DomainView, error)
	GetDomainStats(ctx context.Context, id string) (maildb.DomainStatsView, error)
	VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error)
	ListDomainDNSChecks(ctx context.Context, req maildb.DomainDNSCheckListRequest) ([]maildb.DomainDNSCheckView, error)
	CreateDomain(ctx context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error)
	UpdateDomainStatus(ctx context.Context, req maildb.UpdateDomainStatusRequest) error
	UpdateDomainQuota(ctx context.Context, req maildb.UpdateDomainQuotaRequest) error
	DeleteDomain(ctx context.Context, id string) error
	UpdateDomainPolicy(ctx context.Context, req maildb.UpdateDomainPolicyRequest) (maildb.DomainPolicyView, error)
	GetDomainSettings(ctx context.Context, domainID string) (*admin.DomainSettings, error)
	UpdateDomainSettings(ctx context.Context, settings *admin.DomainSettings) error
	GetAPISettings(ctx context.Context, domainID string) (*admin.APISettings, error)
	UpdateAPISettings(ctx context.Context, settings *admin.APISettings) error
	CreateAPIKey(ctx context.Context, key *admin.APIKey) (secret string, err error)
	ListAPIKeys(ctx context.Context, domainID string) ([]admin.APIKey, error)
	DeleteAPIKey(ctx context.Context, keyID string) error
	RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error)
	ListUsers(ctx context.Context, req maildb.UserListRequest) ([]maildb.UserView, error)
	GetUser(ctx context.Context, id string) (maildb.UserView, error)
	CreateUser(ctx context.Context, req maildb.CreateUserRequest) (maildb.UserView, error)
	UpdateUserStatus(ctx context.Context, req maildb.UpdateUserStatusRequest) error
	UpdateUserQuota(ctx context.Context, req maildb.UpdateUserQuotaRequest) error
	UpdateUserPasswordHash(ctx context.Context, req maildb.UpdateUserPasswordHashRequest) error
	UpdateUserRole(ctx context.Context, req maildb.UpdateUserRoleRequest) error
	AuthenticateUser(ctx context.Context, email, password string) (maildb.AuthenticatedUser, error)
	ListQueueStats(ctx context.Context) ([]maildb.QueueStat, error)
	ListOutboxEvents(ctx context.Context, req maildb.OutboxEventListRequest) ([]maildb.OutboxEventView, error)
	GetOutboxEvent(ctx context.Context, id string) (maildb.OutboxEventView, error)
	ListAuditLogs(ctx context.Context, req maildb.AuditLogListRequest) ([]maildb.AuditLogView, error)
	GetAuditLog(ctx context.Context, id string) (maildb.AuditLogView, error)
	CheckAuditLogIntegrity(ctx context.Context, req maildb.AuditLogIntegrityRequest) (maildb.AuditLogIntegrityView, error)
	ListMailFlowLogs(ctx context.Context, req maildb.MailFlowLogListRequest) ([]maildb.MailFlowLogView, error)
	GetMailFlowLog(ctx context.Context, id string) (maildb.MailFlowLogView, error)
	GetMailFlowLogStats(ctx context.Context, req maildb.MailFlowLogStatsRequest) (maildb.MailFlowLogStatsView, error)
	GetMailFlowLogDailyStats(ctx context.Context, req maildb.MailFlowLogDailyStatsRequest) ([]maildb.MailFlowLogDailyStatsView, error)
	ListQuotaUsage(ctx context.Context, req maildb.QuotaUsageListRequest) ([]maildb.QuotaUsageView, error)
	RunAttachmentCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error)
	CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error)
	ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error)
	RunAttachmentUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error)
	CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error)
	ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error)
	ListAttachmentUploadSessions(ctx context.Context, req maildb.AttachmentUploadSessionListRequest) ([]maildb.AttachmentUploadSession, error)
	SearchDirectoryPrincipals(ctx context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error)
	CreateDirectoryAlias(ctx context.Context, req directory.CreateAliasRequest) (directory.Alias, error)
	CreateDirectoryDelegation(ctx context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error)
	CreateDirectoryGroupMembership(ctx context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error)
	DeleteDirectoryAlias(ctx context.Context, id string) (directory.Alias, error)
	DeleteDirectoryDelegation(ctx context.Context, id string) (directory.Delegation, error)
	DeleteDirectoryGroupMembership(ctx context.Context, id string) (directory.GroupMembership, error)
	ListDirectoryGroupMemberships(ctx context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error)
	ResolveDirectoryAlias(ctx context.Context, req directory.ResolveAliasRequest) (directory.Alias, error)
	ListDirectoryAliases(ctx context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error)
	ListDirectoryDelegations(ctx context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error)
	UpdateDirectoryDelegationRole(ctx context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error)
	ReassignDirectoryDelegation(ctx context.Context, req directory.ReassignDelegationRequest) (directory.Delegation, error)
	ReassignDirectoryGroupMembership(ctx context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error)
	UpdateDirectoryGroupMembershipRole(ctx context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error)
	ListDriveUploadSessions(ctx context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error)
	ListDriveNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetDriveNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	GetDriveUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
	CountStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) (drive.StaleUploadSessionCount, error)
	ListStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error)
	RunDriveUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error)
	ListDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error)
	ResolveDriveObjectCleanupFailure(ctx context.Context, id string) (drive.ObjectCleanupFailure, error)
	RetryDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error)
	ListAPIUsageDaily(ctx context.Context, req maildb.APIUsageAggregateListRequest) ([]maildb.APIUsageDailyView, error)
	ListAPIUsageMonthly(ctx context.Context, req maildb.APIUsageAggregateListRequest) ([]maildb.APIUsageMonthlyView, error)
	ListAPIUsageLedger(ctx context.Context, req maildb.APIUsageLedgerListRequest) ([]maildb.APIUsageLedgerView, error)
	GetAPIUsageLedgerStats(ctx context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageLedgerStatsView, error)
	GetAPIUsageLedgerRetentionReadiness(ctx context.Context, req maildb.APIUsageLedgerRetentionRequest) (maildb.APIUsageLedgerRetentionReadinessView, error)
	RunAPIUsageLedgerRetention(ctx context.Context, req maildb.APIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunView, error)
	ListAPIUsageLedgerRetentionRuns(ctx context.Context, req maildb.APIUsageLedgerRetentionRunListRequest) ([]maildb.APIUsageLedgerRetentionRunView, error)
	GetAPIUsageLedgerRetentionRun(ctx context.Context, id string) (maildb.APIUsageLedgerRetentionRunView, error)
	RunDAVSyncRetention(ctx context.Context, req davsyncretention.RunRequest) (davsyncretention.RunRecord, error)
	ListDAVSyncRetentionRuns(ctx context.Context, req davsyncretention.RunListRequest) ([]davsyncretention.RunRecord, error)
	GetDAVSyncRetentionRun(ctx context.Context, id string) (davsyncretention.RunRecord, error)
	GetDAVSyncRetentionReadiness(ctx context.Context, req davsyncretention.ReadinessRequest) (davsyncretention.ReadinessView, error)
	GetAPIUsageExportCapabilities(ctx context.Context) (maildb.APIUsageExportCapabilityView, error)
	CreateAPIUsageExportBatch(ctx context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageExportBatchView, error)
	ListAPIUsageExportBatches(ctx context.Context, req maildb.APIUsageExportBatchListRequest) ([]maildb.APIUsageExportBatchView, error)
	GetAPIUsageExportBatch(ctx context.Context, id string) (maildb.APIUsageExportBatchView, error)
	GetAPIUsageExportHandoff(ctx context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error)
	CreateAPIUsageExportArtifact(ctx context.Context, req maildb.CreateAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error)
	WriteAPIUsageExportArtifact(ctx context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error)
	ListAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int) ([]maildb.APIUsageExportArtifactView, error)
	GetAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, error)
	OpenAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error)
	VerifyAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error)
	CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (maildb.APIUsageExportManifestDigestView, error)
	ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]maildb.APIUsageExportManifestDigestView, error)
	GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestView, error)
	VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestVerificationView, error)
	CreateAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error)
	ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]maildb.APIUsageExportManifestSignatureView, error)
	GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureView, error)
	VerifyAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error)
	ListQuotaReconciliation(ctx context.Context, limit int) ([]maildb.QuotaReconciliationView, error)
	CorrectQuotaReconciliation(ctx context.Context, req maildb.CorrectQuotaReconciliationRequest) (maildb.QuotaCorrectionResult, error)
	ListDeliveryAttempts(ctx context.Context, req maildb.DeliveryAttemptListRequest) ([]maildb.DeliveryAttemptView, error)
	GetDeliveryAttemptStats(ctx context.Context, req maildb.DeliveryAttemptStatsRequest) (maildb.DeliveryAttemptStatsView, error)
	ListExhaustedAttempts(ctx context.Context, req maildb.ExhaustedAttemptListRequest) ([]maildb.DeliveryAttemptView, error)
	ListPushNotificationAttempts(ctx context.Context, req maildb.PushNotificationAttemptListRequest) ([]maildb.PushNotificationAttemptView, error)
	GetPushNotificationAttempt(ctx context.Context, id string) (maildb.PushNotificationAttemptView, error)
	UpdatePushNotificationOutcome(ctx context.Context, req maildb.UpdatePushNotificationOutcomeRequest) error
	GetPushNotificationStats(ctx context.Context, req maildb.PushNotificationStatsRequest) (maildb.PushNotificationStatsView, error)
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
	DeletePushDevice(ctx context.Context, userID string, id string) error
	DeleteAllPushDevices(ctx context.Context, userID string) (int, error)
	ListSuppressionEntries(ctx context.Context, req maildb.SuppressionEntryListRequest) ([]maildb.SuppressionEntry, error)
	ListTrustedRelays(ctx context.Context, req maildb.TrustedRelayListRequest) ([]maildb.TrustedRelayView, error)
	CreateTrustedRelay(ctx context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error)
	DeleteTrustedRelay(ctx context.Context, id string) error
	ListDeliveryRoutes(ctx context.Context, req maildb.DeliveryRouteListRequest) ([]maildb.DeliveryRouteView, error)
	CreateDeliveryRoute(ctx context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error)
	ResolveDeliveryRoute(ctx context.Context, domain string) (maildb.DeliveryRouteResolveView, error)
	UpdateDeliveryRouteStatus(ctx context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error
	DeleteDeliveryRoute(ctx context.Context, id string) error
	ListDKIMKeys(ctx context.Context, req maildb.DKIMKeyListRequest) ([]maildb.DKIMKeyView, error)
	CreateDKIMKey(ctx context.Context, input maildb.CreateDKIMKeyInput) (string, error)
	DeactivateDKIMKey(ctx context.Context, id string) error
	VerifyDKIMKeyDNS(ctx context.Context, keyID string) (maildb.DKIMKeyDNSVerificationResult, error)
	RetryOutbox(ctx context.Context, id string) error
	DeleteSuppressionEntry(ctx context.Context, id string) error
	BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error)
	ListQuotaAlertThresholds(ctx context.Context, req maildb.QuotaAlertThresholdListRequest) ([]maildb.QuotaAlertThresholdView, error)
	GetQuotaAlertThreshold(ctx context.Context, id string) (maildb.QuotaAlertThresholdView, error)
	CreateQuotaAlertThreshold(ctx context.Context, req maildb.CreateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error)
	UpdateQuotaAlertThreshold(ctx context.Context, req maildb.UpdateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error)
	DeleteQuotaAlertThreshold(ctx context.Context, id string) error
	ListQuotaAlerts(ctx context.Context, req maildb.QuotaAlertListRequest) ([]maildb.QuotaAlertView, error)
	GetQuotaAlert(ctx context.Context, id string) (maildb.QuotaAlertView, error)
	GetCompanyConfig(ctx context.Context, companyID, key string) (configstore.ConfigEntry, error)
	SetCompanyConfig(ctx context.Context, companyID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error)
	DeleteCompanyConfig(ctx context.Context, companyID, key string, expectedVersion int64) error
	ListCompanyConfig(ctx context.Context, companyID string) ([]configstore.ConfigEntry, error)
	GetDomainConfig(ctx context.Context, domainID, key string) (configstore.ConfigEntry, error)
	SetDomainConfig(ctx context.Context, domainID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error)
	DeleteDomainConfig(ctx context.Context, domainID, key string, expectedVersion int64) error
	ListDomainConfig(ctx context.Context, domainID string) ([]configstore.ConfigEntry, error)
	GetUserConfig(ctx context.Context, userID, key string) (configstore.ConfigEntry, error)
	SetUserConfig(ctx context.Context, userID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error)
	DeleteUserConfig(ctx context.Context, userID, key string, expectedVersion int64) error
	ListUserConfig(ctx context.Context, userID string) ([]configstore.ConfigEntry, error)
	PropagateCompanyConfig(ctx context.Context, companyID string, scope configstore.PropagateScope, key string, value json.RawMessage, locked bool) error
	CreateAlertRule(ctx context.Context, rule *admin.AlertRule) error
	GetAlertRule(ctx context.Context, ruleID string) (*admin.AlertRule, error)
	ListAlertRules(ctx context.Context, companyID string) ([]admin.AlertRule, error)
	UpdateAlertRule(ctx context.Context, rule *admin.AlertRule) error
	DeleteAlertRule(ctx context.Context, ruleID string) error
	CreateAlertChannel(ctx context.Context, channel *admin.AlertChannel) error
	GetAlertChannel(ctx context.Context, channelID string) (*admin.AlertChannel, error)
	ListAlertChannels(ctx context.Context, companyID string) ([]admin.AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, channel *admin.AlertChannel) error
	DeleteAlertChannel(ctx context.Context, channelID string) error
	ListAlertEvents(ctx context.Context, filter admin.AlertEventFilter) ([]admin.AlertEvent, error)
	LogAlertEvent(ctx context.Context, event *admin.AlertEvent) error
	GetUserMFAStatus(ctx context.Context, userID string) (maildb.UserMFAStatus, error)
	ResetUserMFA(ctx context.Context, userID string) error
	GetMFAStats(ctx context.Context, companyID string) (maildb.MFAStats, error)
	CreateInviteToken(ctx context.Context, userID, createdBy string) (maildb.InviteToken, error)
	GetInviteToken(ctx context.Context, token string) (maildb.InviteToken, error)
	AcceptInviteToken(ctx context.Context, token, passwordHash string) (maildb.UserView, error)
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
	ContractVersion string                            `json:"contract_version"`
	Modules         map[string]string                 `json:"modules"`
	Limits          adminConsoleLimits                `json:"limits"`
	Tenancy         adminConsoleTenancyCapabilities   `json:"tenancy"`
	Operations      adminConsoleOperationCapabilities `json:"operations"`
	Security        adminConsoleSecurityCapabilities  `json:"security"`
	Storage         storage.BackendCapabilities       `json:"storage"`
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
	Value    json.RawMessage `json:"value"`
	Locked   bool            `json:"locked"`
	Version  int64          `json:"version,omitempty"`
}

type adminConfigPropagateRequest struct {
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value"`
	Locked bool            `json:"locked"`
}

func parseAdminAttachmentCleanupRequest(w http.ResponseWriter, req adminAttachmentCleanupRunRequest) (time.Time, bool) {
	beforeRaw := strings.TrimSpace(req.Before)
	if beforeRaw == "" {
		writeError(w, http.StatusBadRequest, "before is required")
		return time.Time{}, false
	}
	before, err := time.Parse(time.RFC3339, beforeRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "before must be RFC3339 timestamp")
		return time.Time{}, false
	}
	before = before.UTC()
	if before.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "before must not be in the future")
		return time.Time{}, false
	}
	if req.Limit < 0 {
		writeError(w, http.StatusBadRequest, "limit must not be negative")
		return time.Time{}, false
	}
	return before, true
}

func RegisterAdminRoutes(mux *http.ServeMux, service AdminService, token string, opts ...AdminRouteOption) {
	var cfg adminRouteConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	// adminAuth closes over token and cfg.tokenMgr so call sites only pass the handler.
	adminAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return adminJWTOrStaticAuth(token, cfg.tokenMgr, next)
	}
	mux.HandleFunc("GET /admin/v1/console/capabilities", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		writeJSON(w, http.StatusOK, adminConsoleCapabilitiesEnvelope{AdminConsoleCapabilities: currentAdminConsoleCapabilities(storageCapabilitiesFromRouteConfig(cfg))})
	}))

	if cfg.routeCounters != nil {
		mux.HandleFunc("GET /admin/v1/delivery-routes/counters", adminAuth(func(w http.ResponseWriter, r *http.Request) {
			if !rejectUnknownQueryKeys(w, r) {
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"route_counters": cfg.routeCounters.Snapshot()})
		}))
	}

	mux.HandleFunc("GET /admin/v1/companies", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		companies, err := service.ListCompanies(r.Context(), maildb.CompanyListRequest{
			Limit:  limit,
			Status: status,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"companies": companies})
	}))

	mux.HandleFunc("POST /admin/v1/companies", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateCompanyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		company, err := service.CreateCompany(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"company": company})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		company, err := service.GetCompany(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"company": company})
	}))

	mux.HandleFunc("PATCH /admin/v1/companies/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateCompanyQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateCompanyQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateCompanyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		company, err := service.UpdateCompany(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"company": company})
	}))

	mux.HandleFunc("DELETE /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteCompany(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/users/bulk-import", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		_ = id // company id validated but users are domain-scoped; domain_id comes from payload
		if !ok {
			return
		}
		var req struct {
			Users []struct {
				Email       string `json:"email"`
				DisplayName string `json:"display_name"`
				DomainID    string `json:"domain_id"`
				Password    string `json:"password"`
			} `json:"users"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		type failure struct {
			Email string `json:"email"`
			Error string `json:"error"`
		}
		var failures []failure
		successCount := 0
		for _, u := range req.Users {
			parts := strings.SplitN(u.Email, "@", 2)
			username := parts[0]
			createReq := maildb.CreateUserRequest{
				DomainID:    u.DomainID,
				Username:    username,
				DisplayName: u.DisplayName,
				Address:     u.Email,
				Password:    u.Password,
			}
			if u.Password != "" {
				salt := make([]byte, 16)
				if _, err := rand.Read(salt); err != nil {
					failures = append(failures, failure{Email: u.Email, Error: "generate salt"})
					continue
				}
				hash, err := auth.HashPasswordPBKDF2SHA256(u.Password, salt, 0)
				if err != nil {
					failures = append(failures, failure{Email: u.Email, Error: "hash password: " + err.Error()})
					continue
				}
				createReq.PasswordHash = hash
				createReq.Password = ""
				createReq.MustChangePassword = true
			}
			if _, err := service.CreateUser(r.Context(), createReq); err != nil {
				failures = append(failures, failure{Email: u.Email, Error: err.Error()})
				continue
			}
			successCount++
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"total":    len(req.Users),
			"success":  successCount,
			"failed":   len(failures),
			"failures": failures,
		})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/users/bulk-export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		users, err := listCompanyUsers(r.Context(), service, id, 1000)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var buf bytes.Buffer
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{"email", "display_name", "domain_id", "status", "quota_used", "quota_limit", "created_at"})
		for _, u := range users {
			_ = cw.Write([]string{
				u.Username,
				u.DisplayName,
				u.DomainID,
				u.Status,
				strconv.FormatInt(u.QuotaUsed, 10),
				strconv.FormatInt(u.QuotaLimit, 10),
				u.CreatedAt.Format(time.RFC3339),
			})
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="users-export.csv"`))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		entries, err := service.ListCompanyConfig(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetCompanyConfig(r.Context(), id, key)
		if err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		var req adminConfigSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		entry, err := service.SetCompanyConfig(r.Context(), id, key, req.Value, req.Locked, req.Version)
		if err != nil {
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("DELETE /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		version := int64(-1)
		if v := r.URL.Query().Get("version"); v != "" {
			var err error
			version, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid version")
				return
			}
		}
		if err := service.DeleteCompanyConfig(r.Context(), id, key, version); err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrConfigLocked) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/config/propagate", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "scope") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		scopeStr := r.URL.Query().Get("scope")
		if scopeStr == "" {
			writeError(w, http.StatusBadRequest, "scope is required")
			return
		}
		scope := configstore.PropagateScope(scopeStr)
		if !scope.IsValid() {
			writeError(w, http.StatusBadRequest, "invalid scope")
			return
		}
		var req adminConfigPropagateRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := service.PropagateCompanyConfig(r.Context(), id, scope, req.Key, req.Value, req.Locked); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("GET /admin/v1/domains", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "status", "dns_status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		dnsStatus, ok := parseBoundedAdminQuery(w, r, "dns_status")
		if !ok {
			return
		}
		listReq := maildb.DomainListRequest{
			Limit:     limit,
			CompanyID: companyID,
			Status:    status,
			DNSStatus: dnsStatus,
		}
		if err := maildb.ValidateDomainListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		domains, err := service.ListDomains(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		stats, err := service.GetDomainStats(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-check", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		report, err := service.VerifyDomainDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_check": report})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-checks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status", "since") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		listReq := maildb.DomainDNSCheckListRequest{
			DomainID: id,
			Limit:    limit,
			Status:   status,
			Since:    since,
		}
		if err := maildb.ValidateDomainDNSCheckListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		checks, err := service.ListDomainDNSChecks(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_checks": checks})
	}))

	mux.HandleFunc("POST /admin/v1/domains", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateDomainRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		domain, err := service.CreateDomain(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("POST /admin/v1/domains/bulk", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleBulkDomains(w, r, service)
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDomainStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDomainQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteDomain(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		settings, err := service.GetDomainSettings(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var settings admin.DomainSettings
		if err := decodeJSONBody(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		settings.DomainID = id
		if err := service.UpdateDomainSettings(r.Context(), &settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/api-settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		settings, err := service.GetAPISettings(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/api-settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var settings admin.APISettings
		if err := decodeJSONBody(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		settings.DomainID = id
		if err := service.UpdateAPISettings(r.Context(), &settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/domains/{id}/api-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req struct {
			Name      string `json:"name"`
			CreatedBy string `json:"created_by"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		key := &admin.APIKey{
			DomainID:  id,
			Name:      req.Name,
			CreatedBy: req.CreatedBy,
		}
		secret, err := service.CreateAPIKey(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     key.ID,
			"secret": secret,
		})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/api-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		keys, err := service.ListAPIKeys(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}/api-keys/{keyid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		keyID, ok := parseBoundedAdminPathValue(w, r, "keyid")
		if !ok {
			return
		}
		if err := service.DeleteAPIKey(r.Context(), keyID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("POST /admin/v1/domains/{id}/api-keys/{keyid}/rotate", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		keyID, ok := parseBoundedAdminPathValue(w, r, "keyid")
		if !ok {
			return
		}
		newSecret, err := service.RotateAPIKey(r.Context(), keyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"secret": newSecret,
		})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		entries, err := service.ListDomainConfig(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetDomainConfig(r.Context(), id, key)
		if err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		var req adminConfigSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		entry, err := service.SetDomainConfig(r.Context(), id, key, req.Value, req.Locked, req.Version)
		if err != nil {
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		version := int64(-1)
		if v := r.URL.Query().Get("version"); v != "" {
			var err error
			version, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid version")
				return
			}
		}
		if err := service.DeleteDomainConfig(r.Context(), id, key, version); err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrConfigLocked) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainPolicyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		policy, err := service.UpdateDomainPolicy(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain_policy": policy})
	}))

	mux.HandleFunc("GET /admin/v1/users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "status", "password_configured") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		passwordConfigured, ok := parseOptionalBoolQuery(w, r, "password_configured")
		if !ok {
			return
		}
		listReq := maildb.UserListRequest{
			DomainID:           domainID,
			Status:             status,
			PasswordConfigured: passwordConfigured,
			Limit:              limit,
		}
		if err := maildb.ValidateUserListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		users, err := service.ListUsers(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("POST /admin/v1/users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateUserRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Password != "" && req.PasswordHash == "" {
			salt := make([]byte, 16)
			if _, err := rand.Read(salt); err != nil {
				writeError(w, http.StatusInternalServerError, "generate salt")
				return
			}
			hash, err := auth.HashPasswordPBKDF2SHA256(req.Password, salt, 0)
			if err != nil {
				writeError(w, http.StatusBadRequest, "hash password: "+err.Error())
				return
			}
			req.PasswordHash = hash
			req.Password = ""
			req.MustChangePassword = true
		}
		user, err := service.CreateUser(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	}))

	mux.HandleFunc("POST /admin/v1/users/{id}/invite", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		it, err := service.CreateInviteToken(r.Context(), id, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"invite_token": it})
	}))

	mux.HandleFunc("POST /admin/invite/{token}/accept", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		rawToken := r.PathValue("token")
		if len(rawToken) < 8 || len(rawToken) > 128 {
			writeError(w, http.StatusBadRequest, "invalid token")
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(body.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			writeError(w, http.StatusInternalServerError, "generate salt")
			return
		}
		hash, err := auth.HashPasswordPBKDF2SHA256(body.Password, salt, 0)
		if err != nil {
			writeError(w, http.StatusBadRequest, "hash password: "+err.Error())
			return
		}
		user, err := service.AcceptInviteToken(r.Context(), rawToken, hash)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	})

	mux.HandleFunc("PATCH /admin/v1/users/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("POST /admin/v1/users/bulk", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input struct {
			IDs    []string `json:"ids"`
			Action string   `json:"action"` // "activate", "suspend"
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(input.IDs) == 0 {
			writeError(w, http.StatusBadRequest, "ids is required")
			return
		}
		var targetStatus string
		switch input.Action {
		case "activate":
			targetStatus = "active"
		case "suspend":
			targetStatus = "suspended"
		default:
			writeError(w, http.StatusBadRequest, "unsupported action: "+input.Action)
			return
		}
		ctx := r.Context()
		succeeded := []string{}
		failed := []map[string]string{}
		for _, id := range input.IDs {
			err := service.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: targetStatus})
			if err != nil {
				failed = append(failed, map[string]string{"id": id, "error": err.Error()})
			} else {
				succeeded = append(succeeded, id)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"succeeded": succeeded,
			"failed":    failed,
		})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/password-hash", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserPasswordHashRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserPasswordHash(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserRole(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID, "role": req.Role})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		entries, err := service.ListUserConfig(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetUserConfig(r.Context(), id, key)
		if err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("PUT /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		writeError(w, http.StatusForbidden, "admin cannot modify user scope config directly")
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		writeError(w, http.StatusForbidden, "admin cannot modify user scope config directly")
	}))

	// MFA management routes
	mux.HandleFunc("GET /admin/v1/users/{id}/mfa", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		status, err := service.GetUserMFAStatus(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_status": status})
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/mfa", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.ResetUserMFA(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/mfa/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		stats, err := service.GetMFAStats(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_stats": stats})
	}))

	registerAdminDeviceTokenRoutes(mux, service, token)

	mux.HandleFunc("GET /admin/v1/config/stream", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flush := func() {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		flush()
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected"}`)
		flush()
		if cfg.configNotifier == nil {
			<-r.Context().Done()
			return
		}
		ch := cfg.configNotifier.Subscribe()
		defer cfg.configNotifier.Unsubscribe(ch)
		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				payload, err := json.Marshal(map[string]string{
					"type":       "config.changed",
					"scope_type": string(event.ScopeType),
					"scope_id":   event.ScopeID,
					"key":        event.Key,
					"action":     event.Action,
				})
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flush()
			}
		}
	}))

	mux.HandleFunc("GET /admin/v1/queue", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		stats, err := service.ListQueueStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"queues": stats})
	}))

	mux.HandleFunc("POST /admin/v1/imap/mailboxes/{id}/uid-backfill", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "limit") {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		mailboxID, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if userID == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		assigned, err := service.BackfillIMAPMailboxUIDs(r.Context(), userID, mailboxID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		items := make([]adminIMAPUIDBackfillItem, 0, len(assigned))
		for _, item := range assigned {
			items = append(items, adminIMAPUIDBackfillItem{
				MessageID: string(item.MessageID),
				MailboxID: string(item.MailboxID),
				UID:       uint32(item.UID),
				ModSeq:    item.ModSeq,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"imap_uid_backfill": items})
	}))

	mux.HandleFunc("GET /admin/v1/outbox-events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "topic", "partition_key", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		topic, ok := parseBoundedAdminQuery(w, r, "topic")
		if !ok {
			return
		}
		partitionKey, ok := parseBoundedAdminQuery(w, r, "partition_key")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		events, err := service.ListOutboxEvents(r.Context(), maildb.OutboxEventListRequest{
			Limit:        limit,
			Topic:        topic,
			PartitionKey: partitionKey,
			Status:       status,
			Since:        since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbox_events": events})
	}))

	mux.HandleFunc("GET /admin/v1/outbox-events/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		event, err := service.GetOutboxEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbox_event": event})
	}))

	mux.HandleFunc("GET /admin/v1/audit-logs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "category", "action", "action_prefix", "result", "target_type", "company_id", "domain_id", "user_id", "actor_id", "target_id", "since") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAuditLogListRequest(w, r, limit)
		if !ok {
			return
		}
		logs, err := service.ListAuditLogs(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_logs": logs})
	}))

	mux.HandleFunc("GET /admin/v1/audit-logs/integrity", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		view, err := service.CheckAuditLogIntegrity(r.Context(), maildb.AuditLogIntegrityRequest{
			Limit: limit,
			Since: since,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_log_integrity": view})
	}))

	mux.HandleFunc("GET /admin/v1/audit-logs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		log, err := service.GetAuditLog(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_log": log})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "direction", "company_id", "domain_id", "user_id", "message_id", "rfc_message_id", "from_addr", "to_addr", "subject", "flow_status", "since", "until") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseMailFlowLogListRequest(w, r, limit)
		if !ok {
			return
		}
		logs, err := service.ListMailFlowLogs(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_logs": logs})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "direction", "company_id", "domain_id", "user_id", "since", "until") {
			return
		}
		req, ok := parseMailFlowLogStatsRequest(w, r)
		if !ok {
			return
		}
		stats, err := service.GetMailFlowLogStats(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/daily-stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "direction", "company_id", "domain_id", "user_id", "since", "until") {
			return
		}
		req, ok := parseMailFlowLogDailyStatsRequest(w, r)
		if !ok {
			return
		}
		stats, err := service.GetMailFlowLogDailyStats(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_daily_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		log, err := service.GetMailFlowLog(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_log": log})
	}))

	mux.HandleFunc("GET /admin/v1/directory/principals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "organization_id", "kinds", "q", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryPrincipalSearchRequest(w, r, limit)
		if !ok {
			return
		}
		principals, err := service.SearchDirectoryPrincipals(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_principals": principals})
	}))

	mux.HandleFunc("GET /admin/v1/directory/aliases/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "address", "active_only") {
			return
		}
		req, ok := parseDirectoryAliasResolveRequest(w, r)
		if !ok {
			return
		}
		alias, err := service.ResolveDirectoryAlias(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("GET /admin/v1/directory/aliases", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "target_kind", "target_id", "q", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryAliasListRequest(w, r, limit)
		if !ok {
			return
		}
		aliases, err := service.ListDirectoryAliases(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_aliases": aliases})
	}))

	mux.HandleFunc("POST /admin/v1/directory/aliases", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateAliasRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		alias, err := service.CreateDirectoryAlias(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/aliases/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		alias, err := service.DeleteDirectoryAlias(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("GET /admin/v1/directory/delegations", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "owner_kind", "owner_id", "delegate_kind", "delegate_id", "scope", "role", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryDelegationListRequest(w, r, limit)
		if !ok {
			return
		}
		delegations, err := service.ListDirectoryDelegations(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegations": delegations})
	}))

	mux.HandleFunc("POST /admin/v1/directory/delegations", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateDelegationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		delegation, err := service.CreateDirectoryDelegation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("GET /admin/v1/directory/group-memberships", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "group_id", "member_kind", "member_id", "role", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryGroupMembershipListRequest(w, r, limit)
		if !ok {
			return
		}
		memberships, err := service.ListDirectoryGroupMemberships(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_memberships": memberships})
	}))

	mux.HandleFunc("POST /admin/v1/directory/group-memberships", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateGroupMembershipRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		membership, err := service.CreateDirectoryGroupMembership(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/group-memberships/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		membership, err := service.DeleteDirectoryGroupMembership(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/group-memberships/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req directory.UpdateGroupMembershipRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		membership, err := service.UpdateDirectoryGroupMembershipRole(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/group-memberships/{id}/assignment", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req directory.ReassignGroupMembershipRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		membership, err := service.ReassignDirectoryGroupMembership(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/delegations/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req directory.UpdateDelegationRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		delegation, err := service.UpdateDirectoryDelegationRole(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/delegations/{id}/assignment", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req directory.ReassignDelegationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		delegation, err := service.ReassignDirectoryDelegation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/delegations/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		delegation, err := service.DeleteDirectoryDelegation(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("GET /admin/v1/backpressure", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		state, err := backpressureService.GetBackpressure(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))

	mux.HandleFunc("PATCH /admin/v1/backpressure", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		var req backpressure.StateUpdate
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		state, err := backpressureService.UpdateBackpressure(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))

	mux.HandleFunc("GET /admin/v1/quota-usage", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "scope", "domain_id", "over_limit", "over_allocated") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		scope, ok := parseBoundedAdminQuery(w, r, "scope")
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		overLimit, ok := parseOptionalBoolQuery(w, r, "over_limit")
		if !ok {
			return
		}
		overAllocated, ok := parseOptionalBoolQuery(w, r, "over_allocated")
		if !ok {
			return
		}
		usages, err := service.ListQuotaUsage(r.Context(), maildb.QuotaUsageListRequest{
			Limit:         limit,
			Scope:         scope,
			DomainID:      domainID,
			OverLimit:     overLimit,
			OverAllocated: overAllocated,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_usage": usages})
	}))

	mux.HandleFunc("POST /admin/v1/attachment-cleanup/candidates", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req adminAttachmentCleanupRunRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		before, ok := parseAdminAttachmentCleanupRequest(w, req)
		if !ok {
			return
		}
		counts, err := service.CountStaleAttachmentUploads(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessionCounts, err := service.CountStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		candidates, err := service.ListStaleAttachmentUploads(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessionCandidates, err := service.ListStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"attachment_cleanup_candidates": map[string]any{
				"candidates":              candidates,
				"candidate_count":         counts.TotalCount,
				"limited_count":           counts.LimitedCount,
				"session_candidates":      sessionCandidates,
				"session_candidate_count": sessionCounts.TotalCount,
				"session_limited_count":   sessionCounts.LimitedCount,
				"before":                  before.Format(time.RFC3339),
				"limit":                   maildb.NormalizeAttachmentCleanupLimit(req.Limit),
			},
		})
	}))

	mux.HandleFunc("GET /admin/v1/attachment-upload-sessions", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "user_id", "draft_id", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		draftID, ok := parseBoundedAdminQuery(w, r, "draft_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		req := maildb.AttachmentUploadSessionListRequest{
			Limit:   limit,
			UserID:  userID,
			DraftID: draftID,
			Status:  status,
		}
		if err := maildb.ValidateAttachmentUploadSessionListRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sessions, err := service.ListAttachmentUploadSessions(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment_upload_sessions": sessions})
	}))

	mux.HandleFunc("GET /admin/v1/drive-upload-sessions", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "user_id", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		req := drive.ListUploadSessionsRequest{
			UserID: userID,
			Status: status,
			Limit:  limit,
		}
		req, err := drive.ValidateListUploadSessionsRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sessions, err := service.ListDriveUploadSessions(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_upload_sessions": sessions})
	}))

	mux.HandleFunc("GET /admin/v1/drive-nodes", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "limit", "user_id", "parent_id", "status", "node_type", "q", "sort", "all_parents") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		parentID, ok := parseBoundedAdminQuery(w, r, "parent_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		nodeType, ok := parseBoundedAdminQuery(w, r, "node_type")
		if !ok {
			return
		}
		searchQuery, ok := parseBoundedAdminQuery(w, r, "q")
		if !ok {
			return
		}
		sortMode, ok := parseBoundedAdminQuery(w, r, "sort")
		if !ok {
			return
		}
		allParentsValue, ok := parseBoolQueryDefaultFalse(w, r, "all_parents")
		if !ok {
			return
		}
		req := drive.ListNodesRequest{
			UserID:     userID,
			ParentID:   parentID,
			Status:     status,
			NodeType:   nodeType,
			Query:      searchQuery,
			Sort:       sortMode,
			AllParents: allParentsValue,
			Limit:      limit,
		}
		req, err := drive.ValidateListNodesRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		nodes, err := service.ListDriveNodes(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_nodes": nodes})
	}))

	mux.HandleFunc("GET /admin/v1/drive-nodes/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "status") {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		nodeID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		req := drive.GetNodeRequest{UserID: userID, NodeID: nodeID, Status: status}
		req, err := drive.ValidateGetNodeRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		node, err := service.GetDriveNode(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node})
	}))

	mux.HandleFunc("GET /admin/v1/drive-usage", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		req, err := drive.ValidateGetUsageSummaryRequest(drive.GetUsageSummaryRequest{UserID: userID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		summary, err := service.GetDriveUsageSummary(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_usage_summary": summary})
	}))

	mux.HandleFunc("POST /admin/v1/drive-upload-cleanup/candidates", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req adminAttachmentCleanupRunRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		before, ok := parseAdminAttachmentCleanupRequest(w, req)
		if !ok {
			return
		}
		counts, err := service.CountStaleDriveUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessionCandidates, err := service.ListStaleDriveUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"drive_upload_cleanup_candidates": map[string]any{
				"session_candidates":      sessionCandidates,
				"session_candidate_count": counts.TotalCount,
				"session_limited_count":   counts.LimitedCount,
				"before":                  before.Format(time.RFC3339),
				"limit":                   drive.NormalizeUploadSessionCleanupLimit(req.Limit),
			},
		})
	}))

	mux.HandleFunc("POST /admin/v1/drive-upload-cleanup/runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req adminAttachmentCleanupRunRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		before, ok := parseAdminAttachmentCleanupRequest(w, req)
		if !ok {
			return
		}
		counts, err := service.CountStaleDriveUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		expired, err := service.RunDriveUploadSessionCleanup(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"drive_upload_cleanup_run": map[string]any{
				"expired_sessions":        expired,
				"session_candidate_count": counts.TotalCount,
				"session_limited_count":   counts.LimitedCount,
				"expired_session_count":   len(expired),
				"before":                  before.Format(time.RFC3339),
				"limit":                   drive.NormalizeUploadSessionCleanupLimit(req.Limit),
			},
		})
	}))

	mux.HandleFunc("GET /admin/v1/drive-cleanup-failures", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "user_id", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		req := drive.ListObjectCleanupFailuresRequest{
			UserID: userID,
			Status: status,
			Limit:  limit,
		}
		req, err := drive.ValidateListObjectCleanupFailuresRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		failures, err := service.ListDriveObjectCleanupFailures(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_cleanup_failures": failures})
	}))

	mux.HandleFunc("POST /admin/v1/drive-cleanup-failures/{id}/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		resolved, err := service.ResolveDriveObjectCleanupFailure(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_cleanup_failure": resolved})
	}))

	mux.HandleFunc("POST /admin/v1/drive-cleanup-failures/retry-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var body adminDriveCleanupFailureRetryRunRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req := drive.ListObjectCleanupFailuresRequest{
			UserID: body.UserID,
			Status: drive.ObjectCleanupFailureStatusPending,
			Limit:  body.Limit,
		}
		req, err := drive.ValidateListObjectCleanupFailuresRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := service.RetryDriveObjectCleanupFailures(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"drive_cleanup_retry_run": map[string]any{
				"user_id":  req.UserID,
				"limit":    req.Limit,
				"scanned":  result.Scanned,
				"deleted":  result.Deleted,
				"resolved": result.Resolved,
				"failed":   result.Failed,
			},
		})
	}))

	mux.HandleFunc("POST /admin/v1/attachment-cleanup/runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req adminAttachmentCleanupRunRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		before, ok := parseAdminAttachmentCleanupRequest(w, req)
		if !ok {
			return
		}
		counts, err := service.CountStaleAttachmentUploads(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessionCounts, err := service.CountStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		expiredCount := 0
		expiredSessionCount := 0
		if !req.DryRun {
			expired, err := service.RunAttachmentCleanup(r.Context(), before, req.Limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			expiredCount = len(expired)
			expiredSessions, err := service.RunAttachmentUploadSessionCleanup(r.Context(), before, req.Limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			expiredSessionCount = len(expiredSessions)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"attachment_cleanup_run": map[string]any{
				"dry_run":                 req.DryRun,
				"candidate_count":         counts.TotalCount,
				"limited_count":           counts.LimitedCount,
				"expired_count":           expiredCount,
				"session_candidate_count": sessionCounts.TotalCount,
				"session_limited_count":   sessionCounts.LimitedCount,
				"expired_session_count":   expiredSessionCount,
				"before":                  before.Format(time.RFC3339),
				"limit":                   maildb.NormalizeAttachmentCleanupLimit(req.Limit),
			},
		})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/daily", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageAggregateQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageAggregateListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageDaily(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_daily": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/monthly", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageAggregateQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageAggregateListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageMonthly(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_monthly": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerStatsQuery(w, r) {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, 0)
		if !ok {
			return
		}
		stats, err := service.GetAPIUsageLedgerStats(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageRetentionReadinessQuery(w, r) {
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRequest(w, r)
		if !ok {
			return
		}
		readiness, err := service.GetAPIUsageLedgerRetentionReadiness(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_readiness": readiness})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/ledger/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()

		var body adminAPIUsageLedgerRetentionRunRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRunRequest(w, body)
		if !ok {
			return
		}
		run, err := service.RunAPIUsageLedgerRetention(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageRetentionRunListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRunListRequest(w, r, limit)
		if !ok {
			return
		}
		runs, err := service.ListAPIUsageLedgerRetentionRuns(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_runs": runs})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-runs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		run, err := service.GetAPIUsageLedgerRetentionRun(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownDAVSyncRetentionReadinessQuery(w, r) {
			return
		}
		req, ok := parseDAVSyncRetentionReadinessRequest(w, r)
		if !ok {
			return
		}
		readiness, err := service.GetDAVSyncRetentionReadiness(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_readiness": readiness})
	}))

	mux.HandleFunc("POST /admin/v1/dav-sync/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()

		var body adminDAVSyncRetentionRunRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req, ok := parseDAVSyncRetentionRunRequest(w, body)
		if !ok {
			return
		}
		run, err := service.RunDAVSyncRetention(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownDAVSyncRetentionRunListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDAVSyncRetentionRunListRequest(w, r, limit)
		if !ok {
			return
		}
		runs, err := service.ListDAVSyncRetentionRuns(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_runs": runs})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-runs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		run, err := service.GetDAVSyncRetentionRun(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-capabilities", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		capabilities, err := service.GetAPIUsageExportCapabilities(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_capabilities": capabilities})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownAPIUsageExportBatchCreateQuery(w, r) {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, 0)
		if !ok {
			return
		}
		if req.From.IsZero() || req.To.IsZero() {
			writeError(w, http.StatusBadRequest, "from and to are required")
			return
		}
		batch, err := service.CreateAPIUsageExportBatch(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_batch": batch})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageExportBatchListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageExportBatchListRequest(w, r, limit)
		if !ok {
			return
		}
		batches, err := service.ListAPIUsageExportBatches(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_batches": batches})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		batch, err := service.GetAPIUsageExportBatch(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_batch": batch})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/handoff-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "deep") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		deep, ok := parseBoolQueryDefaultFalse(w, r, "deep")
		if !ok {
			return
		}
		handoff, err := service.GetAPIUsageExportHandoff(r.Context(), id, deep)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_handoff_readiness": handoff})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		batch, err := service.GetAPIUsageExportBatch(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		req := apiUsageLedgerRequestFromBatch(batch, limit)
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.CreateAPIUsageExportArtifactRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.BatchID = id
		artifact, err := service.CreateAPIUsageExportArtifact(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		artifacts, err := service.ListAPIUsageExportArtifacts(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifacts": artifacts})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts/write", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.WriteAPIUsageExportArtifactRequest
		if r.ContentLength != 0 {
			if err := decodeJSONBody(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}
		artifact, err := service.WriteAPIUsageExportArtifact(r.Context(), id, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		artifact, err := service.GetAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/download", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		artifact, body, err := service.OpenAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", safeContentType(artifact.ContentType, "application/x-ndjson"))
		if sha256Hex := safeSHA256Header(artifact.SHA256Hex); sha256Hex != "" {
			w.Header().Set("X-Gogomail-Artifact-SHA256", sha256Hex)
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, body)
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifact_verification": verification})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		digest, err := service.CreateAPIUsageExportManifestDigest(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_manifest_digest": digest})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		digests, err := service.ListAPIUsageExportManifestDigests(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digests": digests})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		digest, err := service.GetAPIUsageExportManifestDigest(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digest": digest})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportManifestDigest(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digest_verification": verification})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		signature, err := service.CreateAPIUsageExportManifestSignature(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_manifest_signature": signature})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		signatures, err := service.ListAPIUsageExportManifestSignatures(r.Context(), id, digestID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signatures": signatures})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, digestID, signatureID, ok := parseBoundedAdminPathTriple(w, r, "id", "digest_id", "signature_id")
		if !ok {
			return
		}
		signature, err := service.GetAPIUsageExportManifestSignature(r.Context(), id, digestID, signatureID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signature": signature})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, digestID, signatureID, ok := parseBoundedAdminPathTriple(w, r, "id", "digest_id", "signature_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportManifestSignature(r.Context(), id, digestID, signatureID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signature_verification": verification})
	}))

	mux.HandleFunc("GET /admin/v1/quota-reconciliation", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		reconciliation, err := service.ListQuotaReconciliation(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_reconciliation": reconciliation})
	}))

	mux.HandleFunc("POST /admin/v1/quota-reconciliation/corrections", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CorrectQuotaReconciliationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		result, err := service.CorrectQuotaReconciliation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_correction": result})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alert-thresholds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "scope") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		scope, ok := parseBoundedAdminQuery(w, r, "scope")
		if !ok {
			return
		}
		thresholds, err := service.ListQuotaAlertThresholds(r.Context(), maildb.QuotaAlertThresholdListRequest{
			Limit:     limit,
			CompanyID: companyID,
			Scope:     scope,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_thresholds": thresholds})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		threshold, err := service.GetQuotaAlertThreshold(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("POST /admin/v1/quota-alert-thresholds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateQuotaAlertThresholdRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		threshold, err := service.CreateQuotaAlertThreshold(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("PATCH /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.UpdateQuotaAlertThresholdRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		threshold, err := service.UpdateQuotaAlertThreshold(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("DELETE /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteQuotaAlertThreshold(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alerts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "user_id", "scope", "alert_type", "since", "until") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		scope, ok := parseBoundedAdminQuery(w, r, "scope")
		if !ok {
			return
		}
		alertType, ok := parseBoundedAdminQuery(w, r, "alert_type")
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		until, ok := parseOptionalRFC3339Query(w, r, "until")
		if !ok {
			return
		}
		alerts, err := service.ListQuotaAlerts(r.Context(), maildb.QuotaAlertListRequest{
			Limit:     limit,
			CompanyID: companyID,
			DomainID:  domainID,
			UserID:    userID,
			Scope:     scope,
			AlertType: alertType,
			Since:     since,
			Until:     until,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alerts": alerts})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alerts/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		alert, err := service.GetQuotaAlert(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert": alert})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "status", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		attempts, err := service.ListDeliveryAttempts(r.Context(), maildb.DeliveryAttemptListRequest{
			Limit:           limit,
			Status:          status,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "since", "status", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		stats, err := service.GetDeliveryAttemptStats(r.Context(), maildb.DeliveryAttemptStatsRequest{
			Status:          status,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempt_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/exhausted", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		attempts, err := service.ListExhaustedAttempts(r.Context(), maildb.ExhaustedAttemptListRequest{
			Limit:           limit,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"exhausted_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-attempts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "status", "user_id", "message_id", "platform", "device_id", "provider_status", "provider_message_id") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		platform, ok := parseBoundedAdminQuery(w, r, "platform")
		if !ok {
			return
		}
		deviceID, ok := parseBoundedAdminQuery(w, r, "device_id")
		if !ok {
			return
		}
		providerStatus, ok := parseBoundedAdminQuery(w, r, "provider_status")
		if !ok {
			return
		}
		providerMessageID, ok := parseBoundedAdminQuery(w, r, "provider_message_id")
		if !ok {
			return
		}
		attempts, err := service.ListPushNotificationAttempts(r.Context(), maildb.PushNotificationAttemptListRequest{
			Limit:             limit,
			MessageID:         messageID,
			Status:            status,
			UserID:            userID,
			Platform:          platform,
			DeviceID:          deviceID,
			ProviderStatus:    providerStatus,
			ProviderMessageID: providerMessageID,
			Since:             since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-attempts/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		attempt, err := service.GetPushNotificationAttempt(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_attempt": attempt})
	}))

	mux.HandleFunc("PATCH /admin/v1/push-notification-attempts/{id}/outcome", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.UpdatePushNotificationOutcomeRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.AttemptID = id
		if err := service.UpdatePushNotificationOutcome(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "since", "user_id", "message_id", "platform", "device_id") {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		platform, ok := parseBoundedAdminQuery(w, r, "platform")
		if !ok {
			return
		}
		deviceID, ok := parseBoundedAdminQuery(w, r, "device_id")
		if !ok {
			return
		}
		stats, err := service.GetPushNotificationStats(r.Context(), maildb.PushNotificationStatsRequest{
			MessageID: messageID,
			UserID:    userID,
			Platform:  platform,
			DeviceID:  deviceID,
			Since:     since,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/suppression-list", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "email", "reason") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		email, ok := parseBoundedAdminQuery(w, r, "email")
		if !ok {
			return
		}
		reason, ok := parseBoundedAdminQuery(w, r, "reason")
		if !ok {
			return
		}
		listReq := maildb.SuppressionEntryListRequest{
			Limit:    limit,
			DomainID: domainID,
			Email:    email,
			Reason:   reason,
		}
		if err := maildb.ValidateSuppressionEntryListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		entries, err := service.ListSuppressionEntries(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suppression_list": entries})
	}))

	mux.HandleFunc("GET /admin/v1/trusted-relays", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "cidr", "description") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		cidr, ok := parseBoundedAdminQuery(w, r, "cidr")
		if !ok {
			return
		}
		description, ok := parseBoundedAdminQuery(w, r, "description")
		if !ok {
			return
		}
		listReq := maildb.TrustedRelayListRequest{
			Limit:       limit,
			CIDR:        cidr,
			Description: description,
		}
		if err := maildb.ValidateTrustedRelayListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		relays, err := service.ListTrustedRelays(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"trusted_relays": relays})
	}))

	mux.HandleFunc("POST /admin/v1/trusted-relays", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateTrustedRelayRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		relay, err := service.CreateTrustedRelay(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"trusted_relay": relay})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-routes", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status", "farm", "domain_pattern") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		domainPattern, ok := parseBoundedAdminQuery(w, r, "domain_pattern")
		if !ok {
			return
		}
		listReq := maildb.DeliveryRouteListRequest{
			Limit:         limit,
			Status:        status,
			Farm:          farm,
			DomainPattern: domainPattern,
		}
		if err := maildb.ValidateDeliveryRouteListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		routes, err := service.ListDeliveryRoutes(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_routes": routes})
	}))

	mux.HandleFunc("POST /admin/v1/delivery-routes", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateDeliveryRouteRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		route, err := service.CreateDeliveryRoute(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"delivery_route": route})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-routes/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "domain") {
			return
		}
		domain, ok := parseBoundedAdminQuery(w, r, "domain")
		if !ok {
			return
		}
		result, err := service.ResolveDeliveryRoute(r.Context(), domain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_route_resolution": result})
	}))

	mux.HandleFunc("PATCH /admin/v1/delivery-routes/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDeliveryRouteStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDeliveryRouteStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/dkim-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		listReq := maildb.DKIMKeyListRequest{
			DomainID: domainID,
			Status:   status,
			Limit:    limit,
		}
		if err := maildb.ValidateDKIMKeyListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		keys, err := service.ListDKIMKeys(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_keys": keys})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var input maildb.CreateDKIMKeyInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, err := service.CreateDKIMKey(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/dkim-keys/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeactivateDKIMKey(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys/{id}/verify-dns", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		result, err := service.VerifyDKIMKeyDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_verification": result})
	}))

	mux.HandleFunc("POST /admin/v1/outbox/{id}/retry", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.RetryOutbox(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/suppression-list/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteSuppressionEntry(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/trusted-relays/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteTrustedRelay(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/delivery-routes/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteDeliveryRoute(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/alert-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateAlertRule(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/alert-rules/{ruleid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetAlertRule(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/alert-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAlertRules(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/alert-rules/{ruleid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleUpdateAlertRule(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/alert-rules/{ruleid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAlertRule(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/alert-channels", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateAlertChannel(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/alert-channels", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAlertChannels(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/alert-events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAlertEvents(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogin(w, r, service, cfg.tokenMgr)
	})

	mux.HandleFunc("POST /admin/v1/auth/setup", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAdminSetup(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogout(w, r)
	})

	mux.HandleFunc("GET /admin/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		handleAdminVerify(w, r)
	})

	mux.HandleFunc("GET /admin/v1/admin-users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAdminUsers(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/admin-users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateAdminUser(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/admin-users/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAdminUser(w, r)
	}))

	mux.HandleFunc("GET /admin/v1/health", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAdminHealth(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/organization/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetOrganizationSettings(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/organization/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleUpdateOrganizationSettings(w, r)
	}))

	mux.HandleFunc("GET /admin/v1/compliance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListCompliance(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/roles", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListRoles(w, r)
	}))

	mux.HandleFunc("POST /admin/v1/roles", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateRole(w, r)
	}))

	mux.HandleFunc("GET /admin/v1/reports", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListReports(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyIPPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyIPPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainIPPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainIPPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyAuthPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyAuthPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRetentionPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRetentionPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRetentionPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRetentionPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/session-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySessionPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/session-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySessionPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/sessions", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySessions(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/companies/{id}/sessions/{userId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteCompanySession(w, r)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRateLimitPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRateLimitPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRateLimitPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRateLimitPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/dmarc-spf", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainDmarcSpfPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/dmarc-spf", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainDmarcSpfPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySpamFilterPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySpamFilterPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSpamFilterPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSpamFilterPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/quota-summary", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyQuotaSummary(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRoutingRules(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRoutingRules(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRoutingRules(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRoutingRules(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/sso/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySSOConfig(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/sso/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySSOConfig(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/sso/test", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePostCompanySSOTest(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/smtp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSMTPPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/smtp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSMTPPolicy(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/onboarding/validate-domain", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			writeJSON(w, http.StatusOK, map[string]any{"valid": false, "message": "domain name is required"})
			return
		}
		// Simple format check: must contain a dot, no spaces, reasonable length.
		if len(name) > 253 || strings.Contains(name, " ") || !strings.Contains(name, ".") {
			writeJSON(w, http.StatusOK, map[string]any{"valid": false, "message": "invalid domain format"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"valid": true, "message": "domain format is valid"})
	}))

	// ─── Audit Log Export ─────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/audit-logs/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleExportCompanyAuditLogs(w, r, service)
	}))

	// ─── Tenant Health ────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/health", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyHealth(w, r, service)
	}))

	// ─── Change History / Approval Queue ─────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/change-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyChangeHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetPendingApprovals(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreatePendingApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/approve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleApproveApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/reject", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRejectApproval(w, r, service)
	}))

	// ─── Webhooks ─────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyWebhooks(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePostCompanyWebhook(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/companies/{id}/webhooks/{webhookId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteCompanyWebhook(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks/{webhookId}/test", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleTestCompanyWebhook(w, r, service)
	}))

	// ─── Notification Templates ───────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/notification-templates", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetNotifTemplates(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/notification-templates/{templateId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutNotifTemplate(w, r, service)
	}))

	// ─── Security Posture ──────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/posture", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		handleGetSecurityPosture(w, r, service, id)
	}))

	// ─── Global Signature ──────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/signature", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSignature(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/signature", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutSignature(w, r, service)
	}))

	// ─── Legal Holds ───────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/legal-holds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetLegalHolds(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/legal-holds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateLegalHold(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/companies/{id}/legal-holds/{holdId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleDeleteLegalHold(w, r, service)
	}))

	// ─── SCIM Status ───────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/scim/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSCIMStatus(w, r, service)
	}))

	// ─── Seat Usage ────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/seat-usage", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSeatUsage(w, r, service)
	}))
}

func handleAdminHealth(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	start := time.Now()
	_, dbErr := service.ListQueueStats(r.Context())
	dbElapsed := time.Since(start).Milliseconds()

	dbStatus := "healthy"
	if dbErr != nil {
		dbStatus = "unhealthy"
	}

	auditStart := time.Now()
	_, auditErr := service.ListAuditLogs(r.Context(), maildb.AuditLogListRequest{Limit: 1})
	auditElapsed := time.Since(auditStart).Milliseconds()
	auditStatus := "healthy"
	if auditErr != nil {
		auditStatus = "degraded"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, map[string]any{
		"checks": []map[string]any{
			{
				"service":          "database",
				"status":           dbStatus,
				"response_time_ms": dbElapsed,
				"last_check":       now,
			},
			{
				"service":          "audit_log",
				"status":           auditStatus,
				"response_time_ms": auditElapsed,
				"last_check":       now,
			},
		},
	})
}

func handleGetOrganizationSettings(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	companies, err := service.ListCompanies(r.Context(), maildb.CompanyListRequest{Limit: 200})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch organization settings")
		return
	}
	totalDomains, _ := service.ListDomains(r.Context(), maildb.DomainListRequest{Limit: 1000})
	totalUsers, _ := service.ListUsers(r.Context(), maildb.UserListRequest{Limit: 1})

	name := "gogomail"
	description := ""
	var createdAt time.Time
	if len(companies) > 0 {
		name = companies[0].Name
		createdAt = companies[0].CreatedAt
	}
	_ = totalUsers

	writeJSON(w, http.StatusOK, map[string]any{
		"settings": map[string]any{
			"name":        name,
			"description": description,
			"max_users":   len(companies) * 100,
			"max_domains": len(totalDomains),
			"created_at":  createdAt.UTC().Format(time.RFC3339),
		},
	})
}

func handleUpdateOrganizationSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		MaxUsers    int    `json:"max_users"`
		MaxDomains  int    `json:"max_domains"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": map[string]any{
			"name":        req.Name,
			"description": req.Description,
			"max_users":   req.MaxUsers,
			"max_domains": req.MaxDomains,
			"created_at":  time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func handleListCompliance(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	logs, err := service.ListAuditLogs(r.Context(), maildb.AuditLogListRequest{Limit: 100})
	auditCount := 0
	if err == nil {
		auditCount = len(logs)
	}

	status := "compliant"
	if auditCount == 0 {
		status = "pending"
	}

	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": []map[string]any{
			{
				"id":         "gdpr-001",
				"framework":  "GDPR",
				"status":     status,
				"last_audit": now.Format(time.RFC3339),
				"findings":   0,
			},
			{
				"id":         "hipaa-001",
				"framework":  "HIPAA",
				"status":     "pending",
				"last_audit": now.AddDate(0, -1, 0).Format(time.RFC3339),
				"findings":   2,
			},
			{
				"id":         "soc2-001",
				"framework":  "SOC 2",
				"status":     "partial",
				"last_audit": now.AddDate(0, -2, 0).Format(time.RFC3339),
				"findings":   1,
			},
		},
	})
}

func handleListRoles(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, map[string]any{
		"roles": []map[string]any{
			{
				"id":               "role-admin",
				"name":             "Administrator",
				"description":      "Full system access",
				"permissions_count": 42,
				"assigned_users":   1,
				"created_at":       now,
			},
			{
				"id":               "role-operator",
				"name":             "Operator",
				"description":      "Read and manage mail flow",
				"permissions_count": 18,
				"assigned_users":   0,
				"created_at":       now,
			},
			{
				"id":               "role-viewer",
				"name":             "Viewer",
				"description":      "Read-only access",
				"permissions_count": 8,
				"assigned_users":   0,
				"created_at":       now,
			},
		},
	})
}

func handleCreateRole(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusCreated, map[string]any{
		"role": map[string]any{
			"id":               "role-" + req.Name,
			"name":             req.Name,
			"description":      req.Description,
			"permissions_count": 0,
			"assigned_users":   0,
			"created_at":       now,
		},
	})
}

func handleListReports(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	now := time.Now().UTC()

	stats, err := service.GetMailFlowLogStats(r.Context(), maildb.MailFlowLogStatsRequest{})
	mailCount := int64(0)
	if err == nil {
		mailCount = stats.TotalMessages
	}

	fileSizeEstimate := mailCount * 512
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": []map[string]any{
			{
				"id":           "report-mailflow-" + now.Format("20060102"),
				"name":         "Mail Flow Summary — " + now.Format("January 2006"),
				"type":         "mail_flow",
				"generated_at": now.Format(time.RFC3339),
				"file_size":    fileSizeEstimate,
			},
			{
				"id":           "report-audit-" + now.Format("20060102"),
				"name":         "Audit Log Export — " + now.Format("January 2006"),
				"type":         "audit",
				"generated_at": now.AddDate(0, 0, -1).Format(time.RFC3339),
				"file_size":    int64(4096),
			},
		},
	})
}

const ipAccessPolicyKey = "ip_access_policy"

type ipAccessPolicy struct {
	Enabled   bool     `json:"enabled"`
	Allowlist []string `json:"allowlist"`
	Denylist  []string `json:"denylist"`
	Protocols []string `json:"protocols"`
	Action    string   `json:"action"`
}

func defaultIPAccessPolicy() ipAccessPolicy {
	return ipAccessPolicy{
		Enabled:   false,
		Allowlist: []string{},
		Denylist:  []string{},
		Protocols: []string{"smtp", "imap", "api"},
		Action:    "deny",
	}
}

func handleGetCompanyIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ipAccessPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultIPAccessPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy ipAccessPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy ipAccessPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, ipAccessPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const retentionPolicyKey = "retention_policy"

type retentionPolicy struct {
	MailRetentionDays         int  `json:"mail_retention_days"`
	DeletedItemsRetentionDays int  `json:"deleted_items_retention_days"`
	AuditLogRetentionDays     int  `json:"audit_log_retention_days"`
	AttachmentRetentionDays   int  `json:"attachment_retention_days"`
	AutoPurgeEnabled          bool `json:"auto_purge_enabled"`
}

func defaultRetentionPolicy() retentionPolicy {
	return retentionPolicy{
		MailRetentionDays:         0,
		DeletedItemsRetentionDays: 30,
		AuditLogRetentionDays:     365,
		AttachmentRetentionDays:   0,
		AutoPurgeEnabled:          false,
	}
}

func handleGetCompanyRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, retentionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRetentionPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy retentionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy retentionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, retentionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, retentionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRetentionPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy retentionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy retentionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, retentionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, ipAccessPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultIPAccessPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy ipAccessPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy ipAccessPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, ipAccessPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	return func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got, ok := adminTokenFromRequest(w, r)
			if !ok {
				return
			}
			if !constantTimeTokenEqual(got, token) {
				writeError(w, http.StatusUnauthorized, "admin token is required")
				return
			}
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
			return
		}
		next(w, r)
	}
}

func constantTimeTokenEqual(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func safeSHA256Header(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return ""
	}
	return value
}

func parseAPIUsageLedgerListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageLedgerListRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}
	from, ok := parseOptionalRFC3339Query(w, r, "from")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	to, ok := parseOptionalRFC3339Query(w, r, "to")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req.From = from
	req.To = to
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return maildb.APIUsageLedgerListRequest{}, false
	}
	return req, true
}

func parseAPIUsageExportBatchListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageExportBatchListRequest, bool) {
	var ok bool
	req := maildb.APIUsageExportBatchListRequest{Limit: limit}
	if req.TenantID, ok = parseBoundedAdminQuery(w, r, "tenant_id"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.PrincipalID, ok = parseBoundedAdminQuery(w, r, "principal_id"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.Status, ok = parseBoundedAdminQuery(w, r, "status"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.From, ok = parseOptionalRFC3339Query(w, r, "from"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.To, ok = parseOptionalRFC3339Query(w, r, "to"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if err := maildb.ValidateAPIUsageExportBatchListRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	return req, true
}

func parseAPIUsageAggregateListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageAggregateListRequest, bool) {
	req := maildb.APIUsageAggregateListRequest{Limit: limit}
	var ok bool
	if req.TenantID, ok = parseBoundedAdminQuery(w, r, "tenant_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.CompanyID, ok = parseBoundedAdminQuery(w, r, "company_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.DomainID, ok = parseBoundedAdminQuery(w, r, "domain_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.UserID, ok = parseBoundedAdminQuery(w, r, "user_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.APIKeyID, ok = parseBoundedAdminQuery(w, r, "api_key_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.PrincipalID, ok = parseBoundedAdminQuery(w, r, "principal_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.AuthSource, ok = parseBoundedAdminQuery(w, r, "auth_source"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.Method, ok = parseBoundedAdminQuery(w, r, "method"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.Route, ok = parseBoundedAdminQuery(w, r, "route"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	var statusOK bool
	if req.Status, statusOK = parseOptionalHTTPStatusQuery(w, r, "status"); !statusOK {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.From, ok = parseOptionalRFC3339Query(w, r, "from"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.To, ok = parseOptionalRFC3339Query(w, r, "to"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return maildb.APIUsageAggregateListRequest{}, false
	}
	return req, true
}

func parseOptionalHTTPStatusQuery(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	status, err := strconv.Atoi(raw)
	if err != nil || status < 100 || status > 599 {
		writeError(w, http.StatusBadRequest, key+" must be an HTTP status code")
		return 0, false
	}
	return status, true
}

func parseAuditLogListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.AuditLogListRequest, bool) {
	category, ok := parseBoundedAdminQuery(w, r, "category")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	action, ok := parseBoundedAdminQuery(w, r, "action")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	actionPrefix, ok := parseBoundedAdminQuery(w, r, "action_prefix")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	result, ok := parseBoundedAdminQuery(w, r, "result")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	targetType, ok := parseBoundedAdminQuery(w, r, "target_type")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	actorID, ok := parseBoundedAdminQuery(w, r, "actor_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	targetID, ok := parseBoundedAdminQuery(w, r, "target_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	return maildb.AuditLogListRequest{
		Limit:        limit,
		Category:     category,
		Action:       action,
		ActionPrefix: actionPrefix,
		Result:       result,
		TargetType:   targetType,
		CompanyID:    companyID,
		DomainID:     domainID,
		UserID:       userID,
		ActorID:      actorID,
		TargetID:     targetID,
		Since:        since,
	}, true
}

func parseMailFlowLogListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.MailFlowLogListRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	rfcMessageID, ok := parseBoundedAdminQuery(w, r, "rfc_message_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	fromAddr, ok := parseBoundedAdminQuery(w, r, "from_addr")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	toAddr, ok := parseBoundedAdminQuery(w, r, "to_addr")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	subject, ok := parseBoundedAdminQuery(w, r, "subject")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	flowStatus, ok := parseBoundedAdminQuery(w, r, "flow_status")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	return maildb.MailFlowLogListRequest{
		Limit:        limit,
		Direction:    direction,
		CompanyID:    companyID,
		DomainID:     domainID,
		UserID:       userID,
		MessageID:    messageID,
		RFCMessageID: rfcMessageID,
		FromAddr:    fromAddr,
		ToAddr:      toAddr,
		Subject:     subject,
		FlowStatus:  flowStatus,
		Since:       since,
		Until:       until,
	}, true
}

func parseMailFlowLogStatsRequest(w http.ResponseWriter, r *http.Request) (maildb.MailFlowLogStatsRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	return maildb.MailFlowLogStatsRequest{
		Direction: direction,
		CompanyID: companyID,
		DomainID:  domainID,
		UserID:    userID,
		Since:     since,
		Until:     until,
	}, true
}

func parseMailFlowLogDailyStatsRequest(w http.ResponseWriter, r *http.Request) (maildb.MailFlowLogDailyStatsRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	return maildb.MailFlowLogDailyStatsRequest{
		Direction: direction,
		CompanyID: companyID,
		DomainID:  domainID,
		UserID:    userID,
		Since:     since,
		Until:     until,
	}, true
}

func parseDirectoryPrincipalSearchRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.SearchPrincipalsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	organizationID, ok := parseBoundedAdminQuery(w, r, "organization_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	rawKinds, ok := parseBoundedAdminQuery(w, r, "kinds")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	query, ok := parseBoundedAdminQuery(w, r, "q")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.SearchPrincipalsRequest{
		CompanyID:      companyID,
		DomainID:       domainID,
		OrganizationID: organizationID,
		Kinds:          splitDirectoryPrincipalKinds(rawKinds),
		Query:          query,
		ActiveOnly:     activeOnly,
		Limit:          limit,
	}, true
}

func splitDirectoryPrincipalKinds(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	kinds := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			kinds = append(kinds, field)
		}
	}
	return kinds
}

func parseDirectoryAliasResolveRequest(w http.ResponseWriter, r *http.Request) (directory.ResolveAliasRequest, bool) {
	address, ok := parseBoundedAdminQuery(w, r, "address")
	if !ok {
		return directory.ResolveAliasRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ResolveAliasRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ResolveAliasRequest{
		Address:    address,
		ActiveOnly: activeOnly,
	}, true
}

func parseDirectoryAliasListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListAliasesRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	targetKind, ok := parseBoundedAdminQuery(w, r, "target_kind")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	targetID, ok := parseBoundedAdminQuery(w, r, "target_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	query, ok := parseBoundedAdminQuery(w, r, "q")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListAliasesRequest{
		CompanyID:  companyID,
		DomainID:   domainID,
		TargetKind: targetKind,
		TargetID:   targetID,
		Query:      query,
		ActiveOnly: activeOnly,
		Limit:      limit,
	}, true
}

func parseDirectoryDelegationListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListDelegationsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	ownerKind, ok := parseBoundedAdminQuery(w, r, "owner_kind")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	ownerID, ok := parseBoundedAdminQuery(w, r, "owner_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	delegateKind, ok := parseBoundedAdminQuery(w, r, "delegate_kind")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	delegateID, ok := parseBoundedAdminQuery(w, r, "delegate_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	scope, ok := parseBoundedAdminQuery(w, r, "scope")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	role, ok := parseBoundedAdminQuery(w, r, "role")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListDelegationsRequest{
		CompanyID:    companyID,
		OwnerKind:    ownerKind,
		OwnerID:      ownerID,
		DelegateKind: delegateKind,
		DelegateID:   delegateID,
		Scope:        scope,
		Role:         role,
		ActiveOnly:   activeOnly,
		Limit:        limit,
	}, true
}

func parseDirectoryGroupMembershipListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListGroupMembershipsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	groupID, ok := parseBoundedAdminQuery(w, r, "group_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	memberKind, ok := parseBoundedAdminQuery(w, r, "member_kind")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	memberID, ok := parseBoundedAdminQuery(w, r, "member_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	role, ok := parseBoundedAdminQuery(w, r, "role")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListGroupMembershipsRequest{
		CompanyID:  companyID,
		GroupID:    groupID,
		MemberKind: memberKind,
		MemberID:   memberID,
		Role:       role,
		ActiveOnly: activeOnly,
		Limit:      limit,
	}, true
}

func parseAPIUsageLedgerRetentionRequest(w http.ResponseWriter, r *http.Request) (maildb.APIUsageLedgerRetentionRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	cutoff, ok := parseOptionalRFC3339Query(w, r, "cutoff")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.IsZero() {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "cutoff must not be in the future")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRequest{
		Cutoff:      cutoff,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}, true
}

func parseAPIUsageLedgerRetentionRunRequest(w http.ResponseWriter, req adminAPIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunRequest, bool) {
	tenantID := strings.TrimSpace(req.TenantID)
	if strings.ContainsAny(tenantID, "\r\n") {
		writeError(w, http.StatusBadRequest, "tenant_id must not contain CR or LF")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if len(tenantID) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, "tenant_id is too long")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if strings.ContainsAny(principalID, "\r\n") {
		writeError(w, http.StatusBadRequest, "principal_id must not contain CR or LF")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if len(principalID) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, "principal_id is too long")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoffRaw := strings.TrimSpace(req.Cutoff)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoff = cutoff.UTC()
	if cutoff.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "cutoff must not be in the future")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if req.Limit < 0 {
		writeError(w, http.StatusBadRequest, "limit must not be negative")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if !req.DryRun && !req.ConfirmReady {
		writeError(w, http.StatusBadRequest, "confirm_ready is required for destructive retention runs")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     tenantID,
		PrincipalID:  principalID,
		Limit:        req.Limit,
		DryRun:       req.DryRun,
		ConfirmReady: req.ConfirmReady,
	}, true
}

func parseAPIUsageLedgerRetentionRunListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageLedgerRetentionRunListRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	createdFrom, ok := parseOptionalRFC3339Query(w, r, "created_from")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	createdTo, ok := parseOptionalRFC3339Query(w, r, "created_to")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	if !createdFrom.IsZero() && !createdTo.IsZero() && !createdFrom.Before(createdTo) {
		writeError(w, http.StatusBadRequest, "created_from must be before created_to")
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRunListRequest{
		Limit:       limit,
		TenantID:    tenantID,
		PrincipalID: principalID,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, true
}

func parseDAVSyncRetentionRunListRequest(w http.ResponseWriter, r *http.Request, limit int) (davsyncretention.RunListRequest, bool) {
	statusRaw, ok := parseBoundedAdminQuery(w, r, "status")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	status := davsyncretention.RunStatus(statusRaw)
	if status != "" && status != davsyncretention.RunStatusCompleted && status != davsyncretention.RunStatusFailed {
		writeError(w, http.StatusBadRequest, "status is unsupported")
		return davsyncretention.RunListRequest{}, false
	}
	createdFrom, ok := parseOptionalRFC3339Query(w, r, "created_from")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	createdTo, ok := parseOptionalRFC3339Query(w, r, "created_to")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	if !createdFrom.IsZero() && !createdTo.IsZero() && !createdFrom.Before(createdTo) {
		writeError(w, http.StatusBadRequest, "created_from must be before created_to")
		return davsyncretention.RunListRequest{}, false
	}
	return davsyncretention.RunListRequest{
		Limit:       limit,
		Status:      status,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, true
}

func parseDAVSyncRetentionRunRequest(w http.ResponseWriter, req adminDAVSyncRetentionRunRequest) (davsyncretention.RunRequest, bool) {
	cutoffRaw := strings.TrimSpace(req.Cutoff)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return davsyncretention.RunRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return davsyncretention.RunRequest{}, false
	}
	normalized, err := davsyncretention.NormalizeRunRequest(davsyncretention.RunRequest{
		Cutoff:       cutoff,
		Limit:        req.Limit,
		DryRun:       req.DryRun,
		ConfirmReady: req.ConfirmReady,
	}, time.Now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return davsyncretention.RunRequest{}, false
	}
	return normalized, true
}

func parseDAVSyncRetentionReadinessRequest(w http.ResponseWriter, r *http.Request) (davsyncretention.ReadinessRequest, bool) {
	cutoffRaw, ok := singleQueryValue(w, r, "cutoff")
	if !ok {
		return davsyncretention.ReadinessRequest{}, false
	}
	cutoffRaw = strings.TrimSpace(cutoffRaw)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return davsyncretention.ReadinessRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return davsyncretention.ReadinessRequest{}, false
	}
	limit, ok := parseDAVSyncRetentionLimit(w, r)
	if !ok {
		return davsyncretention.ReadinessRequest{}, false
	}
	req, err := davsyncretention.NormalizeReadinessRequest(davsyncretention.ReadinessRequest{
		Cutoff: cutoff,
		Limit:  limit,
	}, time.Now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return davsyncretention.ReadinessRequest{}, false
	}
	return req, true
}

func parseDAVSyncRetentionLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw, ok := singleQueryValue(w, r, "limit")
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, "limit is too long")
		return 0, false
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return 0, false
	}
	if limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be positive")
		return 0, false
	}
	if limit > davsyncretention.MaxReadinessLimit {
		writeError(w, http.StatusBadRequest, "limit must be at most "+strconv.Itoa(davsyncretention.MaxReadinessLimit))
		return 0, false
	}
	return limit, true
}

func apiUsageLedgerRequestFromBatch(batch maildb.APIUsageExportBatchView, limit int) maildb.APIUsageLedgerListRequest {
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    batch.TenantID,
		PrincipalID: batch.PrincipalID,
	}
	if batch.WindowStart != nil {
		req.From = batch.WindowStart.UTC()
	}
	if batch.WindowEnd != nil {
		req.To = batch.WindowEnd.UTC()
	}
	return req
}

func parseOptionalRFC3339Query(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return time.Time{}, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, true
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, key+" must be RFC3339 timestamp")
		return time.Time{}, false
	}
	return value.UTC(), true
}

const maxAdminQueryFilterBytes = 1024

func parseBoundedAdminQuery(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value, ok := singleQueryValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathPair(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string) (string, string, bool) {
	first, ok := parseBoundedAdminPathValue(w, r, firstKey)
	if !ok {
		return "", "", false
	}
	second, ok := parseBoundedAdminPathValue(w, r, secondKey)
	if !ok {
		return "", "", false
	}
	return first, second, true
}

func parseBoundedAdminPathTriple(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string, thirdKey string) (string, string, string, bool) {
	first, second, ok := parseBoundedAdminPathPair(w, r, firstKey, secondKey)
	if !ok {
		return "", "", "", false
	}
	third, ok := parseBoundedAdminPathValue(w, r, thirdKey)
	if !ok {
		return "", "", "", false
	}
	return first, second, third, true
}

func adminTokenFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	adminToken, ok := singleHTTPHeaderValue(w, r, "X-Admin-Token", maxHTTPAuthHeaderBytes)
	if !ok {
		return "", false
	}
	auth, ok := singleHTTPHeaderValue(w, r, "Authorization", maxHTTPAuthHeaderBytes)
	if !ok {
		return "", false
	}
	if adminToken != "" && auth != "" {
		writeError(w, http.StatusBadRequest, "X-Admin-Token and Authorization must not both be set")
		return "", false
	}
	if adminToken != "" {
		return adminToken, true
	}
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):]), true
	}
	return "", true
}

func handleAdminLogin(w http.ResponseWriter, r *http.Request, service AdminService, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	issueToken := func(claims auth.Claims) {
		var accessToken string
		if tokenMgr != nil {
			var err error
			accessToken, err = tokenMgr.Sign(claims, 24*time.Hour)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue token")
				return
			}
		} else {
			accessToken = "test-token-" + fmt.Sprintf("%d", time.Now().Unix())
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-" + accessToken,
			"user": map[string]any{
				"id":         claims.UserID,
				"role":       claims.Role,
				"company_id": claims.CompanyID,
			},
		})
	}

	// Bootstrap system admin (no DB user required)
	if req.Email == "admin@system" && req.Password == "admin1234" {
		issueToken(auth.Claims{
			UserID:    "system-admin",
			DomainID:  "system",
			CompanyID: "",
			Role:      "system_admin",
		})
		return
	}

	// Authenticate real user from DB
	authedUser, err := service.AuthenticateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	userView, err := service.GetUser(r.Context(), authedUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	if userView.Role != "company_admin" && userView.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	domain, err := service.GetDomain(r.Context(), authedUser.DomainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve company")
		return
	}

	issueToken(auth.Claims{
		UserID:         authedUser.UserID,
		DomainID:       authedUser.DomainID,
		CompanyID:      domain.CompanyID,
		Role:           userView.Role,
		SessionVersion: authedUser.SessionVersion,
	})
}

func handleAdminSetup(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Logout is client-side: clear cookies
	// Server just confirms the logout
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "logged out",
	})
}

func handleAdminVerify(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if user has valid auth cookie
	// For now, just return 200 if any request is made
	// In production, verify the token/cookie is valid
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
	})
}

func handleListAdminUsers(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if !rejectUnknownQueryKeys(w, r) {
		return
	}

	// Return mock data - in production, query admin_user_roles joined with users
	mockUsers := []map[string]any{
		{
			"id":       "system-admin-1",
			"username": "admin",
			"email":    "admin@system",
			"role":     "system_admin",
			"created_at": "2026-05-10T13:00:00Z",
			"status": "active",
		},
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": mockUsers})
}

func handleCreateAdminUser(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Return mock success
	writeJSON(w, http.StatusOK, map[string]any{
		"id": "new-admin-user",
		"username": req.Username,
		"email": req.Email,
		"role": req.Role,
		"status": "active",
	})
}

func handleDeleteAdminUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "DELETE" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "user deleted",
	})
}

const authPolicyKey = "auth_policy"

type authPolicy struct {
	MinLength             int      `json:"min_length"`
	RequireUppercase      bool     `json:"require_uppercase"`
	RequireNumbers        bool     `json:"require_numbers"`
	RequireSymbols        bool     `json:"require_symbols"`
	MaxAgeDays            int      `json:"max_age_days"`
	HistoryCount          int      `json:"history_count"`
	MFARequired           bool     `json:"mfa_required"`
	MFAMethods            []string `json:"mfa_methods"`
	SessionTimeoutMinutes int      `json:"session_timeout_minutes"`
	MaxConcurrentSessions int      `json:"max_concurrent_sessions"`
}

func defaultAuthPolicy() authPolicy {
	return authPolicy{
		MinLength:             8,
		RequireUppercase:      false,
		RequireNumbers:        false,
		RequireSymbols:        false,
		MaxAgeDays:            0,
		HistoryCount:          0,
		MFARequired:           false,
		MFAMethods:            []string{"totp"},
		SessionTimeoutMinutes: 480,
		MaxConcurrentSessions: 0,
	}
}

func handleGetCompanyAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, authPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultAuthPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy authPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy authPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, authPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const sessionPolicyKey = "session_policy"

type sessionPolicy struct {
	TimeoutMinutes            int  `json:"timeout_minutes"`
	MaxConcurrentSessions     int  `json:"max_concurrent_sessions"`
	RequireReauthForSensitive bool `json:"require_reauth_for_sensitive_ops"`
	IdleTimeoutMinutes        int  `json:"idle_timeout_minutes"`
}

func defaultSessionPolicy() sessionPolicy {
	return sessionPolicy{
		TimeoutMinutes:            480,
		MaxConcurrentSessions:     0,
		RequireReauthForSensitive: false,
		IdleTimeoutMinutes:        0,
	}
}

func handleGetCompanySessionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, sessionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultSessionPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy sessionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySessionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy sessionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, sessionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetCompanySessions(w http.ResponseWriter, r *http.Request, _ AdminService) {
	defer r.Body.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": []map[string]any{
			{
				"user_id":     "usr-001",
				"email":       "admin@example.com",
				"ip":          "192.168.1.1",
				"started_at":  time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				"last_active": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
				"user_agent":  "Mozilla/5.0",
			},
		},
	})
}

func handleDeleteCompanySession(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"terminated": true,
		"user_id":    r.PathValue("userId"),
	})
}

const rateLimitPolicyKey = "rate_limit_policy"

type rateLimitPolicy struct {
	Enabled             bool   `json:"enabled"`
	MaxPerHour          int    `json:"max_per_hour"`
	MaxPerDay           int    `json:"max_per_day"`
	MaxRecipientsPerMsg int    `json:"max_recipients_per_msg"`
	MaxMessageSizeMB    int    `json:"max_message_size_mb"`
	ActionOnExceed      string `json:"action_on_exceed"`
	PerUserMaxPerHour   int    `json:"per_user_max_per_hour"`
	PerUserMaxPerDay    int    `json:"per_user_max_per_day"`
}

func defaultRateLimitPolicy() rateLimitPolicy {
	return rateLimitPolicy{
		Enabled:             false,
		MaxPerHour:          0,
		MaxPerDay:           0,
		MaxRecipientsPerMsg: 100,
		MaxMessageSizeMB:    25,
		ActionOnExceed:      "queue",
		PerUserMaxPerHour:   0,
		PerUserMaxPerDay:    500,
	}
}

func handleGetCompanyRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, rateLimitPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRateLimitPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy rateLimitPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy rateLimitPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, rateLimitPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, rateLimitPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRateLimitPolicy()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var policy rateLimitPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy rateLimitPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, rateLimitPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const dmarcSpfPolicyKey = "dmarc_spf_policy"

type dmarcSpfPolicy struct {
	DMARCPolicy     string   `json:"dmarc_policy"`
	DMARCPct        int      `json:"dmarc_pct"`
	DMARCRua        string   `json:"dmarc_rua"`
	DMARCRuf        string   `json:"dmarc_ruf"`
	DMARCSubdomains string   `json:"dmarc_subdomains"`
	DMARCAlignMode  string   `json:"dmarc_align_mode"`
	SPFIncludes     []string `json:"spf_includes"`
	SPFAllMechanism string   `json:"spf_all_mechanism"`
	SPFIP4List      []string `json:"spf_ip4_list"`
}

func defaultDmarcSpfPolicy() dmarcSpfPolicy {
	return dmarcSpfPolicy{
		DMARCPolicy:     "none",
		DMARCPct:        100,
		DMARCSubdomains: "none",
		DMARCAlignMode:  "r",
		SPFIncludes:     []string{},
		SPFAllMechanism: "~all",
		SPFIP4List:      []string{},
	}
}

func buildDmarcRecord(p dmarcSpfPolicy) string {
	record := fmt.Sprintf("v=DMARC1; p=%s; pct=%d; adkim=%s; aspf=%s", p.DMARCPolicy, p.DMARCPct, p.DMARCAlignMode, p.DMARCAlignMode)
	if p.DMARCRua != "" {
		record += "; rua=mailto:" + p.DMARCRua
	}
	if p.DMARCRuf != "" {
		record += "; ruf=mailto:" + p.DMARCRuf
	}
	if p.DMARCSubdomains != "none" && p.DMARCSubdomains != "" {
		record += "; sp=" + p.DMARCSubdomains
	}
	return record
}

func buildSpfRecord(p dmarcSpfPolicy) string {
	parts := []string{"v=spf1"}
	for _, inc := range p.SPFIncludes {
		parts = append(parts, "include:"+inc)
	}
	for _, ip := range p.SPFIP4List {
		parts = append(parts, "ip4:"+ip)
	}
	parts = append(parts, p.SPFAllMechanism)
	return strings.Join(parts, " ")
}

// ─── Spam / Content Filter Policy ────────────────────────────────────────────

const spamFilterPolicyKey = "spam_filter_policy"

type spamFilterPolicy struct {
	Enabled           bool     `json:"enabled"`
	SpamThreshold     int      `json:"spam_threshold"`
	VirusScanEnabled  bool     `json:"virus_scan_enabled"`
	BlockedExtensions []string `json:"blocked_extensions"`
	BlockedSenders    []string `json:"blocked_senders"`
	AllowedSenders    []string `json:"allowed_senders"`
	QuarantineEnabled bool     `json:"quarantine_enabled"`
	MaxAttachmentMB   int      `json:"max_attachment_mb"`
}

func defaultSpamFilterPolicy() spamFilterPolicy {
	return spamFilterPolicy{
		Enabled:           true,
		SpamThreshold:     5,
		VirusScanEnabled:  true,
		BlockedExtensions: []string{".exe", ".bat", ".scr", ".vbs", ".pif"},
		BlockedSenders:    []string{},
		AllowedSenders:    []string{},
		QuarantineEnabled: true,
		MaxAttachmentMB:   25,
	}
}

func handleGetCompanySpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, spamFilterPolicyKey)
	policy := defaultSpamFilterPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		if policy.BlockedExtensions == nil {
			policy.BlockedExtensions = []string{}
		}
		if policy.BlockedSenders == nil {
			policy.BlockedSenders = []string{}
		}
		if policy.AllowedSenders == nil {
			policy.AllowedSenders = []string{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy spamFilterPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.SpamThreshold < 1 || policy.SpamThreshold > 10 {
		writeError(w, http.StatusBadRequest, "spam_threshold must be 1-10")
		return
	}
	if policy.MaxAttachmentMB < 0 {
		writeError(w, http.StatusBadRequest, "max_attachment_mb must be >= 0")
		return
	}
	if policy.BlockedExtensions == nil {
		policy.BlockedExtensions = []string{}
	}
	if policy.BlockedSenders == nil {
		policy.BlockedSenders = []string{}
	}
	if policy.AllowedSenders == nil {
		policy.AllowedSenders = []string{}
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainSpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, spamFilterPolicyKey)
	policy := defaultSpamFilterPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		if policy.BlockedExtensions == nil {
			policy.BlockedExtensions = []string{}
		}
		if policy.BlockedSenders == nil {
			policy.BlockedSenders = []string{}
		}
		if policy.AllowedSenders == nil {
			policy.AllowedSenders = []string{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainSpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy spamFilterPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.SpamThreshold < 1 || policy.SpamThreshold > 10 {
		writeError(w, http.StatusBadRequest, "spam_threshold must be 1-10")
		return
	}
	if policy.MaxAttachmentMB < 0 {
		writeError(w, http.StatusBadRequest, "max_attachment_mb must be >= 0")
		return
	}
	if policy.BlockedExtensions == nil {
		policy.BlockedExtensions = []string{}
	}
	if policy.BlockedSenders == nil {
		policy.BlockedSenders = []string{}
	}
	if policy.AllowedSenders == nil {
		policy.AllowedSenders = []string{}
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

// ─── Quota Summary ────────────────────────────────────────────────────────────

func handleGetCompanyQuotaSummary(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	quotaItems, err := service.ListQuotaUsage(r.Context(), maildb.QuotaUsageListRequest{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter to this company — quota items have domain_id; filter by company id via scope or keep all if no filter
	var totalUsed, totalLimit int64
	var overLimitCount int
	for _, q := range quotaItems {
		totalUsed += q.QuotaUsed
		totalLimit += q.QuotaLimit
		if q.OverLimit {
			overLimitCount++
		}
	}

	// Top 5 by usage (already sorted descending by the DB query)
	top := quotaItems
	if len(top) > 5 {
		top = top[:5]
	}

	usageRatio := 0.0
	if totalLimit > 0 {
		usageRatio = float64(totalUsed) / float64(totalLimit)
	}

	_ = id // company scoping handled by service layer
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_entries":     len(quotaItems),
			"total_used_bytes":  totalUsed,
			"total_limit_bytes": totalLimit,
			"over_limit_count":  overLimitCount,
			"usage_ratio":       usageRatio,
		},
		"top_consumers": top,
	})
}

// ─── Routing Rules ────────────────────────────────────────────────────────────

const routingRulesKey = "routing_rules"

type routingRule struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Priority     int    `json:"priority"`
	MatchFrom    string `json:"match_from"`
	MatchTo      string `json:"match_to"`
	MatchSubject string `json:"match_subject"`
	Action       string `json:"action"`
	ActionValue  string `json:"action_value"`
}

type routingRulesConfig struct {
	Rules []routingRule `json:"rules"`
}

func handleGetCompanyRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, routingRulesKey)
	cfg := routingRulesConfig{Rules: []routingRule{}}
	if err == nil {
		_ = json.Unmarshal(entry.Value, &cfg)
		if cfg.Rules == nil {
			cfg.Rules = []routingRule{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handlePutCompanyRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var cfg routingRulesConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if cfg.Rules == nil {
		cfg.Rules = []routingRule{}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal rules")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, routingRulesKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handleGetDomainRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, routingRulesKey)
	cfg := routingRulesConfig{Rules: []routingRule{}}
	if err == nil {
		_ = json.Unmarshal(entry.Value, &cfg)
		if cfg.Rules == nil {
			cfg.Rules = []routingRule{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handlePutDomainRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var cfg routingRulesConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if cfg.Rules == nil {
		cfg.Rules = []routingRule{}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal rules")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, routingRulesKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

// ─── SSO / SAML Configuration ─────────────────────────────────────────────────

const ssoConfigKey = "sso_config"

type ssoConfig struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	EntityID       string `json:"entity_id"`
	MetadataURL    string `json:"metadata_url"`
	SSOLoginURL    string `json:"sso_login_url"`
	Certificate    string `json:"certificate"`
	AttributeEmail string `json:"attribute_email"`
	AttributeName  string `json:"attribute_name"`
	ForceSSO       bool   `json:"force_sso"`
	AutoProvision  bool   `json:"auto_provision"`
	DefaultRole    string `json:"default_role"`
}

func defaultSSOConfig() ssoConfig {
	return ssoConfig{
		Enabled:        false,
		Provider:       "saml",
		EntityID:       "",
		MetadataURL:    "",
		SSOLoginURL:    "",
		Certificate:    "",
		AttributeEmail: "email",
		AttributeName:  "displayName",
		ForceSSO:       false,
		AutoProvision:  false,
		DefaultRole:    "viewer",
	}
}

func handleGetCompanySSOConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ssoConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"config": defaultSSOConfig()})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var cfg ssoConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse sso config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func handlePutCompanySSOConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var cfg ssoConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal sso config")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, ssoConfigKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func handlePostCompanySSOTest(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ssoConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeError(w, http.StatusBadRequest, "SSO is not configured")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var cfg ssoConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse sso config")
		return
	}
	if cfg.MetadataURL == "" && cfg.SSOLoginURL == "" {
		writeError(w, http.StatusBadRequest, "metadata_url or sso_login_url is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "SSO configuration validated (simulation)",
	})
}

// ─── Outbound SMTP Policy ─────────────────────────────────────────────────────

const smtpPolicyKey = "smtp_policy"

type smtpPolicy struct {
	TLSRequired          bool     `json:"tls_required"`
	TLSMinVersion        string   `json:"tls_min_version"`
	STARTTLSEnabled      bool     `json:"starttls_enabled"`
	DedicatedIPEnabled   bool     `json:"dedicated_ip_enabled"`
	DedicatedIPs         []string `json:"dedicated_ips"`
	RetryCount           int      `json:"retry_count"`
	RetryIntervalMinutes int      `json:"retry_interval_minutes"`
	ConnectionTimeout    int      `json:"connection_timeout_seconds"`
	HELOHostname         string   `json:"helo_hostname"`
	BounceAddress        string   `json:"bounce_address"`
}

func defaultSMTPPolicy() smtpPolicy {
	return smtpPolicy{
		TLSRequired:          false,
		TLSMinVersion:        "tls1.2",
		STARTTLSEnabled:      true,
		DedicatedIPEnabled:   false,
		DedicatedIPs:         []string{},
		RetryCount:           3,
		RetryIntervalMinutes: 60,
		ConnectionTimeout:    30,
		HELOHostname:         "",
		BounceAddress:        "",
	}
}

func handleGetDomainSMTPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, smtpPolicyKey)
	policy := defaultSMTPPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		if policy.DedicatedIPs == nil {
			policy.DedicatedIPs = []string{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainSMTPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy smtpPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.RetryCount < 0 || policy.RetryCount > 10 {
		writeError(w, http.StatusBadRequest, "retry_count must be 0-10")
		return
	}
	if policy.DedicatedIPs == nil {
		policy.DedicatedIPs = []string{}
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal smtp policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, smtpPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainDmarcSpfPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, dmarcSpfPolicyKey)
	policy := defaultDmarcSpfPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policy,
		"generated_records": map[string]any{
			"dmarc":      buildDmarcRecord(policy),
			"spf":        buildSpfRecord(policy),
			"dmarc_host": "_dmarc.<domain>",
			"spf_host":   "<domain>",
		},
	})
}

func handlePutDomainDmarcSpfPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy dmarcSpfPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.DMARCPct < 0 || policy.DMARCPct > 100 {
		writeError(w, http.StatusBadRequest, "dmarc_pct must be 0-100")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, dmarcSpfPolicyKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policy,
		"generated_records": map[string]any{
			"dmarc":      buildDmarcRecord(policy),
			"spf":        buildSpfRecord(policy),
			"dmarc_host": "_dmarc.<domain>",
			"spf_host":   "<domain>",
		},
	})
}

// ─── Webhooks ─────────────────────────────────────────────────────────────────

const webhooksConfigKey = "webhooks_config"

type webhook struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Secret          string   `json:"secret"`
	Events          []string `json:"events"`
	Enabled         bool     `json:"enabled"`
	CreatedAt       string   `json:"created_at"`
	LastTriggeredAt string   `json:"last_triggered_at,omitempty"`
}

type webhooksConfig struct {
	Webhooks []webhook `json:"webhooks"`
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getWebhooksConfig(ctx context.Context, service AdminService, companyID string) (webhooksConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, webhooksConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return webhooksConfig{Webhooks: []webhook{}}, nil
		}
		return webhooksConfig{}, err
	}
	var cfg webhooksConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return webhooksConfig{Webhooks: []webhook{}}, nil
	}
	if cfg.Webhooks == nil {
		cfg.Webhooks = []webhook{}
	}
	return cfg, nil
}

func saveWebhooksConfig(ctx context.Context, service AdminService, companyID string, cfg webhooksConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = service.SetCompanyConfig(ctx, companyID, webhooksConfigKey, json.RawMessage(b), false, 0)
	return err
}

func handleGetCompanyWebhooks(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": cfg.Webhooks})
}

func handlePostCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Events  []string `json:"events"`
		Enabled bool     `json:"enabled"`
	}
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.Name == "" || input.URL == "" {
		writeError(w, http.StatusBadRequest, "name and url are required")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	wh := webhook{
		ID:        fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:      input.Name,
		URL:       input.URL,
		Secret:    randomHex(16),
		Events:    input.Events,
		Enabled:   input.Enabled,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if wh.Events == nil {
		wh.Events = []string{}
	}
	cfg.Webhooks = append(cfg.Webhooks, wh)
	if err := saveWebhooksConfig(r.Context(), service, id, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"webhook": wh})
}

func handleDeleteCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookId is required")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	found := false
	filtered := cfg.Webhooks[:0]
	for _, wh := range cfg.Webhooks {
		if wh.ID == webhookID {
			found = true
			continue
		}
		filtered = append(filtered, wh)
	}
	if !found {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	cfg.Webhooks = filtered
	if err := saveWebhooksConfig(r.Context(), service, id, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleTestCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookId is required")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *webhook
	for i := range cfg.Webhooks {
		if cfg.Webhooks[i].ID == webhookID {
			target = &cfg.Webhooks[i]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	payload := fmt.Sprintf(`{"event":"test","timestamp":"%s","data":{"message":"Test webhook from gogomail"}}`,
		time.Now().UTC().Format(time.RFC3339))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.URL, strings.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": fmt.Sprintf("failed to build request: %v", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gogomail-Event", "test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": fmt.Sprintf("request failed: %v", err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "status_code": resp.StatusCode, "message": fmt.Sprintf("webhook responded with %d", resp.StatusCode)})
	} else {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": resp.StatusCode, "message": fmt.Sprintf("webhook responded with %d", resp.StatusCode)})
	}
}

// ─── Notification Templates ───────────────────────────────────────────────────

const notifTemplatesKey = "notification_templates"

type notifTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Enabled bool   `json:"enabled"`
}

type notifTemplatesConfig struct {
	Templates []notifTemplate `json:"templates"`
}

func defaultNotifTemplates() []notifTemplate {
	return []notifTemplate{
		{ID: "password_reset", Name: "Password Reset", Subject: "Reset your {{.CompanyName}} password", Body: "<p>Click the link below to reset your password:</p><p><a href='{{.ResetURL}}'>Reset Password</a></p>", Enabled: true},
		{ID: "welcome", Name: "Welcome Email", Subject: "Welcome to {{.CompanyName}}", Body: "<p>Welcome, {{.UserName}}! Your account has been created.</p>", Enabled: true},
		{ID: "quota_warning", Name: "Quota Warning", Subject: "Storage quota warning — {{.UsagePercent}}% used", Body: "<p>Your mailbox is {{.UsagePercent}}% full. Please free up space or contact your admin.</p>", Enabled: true},
		{ID: "account_locked", Name: "Account Locked", Subject: "Your account has been locked", Body: "<p>Your account has been locked due to too many failed login attempts. Contact your administrator.</p>", Enabled: true},
	}
}

func getNotifTemplatesConfig(ctx context.Context, service AdminService, companyID string) (notifTemplatesConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, notifTemplatesKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return notifTemplatesConfig{Templates: defaultNotifTemplates()}, nil
		}
		return notifTemplatesConfig{}, err
	}
	var cfg notifTemplatesConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return notifTemplatesConfig{Templates: defaultNotifTemplates()}, nil
	}
	if cfg.Templates == nil {
		cfg.Templates = defaultNotifTemplates()
	}
	return cfg, nil
}

func handleGetNotifTemplates(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	cfg, err := getNotifTemplatesConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": cfg.Templates})
}

func handlePutNotifTemplate(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	templateID := r.PathValue("templateId")
	if templateID == "" {
		writeError(w, http.StatusBadRequest, "templateId is required")
		return
	}
	var input notifTemplate
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cfg, err := getNotifTemplatesConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	found := false
	for i := range cfg.Templates {
		if cfg.Templates[i].ID == templateID {
			input.ID = templateID
			cfg.Templates[i] = input
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal templates")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, notifTemplatesKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"template": input})
}

// ─── Audit Log Export ─────────────────────────────────────────────────────────

func handleExportCompanyAuditLogs(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := 1000
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 10000 {
			limit = parsed
		}
	}
	req := maildb.AuditLogListRequest{
		CompanyID:    id,
		Limit:        limit,
		Category:     q.Get("category"),
		ActionPrefix: q.Get("action_prefix"),
	}
	logs, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-logs-%s.csv"`, id))
	wr := csv.NewWriter(w)
	_ = wr.Write([]string{"id", "company_id", "actor_id", "category", "action", "target_type", "target_id", "result", "ip_address", "created_at"})
	for _, l := range logs {
		_ = wr.Write([]string{
			l.ID, l.CompanyID, l.ActorID, l.Category, l.Action,
			l.TargetType, l.TargetID, l.Result, l.IPAddress,
			l.CreatedAt.Format(time.RFC3339),
		})
	}
	wr.Flush()
}

// ─── Bulk Domain Operations ───────────────────────────────────────────────────

func handleBulkDomains(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	var input struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // "activate", "suspend", "delete"
	}
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(input.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if input.Action == "" {
		writeError(w, http.StatusBadRequest, "action is required")
		return
	}
	ctx := r.Context()
	succeeded := []string{}
	failed := []map[string]string{}
	for _, id := range input.IDs {
		var err error
		switch input.Action {
		case "activate":
			err = service.UpdateDomainStatus(ctx, maildb.UpdateDomainStatusRequest{ID: id, Status: "active"})
		case "suspend":
			err = service.UpdateDomainStatus(ctx, maildb.UpdateDomainStatusRequest{ID: id, Status: "suspended"})
		case "delete":
			err = service.DeleteDomain(ctx, id)
		default:
			writeError(w, http.StatusBadRequest, "unknown action: "+input.Action)
			return
		}
		if err != nil {
			failed = append(failed, map[string]string{"id": id, "error": err.Error()})
		} else {
			succeeded = append(succeeded, id)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"succeeded": succeeded,
		"failed":    failed,
	})
}

// ─── Change History ───────────────────────────────────────────────────────────

func handleGetCompanyChangeHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := 100
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	req := maildb.AuditLogListRequest{
		CompanyID:    id,
		Limit:        limit,
		ActionPrefix: q.Get("action_prefix"),
		Category:     q.Get("category"),
		ActorID:      q.Get("actor_id"),
	}
	logs, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": logs, "total": len(logs)})
}

// ─── Pending Approvals ────────────────────────────────────────────────────────

const pendingApprovalsKey = "pending_approvals"

type approvalItem struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Payload     json.RawMessage `json:"payload"`
	RequestedBy string          `json:"requested_by"`
	RequestedAt string          `json:"requested_at"`
	Status      string          `json:"status"`
	ReviewedBy  string          `json:"reviewed_by,omitempty"`
	ReviewedAt  string          `json:"reviewed_at,omitempty"`
	Comment     string          `json:"comment,omitempty"`
}

type approvalsConfig struct {
	Items []approvalItem `json:"items"`
}

func getApprovalsConfig(ctx context.Context, service AdminService, companyID string) (approvalsConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, pendingApprovalsKey)
	if errors.Is(err, configstore.ErrConfigNotFound) {
		return approvalsConfig{Items: []approvalItem{}}, nil
	}
	if err != nil {
		return approvalsConfig{}, err
	}
	var cfg approvalsConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return approvalsConfig{Items: []approvalItem{}}, nil
	}
	if cfg.Items == nil {
		cfg.Items = []approvalItem{}
	}
	return cfg, nil
}

func saveApprovalsConfig(ctx context.Context, service AdminService, companyID string, cfg approvalsConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = service.SetCompanyConfig(ctx, companyID, pendingApprovalsKey, json.RawMessage(b), false, 0)
	return err
}

func handleGetPendingApprovals(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	out := []approvalItem{}
	for _, item := range cfg.Items {
		if item.Status == status {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": out})
}

func handleCreatePendingApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input approvalItem
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	input.ID = fmt.Sprintf("ap-%d", time.Now().UnixNano())
	input.Status = "pending"
	input.RequestedAt = time.Now().UTC().Format(time.RFC3339)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg.Items = append(cfg.Items, input)
	if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"approval": input})
}

func handleApproveApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	approvalID := r.PathValue("approvalId")
	var input struct {
		ReviewedBy string `json:"reviewed_by"`
		Comment    string `json:"comment"`
	}
	_ = decodeJSONBody(r, &input)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "approved"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"approval": cfg.Items[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "approval not found")
}

func handleRejectApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	approvalID := r.PathValue("approvalId")
	var input struct {
		ReviewedBy string `json:"reviewed_by"`
		Comment    string `json:"comment"`
	}
	_ = decodeJSONBody(r, &input)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "rejected"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"approval": cfg.Items[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "approval not found")
}

func handleGetCompanyHealth(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	company, err := service.GetCompany(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "company not found")
		return
	}

	domains, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})

	activeDomains := 0
	totalQuotaBytes := int64(0)
	usedQuotaBytes := int64(0)
	overAllocated := false
	for _, d := range domains {
		if d.Status == "active" {
			activeDomains++
		}
		totalQuotaBytes += d.QuotaLimit
		usedQuotaBytes += d.QuotaUsed
		if d.OverAllocated {
			overAllocated = true
		}
	}

	webhooksCfg, _ := getWebhooksConfig(ctx, service, id)
	activeWebhooks := 0
	for _, wh := range webhooksCfg.Webhooks {
		if wh.Enabled {
			activeWebhooks++
		}
	}

	usagePct := 0.0
	if totalQuotaBytes > 0 {
		usagePct = float64(usedQuotaBytes) / float64(totalQuotaBytes) * 100
	}

	healthStatus := "healthy"
	if overAllocated || usagePct > 90 {
		healthStatus = "warning"
	}
	if activeDomains == 0 && len(domains) > 0 {
		healthStatus = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"health": map[string]any{
			"status":           healthStatus,
			"company_id":       id,
			"company_name":     company.Name,
			"domain_count":     len(domains),
			"active_domains":   activeDomains,
			"active_webhooks":  activeWebhooks,
			"over_allocated":   overAllocated,
			"quota": map[string]any{
				"total_bytes": totalQuotaBytes,
				"used_bytes":  usedQuotaBytes,
				"usage_pct":   usagePct,
			},
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func listCompanyUsers(ctx context.Context, service AdminService, companyID string, perDomainLimit int) ([]maildb.UserView, error) {
	domains, err := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: companyID, Limit: 200})
	if err != nil {
		return nil, err
	}
	return listUsersForDomains(ctx, service, domains, perDomainLimit)
}

func listUsersForDomains(ctx context.Context, service AdminService, domains []maildb.DomainView, perDomainLimit int) ([]maildb.UserView, error) {
	users := []maildb.UserView{}
	for _, domain := range domains {
		domainUsers, err := service.ListUsers(ctx, maildb.UserListRequest{DomainID: domain.ID, Limit: perDomainLimit})
		if err != nil {
			return nil, err
		}
		users = append(users, domainUsers...)
	}
	return users, nil
}

// ─── Security Posture ─────────────────────────────────────────────────────────

func handleGetSecurityPosture(w http.ResponseWriter, r *http.Request, service AdminService, companyID string) {
	ctx := r.Context()

	mfaStats, _ := service.GetMFAStats(ctx, companyID)
	domains, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: companyID, Limit: 200})

	users, _ := listUsersForDomains(ctx, service, domains, 1000)
	usersWithoutPassword := 0
	for _, u := range users {
		if !u.PasswordConfigured {
			usersWithoutPassword++
		}
	}

	ipPolicyCfg, ipErr := service.GetCompanyConfig(ctx, companyID, ipAccessPolicyKey)
	ipPolicyConfigured := ipErr == nil && ipPolicyCfg.Value != nil

	score := 100
	if mfaStats.Total > 0 && mfaStats.Enabled == 0 {
		score -= 30
	}
	if !ipPolicyConfigured {
		score -= 10
	}
	if usersWithoutPassword > 0 {
		score -= 20
	}

	mfaRate := 0.0
	if mfaStats.Total > 0 {
		mfaRate = float64(mfaStats.Enabled) / float64(mfaStats.Total) * 100
	}

	activeDomains := 0
	for _, d := range domains {
		if d.Status == "active" {
			activeDomains++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"score": score,
		"mfa": map[string]any{
			"total":   mfaStats.Total,
			"enabled": mfaStats.Enabled,
			"rate":    mfaRate,
		},
		"ip_policy_configured":    ipPolicyConfigured,
		"users_without_password":  usersWithoutPassword,
		"domain_count":            len(domains),
		"active_domains":          activeDomains,
	})
}

// ─── Global Signature ─────────────────────────────────────────────────────────

const emailSignatureKey = "email_signature"

type signatureConfig struct {
	HTML    string `json:"html"`
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

func handleGetSignature(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, emailSignatureKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"signature": signatureConfig{}})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var cfg signatureConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse signature config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signature": cfg})
}

func handlePutSignature(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var cfg signatureConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal signature config")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, emailSignatureKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signature": cfg})
}

// ─── Legal Holds ──────────────────────────────────────────────────────────────

const legalHoldsKey = "legal_holds"

type legalHold struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

type legalHoldsConfig struct {
	Holds []legalHold `json:"holds"`
}

func getLegalHoldsConfig(ctx context.Context, service AdminService, companyID string) (legalHoldsConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, legalHoldsKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return legalHoldsConfig{Holds: []legalHold{}}, nil
		}
		return legalHoldsConfig{}, err
	}
	var cfg legalHoldsConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return legalHoldsConfig{Holds: []legalHold{}}, nil
	}
	if cfg.Holds == nil {
		cfg.Holds = []legalHold{}
	}
	return cfg, nil
}

func handleGetLegalHolds(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	cfg, err := getLegalHoldsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"holds": cfg.Holds})
}

func handleCreateLegalHold(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input legalHold
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	input.ID = fmt.Sprintf("hold-%d", time.Now().UnixNano())
	input.CreatedAt = time.Now().UTC()

	cfg, err := getLegalHoldsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg.Holds = append(cfg.Holds, input)

	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal legal holds")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, legalHoldsKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"hold": input})
}

func handleDeleteLegalHold(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	holdID, ok := parseBoundedAdminPathValue(w, r, "holdId")
	if !ok {
		return
	}

	cfg, err := getLegalHoldsConfig(r.Context(), service, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	found := false
	filtered := cfg.Holds[:0]
	for _, h := range cfg.Holds {
		if h.ID == holdID {
			found = true
		} else {
			filtered = append(filtered, h)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "legal hold not found")
		return
	}
	cfg.Holds = filtered

	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal legal holds")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, legalHoldsKey, json.RawMessage(b), false, 0); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ─── SCIM Status ──────────────────────────────────────────────────────────────

func handleGetSCIMStatus(w http.ResponseWriter, r *http.Request, service AdminService) {
	ctx := r.Context()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domains, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})
	domainID := ""
	if len(domains) > 0 {
		domainID = domains[0].ID
	}
	users, _ := listUsersForDomains(ctx, service, domains, 1000)
	writeJSON(w, http.StatusOK, map[string]any{
		"endpoint":            "/scim/v2",
		"supported_resources": []string{"Users"},
		"domain_id":           domainID,
		"user_count":          len(users),
		"status":              "active",
	})
}

// ─── Seat Usage ───────────────────────────────────────────────────────────────

func handleGetSeatUsage(w http.ResponseWriter, r *http.Request, service AdminService) {
	ctx := r.Context()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domains, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})
	totalUsers := 0
	activeUsers := 0
	suspendedUsers := 0
	for _, d := range domains {
		us, _ := service.ListUsers(ctx, maildb.UserListRequest{DomainID: d.ID, Limit: 1000})
		totalUsers += len(us)
		for _, u := range us {
			if u.Status == "active" {
				activeUsers++
			} else {
				suspendedUsers++
			}
		}
	}
	company, _ := service.GetCompany(ctx, id)
	writeJSON(w, http.StatusOK, map[string]any{
		"total_users":     totalUsers,
		"active_users":    activeUsers,
		"suspended_users": suspendedUsers,
		"domain_count":    len(domains),
		"storage_used":    company.QuotaUsed,
		"storage_limit":   company.AllocatedDomainQuota,
	})
}
