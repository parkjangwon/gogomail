package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	registerConsoleRoutes(mux, cfg, adminAuth, service)
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

	// ─── Alert rules / channels / events ────────────────────────────────────────
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

	// ─── Organization settings, compliance, roles, reports ──────────────────────
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

	// ─── Login audits (still inline, security-adjacent) ─────────────────────────
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/login-audits", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCompanyLoginAudits(service)(w, r)
	}))

	// ─── Onboarding utility ──────────────────────────────────────────────────────
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

	// ─── Delegated to focused register functions ─────────────────────────────────
	registerAuthAndAdminUserRoutes(mux, service, cfg, loginLimiter, adminAuth)
	registerAccessPolicyRoutes(mux, service, adminAuth)
	registerSecurityConfigRoutes(mux, service, adminAuth)
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
