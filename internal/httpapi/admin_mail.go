package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/maildb"
)

func registerDeliveryAndMailRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/delivery-attempts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "status", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		attempts, hasMore, err := service.ListDeliveryAttempts(r.Context(), maildb.DeliveryAttemptListRequest{
			Limit:           limit,
			Status:          status,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
			ProbeMore:       true,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempts": attempts, "has_more": hasMore})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "since", "status", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		stats, err := service.GetDeliveryAttemptStats(r.Context(), maildb.DeliveryAttemptStatsRequest{
			Status:          status,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempt_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/exhausted", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "recipient_domain", "message_id", "farm", "sender") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		sender, ok := parseBoundedAdminQuery(w, r, "sender")
		if !ok {
			return
		}
		attempts, err := service.ListExhaustedAttempts(r.Context(), maildb.ExhaustedAttemptListRequest{
			Limit:           limit,
			RecipientDomain: recipientDomain,
			MessageID:       messageID,
			Farm:            farm,
			Sender:          sender,
			Since:           since,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"exhausted_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-attempts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "status", "user_id", "message_id", "platform", "device_id", "provider_status", "provider_message_id") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		platform, ok := parseBoundedAdminQuery(w, r, "platform")
		if !ok {
			return
		}
		deviceID, ok := parseBoundedAdminQuery(w, r, "device_id")
		if !ok {
			return
		}
		providerStatus, ok := parseBoundedAdminQuery(w, r, "provider_status")
		if !ok {
			return
		}
		providerMessageID, ok := parseBoundedAdminQuery(w, r, "provider_message_id")
		if !ok {
			return
		}
		attempts, err := service.ListPushNotificationAttempts(r.Context(), maildb.PushNotificationAttemptListRequest{
			Limit:             limit,
			MessageID:         messageID,
			Status:            status,
			UserID:            userID,
			Platform:          platform,
			DeviceID:          deviceID,
			ProviderStatus:    providerStatus,
			ProviderMessageID: providerMessageID,
			Since:             since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-attempts/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		attempt, err := service.GetPushNotificationAttempt(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), attempt.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_attempt": attempt})
	}))

	mux.HandleFunc("PATCH /admin/v1/push-notification-attempts/{id}/outcome", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		attempt, err := service.GetPushNotificationAttempt(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "push notification attempt not found")
			return
		}
		if err := requiresCompanyAccess(r.Context(), attempt.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var req maildb.UpdatePushNotificationOutcomeRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.AttemptID = id
		if err := service.UpdatePushNotificationOutcome(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "since", "user_id", "message_id", "platform", "device_id") {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
		if !ok {
			return
		}
		platform, ok := parseBoundedAdminQuery(w, r, "platform")
		if !ok {
			return
		}
		deviceID, ok := parseBoundedAdminQuery(w, r, "device_id")
		if !ok {
			return
		}
		stats, err := service.GetPushNotificationStats(r.Context(), maildb.PushNotificationStatsRequest{
			MessageID: messageID,
			UserID:    userID,
			Platform:  platform,
			DeviceID:  deviceID,
			Since:     since,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/suppression-list", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "email", "reason") {
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
		email, ok := parseBoundedAdminQuery(w, r, "email")
		if !ok {
			return
		}
		reason, ok := parseBoundedAdminQuery(w, r, "reason")
		if !ok {
			return
		}
		listReq := maildb.SuppressionEntryListRequest{
			Limit:    limit,
			DomainID: domainID,
			Email:    email,
			Reason:   reason,
		}
		if err := maildb.ValidateSuppressionEntryListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		entries, err := service.ListSuppressionEntries(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suppression_list": entries})
	}))

	mux.HandleFunc("GET /admin/v1/trusted-relays", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "cidr", "description") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		cidr, ok := parseBoundedAdminQuery(w, r, "cidr")
		if !ok {
			return
		}
		description, ok := parseBoundedAdminQuery(w, r, "description")
		if !ok {
			return
		}
		listReq := maildb.TrustedRelayListRequest{
			Limit:       limit,
			CIDR:        cidr,
			Description: description,
		}
		if err := maildb.ValidateTrustedRelayListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		relays, err := service.ListTrustedRelays(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"trusted_relays": relays})
	}))

	mux.HandleFunc("POST /admin/v1/trusted-relays", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateTrustedRelayRequest
		if err := decodeJSONBody(r, &req); err != nil {
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

	mux.HandleFunc("GET /admin/v1/delivery-routes", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "status", "farm", "domain_pattern") {
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
		farm, ok := parseBoundedAdminQuery(w, r, "farm")
		if !ok {
			return
		}
		domainPattern, ok := parseBoundedAdminQuery(w, r, "domain_pattern")
		if !ok {
			return
		}
		listReq := maildb.DeliveryRouteListRequest{
			Limit:         limit,
			Status:        status,
			Farm:          farm,
			DomainPattern: domainPattern,
		}
		if err := maildb.ValidateDeliveryRouteListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		routes, err := service.ListDeliveryRoutes(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_routes": routes})
	}))

	mux.HandleFunc("POST /admin/v1/delivery-routes", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateDeliveryRouteRequest
		if err := decodeJSONBody(r, &req); err != nil {
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

	mux.HandleFunc("GET /admin/v1/delivery-routes/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "domain") {
			return
		}
		domain, ok := parseBoundedAdminQuery(w, r, "domain")
		if !ok {
			return
		}
		result, err := service.ResolveDeliveryRoute(r.Context(), domain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_route_resolution": result})
	}))

	mux.HandleFunc("PATCH /admin/v1/delivery-routes/{id}/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.UpdateDeliveryRouteStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDeliveryRouteStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/recovery-email", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		// Enforce company isolation before updating sensitive contact data.
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
		var req maildb.UpdateUserRecoveryEmailRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		if err := service.UpdateUserRecoveryEmail(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/dkim-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "status") {
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
		listReq := maildb.DKIMKeyListRequest{
			DomainID: domainID,
			Status:   status,
			Limit:    limit,
		}
		if err := maildb.ValidateDKIMKeyListRequest(listReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		keys, err := service.ListDKIMKeys(r.Context(), listReq)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_keys": keys})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var input maildb.CreateDKIMKeyInput
		if err := decodeJSONBody(r, &input); err != nil {
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

	mux.HandleFunc("DELETE /admin/v1/dkim-keys/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		key, err := service.GetDKIMKey(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "dkim key not found")
			return
		}
		domain, err := service.GetDomain(r.Context(), key.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeactivateDKIMKey(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys/{id}/verify-dns", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		key, err := service.GetDKIMKey(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "dkim key not found")
			return
		}
		domain, err := service.GetDomain(r.Context(), key.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		result, err := service.VerifyDKIMKeyDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_verification": result})
	}))

	mux.HandleFunc("POST /admin/v1/outbox/{id}/retry", adminAuth(func(w http.ResponseWriter, r *http.Request) {
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
		if err := service.RetryOutbox(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/suppression-list/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		entry, err := service.GetSuppressionEntry(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "suppression entry not found")
			return
		}
		domain, err := service.GetDomain(r.Context(), entry.DomainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve domain")
			return
		}
		if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeleteSuppressionEntry(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/trusted-relays/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteTrustedRelay(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/delivery-routes/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteDeliveryRoute(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))
}
