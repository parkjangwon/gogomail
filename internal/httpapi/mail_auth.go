package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerAuthRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	mux.HandleFunc("POST /api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		if !opts.LoginLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		if opts.Authenticator == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication not configured")
			return
		}
		if tokenManager == nil {
			writeError(w, http.StatusServiceUnavailable, "token signing not configured")
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		req.Password = strings.TrimSpace(req.Password)
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "email and password are required")
			return
		}
		if len(req.Password) > maxPasswordResetBytes {
			writeError(w, http.StatusBadRequest, "password is too long")
			return
		}
		user, err := opts.Authenticator.AuthenticateUser(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, maildb.ErrCompanySuspended) {
				writeError(w, http.StatusForbidden, "account suspended")
				return
			}
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		clientIP := extractClientIP(r)

		// MFA policy check.
		var mfaSetupRequired bool
		if opts.MFAStore != nil && opts.ConfigResolver != nil {
			mfaResult, checkErr := checkMFARequired(r.Context(), opts, tokenManager, user, clientIP)
			if checkErr != nil {
				writeError(w, http.StatusInternalServerError, "mfa policy check failed")
				return
			}
			if mfaResult.TOTPRequired {
				// Enrolled — block login until TOTP is verified.
				writeJSON(w, http.StatusOK, map[string]any{
					"mfa_required":  true,
					"pending_token": mfaResult.PendingToken,
				})
				return
			}
			mfaSetupRequired = mfaResult.SetupRequired
		}

		const tokenTTL = 24 * time.Hour
		claims := auth.Claims{
			UserID:         user.UserID,
			DomainID:       user.DomainID,
			CompanyID:      user.CompanyID,
			SessionVersion: user.SessionVersion,
		}
		token, err := tokenManager.Sign(claims, tokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		resp := map[string]any{
			"token":                token,
			"expires_at":           time.Now().UTC().Add(tokenTTL).Format(time.RFC3339),
			"must_change_password": user.MustChangePassword,
			"client_ip":            clientIP,
		}
		if opts.RefreshTokenStore != nil {
			refreshToken, err := opts.RefreshTokenStore.CreateUserRefreshToken(r.Context(), user.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue refresh token")
				return
			}
			resp["refresh_token"] = refreshToken
		}
		if mfaSetupRequired {
			resp["mfa_setup_required"] = true
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if !opts.LoginLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		if opts.RefreshTokenStore == nil || tokenManager == nil {
			writeError(w, http.StatusServiceUnavailable, "token refresh not configured")
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		if req.RefreshToken == "" {
			writeError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}
		rotated, err := opts.RefreshTokenStore.RotateUserRefreshToken(r.Context(), req.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		const tokenTTL = 24 * time.Hour
		token, err := tokenManager.Sign(auth.Claims{
			UserID:         rotated.User.UserID,
			DomainID:       rotated.User.DomainID,
			CompanyID:      rotated.User.CompanyID,
			SessionVersion: rotated.User.SessionVersion,
		}, tokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token":                token,
			"refresh_token":        rotated.Token,
			"expires_at":           time.Now().UTC().Add(tokenTTL).Format(time.RFC3339),
			"must_change_password": rotated.User.MustChangePassword,
		})
	})

	mux.HandleFunc("POST /api/v1/auth/sessions/revoke-all", func(w http.ResponseWriter, r *http.Request) {
		if tokenManager == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication not configured")
			return
		}
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if opts.SessionRevoker == nil {
			writeError(w, http.StatusServiceUnavailable, "session revocation not configured")
			return
		}
		if _, err := opts.SessionRevoker.IncrementSessionVersion(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
