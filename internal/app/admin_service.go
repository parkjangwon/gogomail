package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
)

type backpressureStore interface {
	State(ctx context.Context) (backpressure.State, error)
	SetState(ctx context.Context, update backpressure.StateUpdate) (backpressure.State, error)
}

type auditWriter interface {
	Insert(ctx context.Context, log audit.Log) error
}

type adminService struct {
	*maildb.Repository
	adminSvc                    *admin.Service
	backpressure                backpressureStore
	audit                       auditWriter
	exportStore                 apimeter.ExportArtifactStore
	exportManifestSigner        apimeter.ExportManifestSigner
	exportManifestSignerBackend string
	exportManifestVerifier      apimeter.ExportManifestSignatureVerifier
	directory                   interface {
		CreateAliasWithAudit(ctx context.Context, req directory.CreateAliasRequest) (directory.Alias, error)
		CreateDelegationWithAudit(ctx context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error)
		CreateGroupMembershipWithAudit(ctx context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error)
		DeleteAliasWithAudit(ctx context.Context, id string) (directory.Alias, error)
		DeleteDelegationWithAudit(ctx context.Context, id string) (directory.Delegation, error)
		DeleteGroupMembershipWithAudit(ctx context.Context, id string) (directory.GroupMembership, error)
		GetAlias(ctx context.Context, id string) (directory.Alias, error)
		GetDelegation(ctx context.Context, id string) (directory.Delegation, error)
		GetGroupMembership(ctx context.Context, id string) (directory.GroupMembership, error)
		ListAliases(ctx context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error)
		ListDelegations(ctx context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error)
		ListGroupMemberships(ctx context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error)
		ResolveAlias(ctx context.Context, req directory.ResolveAliasRequest) (directory.Alias, error)
		ReassignDelegationWithAudit(ctx context.Context, req directory.ReassignDelegationRequest) (directory.Delegation, error)
		ReassignGroupMembershipWithAudit(ctx context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error)
		SearchPrincipals(ctx context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error)
		UpdateDelegationRoleWithAudit(ctx context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error)
		UpdateGroupMembershipRoleWithAudit(ctx context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error)
	}
	drive interface {
		ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
		GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
		GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
		ListUploadSessions(ctx context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error)
		CountStaleUploadSessions(ctx context.Context, req drive.ExpireUploadSessionsRequest) (drive.StaleUploadSessionCount, error)
		ListStaleUploadSessions(ctx context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error)
		ExpireUploadSessions(ctx context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error)
		ListObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error)
		ResolveObjectCleanupFailure(ctx context.Context, req drive.ResolveObjectCleanupFailureRequest) (drive.ObjectCleanupFailure, error)
		RetryObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error)
	}
	davSyncRetention interface {
		RecordRun(ctx context.Context, record davsyncretention.RunRecord) (davsyncretention.RunRecord, error)
		ListRuns(ctx context.Context, req davsyncretention.RunListRequest) ([]davsyncretention.RunRecord, error)
		GetRun(ctx context.Context, id string) (davsyncretention.RunRecord, error)
	}
	calDAVSyncRetention  calDAVSyncRetentionRunner
	cardDAVSyncRetention cardDAVSyncRetentionRunner
	attachmentCleanup    interface {
		ExpireStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error)
		CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error)
		ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error)
		ExpireAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error)
		CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error)
		ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error)
	}
	mailFlowStats  mailflow.MailFlowStatsProvider
	idpConfigRepo  *idprovider.ConfigRepository
	configStore    interface {
		Get(ctx context.Context, scopeType configstore.ScopeType, scopeID, key string) (*configstore.ConfigEntry, error)
		Set(ctx context.Context, scopeType configstore.ScopeType, scopeID, key string, value json.RawMessage, locked bool, expectedVersion int64) (*configstore.ConfigEntry, error)
		Delete(ctx context.Context, scopeType configstore.ScopeType, scopeID, key string, expectedVersion int64) error
		List(ctx context.Context, scopeType configstore.ScopeType, scopeID string) ([]configstore.ConfigEntry, error)
		Propagate(ctx context.Context, companyID string, scope configstore.PropagateScope, key string, value json.RawMessage, locked bool) error
		PropagateFromParent(ctx context.Context, scopeType configstore.ScopeType, scopeID string, parentScopeType configstore.ScopeType, parentScopeID string) error
	}
}
