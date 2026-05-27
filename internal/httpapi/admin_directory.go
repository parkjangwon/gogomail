package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/directory"
)

func registerDirectoryRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/directory/principals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "organization_id", "kinds", "q", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryPrincipalSearchRequest(w, r, limit)
		if !ok {
			return
		}
		principals, err := service.SearchDirectoryPrincipals(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_principals": principals})
	}))

	mux.HandleFunc("GET /admin/v1/directory/aliases/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "address", "active_only") {
			return
		}
		req, ok := parseDirectoryAliasResolveRequest(w, r)
		if !ok {
			return
		}
		alias, err := service.ResolveDirectoryAlias(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("GET /admin/v1/directory/aliases", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "target_kind", "target_id", "q", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryAliasListRequest(w, r, limit)
		if !ok {
			return
		}
		aliases, err := service.ListDirectoryAliases(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_aliases": aliases})
	}))

	mux.HandleFunc("POST /admin/v1/directory/aliases", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateAliasRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		alias, err := service.CreateDirectoryAlias(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/aliases/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryAlias(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		alias, err := service.DeleteDirectoryAlias(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_alias": alias})
	}))

	mux.HandleFunc("GET /admin/v1/directory/delegations", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "owner_kind", "owner_id", "delegate_kind", "delegate_id", "scope", "role", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryDelegationListRequest(w, r, limit)
		if !ok {
			return
		}
		delegations, err := service.ListDirectoryDelegations(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegations": delegations})
	}))

	mux.HandleFunc("POST /admin/v1/directory/delegations", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateDelegationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		delegation, err := service.CreateDirectoryDelegation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("GET /admin/v1/directory/group-memberships", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "group_id", "member_kind", "member_id", "role", "active_only") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDirectoryGroupMembershipListRequest(w, r, limit)
		if !ok {
			return
		}
		memberships, err := service.ListDirectoryGroupMemberships(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_memberships": memberships})
	}))

	mux.HandleFunc("POST /admin/v1/directory/group-memberships", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req directory.CreateGroupMembershipRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		membership, err := service.CreateDirectoryGroupMembership(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/group-memberships/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryGroupMembership(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		membership, err := service.DeleteDirectoryGroupMembership(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/group-memberships/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryGroupMembership(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var req directory.UpdateGroupMembershipRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		membership, err := service.UpdateDirectoryGroupMembershipRole(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/group-memberships/{id}/assignment", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryGroupMembership(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var req directory.ReassignGroupMembershipRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		membership, err := service.ReassignDirectoryGroupMembership(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_group_membership": membership})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/delegations/{id}/role", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryDelegation(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var req directory.UpdateDelegationRoleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		delegation, err := service.UpdateDirectoryDelegationRole(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("PATCH /admin/v1/directory/delegations/{id}/assignment", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryDelegation(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var req directory.ReassignDelegationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		delegation, err := service.ReassignDirectoryDelegation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("DELETE /admin/v1/directory/delegations/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetDirectoryDelegation(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		delegation, err := service.DeleteDirectoryDelegation(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"directory_delegation": delegation})
	}))

	mux.HandleFunc("GET /admin/v1/backpressure", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		state, err := backpressureService.GetBackpressure(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))

	mux.HandleFunc("PATCH /admin/v1/backpressure", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		var req backpressure.StateUpdate
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		state, err := backpressureService.UpdateBackpressure(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))
}
