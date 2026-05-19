package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/authmfa"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerAdminMFARoutes(mux *http.ServeMux, cfg adminRouteConfig, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	if cfg.adminMFAStore == nil || cfg.tokenMgr == nil {
		return
	}

	// POST /admin/v1/auth/mfa/verify — pending_token + TOTP/recovery → access+refresh token pair
	mux.HandleFunc("POST /admin/v1/auth/mfa/verify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PendingToken string `json:"pending_token"`
			Code         string `json:"code"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		if req.PendingToken == "" || req.Code == "" {
			writeError(w, http.StatusBadRequest, "pending_token and code are required")
			return
		}

		claims, err := cfg.tokenMgr.Verify(req.PendingToken)
		if err != nil || claims.TokenType != "mfa_pending" {
			writeError(w, http.StatusUnauthorized, "invalid or expired pending token")
			return
		}

		if err := verifyMFACode(r.Context(), cfg.adminMFAStore, claims.UserID, req.Code); err != nil {
			if errors.Is(err, maildb.ErrMFAInvalidCode) || errors.Is(err, maildb.ErrMFACodeAlreadyUsed) {
				writeError(w, http.StatusUnauthorized, "invalid mfa code")
				return
			}
			if errors.Is(err, maildb.ErrMFANotEnrolled) {
				writeError(w, http.StatusUnprocessableEntity, "mfa not enrolled")
				return
			}
			writeError(w, http.StatusInternalServerError, "mfa verification failed")
			return
		}

		fullClaims := auth.Claims{
			UserID:         claims.UserID,
			DomainID:       claims.DomainID,
			CompanyID:      claims.CompanyID,
			Role:           claims.Role,
			SessionVersion: claims.SessionVersion,
			MFAVerified:    true,
		}
		accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, fullClaims)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		})
	})

	// GET /admin/v1/auth/mfa/status
	mux.HandleFunc("GET /admin/v1/auth/mfa/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := adminClaimsFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		status, err := cfg.adminMFAStore.GetUserMFAStatus(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_status": status})
	}))

	// POST /admin/v1/auth/mfa/setup
	mux.HandleFunc("POST /admin/v1/auth/mfa/setup", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := adminClaimsFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req struct {
			Issuer string `json:"issuer"`
			Email  string `json:"email"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Issuer == "" {
			req.Issuer = "GoGoMail Admin"
		}
		if req.Email == "" {
			req.Email = claims.UserID
		}

		secret, err := authmfa.GenerateSecret()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate secret")
			return
		}
		codes, err := authmfa.GenerateRecoveryCodes(8)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate recovery codes")
			return
		}
		if err := cfg.adminMFAStore.SetupMFASecret(r.Context(), claims.UserID, secret, codes); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store mfa secret")
			return
		}

		qrURI := fmt.Sprintf(
			"otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=6&period=30",
			req.Issuer, req.Email, secret, req.Issuer,
		)
		qrPNG, err := qrcode.Encode(qrURI, qrcode.Medium, 180)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate qr code")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"secret":         secret,
			"qr_uri":         qrURI,
			"qr_image":       "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrPNG),
			"recovery_codes": codes,
		})
	}))

	// POST /admin/v1/auth/mfa/setup/confirm
	mux.HandleFunc("POST /admin/v1/auth/mfa/setup/confirm", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := adminClaimsFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		if req.Code == "" {
			writeError(w, http.StatusBadRequest, "code is required")
			return
		}

		secret, err := cfg.adminMFAStore.GetPendingMFASecret(r.Context(), claims.UserID)
		if errors.Is(err, maildb.ErrMFANotEnrolled) {
			writeError(w, http.StatusUnprocessableEntity, "mfa setup not started")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retrieve mfa secret")
			return
		}
		if !authmfa.VerifyTOTP(secret, req.Code, time.Now()) {
			writeError(w, http.StatusUnauthorized, "invalid code")
			return
		}
		if err := cfg.adminMFAStore.ActivateMFA(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to activate mfa")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "mfa enabled"})
	}))

	// DELETE /admin/v1/auth/mfa
	mux.HandleFunc("DELETE /admin/v1/auth/mfa", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := adminClaimsFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err := cfg.adminMFAStore.DisableMFA(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to disable mfa")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "mfa disabled"})
	}))
}

// adminMFASetupRequired returns true when the role+policy combination demands
// MFA enrollment but the user has not yet enrolled.
func adminMFASetupRequired(ctx context.Context, role string, user maildb.AuthenticatedUser, cfg adminRouteConfig) bool {
	if role == "system_admin" {
		return cfg.adminMFARequired
	}
	if cfg.configResolver == nil {
		return false
	}
	raw, err := cfg.configResolver.Resolve(ctx, user.UserID, user.DomainID, "", "auth_policy")
	if err != nil {
		return false
	}
	var policy struct {
		MFARequired bool `json:"mfa_required"`
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return false
	}
	return policy.MFARequired
}
