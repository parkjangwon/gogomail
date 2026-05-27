package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerCompanyRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/companies", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status") {
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
		// company_admin may only see their own company.
		claims, hasClaims := adminClaimsFromCtx(r.Context())
		if hasClaims && claims.Role == "company_admin" {
			if claims.CompanyID == "" {
				writeJSON(w, http.StatusOK, map[string]any{"companies": []any{}, "has_more": false})
				return
			}
			company, err := service.GetCompany(r.Context(), claims.CompanyID)
			if err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"companies": []any{company}, "has_more": false})
			return
		}
		companies, hasMore, err := service.ListCompanies(r.Context(), maildb.CompanyListRequest{
			Limit:     limit,
			Status:    status,
			ProbeMore: true,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"companies": companies, "has_more": hasMore})
	}))

	mux.HandleFunc("POST /admin/v1/companies", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateCompanyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		company, err := service.CreateCompany(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"company": company})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		company, err := service.GetCompany(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"company": company})
	}))

	mux.HandleFunc("PATCH /admin/v1/companies/{id}/quota", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateCompanyQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		req.ID = id
		if err := service.UpdateCompanyQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateCompanyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		req.ID = id
		company, err := service.UpdateCompany(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"company": company})
	}))

	mux.HandleFunc("DELETE /admin/v1/companies/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		if err := service.DeleteCompany(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/companies/{id}/users/bulk-import", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		var req struct {
			Users []struct {
				Email       string `json:"email"`
				DisplayName string `json:"display_name"`
				DomainID    string `json:"domain_id"`
				Password    string `json:"password"`
			} `json:"users"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		type failure struct {
			Email string `json:"email"`
			Error string `json:"error"`
		}
		var failures []failure
		successCount := 0
		for _, u := range req.Users {
			parts := strings.SplitN(u.Email, "@", 2)
			username := parts[0]
			createReq := maildb.CreateUserRequest{
				DomainID:    u.DomainID,
				Username:    username,
				DisplayName: u.DisplayName,
				Address:     u.Email,
				Password:    u.Password,
			}
			if u.Password != "" {
				if len(u.Password) > maxPasswordResetBytes {
					failures = append(failures, failure{Email: u.Email, Error: "password is too long"})
					continue
				}
				salt := make([]byte, 16)
				if _, err := rand.Read(salt); err != nil {
					failures = append(failures, failure{Email: u.Email, Error: "generate salt"})
					continue
				}
				hash, err := auth.HashPasswordPBKDF2SHA256(u.Password, salt, 0)
				if err != nil {
					failures = append(failures, failure{Email: u.Email, Error: "hash password failed"})
					continue
				}
				createReq.PasswordHash = hash
				createReq.Password = ""
				createReq.MustChangePassword = true
			}
			if _, err := service.CreateUser(r.Context(), createReq); err != nil {
				failures = append(failures, failure{Email: u.Email, Error: err.Error()})
				continue
			}
			successCount++
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"total":    len(req.Users),
			"success":  successCount,
			"failed":   len(failures),
			"failures": failures,
		})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/users/bulk-export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		users, err := listCompanyUsers(r.Context(), service, id, 1000)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		var buf bytes.Buffer
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{"email", "display_name", "domain_id", "status", "quota_used", "quota_limit", "created_at"})
		for _, u := range users {
			_ = cw.Write([]string{
				u.Username,
				u.DisplayName,
				u.DomainID,
				u.Status,
				strconv.FormatInt(u.QuotaUsed, 10),
				strconv.FormatInt(u.QuotaLimit, 10),
				u.CreatedAt.Format(time.RFC3339),
			})
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="users-export.csv"`))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		entries, err := service.ListCompanyConfig(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": entries})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		entry, err := service.GetCompanyConfig(r.Context(), id, key)
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

	mux.HandleFunc("PUT /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		key, ok := parseBoundedAdminPathValue(w, r, "key")
		if !ok {
			return
		}
		var req adminConfigSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		entry, err := service.SetCompanyConfig(r.Context(), id, key, req.Value, req.Locked, req.Version)
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

	mux.HandleFunc("DELETE /admin/v1/companies/{id}/config/{key}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		if err := service.DeleteCompanyConfig(r.Context(), id, key, version); err != nil {
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

	mux.HandleFunc("POST /admin/v1/companies/{id}/config/propagate", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "scope") {
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
		scopeStr := r.URL.Query().Get("scope")
		if scopeStr == "" {
			writeError(w, http.StatusBadRequest, "scope is required")
			return
		}
		scope := configstore.PropagateScope(scopeStr)
		if !scope.IsValid() {
			writeError(w, http.StatusBadRequest, "invalid scope")
			return
		}
		var req adminConfigPropagateRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := service.PropagateCompanyConfig(r.Context(), id, scope, req.Key, req.Value, req.Locked); err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))
}
