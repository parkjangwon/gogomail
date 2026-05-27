package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
)

const maxAttachmentCleanupAuditSample = 10

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

func (s adminService) GetDriveObjectCleanupFailure(ctx context.Context, id string) (drive.ObjectCleanupFailure, error) {
	if s.drive == nil {
		return drive.ObjectCleanupFailure{}, fmt.Errorf("drive service is not configured")
	}
	return s.drive.GetObjectCleanupFailure(ctx, id)
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
