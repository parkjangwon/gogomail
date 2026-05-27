package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	rdbmsidp "github.com/gogomail/gogomail/internal/idprovider/rdbms"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerRDBMSSyncRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("POST /admin/v1/domains/{id}/rdbms/sync", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSync(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/rdbms/sync-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSyncHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/rdbms/conflicts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSyncConflicts(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/domains/{id}/rdbms/conflicts/{conflictId}/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleResolveRDBMSConflict(w, r, service)
	}))
}

func handleRDBMSSync(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "sync_type") {
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
	syncType, ok := parseBoundedAdminQuery(w, r, "sync_type")
	if !ok {
		return
	}
	if syncType == "" {
		writeError(w, http.StatusBadRequest, "sync_type is required")
		return
	}
	result, err := service.TriggerRDBMSSync(r.Context(), id, syncType)
	if err != nil {
		if errors.Is(err, rdbmsidp.ErrSyncNotConfigured) || errors.Is(err, rdbmsidp.ErrMembershipSyncUnsupported) {
			writeError(w, http.StatusNotImplemented, "rdbms sync is not configured")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleRDBMSSyncHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
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
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
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
	cursor, err := maildb.DecodeRDBMSSyncRunCursor(cursorRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	runs, err := service.GetRDBMSSyncRuns(r.Context(), maildb.RDBMSSyncRunListRequest{
		DomainID: id,
		Limit:    limit + 1,
		Offset:   offset,
		Cursor:   cursor,
	})
	if err != nil {
		writeInternalServerError(w)
		return
	}
	hasMore := len(runs) > limit
	if hasMore {
		runs = runs[:limit]
	}
	nextCursor := ""
	if hasMore && len(runs) > 0 {
		nextCursor, err = maildb.EncodeRDBMSSyncRunCursor(runs[len(runs)-1])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_runs": runs, "limit": limit, "offset": offset, "has_more": hasMore, "next_cursor": nextCursor})
}

func handleRDBMSSyncConflicts(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "unresolved_only", "limit", "offset", "cursor") {
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
	cursor, err := maildb.DecodeRDBMSSyncConflictCursor(cursorRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	unresolvedOnly := r.URL.Query().Get("unresolved_only") == "true"
	conflicts, err := service.GetRDBMSSyncConflicts(r.Context(), maildb.RDBMSSyncConflictListRequest{
		DomainID:       id,
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
		nextCursor, err = maildb.EncodeRDBMSSyncConflictCursor(conflicts[len(conflicts)-1])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts, "limit": limit, "offset": offset, "has_more": hasMore, "next_cursor": nextCursor})
}

func handleResolveRDBMSConflict(w http.ResponseWriter, r *http.Request, service AdminService) {
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
	conflictID, ok := parseBoundedAdminPathValue(w, r, "conflictId")
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"` // 'prefer_local', 'prefer_rdbms', 'manual'
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Resolution == "" {
		writeError(w, http.StatusBadRequest, "resolution is required")
		return
	}
	if err := service.ResolveRDBMSSyncConflict(r.Context(), conflictID, req.Resolution); err != nil {
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
