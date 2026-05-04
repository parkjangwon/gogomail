package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/maildb"
)

type AdminService interface {
	ListDomains(ctx context.Context, limit int) ([]maildb.DomainView, error)
	GetDomain(ctx context.Context, id string) (maildb.DomainView, error)
	VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error)
	CreateDomain(ctx context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error)
	UpdateDomainStatus(ctx context.Context, req maildb.UpdateDomainStatusRequest) error
	UpdateDomainQuota(ctx context.Context, req maildb.UpdateDomainQuotaRequest) error
	UpdateDomainPolicy(ctx context.Context, req maildb.UpdateDomainPolicyRequest) (maildb.DomainPolicyView, error)
	ListUsers(ctx context.Context, domainID string, limit int) ([]maildb.UserView, error)
	GetUser(ctx context.Context, id string) (maildb.UserView, error)
	CreateUser(ctx context.Context, req maildb.CreateUserRequest) (maildb.UserView, error)
	UpdateUserStatus(ctx context.Context, req maildb.UpdateUserStatusRequest) error
	UpdateUserQuota(ctx context.Context, req maildb.UpdateUserQuotaRequest) error
	ListQueueStats(ctx context.Context) ([]maildb.QueueStat, error)
	ListQuotaUsage(ctx context.Context, limit int) ([]maildb.QuotaUsageView, error)
	ListDeliveryAttempts(ctx context.Context, limit int) ([]maildb.DeliveryAttemptView, error)
	ListSuppressionEntries(ctx context.Context, limit int) ([]maildb.SuppressionEntry, error)
	ListTrustedRelays(ctx context.Context, limit int) ([]maildb.TrustedRelayView, error)
	CreateTrustedRelay(ctx context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error)
	DeleteTrustedRelay(ctx context.Context, id string) error
	ListDeliveryRoutes(ctx context.Context, limit int) ([]maildb.DeliveryRouteView, error)
	CreateDeliveryRoute(ctx context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error)
	UpdateDeliveryRouteStatus(ctx context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error
	DeleteDeliveryRoute(ctx context.Context, id string) error
	ListDKIMKeys(ctx context.Context, domainID string, limit int) ([]maildb.DKIMKeyView, error)
	CreateDKIMKey(ctx context.Context, input maildb.CreateDKIMKeyInput) (string, error)
	DeactivateDKIMKey(ctx context.Context, id string) error
	RetryOutbox(ctx context.Context, id string) error
	DeleteSuppressionEntry(ctx context.Context, id string) error
}

func RegisterAdminRoutes(mux *http.ServeMux, service AdminService, token string) {
	mux.HandleFunc("GET /admin/v1/domains", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domains, err := service.ListDomains(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-check", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		report, err := service.VerifyDomainDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_check": report})
	}))

	mux.HandleFunc("POST /admin/v1/domains", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateDomainRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		domain, err := service.CreateDomain(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		if err := service.UpdateDomainStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/quota", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainQuotaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		if err := service.UpdateDomainQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/policy", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainPolicyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		policy, err := service.UpdateDomainPolicy(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain_policy": policy})
	}))

	mux.HandleFunc("GET /admin/v1/users", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		users, err := service.ListUsers(r.Context(), r.URL.Query().Get("domain_id"), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("POST /admin/v1/users", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		user, err := service.CreateUser(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateUserStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		if err := service.UpdateUserStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/quota", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateUserQuotaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		if err := service.UpdateUserQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/queue", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		stats, err := service.ListQueueStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"queues": stats})
	}))

	mux.HandleFunc("GET /admin/v1/quota-usage", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		usages, err := service.ListQuotaUsage(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_usage": usages})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		attempts, err := service.ListDeliveryAttempts(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/suppression-list", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		entries, err := service.ListSuppressionEntries(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suppression_list": entries})
	}))

	mux.HandleFunc("GET /admin/v1/trusted-relays", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		relays, err := service.ListTrustedRelays(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"trusted_relays": relays})
	}))

	mux.HandleFunc("POST /admin/v1/trusted-relays", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateTrustedRelayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		relay, err := service.CreateTrustedRelay(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"trusted_relay": relay})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-routes", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		routes, err := service.ListDeliveryRoutes(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_routes": routes})
	}))

	mux.HandleFunc("POST /admin/v1/delivery-routes", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateDeliveryRouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		route, err := service.CreateDeliveryRoute(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"delivery_route": route})
	}))

	mux.HandleFunc("PATCH /admin/v1/delivery-routes/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDeliveryRouteStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = r.PathValue("id")
		if err := service.UpdateDeliveryRouteStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/dkim-keys", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		keys, err := service.ListDKIMKeys(r.Context(), r.URL.Query().Get("domain_id"), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_keys": keys})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var input maildb.CreateDKIMKeyInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, err := service.CreateDKIMKey(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/dkim-keys/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if err := service.DeactivateDKIMKey(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/outbox/{id}/retry", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if err := service.RetryOutbox(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/suppression-list/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if err := service.DeleteSuppressionEntry(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/trusted-relays/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if err := service.DeleteTrustedRelay(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/delivery-routes/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if err := service.DeleteDeliveryRoute(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))
}

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !constantTimeTokenEqual(adminTokenFromRequest(r), token) {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		next(w, r)
	}
}

func constantTimeTokenEqual(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func adminTokenFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Admin-Token")); value != "" {
		return value
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return ""
}
