package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/authmfa"
	"github.com/gogomail/gogomail/internal/maildb"
)

const (
	mfaPendingTTL = 5 * time.Minute
	fullTokenTTL  = 24 * time.Hour
	authPolicyMFA = "auth_policy"
)

// mfaPolicy is the MFA-relevant subset of authPolicy stored in configstore.
type mfaPolicy struct {
	MFARequired    bool     `json:"mfa_required"`
	MFAExemptCIDRs []string `json:"mfa_exempt_cidrs"`
}

// mfaCheckResult is returned by checkMFARequired.
type mfaCheckResult struct {
	// TOTPRequired is true when the user is enrolled and must complete TOTP/recovery
	// verification before receiving a full token. PendingToken is set in this case.
	TOTPRequired bool
	PendingToken string
	// SetupRequired is true when the policy mandates MFA but the user has not
	// enrolled yet. The caller should issue a full token but inform the frontend
	// so it can prompt the user to complete enrollment.
	SetupRequired bool
}

// checkMFARequired resolves the effective auth policy for the user, checks IP
// exemption, and determines whether MFA verification or setup is needed.
func checkMFARequired(
	ctx context.Context,
	opts MailRouteOptions,
	tokenManager *auth.TokenManager,
	user maildb.AuthenticatedUser,
	clientIP string,
) (mfaCheckResult, error) {
	// Resolve effective auth_policy for this user/domain/company.
	policyRaw, resolveErr := opts.ConfigResolver.Resolve(ctx, user.UserID, user.DomainID, user.CompanyID, authPolicyMFA)
	if resolveErr != nil {
		// No policy configured → MFA not required.
		return mfaCheckResult{}, nil
	}
	var policy mfaPolicy
	if err := json.Unmarshal(policyRaw, &policy); err != nil {
		return mfaCheckResult{}, fmt.Errorf("parse auth policy: %w", err)
	}
	if !policy.MFARequired {
		return mfaCheckResult{}, nil
	}

	// Check IP exemption.
	if len(policy.MFAExemptCIDRs) > 0 && clientIP != "" {
		ip := net.ParseIP(clientIP)
		if ip != nil && ipIsExempt(ip, policy.MFAExemptCIDRs) {
			return mfaCheckResult{}, nil
		}
	}

	// Check user's MFA enrollment.
	status, err := opts.MFAStore.GetUserMFAStatus(ctx, user.UserID)
	if err != nil {
		return mfaCheckResult{}, fmt.Errorf("get mfa status: %w", err)
	}
	if !status.Enabled {
		// Policy requires MFA but user hasn't enrolled yet.
		// Issue a full token so the user can log in and reach the settings page.
		return mfaCheckResult{SetupRequired: true}, nil
	}

	// User is enrolled — issue a short-lived pending token to gate the TOTP step.
	tok, err := issuePendingToken(tokenManager, user)
	if err != nil {
		return mfaCheckResult{}, err
	}
	return mfaCheckResult{TOTPRequired: true, PendingToken: tok}, nil
}

func issuePendingToken(tokenManager *auth.TokenManager, user maildb.AuthenticatedUser) (string, error) {
	claims := auth.Claims{
		UserID:         user.UserID,
		DomainID:       user.DomainID,
		SessionVersion: user.SessionVersion,
		TokenType:      "mfa_pending",
	}
	return tokenManager.Sign(claims, mfaPendingTTL)
}

func ipIsExempt(ip net.IP, cidrs []string) bool {
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func extractClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if parts := strings.SplitN(fwd, ",", 2); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// RegisterMFARoutes registers MFA-related API routes onto mux.
func RegisterMFARoutes(mux *http.ServeMux, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	// POST /api/v1/auth/mfa/verify — exchange pending token + TOTP/recovery code for a full JWT.
	mux.HandleFunc("POST /api/v1/auth/mfa/verify", func(w http.ResponseWriter, r *http.Request) {
		if opts.MFAStore == nil || tokenManager == nil {
			writeError(w, http.StatusServiceUnavailable, "mfa not configured")
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

		// Validate pending token.
		claims, err := tokenManager.Verify(req.PendingToken)
		if err != nil || claims.TokenType != "mfa_pending" {
			writeError(w, http.StatusUnauthorized, "invalid or expired pending token")
			return
		}

		if err := verifyMFACode(r.Context(), opts.MFAStore, claims.UserID, req.Code); err != nil {
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
			SessionVersion: claims.SessionVersion,
			MFAVerified:    true,
		}
		token, err := tokenManager.Sign(fullClaims, fullTokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token":      token,
			"expires_at": time.Now().UTC().Add(fullTokenTTL).Format(time.RFC3339),
		})
	})

	// GET /api/v1/auth/mfa/status — own MFA enrollment status.
	mux.HandleFunc("GET /api/v1/auth/mfa/status", func(w http.ResponseWriter, r *http.Request) {
		if opts.MFAStore == nil {
			writeError(w, http.StatusServiceUnavailable, "mfa not configured")
			return
		}
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		status, err := opts.MFAStore.GetUserMFAStatus(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mfa_status": status})
	})

	// POST /api/v1/auth/mfa/setup — start TOTP enrollment, returns secret + QR URI.
	mux.HandleFunc("POST /api/v1/auth/mfa/setup", func(w http.ResponseWriter, r *http.Request) {
		if opts.MFAStore == nil {
			writeError(w, http.StatusServiceUnavailable, "mfa not configured")
			return
		}
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
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
			req.Issuer = "GoGoMail"
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
		if err := opts.MFAStore.SetupMFASecret(r.Context(), claims.UserID, secret, codes); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store mfa secret")
			return
		}

		qrURI := fmt.Sprintf(
			"otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=6&period=30",
			req.Issuer, req.Email, secret, req.Issuer,
		)
		writeJSON(w, http.StatusOK, map[string]any{
			"secret":         secret,
			"qr_uri":         qrURI,
			"recovery_codes": codes,
		})
	})

	// POST /api/v1/auth/mfa/setup/confirm — verify first code to activate MFA.
	mux.HandleFunc("POST /api/v1/auth/mfa/setup/confirm", func(w http.ResponseWriter, r *http.Request) {
		if opts.MFAStore == nil {
			writeError(w, http.StatusServiceUnavailable, "mfa not configured")
			return
		}
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
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

		secret, err := opts.MFAStore.GetPendingMFASecret(r.Context(), claims.UserID)
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
		if err := opts.MFAStore.ActivateMFA(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to activate mfa")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "mfa enabled"})
	})

	// DELETE /api/v1/auth/mfa — disable MFA (user self-service).
	mux.HandleFunc("DELETE /api/v1/auth/mfa", func(w http.ResponseWriter, r *http.Request) {
		if opts.MFAStore == nil {
			writeError(w, http.StatusServiceUnavailable, "mfa not configured")
			return
		}
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		if err := opts.MFAStore.DisableMFA(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to disable mfa")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "mfa disabled"})
	})
}

// verifyMFACode tries TOTP first, then recovery code. The code format
// determines which path is taken: 6 digits → TOTP, otherwise recovery.
func verifyMFACode(ctx context.Context, store MFAStore, userID, code string) error {
	if isTOTPCode(code) {
		secret, _, err := store.GetMFASecret(ctx, userID)
		if err != nil {
			return err
		}
		return store.VerifyAndRecordTOTP(ctx, userID, secret, code, time.Now())
	}
	return store.VerifyAndConsumeRecoveryCode(ctx, userID, code)
}

func isTOTPCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
