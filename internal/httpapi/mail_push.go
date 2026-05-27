package httpapi

import (
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

func registerPushRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	mux.HandleFunc("GET /api/v1/push-devices", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email", "limit") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		devices, err := service.ListPushDevices(r.Context(), userID, limit)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_devices": devices})
	})

	mux.HandleFunc("POST /api/v1/push-devices", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		var req maildb.UpsertPushDeviceRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		device, err := service.UpsertPushDevice(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"push_device": device})
	})

	mux.HandleFunc("DELETE /api/v1/push-devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		id, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeletePushDevice(r.Context(), userID, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	})

	// Web Push subscription routes
	mux.HandleFunc("GET /api/v1/config/web-push", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		key := strings.TrimSpace(opts.WebPushVAPIDPublicKey)
		var keyVal any
		if key != "" {
			keyVal = key
		}
		writeJSON(w, http.StatusOK, map[string]any{"vapidPublicKey": keyVal})
	})

	mux.HandleFunc("GET /api/v1/me/push-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		subs, err := service.ListActiveWebPushSubscriptions(r.Context(), userID)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		if subs == nil {
			subs = []maildb.WebPushSubscription{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscriptions": subs})
	})

	mux.HandleFunc("POST /api/v1/me/push-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		var body struct {
			Endpoint  string `json:"endpoint"`
			P256DH    string `json:"p256dh"`
			Auth      string `json:"auth"`
			UserAgent string `json:"userAgent"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		// Validate the push endpoint URL to prevent SSRF: a malicious user could
		// register a subscription pointing at an internal host, causing the server
		// to make requests there when dispatching push notifications.
		if _, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), body.Endpoint, webhookguard.OutboundURLGuardOptions{}); err != nil {
			writeError(w, http.StatusBadRequest, "push subscription endpoint is not allowed")
			return
		}
		sub, err := service.UpsertWebPushSubscription(r.Context(), maildb.UpsertWebPushSubscriptionRequest{
			UserID:    userID,
			Endpoint:  body.Endpoint,
			P256DH:    body.P256DH,
			Auth:      body.Auth,
			UserAgent: body.UserAgent,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"subscription": sub})
	})

	mux.HandleFunc("DELETE /api/v1/me/push-subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		id, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteWebPushSubscription(r.Context(), userID, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	})
}
