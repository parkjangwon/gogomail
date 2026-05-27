package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

func registerProfileRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	mux.HandleFunc("POST /api/v1/messages/send", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		var req mailservice.SendTextRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if !allowAPIKeyRequest(w, r, opts.APIKeyLimiter) {
			return
		}
		if !bindRequestUserID(w, r, tokenManager, service, &req.UserID, req.UserEmail) {
			return
		}
		if notice, ok, err := userMCPGeneratedNotice(r.Context(), service, r, req.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to apply mcp settings")
			return
		} else if ok {
			mailservice.ApplyGeneratedNoticeToSendTextRequest(&req, notice)
		}
		ctx, ok := userMCPSendPolicyContext(w, r, service, r.Context(), req.UserID, &req)
		if !ok {
			return
		}
		if err := mailservice.EnforceMCPSendPolicy(ctx, req); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		result, err := service.SendText(ctx, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		recordUserMCPSend(r, req.UserID)
		result = mailservice.NormalizeSendTextResult(result)

		writeJSON(w, http.StatusAccepted, map[string]any{"message": result})
	})

	mux.HandleFunc("GET /api/v1/preferences", func(w http.ResponseWriter, r *http.Request) {
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
		prefs, err := service.GetWebmailPreferences(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load preferences")
			return
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"preferences": prefs})
	})

	mux.HandleFunc("PUT /api/v1/preferences", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		var prefs json.RawMessage
		if err := decodeJSONBody(r, &prefs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := service.SetWebmailPreferences(r.Context(), userID, prefs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save preferences")
			return
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"preferences": prefs})
	})

	mux.HandleFunc("GET /api/v1/me/mcp/settings", func(w http.ResponseWriter, r *http.Request) {
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
		prefs, err := service.GetWebmailPreferences(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mcp settings")
			return
		}
		var keyInfo *apikeys.KeyInfo
		if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok {
			keyInfo = info
		}
		settings, err := effectiveMCPSettings(r.Context(), service, userID, extractMCPSettings(prefs), keyInfo)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mcp settings")
			return
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"mcp": settings})
	})

	mux.HandleFunc("PUT /api/v1/me/mcp/settings", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := sessionUserIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var mcp json.RawMessage
		if err := decodeJSONBody(r, &mcp); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		merged, err := mergeMCPSettings(r.Context(), service, userID, mcp)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		settings, err := effectiveMCPSettings(r.Context(), service, userID, extractMCPSettings(merged), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mcp settings")
			return
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"mcp": settings})
	})

	mux.HandleFunc("GET /api/v1/me/mcp/access-keys", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := sessionUserIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		keys, err := service.ListUserMCPAccessKeys(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list mcp access keys")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mcp_access_keys": keys})
	})

	mux.HandleFunc("POST /api/v1/me/mcp/access-keys", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := sessionUserIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req maildb.CreateUserMCPAccessKeyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		created, err := service.CreateUserMCPAccessKey(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"mcp_access_key": created.Key, "token": created.Token})
	})

	mux.HandleFunc("DELETE /api/v1/me/mcp/access-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := sessionUserIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		id, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		key, err := service.RevokeUserMCPAccessKey(r.Context(), userID, id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mcp_access_key": key})
	})

	mux.HandleFunc("PATCH /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		var req struct {
			DisplayName   *string `json:"display_name"`
			RecoveryEmail *string `json:"recovery_email"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.DisplayName == nil && req.RecoveryEmail == nil {
			writeError(w, http.StatusBadRequest, "at least one profile field is required")
			return
		}
		if req.DisplayName != nil {
			if err := service.UpdateUserDisplayName(r.Context(), userID, *req.DisplayName); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if req.RecoveryEmail != nil {
			if err := service.UpdateOwnRecoveryEmail(r.Context(), userID, *req.RecoveryEmail); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("PUT /api/v1/me/avatar", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		avatarURL, err := readProfileAvatarUpload(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := service.UpdateUserAvatarURL(r.Context(), userID, avatarURL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"avatar_url": avatarURL})
	})

	mux.HandleFunc("DELETE /api/v1/me/avatar", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		if err := service.UpdateUserAvatarURL(r.Context(), userID, ""); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"avatar_url": ""})
	})

	mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
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
		profile, err := service.GetUserProfile(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load profile")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": profile})
	})

	mux.HandleFunc("GET /api/v1/me/addresses", func(w http.ResponseWriter, r *http.Request) {
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
		addrs, err := service.ListUserAddresses(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list addresses")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"addresses": addrs})
	})

	mux.HandleFunc("POST /api/v1/me/password", func(w http.ResponseWriter, r *http.Request) {
		if !opts.LoginLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
		req.NewPassword = strings.TrimSpace(req.NewPassword)
		if req.CurrentPassword == "" || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "current_password and new_password are required")
			return
		}
		if err := service.ChangeUserPassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
}
