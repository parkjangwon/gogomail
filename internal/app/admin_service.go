package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
)

const maxBackpressureAuditReasonBytes = 512
const maxAttachmentCleanupAuditSample = 10

type backpressureStore interface {
	State(ctx context.Context) (backpressure.State, error)
	SetState(ctx context.Context, update backpressure.StateUpdate) (backpressure.State, error)
}

type auditWriter interface {
	Insert(ctx context.Context, log audit.Log) error
}

type adminService struct {
	*maildb.Repository
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
}

const apiUsageExportLocalEd25519Backend = "local-ed25519"
const apiUsageExportLocalHMACBackend = "local-hmac"

func (s adminService) GetBackpressure(ctx context.Context) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	return s.backpressure.State(ctx)
}

func (s adminService) UpdateBackpressure(ctx context.Context, req backpressure.StateUpdate) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	previous, err := s.backpressure.State(ctx)
	if err != nil {
		return backpressure.State{}, fmt.Errorf("read previous backpressure state: %w", err)
	}
	state, err := s.backpressure.SetState(ctx, req)
	if err != nil {
		return backpressure.State{}, err
	}
	if s.audit != nil {
		detail, err := backpressureAuditDetail(previous, state)
		if err != nil {
			return backpressure.State{}, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "backpressure.update",
			TargetType: "backpressure",
			Result:     "updated",
			Detail:     detail,
		}); err != nil {
			return backpressure.State{}, fmt.Errorf("record backpressure audit: %w", err)
		}
	}
	return state, nil
}

func backpressureAuditDetail(previous backpressure.State, current backpressure.State) (json.RawMessage, error) {
	detail := struct {
		Scope    string                 `json:"scope"`
		Previous backpressureAuditState `json:"previous"`
		Current  backpressureAuditState `json:"current"`
	}{
		Scope:    "smtp",
		Previous: backpressureAuditStateFromState(previous),
		Current:  backpressureAuditStateFromState(current),
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal backpressure audit detail: %w", err)
	}
	return raw, nil
}

type backpressureAuditState struct {
	Level     string     `json:"level"`
	Reason    string     `json:"reason,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func backpressureAuditStateFromState(state backpressure.State) backpressureAuditState {
	level := strings.TrimSpace(state.Level)
	if level == "" {
		level = "normal"
	}
	reason := strings.TrimSpace(state.Reason)
	reason = truncateBackpressureAuditString(reason, maxBackpressureAuditReasonBytes)
	var updatedAt *time.Time
	if !state.UpdatedAt.IsZero() {
		normalized := state.UpdatedAt.UTC()
		updatedAt = &normalized
	}
	return backpressureAuditState{
		Level:     level,
		Reason:    reason,
		Until:     state.Until,
		UpdatedAt: updatedAt,
	}
}

func truncateBackpressureAuditString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func (s adminService) RunAttachmentCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	if s.attachmentCleanup == nil {
		return nil, fmt.Errorf("attachment cleanup service is not configured")
	}
	expired, err := s.attachmentCleanup.ExpireStaleAttachmentUploads(ctx, before, limit)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		detail, err := attachmentCleanupAuditDetail("uploads", before, limit, attachmentAuditIDs(expired))
		if err != nil {
			return nil, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "attachment_cleanup.uploads_run",
			TargetType: "attachment_cleanup",
			Result:     "completed",
			Detail:     detail,
		}); err != nil {
			return nil, fmt.Errorf("record attachment cleanup audit: %w", err)
		}
	}
	return expired, nil
}

func (s adminService) CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error) {
	if s.attachmentCleanup == nil {
		return maildb.StaleAttachmentUploadCount{}, fmt.Errorf("attachment cleanup service is not configured")
	}
	return s.attachmentCleanup.CountStaleAttachmentUploads(ctx, before, limit)
}

func (s adminService) ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error) {
	if s.attachmentCleanup == nil {
		return nil, fmt.Errorf("attachment cleanup service is not configured")
	}
	return s.attachmentCleanup.ListStaleAttachmentUploads(ctx, before, limit)
}

func (s adminService) RunAttachmentUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error) {
	if s.attachmentCleanup == nil {
		return nil, fmt.Errorf("attachment cleanup service is not configured")
	}
	expired, err := s.attachmentCleanup.ExpireAttachmentUploadSessions(ctx, before, limit)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		detail, err := attachmentCleanupAuditDetail("upload_sessions", before, limit, attachmentSessionAuditIDs(expired))
		if err != nil {
			return nil, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "attachment_cleanup.sessions_run",
			TargetType: "attachment_cleanup",
			Result:     "completed",
			Detail:     detail,
		}); err != nil {
			return nil, fmt.Errorf("record attachment session cleanup audit: %w", err)
		}
	}
	return expired, nil
}

func attachmentCleanupAuditDetail(scope string, before time.Time, limit int, expiredIDs []string) (json.RawMessage, error) {
	normalizedBefore := before.UTC()
	detail := struct {
		Scope        string   `json:"scope"`
		Before       string   `json:"before"`
		Limit        int      `json:"limit"`
		ExpiredCount int      `json:"expired_count"`
		ExpiredIDs   []string `json:"expired_ids_sample"`
	}{
		Scope:        scope,
		Before:       normalizedBefore.Format(time.RFC3339),
		Limit:        maildb.NormalizeAttachmentCleanupLimit(limit),
		ExpiredCount: len(expiredIDs),
		ExpiredIDs:   sampleAttachmentCleanupIDs(expiredIDs),
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal attachment cleanup audit detail: %w", err)
	}
	return raw, nil
}

func attachmentAuditIDs(attachments []maildb.Attachment) []string {
	ids := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		if id := strings.TrimSpace(attachment.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func attachmentSessionAuditIDs(sessions []maildb.AttachmentUploadSession) []string {
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if id := strings.TrimSpace(session.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func sampleAttachmentCleanupIDs(ids []string) []string {
	if len(ids) > maxAttachmentCleanupAuditSample {
		ids = ids[:maxAttachmentCleanupAuditSample]
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

func (s adminService) CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error) {
	if s.attachmentCleanup == nil {
		return maildb.StaleAttachmentUploadSessionCount{}, fmt.Errorf("attachment cleanup service is not configured")
	}
	return s.attachmentCleanup.CountStaleAttachmentUploadSessions(ctx, before, limit)
}

func (s adminService) ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	if s.attachmentCleanup == nil {
		return nil, fmt.Errorf("attachment cleanup service is not configured")
	}
	return s.attachmentCleanup.ListStaleAttachmentUploadSessions(ctx, before, limit)
}

func (s adminService) ListDriveUploadSessions(ctx context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error) {
	if s.drive == nil {
		return nil, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateListUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	return s.drive.ListUploadSessions(ctx, req)
}

func (s adminService) ListDirectoryDelegations(ctx context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListDelegations(ctx, req)
}

func (s adminService) CreateDirectoryDelegation(ctx context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateDelegationWithAudit(ctx, req)
}

func (s adminService) CreateDirectoryGroupMembership(ctx context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateGroupMembershipWithAudit(ctx, req)
}

func (s adminService) ListDirectoryGroupMemberships(ctx context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListGroupMemberships(ctx, req)
}

func (s adminService) DeleteDirectoryDelegation(ctx context.Context, id string) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteDelegationWithAudit(ctx, id)
}

func (s adminService) DeleteDirectoryGroupMembership(ctx context.Context, id string) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteGroupMembershipWithAudit(ctx, id)
}

func (s adminService) UpdateDirectoryDelegationRole(ctx context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.UpdateDelegationRoleWithAudit(ctx, req)
}

func (s adminService) ReassignDirectoryDelegation(ctx context.Context, req directory.ReassignDelegationRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ReassignDelegationWithAudit(ctx, req)
}

func (s adminService) UpdateDirectoryGroupMembershipRole(ctx context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.UpdateGroupMembershipRoleWithAudit(ctx, req)
}

func (s adminService) ReassignDirectoryGroupMembership(ctx context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ReassignGroupMembershipWithAudit(ctx, req)
}

func (s adminService) SearchDirectoryPrincipals(ctx context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.SearchPrincipals(ctx, req)
}

func (s adminService) ResolveDirectoryAlias(ctx context.Context, req directory.ResolveAliasRequest) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ResolveAlias(ctx, req)
}

func (s adminService) CreateDirectoryAlias(ctx context.Context, req directory.CreateAliasRequest) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateAliasWithAudit(ctx, req)
}

func (s adminService) DeleteDirectoryAlias(ctx context.Context, id string) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteAliasWithAudit(ctx, id)
}

func (s adminService) ListDirectoryAliases(ctx context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListAliases(ctx, req)
}

func (s adminService) ListDriveNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	if s.drive == nil {
		return nil, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateListNodesRequest(req)
	if err != nil {
		return nil, err
	}
	return s.drive.ListNodes(ctx, req)
}

func (s adminService) GetDriveNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	if s.drive == nil {
		return drive.Node{}, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateGetNodeRequest(req)
	if err != nil {
		return drive.Node{}, err
	}
	return s.drive.GetNode(ctx, req)
}

func (s adminService) GetDriveUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	if s.drive == nil {
		return drive.UsageSummary{}, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateGetUsageSummaryRequest(req)
	if err != nil {
		return drive.UsageSummary{}, err
	}
	return s.drive.GetUsageSummary(ctx, req)
}

func (s adminService) CountStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) (drive.StaleUploadSessionCount, error) {
	if s.drive == nil {
		return drive.StaleUploadSessionCount{}, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateExpireUploadSessionsRequest(drive.ExpireUploadSessionsRequest{Before: before, Limit: limit})
	if err != nil {
		return drive.StaleUploadSessionCount{}, err
	}
	return s.drive.CountStaleUploadSessions(ctx, req)
}

func (s adminService) ListStaleDriveUploadSessions(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error) {
	if s.drive == nil {
		return nil, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateExpireUploadSessionsRequest(drive.ExpireUploadSessionsRequest{Before: before, Limit: limit})
	if err != nil {
		return nil, err
	}
	return s.drive.ListStaleUploadSessions(ctx, req)
}

func (s adminService) RunDriveUploadSessionCleanup(ctx context.Context, before time.Time, limit int) ([]drive.UploadSession, error) {
	if s.drive == nil {
		return nil, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateExpireUploadSessionsRequest(drive.ExpireUploadSessionsRequest{Before: before, Limit: limit})
	if err != nil {
		return nil, err
	}
	expired, err := s.drive.ExpireUploadSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		detail, err := attachmentCleanupAuditDetail("drive_upload_sessions", before, limit, driveSessionAuditIDs(expired))
		if err != nil {
			return nil, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "drive_upload_cleanup.sessions_run",
			TargetType: "drive_upload_cleanup",
			Result:     "completed",
			Detail:     detail,
		}); err != nil {
			return nil, fmt.Errorf("record drive upload cleanup audit: %w", err)
		}
	}
	return expired, nil
}

func (s adminService) ListDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error) {
	if s.drive == nil {
		return nil, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateListObjectCleanupFailuresRequest(req)
	if err != nil {
		return nil, err
	}
	return s.drive.ListObjectCleanupFailures(ctx, req)
}

func (s adminService) ResolveDriveObjectCleanupFailure(ctx context.Context, id string) (drive.ObjectCleanupFailure, error) {
	if s.drive == nil {
		return drive.ObjectCleanupFailure{}, fmt.Errorf("drive service is not configured")
	}
	req, err := drive.ValidateResolveObjectCleanupFailureRequest(drive.ResolveObjectCleanupFailureRequest{ID: id})
	if err != nil {
		return drive.ObjectCleanupFailure{}, err
	}
	resolved, err := s.drive.ResolveObjectCleanupFailure(ctx, req)
	if err != nil {
		return drive.ObjectCleanupFailure{}, err
	}
	if s.audit != nil {
		detail, err := driveCleanupFailureAuditDetail(resolved)
		if err != nil {
			return drive.ObjectCleanupFailure{}, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "drive_cleanup_failure.resolve",
			TargetType: "drive_cleanup_failure",
			TargetID:   resolved.ID,
			Result:     "resolved",
			Detail:     detail,
		}); err != nil {
			return drive.ObjectCleanupFailure{}, fmt.Errorf("record drive cleanup failure audit: %w", err)
		}
	}
	return resolved, nil
}

func (s adminService) RetryDriveObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error) {
	if s.drive == nil {
		return drive.RetryObjectCleanupFailuresResult{}, fmt.Errorf("drive service is not configured")
	}
	req.Status = drive.ObjectCleanupFailureStatusPending
	req, err := drive.ValidateListObjectCleanupFailuresRequest(req)
	if err != nil {
		return drive.RetryObjectCleanupFailuresResult{}, err
	}
	result, err := s.drive.RetryObjectCleanupFailures(ctx, req)
	if err != nil && !driveCleanupRetryHasProgress(result) {
		return drive.RetryObjectCleanupFailuresResult{}, err
	}
	if s.audit != nil {
		detail, detailErr := driveCleanupRetryAuditDetail(req, result)
		if detailErr != nil {
			return drive.RetryObjectCleanupFailuresResult{}, detailErr
		}
		auditResult := "completed"
		if result.Failed > 0 {
			auditResult = "partial"
		}
		if insertErr := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "drive_cleanup_failure.retry_run",
			TargetType: "drive_cleanup_failure",
			Result:     auditResult,
			Detail:     detail,
		}); insertErr != nil {
			return drive.RetryObjectCleanupFailuresResult{}, fmt.Errorf("record drive cleanup failure retry audit: %w", insertErr)
		}
	}
	return result, nil
}

func driveCleanupRetryHasProgress(result drive.RetryObjectCleanupFailuresResult) bool {
	return result.Scanned > 0 || result.Deleted > 0 || result.Resolved > 0 || result.Failed > 0
}

func driveCleanupRetryAuditDetail(req drive.ListObjectCleanupFailuresRequest, result drive.RetryObjectCleanupFailuresResult) (json.RawMessage, error) {
	detail := struct {
		UserID   string `json:"user_id,omitempty"`
		Limit    int    `json:"limit"`
		Scanned  int    `json:"scanned"`
		Deleted  int    `json:"deleted"`
		Resolved int    `json:"resolved"`
		Failed   int    `json:"failed"`
	}{
		UserID:   req.UserID,
		Limit:    req.Limit,
		Scanned:  result.Scanned,
		Deleted:  result.Deleted,
		Resolved: result.Resolved,
		Failed:   result.Failed,
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal drive cleanup retry audit detail: %w", err)
	}
	return raw, nil
}

func driveCleanupFailureAuditDetail(failure drive.ObjectCleanupFailure) (json.RawMessage, error) {
	detail := struct {
		ID             string `json:"id"`
		UserID         string `json:"user_id"`
		NodeID         string `json:"node_id,omitempty"`
		StorageBackend string `json:"storage_backend"`
		StoragePath    string `json:"storage_path"`
		Attempts       int    `json:"attempts"`
	}{
		ID:             failure.ID,
		UserID:         failure.UserID,
		NodeID:         failure.NodeID,
		StorageBackend: failure.StorageBackend,
		StoragePath:    failure.StoragePath,
		Attempts:       failure.Attempts,
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal drive cleanup failure audit detail: %w", err)
	}
	return raw, nil
}

func driveSessionAuditIDs(sessions []drive.UploadSession) []string {
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if strings.TrimSpace(session.ID) != "" {
			ids = append(ids, session.ID)
		}
	}
	return ids
}

func (s adminService) GetAPIUsageExportCapabilities(context.Context) (maildb.APIUsageExportCapabilityView, error) {
	backend := strings.TrimSpace(s.exportManifestSignerBackend)
	if backend == "" {
		backend = "disabled"
	}
	productionReady := apiUsageExportManifestSignerProductionReady(backend)
	view := maildb.APIUsageExportCapabilityView{
		ExportFormat:                                "ndjson",
		ArtifactContentType:                         apimeter.ExportArtifactContentTypeNDJSON,
		ManifestDigestAlgorithm:                     "sha256",
		SignerBackend:                               backend,
		SignerConfigured:                            s.exportManifestSigner != nil,
		VerifierConfigured:                          s.exportManifestVerifier != nil,
		ProductionSignatureReady:                    s.exportManifestSigner != nil && s.exportManifestVerifier != nil && productionReady,
		BillingReadySupported:                       s.exportManifestSigner != nil && productionReady,
		VerifiedBillingReadySupported:               s.exportManifestSigner != nil && s.exportManifestVerifier != nil && productionReady,
		RetentionRunsSupported:                      true,
		RetentionWorkerSupported:                    true,
		RetentionWorkerDestructiveRequiresRemoteKey: true,
	}
	if keyID, ok := exportManifestSignerKeyID(s.exportManifestSigner); ok {
		view.SignerKeyID = keyID
	}
	var blocking []string
	if s.exportManifestSigner == nil {
		blocking = append(blocking, "manifest_signer_not_configured")
	}
	if s.exportManifestVerifier == nil {
		blocking = append(blocking, "manifest_signature_verifier_not_configured")
	}
	if apiUsageExportManifestSignerLocalOnly(backend) {
		blocking = append(blocking, "production_manifest_signer_required")
	}
	view.BlockingReasons = uniqueStrings(blocking)
	return view, nil
}

func (s adminService) ListDAVSyncRetentionRuns(ctx context.Context, req davsyncretention.RunListRequest) ([]davsyncretention.RunRecord, error) {
	if s.davSyncRetention == nil {
		return nil, fmt.Errorf("DAV sync retention repository is not configured")
	}
	return s.davSyncRetention.ListRuns(ctx, req)
}

func (s adminService) GetDAVSyncRetentionRun(ctx context.Context, id string) (davsyncretention.RunRecord, error) {
	if s.davSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("DAV sync retention repository is not configured")
	}
	return s.davSyncRetention.GetRun(ctx, id)
}

func (s adminService) GetDAVSyncRetentionReadiness(ctx context.Context, req davsyncretention.ReadinessRequest) (davsyncretention.ReadinessView, error) {
	req, err := davsyncretention.NormalizeReadinessRequest(req, time.Now)
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	if s.calDAVSyncRetention == nil {
		return davsyncretention.ReadinessView{}, fmt.Errorf("CalDAV sync retention repository is not configured")
	}
	if s.cardDAVSyncRetention == nil {
		return davsyncretention.ReadinessView{}, fmt.Errorf("CardDAV sync retention repository is not configured")
	}
	calResult, err := s.calDAVSyncRetention.PruneCalendarSyncChanges(ctx, caldavgw.PruneCalendarSyncChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: true,
	})
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	cardResult, err := s.cardDAVSyncRetention.PruneAddressBookChanges(ctx, carddavgw.PruneAddressBookChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: true,
	})
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	candidateCount := calResult.CandidateCount + cardResult.CandidateCount
	truncated := calResult.CandidateCount >= int64(req.Limit) || cardResult.CandidateCount >= int64(req.Limit)
	return davsyncretention.ReadinessView{
		Cutoff:             req.Cutoff,
		Limit:              req.Limit,
		Ready:              !truncated,
		Truncated:          truncated,
		CandidateCount:     candidateCount,
		CalDAVCandidates:   calResult.CandidateCount,
		CardDAVCandidates:  cardResult.CandidateCount,
		DestructiveGuarded: true,
	}, nil
}

func (s adminService) RunDAVSyncRetention(ctx context.Context, req davsyncretention.RunRequest) (davsyncretention.RunRecord, error) {
	req, err := davsyncretention.NormalizeRunRequest(req, time.Now)
	if err != nil {
		return davsyncretention.RunRecord{}, err
	}
	if s.davSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("DAV sync retention repository is not configured")
	}
	if s.calDAVSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("CalDAV sync retention repository is not configured")
	}
	if s.cardDAVSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("CardDAV sync retention repository is not configured")
	}
	if !req.DryRun {
		readiness, err := s.GetDAVSyncRetentionReadiness(ctx, davsyncretention.ReadinessRequest{
			Cutoff: req.Cutoff,
			Limit:  req.Limit,
		})
		if err != nil {
			return davsyncretention.RunRecord{}, err
		}
		if !readiness.Ready {
			record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
				Cutoff:            req.Cutoff,
				Limit:             req.Limit,
				DryRun:            req.DryRun,
				ConfirmReady:      req.ConfirmReady,
				Status:            davsyncretention.RunStatusFailed,
				ErrorMessage:      "DAV sync retention readiness is truncated",
				CalDAVCandidates:  readiness.CalDAVCandidates,
				CardDAVCandidates: readiness.CardDAVCandidates,
			})
			return record, errors.Join(fmt.Errorf("DAV sync retention readiness is truncated"), recordErr)
		}
	}
	calResult, err := s.calDAVSyncRetention.PruneCalendarSyncChanges(ctx, caldavgw.PruneCalendarSyncChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: req.DryRun,
	})
	if err != nil {
		record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
			Cutoff:       req.Cutoff,
			Limit:        req.Limit,
			DryRun:       req.DryRun,
			ConfirmReady: req.ConfirmReady,
			Status:       davsyncretention.RunStatusFailed,
			ErrorMessage: err.Error(),
		})
		return record, errors.Join(err, recordErr)
	}
	cardResult, err := s.cardDAVSyncRetention.PruneAddressBookChanges(ctx, carddavgw.PruneAddressBookChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: req.DryRun,
	})
	if err != nil {
		record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
			Cutoff:            req.Cutoff,
			Limit:             req.Limit,
			DryRun:            req.DryRun,
			ConfirmReady:      req.ConfirmReady,
			Status:            davsyncretention.RunStatusFailed,
			ErrorMessage:      err.Error(),
			CalDAVCandidates:  calResult.CandidateCount,
			CalDAVDeleted:     calResult.DeletedCount,
			CardDAVCandidates: cardResult.CandidateCount,
			CardDAVDeleted:    cardResult.DeletedCount,
		})
		return record, errors.Join(err, recordErr)
	}
	return s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
		Cutoff:            req.Cutoff,
		Limit:             req.Limit,
		DryRun:            req.DryRun,
		ConfirmReady:      req.ConfirmReady,
		Status:            davsyncretention.RunStatusCompleted,
		CalDAVCandidates:  calResult.CandidateCount,
		CalDAVDeleted:     calResult.DeletedCount,
		CardDAVCandidates: cardResult.CandidateCount,
		CardDAVDeleted:    cardResult.DeletedCount,
	})
}

func exportManifestSignerKeyID(signer apimeter.ExportManifestSigner) (string, bool) {
	switch signer := signer.(type) {
	case apimeter.HMACExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.HMACExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case apimeter.Ed25519ExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.Ed25519ExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case apimeter.RemoteEd25519ExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.RemoteEd25519ExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	default:
		return "", false
	}
}

func apiUsageExportManifestSignerProductionReady(backend string) bool {
	backend = strings.ToLower(strings.TrimSpace(backend))
	return backend != "" && backend != "disabled" && !apiUsageExportManifestSignerLocalOnly(backend)
}

func apiUsageExportManifestSignerLocalOnly(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case apiUsageExportLocalHMACBackend, apiUsageExportLocalEd25519Backend:
		return true
	default:
		return false
	}
}

func (s adminService) GetAPIUsageExportHandoff(ctx context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportHandoffView{}, fmt.Errorf("repository is required")
	}
	handoff, err := s.Repository.GetAPIUsageExportHandoff(ctx, batchID)
	if err != nil {
		return maildb.APIUsageExportHandoffView{}, err
	}
	if !deep {
		return handoff, nil
	}
	s.applyAPIUsageExportDeepHandoff(ctx, &handoff)
	return handoff, nil
}

func (s adminService) applyAPIUsageExportDeepHandoff(ctx context.Context, handoff *maildb.APIUsageExportHandoffView) {
	handoff.DeepVerification = true
	var blocking []string

	artifacts, err := s.Repository.ListAllAPIUsageExportArtifacts(ctx, handoff.BatchID)
	if err != nil {
		handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("list artifacts: %v", err))
		blocking = append(blocking, "artifact_verification_error")
	} else {
		for _, artifact := range artifacts {
			verification, err := s.VerifyAPIUsageExportArtifact(ctx, handoff.BatchID, artifact.ID)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify artifact %s: %v", artifact.ID, err))
				blocking = append(blocking, "artifact_verification_error")
				continue
			}
			handoff.ArtifactVerifications = append(handoff.ArtifactVerifications, verification)
			if !verification.Valid {
				blocking = append(blocking, "artifact_verification_failed")
			}
		}
	}

	if handoff.LatestManifestDigestID != "" {
		verification, err := s.Repository.VerifyAPIUsageExportManifestDigest(ctx, handoff.BatchID, handoff.LatestManifestDigestID)
		if err != nil {
			handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest digest %s: %v", handoff.LatestManifestDigestID, err))
			blocking = append(blocking, "manifest_digest_verification_error")
		} else {
			handoff.ManifestDigestVerification = &verification
			if !verification.Valid {
				blocking = append(blocking, "manifest_digest_verification_failed")
			}
			coverageValid, err := apiUsageExportManifestCoversArtifacts(verification.CanonicalManifest, artifacts)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest artifact coverage: %v", err))
				blocking = append(blocking, "manifest_artifact_coverage_error")
			} else {
				handoff.ManifestArtifactCoverageValid = &coverageValid
				if !coverageValid {
					blocking = append(blocking, "manifest_artifact_mismatch")
				}
			}
		}
	}

	if handoff.LatestManifestDigestID != "" && handoff.LatestSignatureID != "" {
		if s.exportManifestVerifier == nil {
			blocking = append(blocking, "manifest_signature_verifier_unavailable")
		} else {
			verification, err := s.VerifyAPIUsageExportManifestSignature(ctx, handoff.BatchID, handoff.LatestManifestDigestID, handoff.LatestSignatureID)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest signature %s: %v", handoff.LatestSignatureID, err))
				blocking = append(blocking, "manifest_signature_verification_error")
			} else {
				handoff.ManifestSignatureVerification = &verification
				if !verification.Valid {
					blocking = append(blocking, "manifest_signature_verification_failed")
				}
			}
		}
	}

	handoff.DeepBlockingReasons = uniqueStrings(blocking)
	handoff.DeepReady = handoff.Ready && len(handoff.DeepBlockingReasons) == 0
	handoff.VerifiedBillingReady = handoff.BillingReady && handoff.DeepReady
}

func apiUsageExportManifestCoversArtifacts(raw []byte, artifacts []maildb.APIUsageExportArtifactView) (bool, error) {
	var manifest apimeter.ExportManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return false, fmt.Errorf("unmarshal manifest: %w", err)
	}
	current := make([]apimeter.ExportManifestArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		current = append(current, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	sort.Slice(current, func(i, j int) bool { return current[i].ID < current[j].ID })
	manifestArtifacts := append([]apimeter.ExportManifestArtifact(nil), manifest.Artifacts...)
	sort.Slice(manifestArtifacts, func(i, j int) bool { return manifestArtifacts[i].ID < manifestArtifacts[j].ID })
	if len(current) != len(manifestArtifacts) {
		return false, nil
	}
	for i := range current {
		if current[i] != manifestArtifacts[i] {
			return false, nil
		}
	}
	return true, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func (s adminService) WriteAPIUsageExportArtifact(ctx context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("repository is required")
	}
	if s.exportStore == nil {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export artifact store is not configured")
	}
	batch, err := s.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, err
	}
	objectKey := strings.TrimSpace(req.ObjectKey)
	if objectKey == "" {
		objectKey, err = apimeter.DefaultExportArtifactObjectKey(batch.ID)
		if err != nil {
			return maildb.APIUsageExportArtifactView{}, err
		}
	}
	metadata := req.Metadata
	if len(metadata) == 0 {
		metadata, err = json.Marshal(map[string]string{
			"batch_id": batch.ID,
			"writer":   "gogomail-admin-api",
		})
		if err != nil {
			return maildb.APIUsageExportArtifactView{}, fmt.Errorf("marshal export artifact metadata: %w", err)
		}
	}

	ledgerReq := apiUsageLedgerRequestFromBatch(batch, maildb.APIUsageLedgerNoLimit)
	var eventCount int64
	result, err := apimeter.WriteExportArtifact(ctx, s.exportStore, apimeter.ExportArtifactWriteRequest{
		ObjectKey: objectKey,
		Metadata:  metadata,
		Encode: func(w io.Writer) error {
			return s.StreamAPIUsageLedger(ctx, ledgerReq, func(usage maildb.APIUsageLedgerView) error {
				eventCount++
				return json.NewEncoder(w).Encode(usage)
			})
		},
	})
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, err
	}
	if batch.EventCount != eventCount {
		s.cleanupAPIUsageExportArtifactObject(ctx, result.ObjectKey)
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export batch expected %d events but wrote %d", batch.EventCount, eventCount)
	}
	storageBackend := strings.TrimSpace(req.StorageBackend)
	if storageBackend == "" {
		storageBackend = "local"
	}
	artifact, err := s.CreateAPIUsageExportArtifact(ctx, maildb.CreateAPIUsageExportArtifactRequest{
		BatchID:        batch.ID,
		StorageBackend: storageBackend,
		ObjectKey:      result.ObjectKey,
		ContentType:    result.ContentType,
		ByteCount:      result.ByteCount,
		SHA256Hex:      result.SHA256Hex,
		EventCount:     eventCount,
		Metadata:       result.Metadata,
	})
	if err != nil {
		s.cleanupAPIUsageExportArtifactObject(ctx, result.ObjectKey)
		return maildb.APIUsageExportArtifactView{}, err
	}
	return artifact, nil
}

func (s adminService) cleanupAPIUsageExportArtifactObject(ctx context.Context, objectKey string) {
	if deleter, ok := s.exportStore.(interface {
		Delete(context.Context, string) error
	}); ok {
		_ = deleter.Delete(ctx, objectKey)
	}
}

func (s adminService) OpenAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportArtifactView{}, nil, fmt.Errorf("repository is required")
	}
	getter, ok := s.exportStore.(interface {
		Get(context.Context, string) (io.ReadCloser, error)
	})
	if !ok {
		return maildb.APIUsageExportArtifactView{}, nil, fmt.Errorf("api usage export artifact store does not support reads")
	}
	artifact, err := s.GetAPIUsageExportArtifact(ctx, batchID, artifactID)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, nil, err
	}
	body, err := getter.Get(ctx, artifact.ObjectKey)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, nil, err
	}
	return artifact, body, nil
}

func (s adminService) VerifyAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error) {
	artifact, body, err := s.OpenAPIUsageExportArtifact(ctx, batchID, artifactID)
	if err != nil {
		return maildb.APIUsageExportArtifactVerificationView{}, err
	}
	defer body.Close()

	hash := sha256.New()
	byteCount, err := io.Copy(hash, body)
	if err != nil {
		return maildb.APIUsageExportArtifactVerificationView{}, fmt.Errorf("read api usage export artifact: %w", err)
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	return maildb.APIUsageExportArtifactVerificationView{
		BatchID:           artifact.BatchID,
		ArtifactID:        artifact.ID,
		ObjectKey:         artifact.ObjectKey,
		ExpectedByteCount: artifact.ByteCount,
		ActualByteCount:   byteCount,
		ExpectedSHA256Hex: artifact.SHA256Hex,
		ActualSHA256Hex:   actual,
		Valid:             artifact.ByteCount == byteCount && artifact.SHA256Hex == actual,
	}, nil
}

func (s adminService) CreateAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportManifestSignatureView{}, fmt.Errorf("repository is required")
	}
	if s.exportManifestSigner == nil {
		return maildb.APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signer is not configured")
	}
	digest, err := s.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureView{}, err
	}
	signature, err := s.exportManifestSigner.SignExportManifestDigest(digest.DigestHex)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureView{}, err
	}
	backend := strings.TrimSpace(s.exportManifestSignerBackend)
	if backend == "" {
		backend = apiUsageExportLocalHMACBackend
	}
	return s.Repository.CreateAPIUsageExportManifestSignature(ctx, maildb.CreateAPIUsageExportManifestSignatureRequest{
		BatchID:       batchID,
		DigestID:      digestID,
		SignerBackend: backend,
		Signature:     signature,
	})
}

func (s adminService) VerifyAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error) {
	if s.exportManifestVerifier == nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, fmt.Errorf("api usage export manifest signature verifier is not configured")
	}
	digest, err := s.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	signature, err := s.GetAPIUsageExportManifestSignature(ctx, batchID, digestID, signatureID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	valid, err := s.exportManifestVerifier.VerifyExportManifestSignature(apimeter.ExportManifestSignature{
		Algorithm:       signature.SignatureAlgorithm,
		KeyID:           signature.KeyID,
		SignedDigestHex: signature.SignedDigestHex,
		SignatureHex:    signature.SignatureHex,
	})
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	valid = valid && signature.SignedDigestHex == digest.DigestHex
	return maildb.APIUsageExportManifestSignatureVerificationView{
		BatchID:            signature.BatchID,
		DigestID:           signature.DigestID,
		SignatureID:        signature.ID,
		SignerBackend:      signature.SignerBackend,
		KeyID:              signature.KeyID,
		SignatureAlgorithm: signature.SignatureAlgorithm,
		SignedDigestHex:    signature.SignedDigestHex,
		ExpectedDigestHex:  digest.DigestHex,
		Valid:              valid,
	}, nil
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
