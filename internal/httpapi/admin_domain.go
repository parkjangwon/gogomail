package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerDomainRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/domains", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "status", "dns_status") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		dnsStatus, ok := parseBoundedAdminQuery(w, r, "dns_status")
		if !ok {
			return
		}
		// company_admin may only list domains within their own company.
		if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role == "company_admin" {
			if companyID != "" && companyID != claims.CompanyID {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
			companyID = claims.CompanyID
		}
		listReq := maildb.DomainListRequest{
			Limit:     limit,
			CompanyID: companyID,
			Status:    status,
			DNSStatus: dnsStatus,
			ProbeMore: true,
		}
		if err := maildb.ValidateDomainListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		domains, hasMore, err := service.ListDomains(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": domains, "has_more": hasMore})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		stats, err := service.GetDomainStats(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-check", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		report, err := service.VerifyDomainDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_check": report})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-checks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status", "since") {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		listReq := maildb.DomainDNSCheckListRequest{
			DomainID: id,
			Limit:    limit,
			Status:   status,
			Since:    since,
		}
		if err := maildb.ValidateDomainDNSCheckListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		checks, err := service.ListDomainDNSChecks(r.Context(), listReq)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_checks": checks})
	}))

	mux.HandleFunc("POST /admin/v1/domains", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateDomainRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		// Enforce company access before creating a domain under req.CompanyID.
		if err := requiresCompanyAccess(r.Context(), req.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		// Enforce MaxDomains org limit.
		if req.CompanyID != "" {
			if limitMsg := checkDomainLimit(r.Context(), service, req.CompanyID); limitMsg != "" {
				writeError(w, http.StatusForbidden, limitMsg)
				return
			}
		}
		domain, err := service.CreateDomain(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := inheritCompanyDomainSettings(r.Context(), service, domain); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("POST /admin/v1/domains/bulk", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleBulkDomains(w, r, service)
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req.ID = id
		if err := service.UpdateDomainStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req.ID = id
		if err := service.UpdateDomainQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeleteDomain(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		settings, err := service.GetDomainSettings(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var settings admin.DomainSettings
		if err := decodeJSONBody(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		settings.DomainID = id
		if err := service.UpdateDomainSettings(r.Context(), &settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/api-settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		settings, err := service.GetAPISettings(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/api-settings", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var settings admin.APISettings
		if err := decodeJSONBody(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		settings.DomainID = id
		if err := service.UpdateAPISettings(r.Context(), &settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/domains/{id}/api-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var req struct {
			Name         string     `json:"name"`
			CreatedBy    string     `json:"created_by"`
			Scopes       []string   `json:"scopes"`
			AllowedCIDRs []string   `json:"allowed_cidrs"`
			ExpiresAt    *time.Time `json:"expires_at"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		key := &admin.APIKey{
			DomainID:     id,
			Name:         req.Name,
			CreatedBy:    req.CreatedBy,
			Scopes:       req.Scopes,
			AllowedCIDRs: req.AllowedCIDRs,
			ExpiresAt:    req.ExpiresAt,
		}
		secret, err := service.CreateAPIKey(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     key.ID,
			"secret": secret,
		})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/api-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		keys, err := service.ListAPIKeys(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}/api-keys/{keyid}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		keyID, ok := parseBoundedAdminPathValue(w, r, "keyid")
		if !ok {
			return
		}
		existingKey, err := service.GetAPIKey(r.Context(), keyID)
		if err != nil {
			writeError(w, http.StatusNotFound, "api key not found")
			return
		}
		keyDomain, err := service.GetDomain(r.Context(), existingKey.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), keyDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeleteAPIKey(r.Context(), keyID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("POST /admin/v1/domains/{id}/api-keys/{keyid}/rotate", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		keyID, ok := parseBoundedAdminPathValue(w, r, "keyid")
		if !ok {
			return
		}
		existingKey, err := service.GetAPIKey(r.Context(), keyID)
		if err != nil {
			writeError(w, http.StatusNotFound, "api key not found")
			return
		}
		keyDomain, err := service.GetDomain(r.Context(), existingKey.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), keyDomain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		newSecret, err := service.RotateAPIKey(r.Context(), keyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"secret": newSecret,
		})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		entries, err := service.ListDomainConfig(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetDomainConfig(r.Context(), id, key)
		if err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		var req adminConfigSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		entry, err := service.SetDomainConfig(r.Context(), id, key, req.Value, req.Locked, req.Version)
		if err != nil {
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entry})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/mcp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		entry, err := service.GetDomainConfig(r.Context(), id, "mcp.policy")
		if err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{"mcp_policy": defaultDomainMCPPolicy})
				return
			}
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mcp_policy": json.RawMessage(entry.Value), "config": entry})
	}))

	mux.HandleFunc("PUT /admin/v1/domains/{id}/mcp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var req adminConfigSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(req.Value) == 0 {
			req.Value = defaultDomainMCPPolicy
		}
		if !json.Valid(req.Value) {
			writeError(w, http.StatusBadRequest, "mcp policy must be valid JSON")
			return
		}
		entry, err := service.SetDomainConfig(r.Context(), id, "mcp.policy", req.Value, req.Locked, req.Version)
		if err != nil {
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mcp_policy": json.RawMessage(entry.Value), "config": entry})
	}))

	mux.HandleFunc("DELETE /admin/v1/domains/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		version := int64(-1)
		if v := r.URL.Query().Get("version"); v != "" {
			var err error
			version, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid version")
				return
			}
		}
		if err := service.DeleteDomainConfig(r.Context(), id, key, version); err != nil {
			if errors.Is(err, configstore.ErrConfigNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrConfigLocked) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			if errors.Is(err, configstore.ErrVersionConflict) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDomainPolicyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req.ID = id
		policy, err := service.UpdateDomainPolicy(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain_policy": policy})
	}))
}
