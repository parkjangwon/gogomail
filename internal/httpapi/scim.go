package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/scim"
)

// ErrSCIMUserNotFound is returned by SCIMUserService when a user does not exist.
var ErrSCIMUserNotFound = errors.New("scim user not found")

// SCIMUserService is the interface SCIM HTTP routes depend on.
type SCIMUserService interface {
	GetSCIMUser(ctx context.Context, id string) (scim.UserResource, error)
	ListSCIMUsers(ctx context.Context, filter *scim.Filter, startIndex, count int) ([]scim.UserResource, int, error)
	CreateSCIMUser(ctx context.Context, req scim.UserResource) (scim.UserResource, error)
	ReplaceSCIMUser(ctx context.Context, id string, req scim.UserResource) (scim.UserResource, error)
	PatchSCIMUser(ctx context.Context, id string, ops []scim.PatchOperation) (scim.UserResource, error)
	DeleteSCIMUser(ctx context.Context, id string) error
}

// RegisterSCIMRoutes mounts SCIM 2.0 /scim/v2 routes on mux, protected by Bearer token.
func RegisterSCIMRoutes(mux *http.ServeMux, svc SCIMUserService, token string) {
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		writeSCIMJSON(w, http.StatusOK, scimServiceProviderConfig())
	}))

	mux.HandleFunc("GET /scim/v2/ResourceTypes", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		writeSCIMJSON(w, http.StatusOK, scimResourceTypes())
	}))

	mux.HandleFunc("GET /scim/v2/Users", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var filter *scim.Filter
		if raw := q.Get("filter"); raw != "" {
			f, err := scim.ParseFilter(raw)
			if err == nil {
				filter = f
			}
		}
		startIndex := 1
		if s := q.Get("startIndex"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				startIndex = n
			}
		}
		count := 100
		if c := q.Get("count"); c != "" {
			if n, err := strconv.Atoi(c); err == nil && n > 0 {
				count = n
			}
		}
		users, total, err := svc.ListSCIMUsers(r.Context(), filter, startIndex, count)
		if err != nil {
			writeSCIMInternalError(w)
			return
		}
		resp := scim.NewListResponse(users)
		resp.TotalResults = total
		resp.StartIndex = startIndex
		resp.ItemsPerPage = len(users)
		writeSCIMJSON(w, http.StatusOK, resp)
	}))

	mux.HandleFunc("POST /scim/v2/Users", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		var req scim.UserResource
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSCIMError(w, http.StatusBadRequest, "invalidValue", "invalid request body")
			return
		}
		if req.UserName == "" {
			writeSCIMError(w, http.StatusBadRequest, "invalidValue", "userName is required")
			return
		}
		created, err := svc.CreateSCIMUser(r.Context(), req)
		if err != nil {
			writeSCIMInternalError(w)
			return
		}
		writeSCIMJSON(w, http.StatusCreated, created)
	}))

	mux.HandleFunc("GET /scim/v2/Users/{id}", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		user, err := svc.GetSCIMUser(r.Context(), id)
		if err != nil {
			if errors.Is(err, ErrSCIMUserNotFound) {
				writeSCIMError(w, http.StatusNotFound, "notFound", "user not found")
				return
			}
			writeSCIMInternalError(w)
			return
		}
		writeSCIMJSON(w, http.StatusOK, user)
	}))

	mux.HandleFunc("PUT /scim/v2/Users/{id}", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req scim.UserResource
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSCIMError(w, http.StatusBadRequest, "invalidValue", "invalid request body")
			return
		}
		updated, err := svc.ReplaceSCIMUser(r.Context(), id, req)
		if err != nil {
			if errors.Is(err, ErrSCIMUserNotFound) {
				writeSCIMError(w, http.StatusNotFound, "notFound", "user not found")
				return
			}
			writeSCIMInternalError(w)
			return
		}
		writeSCIMJSON(w, http.StatusOK, updated)
	}))

	mux.HandleFunc("PATCH /scim/v2/Users/{id}", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Schemas    []string              `json:"schemas"`
			Operations []scim.PatchOperation `json:"Operations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeSCIMError(w, http.StatusBadRequest, "invalidValue", "invalid request body")
			return
		}
		updated, err := svc.PatchSCIMUser(r.Context(), id, body.Operations)
		if err != nil {
			if errors.Is(err, ErrSCIMUserNotFound) {
				writeSCIMError(w, http.StatusNotFound, "notFound", "user not found")
				return
			}
			writeSCIMInternalError(w)
			return
		}
		writeSCIMJSON(w, http.StatusOK, updated)
	}))

	mux.HandleFunc("DELETE /scim/v2/Users/{id}", scimAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.DeleteSCIMUser(r.Context(), id); err != nil {
			if errors.Is(err, ErrSCIMUserNotFound) {
				writeSCIMError(w, http.StatusNotFound, "notFound", "user not found")
				return
			}
			writeSCIMInternalError(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func scimAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			writeSCIMError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing bearer token")
			return
		}
		next(w, r)
	}
}

func writeSCIMJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("scim: encode response failed", "code", code, "error", err)
	}
}

func writeSCIMInternalError(w http.ResponseWriter) {
	writeSCIMError(w, http.StatusInternalServerError, "internalError", "internal server error")
}

func writeSCIMError(w http.ResponseWriter, code int, scimType, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"schemas":  []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		"scimType": scimType,
		"detail":   detail,
		"status":   strconv.Itoa(code),
	}); err != nil {
		slog.Warn("scim: encode error response failed", "code", code, "error", err)
	}
}

func scimServiceProviderConfig() any {
	return map[string]any{
		"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": 200},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{
			{
				"type":        "oauthbearertoken",
				"name":        "OAuth Bearer Token",
				"description": "Authentication using the OAuth Bearer Token Standard",
			},
		},
	}
}

func scimResourceTypes() any {
	return map[string]any{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": 1,
		"Resources": []map[string]any{
			{
				"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
				"id":          "User",
				"name":        "User",
				"endpoint":    "/Users",
				"description": "User Account",
				"schema":      scim.SchemaUser,
				"meta": map[string]any{
					"resourceType": "ResourceType",
					"location":     "/scim/v2/ResourceTypes/User",
				},
			},
		},
	}
}
