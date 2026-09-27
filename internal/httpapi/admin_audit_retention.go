package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auditretention"
)

// auditRetentionPolicyResponse is the JSON body returned for a company's audit
// log retention policy. When the company keeps logs forever (no stored policy),
// RetentionDays is 0 and KeepForever is true.
type auditRetentionPolicyResponse struct {
	CompanyID     string `json:"company_id"`
	RetentionDays int    `json:"retention_days"`
	KeepForever   bool   `json:"keep_forever"`
	UpdatedBy     string `json:"updated_by,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type auditRetentionPolicyRequest struct {
	RetentionDays int `json:"retention_days"`
}

func registerAuditRetentionRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/audit-retention", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetAuditRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/audit-retention", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleSetAuditRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/audit-retention/runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAuditRetentionRuns(w, r, service)
	}))
}

func handleGetAuditRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	policy, err := service.GetAuditRetentionPolicy(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_retention_policy": auditRetentionPolicyView(id, policy)})
}

func handleSetAuditRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var body auditRetentionPolicyRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auditretention.ValidateRetentionDays(body.RetentionDays); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	policy, err := service.SetAuditRetentionPolicy(r.Context(), id, body.RetentionDays)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// On removal (keep forever) SetAuditRetentionPolicy returns a nil policy; the
	// view helper renders that as keep_forever.
	writeJSON(w, http.StatusOK, map[string]any{"audit_retention_policy": auditRetentionPolicyView(id, policy)})
}

func handleListAuditRetentionRuns(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "limit", "status") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	statusRaw := strings.TrimSpace(r.URL.Query().Get("status"))
	status := auditretention.RunStatus(statusRaw)
	if status != "" && status != auditretention.RunStatusCompleted && status != auditretention.RunStatusFailed {
		writeError(w, http.StatusBadRequest, "status must be completed or failed")
		return
	}
	runs, err := service.ListAuditRetentionRuns(r.Context(), auditretention.RunListRequest{
		CompanyID: id,
		Limit:     limit,
		Status:    status,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if runs == nil {
		runs = []auditretention.RunRecord{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_retention_runs": runs})
}

func auditRetentionPolicyView(companyID string, policy *auditretention.Policy) auditRetentionPolicyResponse {
	if policy == nil {
		return auditRetentionPolicyResponse{
			CompanyID:   companyID,
			KeepForever: true,
		}
	}
	view := auditRetentionPolicyResponse{
		CompanyID:     policy.CompanyID,
		RetentionDays: policy.RetentionDays,
		KeepForever:   false,
		UpdatedBy:     policy.UpdatedBy,
	}
	if !policy.UpdatedAt.IsZero() {
		view.UpdatedAt = policy.UpdatedAt.Format(time.RFC3339)
	}
	return view
}
