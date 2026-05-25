package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
)

func inheritCompanyDomainSettings(ctx context.Context, service AdminService, domain maildb.DomainView) error {
	if domain.CompanyID == "" || domain.ID == "" {
		return nil
	}
	entry, err := service.GetCompanyConfig(ctx, domain.CompanyID, companyDomainSettingsDefaultsKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return nil
		}
		return fmt.Errorf("load company domain settings defaults: %w", err)
	}
	if len(entry.Value) == 0 {
		return nil
	}
	var settings admin.DomainSettings
	if err := json.Unmarshal(entry.Value, &settings); err != nil {
		return fmt.Errorf("decode company domain settings defaults: %w", err)
	}
	settings.DomainID = domain.ID
	if settings.IPWhitelist == nil {
		settings.IPWhitelist = []string{}
	}
	if err := service.UpdateDomainSettings(ctx, &settings); err != nil {
		return fmt.Errorf("apply company domain settings defaults: %w", err)
	}
	return nil
}

func RegisterAdminRoutes(mux *http.ServeMux, service AdminService, token string, opts ...AdminRouteOption) {
	cfg := adminRouteConfig{environment: "development"}
	for _, opt := range opts {
		opt(&cfg)
	}
	// adminAuth closes over token and cfg.tokenMgr so call sites only pass the handler.
	adminAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return adminJWTOrStaticAuthWithEnvironment(token, cfg.tokenMgr, cfg.environment, next)
	}

	// ─── Console & Delivery Routes ─────────────────────────────────────────────
	registerConsoleRoutes(mux, cfg, adminAuth)
	registerCompanyRoutes(mux, service, adminAuth)
	registerDomainRoutes(mux, service, adminAuth)
	registerUserAndConfigRoutes(mux, service, token, cfg, adminAuth)
	registerOperationsRoutes(mux, service, cfg, adminAuth)
	registerDirectoryRoutes(mux, service, adminAuth)
	registerStorageRoutes(mux, service, adminAuth)
	registerUsageAndQuotaRoutes(mux, service, adminAuth)
	registerDeliveryAndMailRoutes(mux, service, adminAuth)
	registerAdminUtilityRoutes(mux, service, cfg, adminAuth)
	registerAdminMFARoutes(mux, service, cfg, adminAuth)
}

func registerAdminUtilityRoutes(mux *http.ServeMux, service AdminService, cfg adminRouteConfig, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	// loginLimiter enforces a strict per-IP rate limit for login attempts (5 req/min).
	loginLimiter := NewAdminIPRateLimiter(5, time.Minute)

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

	mux.HandleFunc("PUT /admin/v1/alert-channels/{channelid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleUpdateAlertChannel(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/alert-channels/{channelid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAlertChannel(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/alert-events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAlertEvents(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/auth/login", loginLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogin(w, r, service, cfg)
	})).ServeHTTP)

	mux.HandleFunc("POST /admin/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRefresh(w, r, cfg.tokenMgr)
	})

	mux.HandleFunc("POST /admin/v1/auth/setup", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAdminSetup(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogout(w, r, service, cfg.tokenMgr)
	})

	mux.HandleFunc("GET /admin/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		handleAdminVerify(w, r, cfg.tokenMgr)
	})

	mux.HandleFunc("GET /admin/v1/admin-users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListAdminUsers(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/admin-users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateAdminUser(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/admin-users/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAdminUser(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/health", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAdminHealth(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/system/metrics", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		memPct := 0.0
		if ms.Sys > 0 {
			memPct = float64(ms.HeapInuse) / float64(ms.Sys) * 100
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"memory": map[string]any{
				"heap_inuse_bytes": ms.HeapInuse,
				"heap_sys_bytes":   ms.HeapSys,
				"sys_bytes":        ms.Sys,
				"alloc_bytes":      ms.Alloc,
				"gc_runs":          ms.NumGC,
				"usage_pct":        memPct,
			},
			"goroutines": runtime.NumGoroutine(),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
	}))

	mux.HandleFunc("GET /admin/v1/organization/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetOrganizationSettings(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/organization/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleUpdateOrganizationSettings(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/compliance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListCompliance(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/roles", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListRoles(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/roles", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateRole(w, r, service)
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

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainAuthPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainAuthPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/audit-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyAuditPolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/audit-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyAuditPolicy(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySecurityGovernancePolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySecurityGovernancePolicy(w, r, service)
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

	mux.HandleFunc("GET /admin/v1/domains/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSecurityGovernancePolicy(w, r, service)
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSecurityGovernancePolicy(w, r, service)
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

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/login-audits", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCompanyLoginAudits(service)(w, r)
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

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter/events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListCompanySpamFilterEvents(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySpamFilterStats(w, r, service)
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

	registerAuditLogExportRoutes(mux, adminAuth, service)
	registerTenantHealthRoutes(mux, adminAuth, service)
	registerChangeHistoryAndApprovalsRoutes(mux, adminAuth, service)
	registerWebhookRoutes(mux, adminAuth, service)
	registerNotificationTemplateRoutes(mux, adminAuth, service)
	registerSecurityPostureRoutes(mux, adminAuth, service)
	registerGlobalSignatureRoutes(mux, adminAuth, service)
	registerLegalHoldsRoutes(mux, adminAuth, service)
	registerSCIMStatusRoutes(mux, adminAuth, service)
	registerSeatUsageRoutes(mux, adminAuth, service)
	registerLDAPSyncRoutes(mux, adminAuth, service)
	registerRDBMSSyncRoutes(mux, adminAuth, service)
	registerIdPConfigRoutes(mux, adminAuth, service)
}
