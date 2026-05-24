package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/orgchart"
)

type OrgChartService interface {
	CreateUnit(ctx context.Context, unit *orgchart.OrganizationUnit) error
	GetUnit(ctx context.Context, id string) (*orgchart.OrganizationUnit, error)
	ListUnits(ctx context.Context, companyID string) ([]orgchart.OrganizationUnit, error)
	UpdateUnit(ctx context.Context, unit *orgchart.OrganizationUnit) error
	DeleteUnit(ctx context.Context, id string) error
	GetHierarchy(ctx context.Context, companyID string) (*orgchart.OrganizationHierarchy, error)
	AssignUserToUnit(ctx context.Context, unitID, userID, role, title string) error
	UpdateMemberTitle(ctx context.Context, memberID, title, role string) error
	GetMembershipsForUser(ctx context.Context, userID string) ([]orgchart.MembershipDetail, error)
	RemoveUserFromUnit(ctx context.Context, memberID string) error
	SyncWithLDAP(ctx context.Context, companyID string) (*orgchart.SyncLog, error)
}

func RegisterOrgChartRoutes(mux *http.ServeMux, service OrgChartService, adminToken string) {
	// GET /admin/v1/organization/units - List units for company
	mux.HandleFunc("GET /admin/v1/organization/units", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "company_id") {
			return
		}

		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}

		units, err := service.ListUnits(r.Context(), companyID)
		if err != nil {
			writeInternalServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"units": units})
	}))

	// GET /admin/v1/organization/units/{id} - Get unit details
	mux.HandleFunc("GET /admin/v1/organization/units/{id}", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}

		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}

		unit, err := service.GetUnit(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "unit not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"unit": unit})
	}))

	// POST /admin/v1/organization/units - Create unit
	mux.HandleFunc("POST /admin/v1/organization/units", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}

		var req orgchart.OrganizationUnit
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if err := service.CreateUnit(r.Context(), &req); err != nil {
			slog.ErrorContext(r.Context(), "create org unit failed", "error", err)
			writeError(w, http.StatusBadRequest, "invalid org unit request")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"unit": req})
	}))

	// PUT /admin/v1/organization/units/{id} - Update unit
	mux.HandleFunc("PUT /admin/v1/organization/units/{id}", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}

		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}

		var req orgchart.OrganizationUnit
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		req.ID = id
		if err := service.UpdateUnit(r.Context(), &req); err != nil {
			writeInternalServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"unit": req})
	}))

	// DELETE /admin/v1/organization/units/{id} - Delete unit
	mux.HandleFunc("DELETE /admin/v1/organization/units/{id}", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
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

		if err := service.DeleteUnit(r.Context(), id); err != nil {
			writeInternalServerError(w)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	// GET /admin/v1/organization/hierarchy - Get full org tree
	mux.HandleFunc("GET /admin/v1/organization/hierarchy", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "company_id") {
			return
		}

		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}

		hierarchy, err := service.GetHierarchy(r.Context(), companyID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"hierarchy": hierarchy})
	}))

	// POST /admin/v1/organization/members - Assign user to unit
	mux.HandleFunc("POST /admin/v1/organization/members", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}

		var req struct {
			UnitID string `json:"unit_id"`
			UserID string `json:"user_id"`
			Role   string `json:"role"`
			Title  string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if err := service.AssignUserToUnit(r.Context(), req.UnitID, req.UserID, req.Role, req.Title); err != nil {
			slog.ErrorContext(r.Context(), "assign user to unit failed", "error", err, "unit_id", req.UnitID, "user_id", req.UserID)
			writeError(w, http.StatusBadRequest, "failed to assign user")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"status": "assigned"})
	}))

	// PATCH /admin/v1/organization/members/{id} - Update member title/role
	mux.HandleFunc("PATCH /admin/v1/organization/members/{id}", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}

		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}

		var req struct {
			Title string `json:"title"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if err := service.UpdateMemberTitle(r.Context(), id, req.Title, req.Role); err != nil {
			writeInternalServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
	}))

	// GET /admin/v1/organization/members?user_id={id} - Get org memberships for a user
	mux.HandleFunc("GET /admin/v1/organization/members", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}

		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}

		memberships, err := service.GetMembershipsForUser(r.Context(), userID)
		if err != nil {
			writeInternalServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"memberships": memberships})
	}))

	// DELETE /admin/v1/organization/members/{id} - Remove user from unit
	mux.HandleFunc("DELETE /admin/v1/organization/members/{id}", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
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

		if err := service.RemoveUserFromUnit(r.Context(), id); err != nil {
			writeInternalServerError(w)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	// POST /admin/v1/organization/sync - Trigger LDAP sync
	mux.HandleFunc("POST /admin/v1/organization/sync", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "company_id") {
			return
		}

		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}

		log, err := service.SyncWithLDAP(r.Context(), companyID)
		if err != nil {
			if errors.Is(err, orgchart.ErrOrgChartSyncNotConfigured) {
				writeError(w, http.StatusNotImplemented, "organization sync is not configured")
				return
			}
			writeInternalServerError(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"sync_log": log})
	}))
}
