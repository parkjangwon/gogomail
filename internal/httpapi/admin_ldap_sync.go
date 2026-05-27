package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	ldapidp "github.com/gogomail/gogomail/internal/idprovider/ldap"
	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/maildb"
)

// ─── LDAP Sync ────────────────────────────────────────────────────────────────

func registerLDAPSyncRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("POST /admin/v1/domains/{id}/ldap/sync", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSync(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/ldap/sync-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSyncHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/ldap/conflicts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSyncConflicts(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/domains/{id}/ldap/conflicts/{conflictId}/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleResolveLDAPConflict(w, r, service)
	}))
}

func handleLDAPSync(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "sync_type") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	syncType, ok := parseBoundedAdminQuery(w, r, "sync_type")
	if !ok {
		return
	}
	if syncType == "" {
		writeError(w, http.StatusBadRequest, "sync_type is required")
		return
	}
	result, err := service.TriggerLDAPSync(r.Context(), id, syncType)
	if err != nil {
		if errors.Is(err, ldapidp.ErrSyncNotConfigured) {
			writeError(w, http.StatusNotImplemented, "ldap sync is not configured")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleLDAPSyncHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "limit", "offset", "cursor") {
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
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	runs, err := service.GetLDAPSyncRuns(r.Context(), maildb.LDAPSyncRunListRequest{
		DomainID: id,
		Limit:    limit + 1,
		Offset:   offset,
	})
	if err != nil {
		writeInternalServerError(w)
		return
	}
	hasMore := len(runs) > limit
	if hasMore {
		runs = runs[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_runs": runs, "limit": limit, "offset": offset, "has_more": hasMore})
}

func handleLDAPSyncConflicts(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "unresolved_only", "sync_run_id", "limit", "offset", "cursor") {
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
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	cursorRaw, ok := singleQueryValue(w, r, "cursor")
	if !ok {
		return
	}
	cursor, err := maildb.DecodeLDAPSyncConflictCursor(cursorRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	unresolvedOnly := r.URL.Query().Get("unresolved_only") == "true"
	syncRunID := r.URL.Query().Get("sync_run_id")
	conflicts, err := service.GetLDAPSyncConflicts(r.Context(), maildb.LDAPSyncConflictListRequest{
		DomainID:       id,
		SyncRunID:      syncRunID,
		UnresolvedOnly: unresolvedOnly,
		Limit:          limit + 1,
		Offset:         offset,
		Cursor:         cursor,
	})
	if err != nil {
		writeInternalServerError(w)
		return
	}
	hasMore := len(conflicts) > limit
	if hasMore {
		conflicts = conflicts[:limit]
	}
	nextCursor := ""
	if hasMore && len(conflicts) > 0 {
		nextCursor, err = maildb.EncodeLDAPSyncConflictCursor(conflicts[len(conflicts)-1])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts, "limit": limit, "offset": offset, "has_more": hasMore, "next_cursor": nextCursor})
}

func handleResolveLDAPConflict(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	conflictID, ok := parseBoundedAdminPathValue(w, r, "conflictId")
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"` // 'prefer_local', 'prefer_ldap', 'manual'
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Resolution == "" {
		writeError(w, http.StatusBadRequest, "resolution is required")
		return
	}
	if err := service.ResolveLDAPSyncConflict(r.Context(), conflictID, req.Resolution); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "resolved",
		"conflict_id": conflictID,
		"domain_id":   id,
		"resolution":  req.Resolution,
	})
}

// ─── IdP Config ───────────────────────────────────────────────────────────────

func registerIdPConfigRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/domains/{id}/idp-config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetIdPConfig(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/idp-config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleSetIdPConfig(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/domains/{id}/idp-config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteIdPConfig(w, r, service)
	}))
}

func handleGetIdPConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	cfg, err := service.GetDomainIdPConfig(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "get idp config failed", "domain_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve IdP configuration")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func handleSetIdPConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var cfg idprovider.Config
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cfg.DomainID = id
	if cfg.ProviderType == "" {
		writeError(w, http.StatusBadRequest, "provider_type is required")
		return
	}
	if err := service.SetDomainIdPConfig(r.Context(), &cfg); err != nil {
		slog.ErrorContext(r.Context(), "set idp config failed", "domain_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save IdP configuration")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func handleDeleteIdPConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := service.DeleteDomainIdPConfig(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "delete idp config failed", "domain_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete IdP configuration")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"domain_id": id, "status": "disabled"})
}
