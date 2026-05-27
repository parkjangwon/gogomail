package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerStorageRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
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
		if strings.TrimSpace(domainID) != "" {
			domain, err := service.GetDomain(r.Context(), domainID)
			if err != nil {
				writeError(w, http.StatusNotFound, "domain not found")
				return
			}
			if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		sessionCounts, err := service.CountStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		candidates, err := service.ListStaleAttachmentUploads(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		sessionCandidates, err := service.ListStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		if strings.TrimSpace(userID) != "" {
			targetUser, err := service.GetUser(r.Context(), userID)
			if err != nil {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to resolve domain")
				return
			}
			if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		targetUser, err := service.GetUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
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
		req, err = drive.ValidateListUploadSessionsRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sessions, err := service.ListDriveUploadSessions(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		targetUser, err := service.GetUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
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
		req, err = drive.ValidateListNodesRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		nodes, err := service.ListDriveNodes(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		targetUser, err := service.GetUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
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
		req, err = drive.ValidateGetNodeRequest(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		node, err := service.GetDriveNode(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		targetUser, err := service.GetUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req, err := drive.ValidateGetUsageSummaryRequest(drive.GetUsageSummaryRequest{UserID: userID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		summary, err := service.GetDriveUsageSummary(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		sessionCandidates, err := service.ListStaleDriveUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		expired, err := service.RunDriveUploadSessionCleanup(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		if strings.TrimSpace(userID) != "" {
			targetUser, err := service.GetUser(r.Context(), userID)
			if err != nil {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to resolve domain")
				return
			}
			if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
		existing, err := service.GetDriveObjectCleanupFailure(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "cleanup failure not found")
			return
		}
		targetUser, err := service.GetUser(r.Context(), existing.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve user")
			return
		}
		targetDomain, err := service.GetDomain(r.Context(), targetUser.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), targetDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		sessionCounts, err := service.CountStaleAttachmentUploadSessions(r.Context(), before, req.Limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		expiredCount := 0
		expiredSessionCount := 0
		if !req.DryRun {
			expired, err := service.RunAttachmentCleanup(r.Context(), before, req.Limit)
			if err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			expiredCount = len(expired)
			expiredSessions, err := service.RunAttachmentUploadSessionCleanup(r.Context(), before, req.Limit)
			if err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
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
}
