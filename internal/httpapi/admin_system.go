package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
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
