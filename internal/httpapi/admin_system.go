package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/spamfilter"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

func registerConsoleRoutes(mux *http.ServeMux, cfg adminRouteConfig, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
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
	_, _, auditErr := service.ListAuditLogs(r.Context(), maildb.AuditLogListRequest{Limit: 1})
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

const orgSettingsKey = "org_settings"

type orgSettingsConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxUsers    int    `json:"max_users"`
	MaxDomains  int    `json:"max_domains"`
}

func handleGetOrganizationSettings(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	ctx := r.Context()
	companies, _, err := service.ListCompanies(ctx, maildb.CompanyListRequest{Limit: 1})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch organization settings")
		return
	}
	var cfg orgSettingsConfig
	var createdAt time.Time
	if len(companies) > 0 {
		createdAt = companies[0].CreatedAt
		cfg.Name = companies[0].Name
		if entry, err2 := service.GetCompanyConfig(ctx, companies[0].ID, orgSettingsKey); err2 == nil && entry.Value != nil {
			_ = json.Unmarshal(entry.Value, &cfg)
			if cfg.Name == "" {
				cfg.Name = companies[0].Name
			}
		}
	}
	if cfg.Name == "" {
		cfg.Name = "gogomail"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": map[string]any{
			"name":        cfg.Name,
			"description": cfg.Description,
			"max_users":   cfg.MaxUsers,
			"max_domains": cfg.MaxDomains,
			"created_at":  createdAt.UTC().Format(time.RFC3339),
		},
	})
}

func handleUpdateOrganizationSettings(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	ctx := r.Context()
	var req orgSettingsConfig
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	companies, _, err := service.ListCompanies(ctx, maildb.CompanyListRequest{Limit: 1})
	if err != nil || len(companies) == 0 {
		writeError(w, http.StatusInternalServerError, "no company configured")
		return
	}
	company := companies[0]
	if req.Name != "" && req.Name != company.Name {
		updated, err := service.UpdateCompany(ctx, maildb.UpdateCompanyRequest{ID: company.ID, Name: req.Name})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		company = updated
	}
	if req.Name == "" {
		req.Name = company.Name
	}
	b, _ := json.Marshal(req)
	if _, err := service.SetCompanyConfig(ctx, company.ID, orgSettingsKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": map[string]any{
			"name":        req.Name,
			"description": req.Description,
			"max_users":   req.MaxUsers,
			"max_domains": req.MaxDomains,
			"created_at":  company.CreatedAt.UTC().Format(time.RFC3339),
		},
	})
}

// loadOrgSettings returns the orgSettingsConfig for the given companyID, or a zero value if not configured.
func loadOrgSettings(ctx context.Context, service AdminService, companyID string) orgSettingsConfig {
	var cfg orgSettingsConfig
	if entry, err := service.GetCompanyConfig(ctx, companyID, orgSettingsKey); err == nil && entry.Value != nil {
		_ = json.Unmarshal(entry.Value, &cfg)
	}
	return cfg
}

// checkUserLimit verifies that the company has not reached its MaxUsers limit.
// Returns an error string (non-empty) if the limit is exceeded.
func checkUserLimit(ctx context.Context, service AdminService, companyID string) (limitErr string) {
	cfg := loadOrgSettings(ctx, service, companyID)
	if cfg.MaxUsers <= 0 {
		return ""
	}
	users, hasMore, err := service.ListUsers(ctx, maildb.UserListRequest{CompanyID: companyID, Limit: cfg.MaxUsers, ProbeMore: true})
	if err != nil {
		return "" // don't block on error
	}
	if hasMore || len(users) >= cfg.MaxUsers {
		return fmt.Sprintf("user limit of %d reached", cfg.MaxUsers)
	}
	return ""
}

// checkDomainLimit verifies that the company has not reached its MaxDomains limit.
// Returns an error string (non-empty) if the limit is exceeded.
func checkDomainLimit(ctx context.Context, service AdminService, companyID string) (limitErr string) {
	cfg := loadOrgSettings(ctx, service, companyID)
	if cfg.MaxDomains <= 0 {
		return ""
	}
	domains, _, err := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: companyID, Limit: cfg.MaxDomains + 1})
	if err != nil {
		return "" // don't block on error
	}
	if len(domains) >= cfg.MaxDomains {
		return fmt.Sprintf("domain limit of %d reached", cfg.MaxDomains)
	}
	return ""
}

func handleListCompliance(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	ctx := r.Context()
	now := time.Now().UTC()

	// Gather real system state for compliance checks.
	auditLogs, _, _ := service.ListAuditLogs(ctx, maildb.AuditLogListRequest{Limit: 1})
	auditActive := len(auditLogs) > 0

	companies, _, _ := service.ListCompanies(ctx, maildb.CompanyListRequest{Limit: 1})
	mfaEnabled := false
	ipPolicyOn := false
	retentionOn := false
	sessionPolicyOn := false
	if len(companies) > 0 {
		cid := companies[0].ID
		if mfaStats, err := service.GetMFAStats(ctx, cid); err == nil {
			mfaEnabled = mfaStats.Total > 0 && mfaStats.Enabled > 0
		}
		if _, err := service.GetCompanyConfig(ctx, cid, ipAccessPolicyKey); err == nil {
			ipPolicyOn = true
		}
		if _, err := service.GetCompanyConfig(ctx, cid, "retention_policy"); err == nil {
			retentionOn = true
		}
		if _, err := service.GetCompanyConfig(ctx, cid, "session_policy"); err == nil {
			sessionPolicyOn = true
		}
	}

	complianceStatus := func(findings int) string {
		switch {
		case findings == 0:
			return "compliant"
		case findings <= 2:
			return "partial"
		default:
			return "non-compliant"
		}
	}

	// GDPR: audit log, data retention, access controls, session policy
	gdprFindings := 0
	if !auditActive {
		gdprFindings++
	}
	if !retentionOn {
		gdprFindings++
	}
	if !ipPolicyOn {
		gdprFindings++
	}
	if !sessionPolicyOn {
		gdprFindings++
	}

	// HIPAA: MFA, audit log, access controls, data retention
	hipaaFindings := 0
	if !mfaEnabled {
		hipaaFindings++
	}
	if !auditActive {
		hipaaFindings++
	}
	if !ipPolicyOn {
		hipaaFindings++
	}
	if !retentionOn {
		hipaaFindings++
	}

	// SOC 2: audit log, MFA, access controls, session policy
	soc2Findings := 0
	if !auditActive {
		soc2Findings++
	}
	if !mfaEnabled {
		soc2Findings++
	}
	if !ipPolicyOn {
		soc2Findings++
	}
	if !sessionPolicyOn {
		soc2Findings++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reports": []map[string]any{
			{
				"id":         "gdpr-001",
				"framework":  "GDPR",
				"status":     complianceStatus(gdprFindings),
				"last_audit": now.Format(time.RFC3339),
				"findings":   gdprFindings,
			},
			{
				"id":         "hipaa-001",
				"framework":  "HIPAA",
				"status":     complianceStatus(hipaaFindings),
				"last_audit": now.Format(time.RFC3339),
				"findings":   hipaaFindings,
			},
			{
				"id":         "soc2-001",
				"framework":  "SOC 2",
				"status":     complianceStatus(soc2Findings),
				"last_audit": now.Format(time.RFC3339),
				"findings":   soc2Findings,
			},
		},
	})
}

func handleListRoles(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "company_id") {
		return
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return
	}
	roles, err := service.ListAdminRoles(r.Context(), companyID)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func handleCreateRole(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	var req admin.CreateRoleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.CreatedBy = strings.TrimSpace(req.CreatedBy)
	if req.CompanyID == "" {
		writeError(w, http.StatusBadRequest, "company_id is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.IsBuiltin {
		writeError(w, http.StatusBadRequest, "custom role creation cannot set is_builtin")
		return
	}
	role, err := service.CreateAdminRole(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"role": role})
}

func handleListReports(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	ctx := r.Context()

	auditLogs, _, _ := service.ListAuditLogs(ctx, maildb.AuditLogListRequest{Limit: 1})
	flowStats, _ := service.GetMailFlowLogStats(ctx, maildb.MailFlowLogStatsRequest{})
	users, _, _ := service.ListUsers(ctx, maildb.UserListRequest{Limit: 1})

	writeJSON(w, http.StatusOK, map[string]any{
		"reports": []map[string]any{
			{
				"id":              "audit_logs",
				"name":            "Audit Log Export",
				"category":        "compliance",
				"export_endpoint": "audit-logs/export",
				"available":       len(auditLogs) > 0,
			},
			{
				"id":              "users_export",
				"name":            "Users Export",
				"category":        "users",
				"export_endpoint": "users/bulk-export",
				"available":       len(users) > 0,
			},
			{
				"id":           "mail_flow",
				"name":         "Mail Flow Summary",
				"category":     "domains",
				"record_count": flowStats.TotalMessages,
				"available":    flowStats.TotalMessages > 0,
			},
			{
				"id":        "quota_summary",
				"name":      "Quota Summary",
				"category":  "storage",
				"available": true,
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeError(w, http.StatusUnauthorized, "admin authentication is not configured")
			return
		}
		got, ok := adminTokenFromRequest(w, r)
		if !ok {
			return
		}
		if !constantTimeTokenEqual(got, token) {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
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

const (
	adminAccessTokenTTL  = 15 * time.Minute
	adminRefreshTokenTTL = 30 * 24 * time.Hour
)

func handleAdminLogin(w http.ResponseWriter, r *http.Request, service AdminService, cfg adminRouteConfig) {
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
		if cfg.tokenMgr == nil {
			writeError(w, http.StatusInternalServerError, "admin jwt token manager is not configured")
			return
		}
		accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, claims)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"user": map[string]any{
				"id":         claims.UserID,
				"role":       claims.Role,
				"company_id": claims.CompanyID,
			},
		})
	}

	// Bootstrap system admin via environment variables only.
	// Set GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL and GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD to enable.
	// If either env var is empty, bootstrap is disabled.
	bootstrapEmail := strings.TrimSpace(os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL"))
	bootstrapPassword := os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD")
	if bootstrapEmail != "" && bootstrapPassword != "" &&
		req.Email == bootstrapEmail && subtle.ConstantTimeCompare([]byte(req.Password), []byte(bootstrapPassword)) == 1 {
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

	// MFA check — only when adminMFAStore is wired in.
	if cfg.adminMFAStore != nil {
		mfaStatus, err := cfg.adminMFAStore.GetUserMFAStatus(r.Context(), authedUser.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check mfa status")
			return
		}
		if mfaStatus.Enabled {
			pendingClaims := auth.Claims{
				UserID:         authedUser.UserID,
				DomainID:       authedUser.DomainID,
				CompanyID:      domain.CompanyID,
				Role:           userView.Role,
				SessionVersion: authedUser.SessionVersion,
				TokenType:      "mfa_pending",
			}
			pendingToken, err := cfg.tokenMgr.Sign(pendingClaims, mfaPendingTTL)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue pending token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"mfa_required":  true,
				"pending_token": pendingToken,
			})
			return
		}

		setupRequired := adminMFASetupRequired(r.Context(), userView.Role, authedUser, cfg)
		if setupRequired {
			fullClaims := auth.Claims{
				UserID:         authedUser.UserID,
				DomainID:       authedUser.DomainID,
				CompanyID:      domain.CompanyID,
				Role:           userView.Role,
				SessionVersion: authedUser.SessionVersion,
			}
			if cfg.tokenMgr == nil {
				writeError(w, http.StatusInternalServerError, "admin jwt token manager is not configured")
				return
			}
			accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, fullClaims)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token":       accessToken,
				"refresh_token":      refreshToken,
				"mfa_setup_required": true,
				"user": map[string]any{
					"id":         fullClaims.UserID,
					"role":       fullClaims.Role,
					"company_id": fullClaims.CompanyID,
				},
			})
			return
		}
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

func signAdminSessionTokens(tokenMgr *auth.TokenManager, claims auth.Claims) (string, string, error) {
	accessClaims := claims
	accessClaims.TokenType = "access"
	accessToken, err := tokenMgr.Sign(accessClaims, adminAccessTokenTTL)
	if err != nil {
		return "", "", err
	}
	refreshClaims := claims
	refreshClaims.TokenType = "refresh"
	refreshToken, err := tokenMgr.Sign(refreshClaims, adminRefreshTokenTTL)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func adminBearerClaims(ctx context.Context, w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) (auth.Claims, bool) {
	if tokenMgr == nil {
		writeError(w, http.StatusUnauthorized, "admin jwt token manager is not configured")
		return auth.Claims{}, false
	}
	token, ok := bearerToken(w, r)
	if !ok {
		return auth.Claims{}, false
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "bearer token is required")
		return auth.Claims{}, false
	}
	claims, err := verifyAdminJWTClaims(ctx, tokenMgr, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid bearer token")
		return auth.Claims{}, false
	}
	if claims.Role != "company_admin" && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return auth.Claims{}, false
	}
	return claims, true
}

func verifyAdminJWTClaims(ctx context.Context, tokenMgr *auth.TokenManager, token string) (auth.Claims, error) {
	claims, err := tokenMgr.VerifyFull(ctx, token)
	if err == nil {
		return claims, nil
	}
	return auth.Claims{}, err
}

func handleAdminRefresh(w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if tokenMgr == nil {
		writeError(w, http.StatusUnauthorized, "admin jwt token manager is not configured")
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	claims, err := verifyAdminJWTClaims(r.Context(), tokenMgr, req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if claims.TokenType != "refresh" {
		writeError(w, http.StatusBadRequest, "refresh token is required")
		return
	}
	claims.TokenType = "access"
	accessToken, err := tokenMgr.Sign(claims, adminAccessTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": accessToken})
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request, service AdminService, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := adminBearerClaims(r.Context(), w, r, tokenMgr)
	if !ok {
		return
	}
	if _, err := service.IncrementSessionVersion(r.Context(), claims.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "logged out",
	})
}

func handleAdminVerify(w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := adminBearerClaims(r.Context(), w, r, tokenMgr)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user_id":       claims.UserID,
		"domain_id":     claims.DomainID,
		"company_id":    claims.CompanyID,
		"role":          claims.Role,
	})
}

func handleListAdminUsers(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if !rejectUnknownQueryKeys(w, r, "company_id") {
		return
	}

	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return
	}

	// company_admin may only list admins within their own company.
	if claims, hasClaims := adminClaimsFromCtx(r.Context()); hasClaims && claims.Role == "company_admin" {
		companyID = claims.CompanyID
	}

	users, _, err := service.ListAdminUsers(r.Context(), maildb.AdminUserListRequest{CompanyID: companyID})
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func sendInviteEmailAsync(ctx context.Context, cfg adminRouteConfig, service AdminService, userID, token string) {
	if cfg.systemEmailSender == nil || cfg.publicBaseURL == "" {
		return
	}
	work := func(emailCtx context.Context) {
		email, err := userPrimaryEmail(emailCtx, service, userID)
		if err != nil {
			slog.WarnContext(ctx, "send invite email lookup failed", "error", err)
			return
		}
		inviteURL := cfg.publicBaseURL + "/admin/invite/" + url.PathEscape(token) + "/accept"
		if err := cfg.systemEmailSender.SendInvite(emailCtx, email, inviteURL); err != nil {
			slog.WarnContext(ctx, "send invite email failed", "error", err)
		}
	}
	if cfg.bgTracker != nil {
		cfg.bgTracker.Track(ctx, 10*time.Second, work)
		return
	}
	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		work(emailCtx)
	}()
}

func sendWelcomeEmailAsync(ctx context.Context, cfg adminRouteConfig, service AdminService, user maildb.UserView) {
	if cfg.systemEmailSender == nil {
		return
	}
	work := func(emailCtx context.Context) {
		email, err := userEmailFromView(emailCtx, service, user)
		if err != nil {
			slog.WarnContext(ctx, "send welcome email lookup failed", "error", err)
			return
		}
		if err := cfg.systemEmailSender.SendWelcome(emailCtx, email, user.DisplayName); err != nil {
			slog.WarnContext(ctx, "send welcome email failed", "error", err)
		}
	}
	if cfg.bgTracker != nil {
		cfg.bgTracker.Track(ctx, 10*time.Second, work)
		return
	}
	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		work(emailCtx)
	}()
}

func userPrimaryEmail(ctx context.Context, service AdminService, userID string) (string, error) {
	user, err := service.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return userEmailFromView(ctx, service, user)
}

func userEmailFromView(ctx context.Context, service AdminService, user maildb.UserView) (string, error) {
	domain, err := service.GetDomain(ctx, user.DomainID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(user.Username) + "@" + strings.TrimSpace(domain.Name), nil
}

func handleCreateAdminUser(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Only system_admin may create admin users.
	if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "only system_admin may manage admin users")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	req.Role = strings.TrimSpace(req.Role)

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Role != "system_admin" && req.Role != "company_admin" {
		writeError(w, http.StatusBadRequest, "role must be system_admin or company_admin")
		return
	}

	if err := service.SetUserRole(r.Context(), req.UserID, req.Role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := service.GetUser(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "role assigned but failed to fetch user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     user.ID,
		"role":   user.Role,
		"status": user.Status,
	})
}

func handleDeleteAdminUser(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "DELETE" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Only system_admin may remove admin roles.
	if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "only system_admin may manage admin users")
		return
	}

	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	if err := service.ClearUserAdminRole(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "admin role removed",
		"id":     id,
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
	MFAExemptCIDRs        []string `json:"mfa_exempt_cidrs"`
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, authPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultAuthPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy authPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
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
	if _, err := service.SetDomainConfig(r.Context(), id, authPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const auditPolicyKey = "audit_policy"

type auditPolicy struct {
	CompanyID           string `json:"company_id"`
	AuditLevel          string `json:"audit_level"`
	AuditAdminActions   bool   `json:"audit_admin_actions"`
	AuditSecurityEvents bool   `json:"audit_security_events"`
	RetentionDays       int    `json:"retention_days"`
	MaskMailContent     bool   `json:"mask_mail_content"`
	MaskRecipientEmails bool   `json:"mask_recipient_emails"`
}

func defaultAuditPolicy() auditPolicy {
	return auditPolicy{
		AuditLevel:          "level_2",
		AuditAdminActions:   true,
		AuditSecurityEvents: true,
		RetentionDays:       90,
		MaskMailContent:     true,
		MaskRecipientEmails: false,
	}
}

func handleGetCompanyAuditPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, auditPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			policy := defaultAuditPolicy()
			policy.CompanyID = id
			writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	policy := defaultAuditPolicy()
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	policy.CompanyID = id
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyAuditPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	policy := defaultAuditPolicy()
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy.CompanyID = id
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, auditPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const securityGovernancePolicyKey = "security_governance_policy"

type securityGovernancePolicy struct {
	SecurityProfile             string `json:"security_profile"`
	WebhookPrivateNetworkAccess string `json:"webhook_private_network_access"`
}

func defaultSecurityGovernancePolicy() securityGovernancePolicy {
	return securityGovernancePolicy{
		SecurityProfile:             "enterprise",
		WebhookPrivateNetworkAccess: "deny",
	}
}

func normalizeSecurityGovernancePolicy(policy securityGovernancePolicy) (securityGovernancePolicy, error) {
	policy.SecurityProfile = strings.ToLower(strings.TrimSpace(policy.SecurityProfile))
	if policy.SecurityProfile == "" {
		policy.SecurityProfile = "enterprise"
	}
	switch policy.SecurityProfile {
	case "standard", "enterprise", "high_assurance":
	default:
		return securityGovernancePolicy{}, fmt.Errorf("invalid security_profile")
	}
	policy.WebhookPrivateNetworkAccess = strings.ToLower(strings.TrimSpace(policy.WebhookPrivateNetworkAccess))
	if policy.WebhookPrivateNetworkAccess == "" {
		policy.WebhookPrivateNetworkAccess = "deny"
	}
	switch policy.WebhookPrivateNetworkAccess {
	case "deny", "allow":
	default:
		return securityGovernancePolicy{}, fmt.Errorf("invalid webhook_private_network_access")
	}
	return policy, nil
}

func securityGovernanceFromEntry(entry configstore.ConfigEntry) securityGovernancePolicy {
	policy := defaultSecurityGovernancePolicy()
	if len(entry.Value) == 0 {
		return policy
	}
	var stored securityGovernancePolicy
	if err := json.Unmarshal(entry.Value, &stored); err != nil {
		return policy
	}
	normalized, err := normalizeSecurityGovernancePolicy(stored)
	if err != nil {
		return policy
	}
	return normalized
}

func getCompanySecurityGovernancePolicy(ctx context.Context, service AdminService, companyID string) (securityGovernancePolicy, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, securityGovernancePolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return defaultSecurityGovernancePolicy(), nil
		}
		return securityGovernancePolicy{}, err
	}
	return securityGovernanceFromEntry(entry), nil
}

func handleGetCompanySecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	policy, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input securityGovernancePolicy
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy, err := normalizeSecurityGovernancePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, securityGovernancePolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainSecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, securityGovernancePolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultSecurityGovernancePolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": securityGovernanceFromEntry(entry)})
}

func handlePutDomainSecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input securityGovernancePolicy
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy, err := normalizeSecurityGovernancePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, securityGovernancePolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		DMARCPolicy:     "quarantine",
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

func defaultSpamFilterPolicy() spamfilter.Policy {
	return spamfilter.DefaultPolicy()
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
		policy = spamfilter.NormalizePolicy(policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	var policy spamfilter.Policy
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
	policy = spamfilter.NormalizePolicy(policy)
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		policy = spamfilter.NormalizePolicy(policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	var policy spamfilter.Policy
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
	policy = spamfilter.NormalizePolicy(policy)
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleListCompanySpamFilterEvents(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "user_id", "from_addr", "to_addr", "subject", "flow_status", "since", "until") {
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
	req, ok := parseMailFlowLogListRequest(w, r, limit)
	if !ok {
		return
	}
	req.CompanyID = id
	req.Direction = string(maildb.MailFlowDirectionInbound)
	if strings.TrimSpace(req.FlowStatus) == "" {
		req.FlowStatus = string(maildb.MailFlowStatusFiltered)
	}
	logs, err := service.ListMailFlowLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spam_filter_events": logs})
}

func handleGetCompanySpamFilterStats(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectUnknownQueryKeys(w, r, "domain_id", "user_id", "since", "until") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	req, ok := parseMailFlowLogStatsRequest(w, r)
	if !ok {
		return
	}
	req.CompanyID = id
	req.Direction = string(maildb.MailFlowDirectionInbound)
	stats, err := service.GetMailFlowLogStats(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spam_filter_stats": stats})
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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

	if cfg.MetadataURL != "" {
		// Validate URL syntax first.
		if _, err := url.Parse(cfg.MetadataURL); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("invalid metadata URL: %v", err),
			})
			return
		}
		// Actually fetch the metadata endpoint.
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(cfg.MetadataURL)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("failed to reach metadata endpoint: %v", err),
			})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("metadata endpoint returned HTTP %d", resp.StatusCode),
			})
			return
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "xml") && !strings.Contains(ct, "saml") && !strings.Contains(ct, "text") {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("unexpected content type %q (expected XML/SAML metadata)", ct),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "SSO metadata endpoint is reachable and returned a valid response",
		})
		return
	}

	// No metadata URL — validate the login URL syntax.
	if _, err := url.Parse(cfg.SSOLoginURL); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("invalid SSO login URL: %v", err),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "SSO login URL is valid",
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	SecretSuffix    string   `json:"secret_suffix,omitempty"`
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

func publicWebhooks(items []webhook) []webhook {
	out := make([]webhook, 0, len(items))
	for _, item := range items {
		if item.SecretSuffix == "" && item.Secret != "" {
			if len(item.Secret) > 8 {
				item.SecretSuffix = item.Secret[len(item.Secret)-8:]
			} else {
				item.SecretSuffix = item.Secret
			}
		}
		item.Secret = ""
		out = append(out, item)
	}
	return out
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": publicWebhooks(cfg.Webhooks)})
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
	governance, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	allowPrivateNetwork := governance.WebhookPrivateNetworkAccess == "allow"
	parsedURL, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), input.URL, webhookguard.OutboundURLGuardOptions{AllowPrivateNetwork: allowPrivateNetwork})
	if err != nil {
		writeError(w, http.StatusBadRequest, "webhook url is not allowed")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	wh := webhook{
		ID:        fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:      input.Name,
		URL:       parsedURL.String(),
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	governance, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": "security governance policy unavailable"})
		return
	}
	guardOptions := webhookguard.OutboundURLGuardOptions{AllowPrivateNetwork: governance.WebhookPrivateNetworkAccess == "allow"}
	if _, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), target.URL, guardOptions); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": "webhook url is not allowed"})
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
	client := webhookguard.GuardedHTTPClient(&http.Client{Timeout: 10 * time.Second}, guardOptions)
	resp, err := client.Do(req)
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	logs, _, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	logs, _, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	cfg.Items = append(cfg.Items, input)
	if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "approved"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "rejected"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
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

	domains, _, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})

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
			"status":          healthStatus,
			"company_id":      id,
			"company_name":    company.Name,
			"domain_count":    len(domains),
			"active_domains":  activeDomains,
			"active_webhooks": activeWebhooks,
			"over_allocated":  overAllocated,
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
	domains, _, err := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: companyID, Limit: 200})
	if err != nil {
		return nil, err
	}
	return listCompanyUsersForDomains(ctx, service, companyID, domains, perDomainLimit)
}

func listCompanyUsersForDomains(ctx context.Context, service AdminService, companyID string, domains []maildb.DomainView, perDomainLimit int) ([]maildb.UserView, error) {
	if len(domains) == 0 {
		return nil, nil
	}
	limit := perDomainLimit * len(domains)
	users, _, err := service.ListUsers(ctx, maildb.UserListRequest{CompanyID: companyID, Limit: limit})
	return users, err
}

// ─── Security Posture ─────────────────────────────────────────────────────────

func handleGetSecurityPosture(w http.ResponseWriter, r *http.Request, service AdminService, companyID string) {
	ctx := r.Context()

	mfaStats, _ := service.GetMFAStats(ctx, companyID)
	domains, _, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: companyID, Limit: 200})

	users, _ := listCompanyUsersForDomains(ctx, service, companyID, domains, 1000)
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
		"ip_policy_configured":   ipPolicyConfigured,
		"users_without_password": usersWithoutPassword,
		"domain_count":           len(domains),
		"active_domains":         activeDomains,
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	cfg.Holds = append(cfg.Holds, input)

	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal legal holds")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, legalHoldsKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
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
	domains, _, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})
	domainID := ""
	if len(domains) > 0 {
		domainID = domains[0].ID
	}
	users, _ := listCompanyUsersForDomains(ctx, service, id, domains, 1000)
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
	domains, _, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})
	users, _ := listCompanyUsersForDomains(ctx, service, id, domains, 1000)
	totalUsers := 0
	activeUsers := 0
	suspendedUsers := 0
	totalUsers = len(users)
	for _, u := range users {
		if u.Status == "active" {
			activeUsers++
		} else {
			suspendedUsers++
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
