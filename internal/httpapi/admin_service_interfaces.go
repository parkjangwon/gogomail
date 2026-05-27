package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/maildb"
)

type AdminService interface {
	adminCompanyService
	adminDomainService
	adminRoleService
	adminUserService
	adminQueueService
	adminAuditService
	adminMailFlowService
	adminStorageService
	adminDirectoryService
	adminDriveService
	adminUsageService
	adminDeliveryService
	adminConfigService
	adminAlertService
	adminSecurityService
	adminSyncService
}

type adminCompanyService interface {
	ListCompanies(ctx context.Context, req maildb.CompanyListRequest) ([]maildb.CompanyView, bool, error)
	CreateCompany(ctx context.Context, req maildb.CreateCompanyRequest) (maildb.CompanyView, error)
	GetCompany(ctx context.Context, id string) (maildb.CompanyView, error)
	UpdateCompanyQuota(ctx context.Context, req maildb.UpdateCompanyQuotaRequest) error
	UpdateCompany(ctx context.Context, req maildb.UpdateCompanyRequest) (maildb.CompanyView, error)
	DeleteCompany(ctx context.Context, id string) error
}

type adminDomainService interface {
	ListDomains(ctx context.Context, req maildb.DomainListRequest) ([]maildb.DomainView, bool, error)
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
	GetAPIKey(ctx context.Context, keyID string) (*admin.APIKey, error)
	CreateAPIKey(ctx context.Context, key *admin.APIKey) (secret string, err error)
	ListAPIKeys(ctx context.Context, domainID string) ([]admin.APIKey, error)
	DeleteAPIKey(ctx context.Context, keyID string) error
	RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error)
}

type adminRoleService interface {
	ListAdminRoles(ctx context.Context, companyID string) ([]admin.RoleSummary, error)
	CreateAdminRole(ctx context.Context, req admin.CreateRoleRequest) (admin.RoleSummary, error)
}

type adminUserService interface {
	ListUsers(ctx context.Context, req maildb.UserListRequest) ([]maildb.UserView, bool, error)
	GetUser(ctx context.Context, id string) (maildb.UserView, error)
	CreateUser(ctx context.Context, req maildb.CreateUserRequest) (maildb.UserView, error)
	DeleteUser(ctx context.Context, id string) error
	UpdateUserStatus(ctx context.Context, req maildb.UpdateUserStatusRequest) error
	BulkUpdateUserStatus(ctx context.Context, req maildb.BulkUpdateUserStatusRequest) (maildb.BulkUpdateUserStatusResult, error)
	UpdateUserQuota(ctx context.Context, req maildb.UpdateUserQuotaRequest) error
	UpdateUserPasswordHash(ctx context.Context, req maildb.UpdateUserPasswordHashRequest) error
	UpdateUserRole(ctx context.Context, req maildb.UpdateUserRoleRequest) error
	UpdateUserRecoveryEmail(ctx context.Context, req maildb.UpdateUserRecoveryEmailRequest) error
	AuthenticateUser(ctx context.Context, email, password string) (maildb.AuthenticatedUser, error)
	IncrementSessionVersion(ctx context.Context, userID string) (int64, error)
	CreateInviteToken(ctx context.Context, userID, createdBy string) (maildb.InviteToken, error)
	GetInviteToken(ctx context.Context, token string) (maildb.InviteToken, error)
	AcceptInviteToken(ctx context.Context, token, passwordHash string) (maildb.UserView, error)
	ListAdminUsers(ctx context.Context, req maildb.AdminUserListRequest) ([]maildb.AdminUserView, bool, error)
	SetUserRole(ctx context.Context, userID, role string) error
	ClearUserAdminRole(ctx context.Context, userID string) error
}

type adminQueueService interface {
	ListQueueStats(ctx context.Context) ([]maildb.QueueStat, error)
	ListOutboxEvents(ctx context.Context, req maildb.OutboxEventListRequest) ([]maildb.OutboxEventView, bool, error)
	GetOutboxEvent(ctx context.Context, id string) (maildb.OutboxEventView, error)
	RetryOutbox(ctx context.Context, id string) error
}

type adminAuditService interface {
	ListAuditLogs(ctx context.Context, req maildb.AuditLogListRequest) ([]maildb.AuditLogView, bool, error)
	GetAuditLog(ctx context.Context, id string) (maildb.AuditLogView, error)
	CheckAuditLogIntegrity(ctx context.Context, req maildb.AuditLogIntegrityRequest) (maildb.AuditLogIntegrityView, error)
}

type adminMailFlowService interface {
	ListMailFlowLogs(ctx context.Context, req maildb.MailFlowLogListRequest) ([]maildb.MailFlowLogView, error)
	GetMailFlowLog(ctx context.Context, id string) (maildb.MailFlowLogView, error)
	GetMailFlowLogStats(ctx context.Context, req maildb.MailFlowLogStatsRequest) (maildb.MailFlowLogStatsView, error)
	GetMailFlowLogDailyStats(ctx context.Context, req maildb.MailFlowLogDailyStatsRequest) ([]maildb.MailFlowLogDailyStatsView, error)
}

type adminStorageService interface {
	ListQuotaUsage(ctx context.Context, req maildb.QuotaUsageListRequest) ([]maildb.QuotaUsageView, error)
	RunAttachmentCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error)
	CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error)
	ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error)
	RunAttachmentUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error)
	CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error)
	ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error)
	ListAttachmentUploadSessions(ctx context.Context, req maildb.AttachmentUploadSessionListRequest) ([]maildb.AttachmentUploadSession, error)
	ListQuotaReconciliation(ctx context.Context, limit int) ([]maildb.QuotaReconciliationView, error)
	CorrectQuotaReconciliation(ctx context.Context, req maildb.CorrectQuotaReconciliationRequest) (maildb.QuotaCorrectionResult, error)
	ListQuotaAlertThresholds(ctx context.Context, req maildb.QuotaAlertThresholdListRequest) ([]maildb.QuotaAlertThresholdView, error)
	GetQuotaAlertThreshold(ctx context.Context, id string) (maildb.QuotaAlertThresholdView, error)
	CreateQuotaAlertThreshold(ctx context.Context, req maildb.CreateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error)
	UpdateQuotaAlertThreshold(ctx context.Context, req maildb.UpdateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error)
	DeleteQuotaAlertThreshold(ctx context.Context, id string) error
	ListQuotaAlerts(ctx context.Context, req maildb.QuotaAlertListRequest) ([]maildb.QuotaAlertView, error)
	GetQuotaAlert(ctx context.Context, id string) (maildb.QuotaAlertView, error)
}

type adminDirectoryService interface {
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
	GetDirectoryAlias(ctx context.Context, id string) (directory.Alias, error)
	GetDirectoryGroupMembership(ctx context.Context, id string) (directory.GroupMembership, error)
	GetDirectoryDelegation(ctx context.Context, id string) (directory.Delegation, error)
}

type adminDriveService interface {
	ListDriveUploadSessions(ctx context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error)
	ListDriveNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetDriveNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	GetDriveUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
	CountStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) (drive.StaleUploadSessionCount, error)
	ListStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error)
	RunDriveUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error)
	GetDriveObjectCleanupFailure(ctx context.Context, id string) (drive.ObjectCleanupFailure, error)
	ListDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error)
	ResolveDriveObjectCleanupFailure(ctx context.Context, id string) (drive.ObjectCleanupFailure, error)
	RetryDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error)
}

type adminUsageService interface {
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
}

type adminDeliveryService interface {
	ListDeliveryAttempts(ctx context.Context, req maildb.DeliveryAttemptListRequest) ([]maildb.DeliveryAttemptView, bool, error)
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
	GetSuppressionEntry(ctx context.Context, id string) (maildb.SuppressionEntry, error)
	DeleteSuppressionEntry(ctx context.Context, id string) error
	ListTrustedRelays(ctx context.Context, req maildb.TrustedRelayListRequest) ([]maildb.TrustedRelayView, error)
	CreateTrustedRelay(ctx context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error)
	DeleteTrustedRelay(ctx context.Context, id string) error
	ListDeliveryRoutes(ctx context.Context, req maildb.DeliveryRouteListRequest) ([]maildb.DeliveryRouteView, error)
	CreateDeliveryRoute(ctx context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error)
	ResolveDeliveryRoute(ctx context.Context, domain string) (maildb.DeliveryRouteResolveView, error)
	UpdateDeliveryRouteStatus(ctx context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error
	DeleteDeliveryRoute(ctx context.Context, id string) error
	ListDKIMKeys(ctx context.Context, req maildb.DKIMKeyListRequest) ([]maildb.DKIMKeyView, error)
	GetDKIMKey(ctx context.Context, id string) (maildb.DKIMKeyView, error)
	CreateDKIMKey(ctx context.Context, input maildb.CreateDKIMKeyInput) (string, error)
	DeactivateDKIMKey(ctx context.Context, id string) error
	VerifyDKIMKeyDNS(ctx context.Context, keyID string) (maildb.DKIMKeyDNSVerificationResult, error)
	BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error)
}

type adminConfigService interface {
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
}

type adminAlertService interface {
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
	ListAlertEvents(ctx context.Context, filter admin.AlertEventFilter) ([]admin.AlertEvent, bool, error)
	LogAlertEvent(ctx context.Context, event *admin.AlertEvent) error
}

type adminSecurityService interface {
	GetUserMFAStatus(ctx context.Context, userID string) (maildb.UserMFAStatus, error)
	ResetUserMFA(ctx context.Context, userID string) error
	GetMFAStats(ctx context.Context, companyID string) (maildb.MFAStats, error)
	ListLoginAttempts(ctx context.Context, filter admin.LoginAuditFilter) ([]admin.LoginAuditLog, error)
}

type adminSyncService interface {
	TriggerLDAPSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error)
	GetLDAPSyncRuns(ctx context.Context, req maildb.LDAPSyncRunListRequest) ([]maildb.LDAPSyncRunView, error)
	GetLDAPSyncRun(ctx context.Context, runID string) (*maildb.LDAPSyncRunView, error)
	GetLDAPSyncConflicts(ctx context.Context, req maildb.LDAPSyncConflictListRequest) ([]maildb.LDAPSyncConflictView, error)
	GetLDAPSyncConflict(ctx context.Context, conflictID string) (*maildb.LDAPSyncConflictView, error)
	ResolveLDAPSyncConflict(ctx context.Context, conflictID, resolution string) error
	TriggerRDBMSSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error)
	GetRDBMSSyncRuns(ctx context.Context, req maildb.RDBMSSyncRunListRequest) ([]maildb.RDBMSSyncRunView, error)
	GetRDBMSSyncRun(ctx context.Context, runID string) (*maildb.RDBMSSyncRunView, error)
	GetRDBMSSyncConflicts(ctx context.Context, req maildb.RDBMSSyncConflictListRequest) ([]maildb.RDBMSSyncConflictView, error)
	GetRDBMSSyncConflict(ctx context.Context, conflictID string) (*maildb.RDBMSSyncConflictView, error)
	ResolveRDBMSSyncConflict(ctx context.Context, conflictID, resolution string) error
	GetDomainIdPConfig(ctx context.Context, domainID string) (*idprovider.Config, error)
	SetDomainIdPConfig(ctx context.Context, cfg *idprovider.Config) error
	DeleteDomainIdPConfig(ctx context.Context, domainID string) error
}
