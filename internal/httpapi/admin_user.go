package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerUserAndConfigRoutes(mux *http.ServeMux, service AdminService, token string, cfg adminRouteConfig, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "status", "password_configured") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		passwordConfigured, ok := parseOptionalBoolQuery(w, r, "password_configured")
		if !ok {
			return
		}
		// company_admin may only list users in domains belonging to their company.
		if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role == "company_admin" {
			if domainID != "" {
				// Verify the requested domain belongs to the company_admin's company.
				domain, err := service.GetDomain(r.Context(), domainID)
				if err != nil || domain.CompanyID != claims.CompanyID {
					writeError(w, http.StatusForbidden, "access denied")
					return
				}
			} else {
				// Without a domain filter, listing all users would expose cross-company data.
				writeError(w, http.StatusForbidden, "company_admin must filter by domain_id")
				return
			}
		}
		listReq := maildb.UserListRequest{
			DomainID:           domainID,
			Status:             status,
			PasswordConfigured: passwordConfigured,
			Limit:              limit,
			ProbeMore:          true,
		}
		if err := maildb.ValidateUserListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		users, hasMore, err := service.ListUsers(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users, "has_more": hasMore})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// company_admin may only access users in their own company's domains.
		if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role == "company_admin" {
			domain, err := service.GetDomain(r.Context(), user.DomainID)
			if err != nil || domain.CompanyID != claims.CompanyID {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		domain, err := service.GetDomain(r.Context(), user.DomainID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeleteUser(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/users", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateUserRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		// Enforce MaxUsers org limit.
		if req.DomainID != "" {
			domain, err := service.GetDomain(r.Context(), req.DomainID)
			if err == nil && domain.CompanyID != "" {
				if limitMsg := checkUserLimit(r.Context(), service, domain.CompanyID); limitMsg != "" {
					writeError(w, http.StatusForbidden, limitMsg)
					return
				}
			}
		}
		if req.Password != "" && req.PasswordHash == "" {
			if len(req.Password) > maxPasswordResetBytes {
				writeError(w, http.StatusBadRequest, "password is too long")
				return
			}
			salt := make([]byte, 16)
			if _, err := rand.Read(salt); err != nil {
				slog.ErrorContext(r.Context(), "create user: generate salt failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			hash, err := auth.HashPasswordPBKDF2SHA256(req.Password, salt, 0)
			if err != nil {
				slog.ErrorContext(r.Context(), "create user: hash password failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			req.PasswordHash = hash
			req.Password = ""
			req.MustChangePassword = true
		}
		user, err := service.CreateUser(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	}))

	mux.HandleFunc("POST /admin/v1/users/{id}/invite", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		it, err := service.CreateInviteToken(r.Context(), id, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendInviteEmailAsync(r.Context(), cfg, service, id, it.Token)
		writeJSON(w, http.StatusCreated, map[string]any{"invite_token": it})
	}))

	mux.HandleFunc("POST /admin/invite/{token}/accept", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		rawToken := r.PathValue("token")
		if len(rawToken) < 8 || len(rawToken) > 128 {
			writeError(w, http.StatusBadRequest, "invalid token")
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(body.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		if len(body.Password) > maxPasswordResetBytes {
			writeError(w, http.StatusBadRequest, "password is too long")
			return
		}
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			slog.ErrorContext(r.Context(), "invite accept: generate salt failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		hash, err := auth.HashPasswordPBKDF2SHA256(body.Password, salt, 0)
		if err != nil {
			slog.ErrorContext(r.Context(), "invite accept: hash password failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		user, err := service.AcceptInviteToken(r.Context(), rawToken, hash)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendWelcomeEmailAsync(r.Context(), cfg, service, user)
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	})

	mux.HandleFunc("PATCH /admin/v1/users/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		domain, err := service.GetDomain(r.Context(), user.DomainID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req.ID = id
		if err := service.UpdateUserStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("POST /admin/v1/users/bulk", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input struct {
			IDs    []string `json:"ids"`
			Action string   `json:"action"` // "activate", "suspend"
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(input.IDs) == 0 {
			writeError(w, http.StatusBadRequest, "ids is required")
			return
		}
		var targetStatus string
		switch input.Action {
		case "activate":
			targetStatus = "active"
		case "suspend":
			targetStatus = "suspended"
		default:
			writeError(w, http.StatusBadRequest, "unsupported action: "+input.Action)
			return
		}
		ctx := r.Context()
		companyID := ""
		if claims, ok := adminClaimsFromCtx(ctx); ok && claims.Role == "company_admin" {
			companyID = claims.CompanyID
			if companyID == "" {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
		}
		result, err := service.BulkUpdateUserStatus(ctx, maildb.BulkUpdateUserStatusRequest{
			IDs:       input.IDs,
			Status:    targetStatus,
			CompanyID: companyID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		failed := make([]map[string]string, 0, len(result.Failed))
		for _, id := range result.Failed {
			failed = append(failed, map[string]string{"id": id, "error": "user not found or access denied"})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"succeeded": result.Updated,
			"failed":    failed,
		})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/password-hash", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserPasswordHashRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		domain, err := service.GetDomain(r.Context(), user.DomainID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		req.ID = id
		if err := service.UpdateUserPasswordHash(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateUserRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserRole(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID, "role": req.Role})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		entries, err := service.ListUserConfig(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetUserConfig(r.Context(), id, key)
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

	mux.HandleFunc("PUT /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		writeError(w, http.StatusForbidden, "admin cannot modify user scope config directly")
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		writeError(w, http.StatusForbidden, "admin cannot modify user scope config directly")
	}))

	// MFA management routes
	mux.HandleFunc("GET /admin/v1/users/{id}/mfa", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		status, err := service.GetUserMFAStatus(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_status": status})
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/mfa", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.ResetUserMFA(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/mfa/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		stats, err := service.GetMFAStats(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_stats": stats})
	}))

	registerAdminDeviceTokenRoutes(mux, service, token)

	mux.HandleFunc("GET /admin/v1/config/stream", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flush := func() {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		flush()
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected"}`)
		flush()
		if cfg.configNotifier == nil {
			<-r.Context().Done()
			return
		}
		ch := cfg.configNotifier.Subscribe()
		defer cfg.configNotifier.Unsubscribe(ch)
		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				payload, err := json.Marshal(map[string]string{
					"type":       "config.changed",
					"scope_type": string(event.ScopeType),
					"scope_id":   event.ScopeID,
					"key":        event.Key,
					"action":     event.Action,
				})
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flush()
			}
		}
	}))
}
