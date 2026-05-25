package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeError(w, http.StatusUnauthorized, "admin authentication is not configured")
			return
		}
		got, ok := adminTokenFromRequest(w, r)
		if !ok {
			return
		}
		if !constantTimeTokenEqual(got, token) {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
			return
		}
		next(w, r)
	}
}

func constantTimeTokenEqual(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func safeSHA256Header(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return ""
	}
	return value
}

func adminTokenFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	adminToken, ok := singleHTTPHeaderValue(w, r, "X-Admin-Token", maxHTTPAuthHeaderBytes)
	if !ok {
		return "", false
	}
	auth, ok := singleHTTPHeaderValue(w, r, "Authorization", maxHTTPAuthHeaderBytes)
	if !ok {
		return "", false
	}
	if adminToken != "" && auth != "" {
		writeError(w, http.StatusBadRequest, "X-Admin-Token and Authorization must not both be set")
		return "", false
	}
	if adminToken != "" {
		return adminToken, true
	}
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):]), true
	}
	return "", true
}

const (
	adminAccessTokenTTL  = 15 * time.Minute
	adminRefreshTokenTTL = 30 * 24 * time.Hour
)

func handleAdminLogin(w http.ResponseWriter, r *http.Request, service AdminService, cfg adminRouteConfig) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	issueToken := func(claims auth.Claims) {
		if cfg.tokenMgr == nil {
			writeError(w, http.StatusInternalServerError, "admin jwt token manager is not configured")
			return
		}
		accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, claims)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"user": map[string]any{
				"id":         claims.UserID,
				"role":       claims.Role,
				"company_id": claims.CompanyID,
			},
		})
	}

	// Bootstrap system admin via environment variables only.
	// Set GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL and GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD to enable.
	// If either env var is empty, bootstrap is disabled.
	bootstrapEmail := strings.TrimSpace(os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL"))
	bootstrapPassword := os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD")
	if bootstrapEmail != "" && bootstrapPassword != "" &&
		req.Email == bootstrapEmail && subtle.ConstantTimeCompare([]byte(req.Password), []byte(bootstrapPassword)) == 1 {
		issueToken(auth.Claims{
			UserID:    "system-admin",
			DomainID:  "system",
			CompanyID: "",
			Role:      "system_admin",
		})
		return
	}

	// Authenticate real user from DB
	authedUser, err := service.AuthenticateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	userView, err := service.GetUser(r.Context(), authedUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	if userView.Role != "company_admin" && userView.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	domain, err := service.GetDomain(r.Context(), authedUser.DomainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve company")
		return
	}

	// MFA check — only when adminMFAStore is wired in.
	if cfg.adminMFAStore != nil {
		mfaStatus, err := cfg.adminMFAStore.GetUserMFAStatus(r.Context(), authedUser.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check mfa status")
			return
		}
		if mfaStatus.Enabled {
			pendingClaims := auth.Claims{
				UserID:         authedUser.UserID,
				DomainID:       authedUser.DomainID,
				CompanyID:      domain.CompanyID,
				Role:           userView.Role,
				SessionVersion: authedUser.SessionVersion,
				TokenType:      "mfa_pending",
			}
			pendingToken, err := cfg.tokenMgr.Sign(pendingClaims, mfaPendingTTL)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue pending token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"mfa_required":  true,
				"pending_token": pendingToken,
			})
			return
		}

		setupRequired := adminMFASetupRequired(r.Context(), userView.Role, authedUser, cfg)
		if setupRequired {
			fullClaims := auth.Claims{
				UserID:         authedUser.UserID,
				DomainID:       authedUser.DomainID,
				CompanyID:      domain.CompanyID,
				Role:           userView.Role,
				SessionVersion: authedUser.SessionVersion,
			}
			if cfg.tokenMgr == nil {
				writeError(w, http.StatusInternalServerError, "admin jwt token manager is not configured")
				return
			}
			accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, fullClaims)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token":       accessToken,
				"refresh_token":      refreshToken,
				"mfa_setup_required": true,
				"user": map[string]any{
					"id":         fullClaims.UserID,
					"role":       fullClaims.Role,
					"company_id": fullClaims.CompanyID,
				},
			})
			return
		}
	}

	issueToken(auth.Claims{
		UserID:         authedUser.UserID,
		DomainID:       authedUser.DomainID,
		CompanyID:      domain.CompanyID,
		Role:           userView.Role,
		SessionVersion: authedUser.SessionVersion,
	})
}

func handleAdminSetup(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func signAdminSessionTokens(tokenMgr *auth.TokenManager, claims auth.Claims) (string, string, error) {
	accessClaims := claims
	accessClaims.TokenType = "access"
	accessToken, err := tokenMgr.Sign(accessClaims, adminAccessTokenTTL)
	if err != nil {
		return "", "", err
	}
	refreshClaims := claims
	refreshClaims.TokenType = "refresh"
	refreshToken, err := tokenMgr.Sign(refreshClaims, adminRefreshTokenTTL)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func adminBearerClaims(ctx context.Context, w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) (auth.Claims, bool) {
	if tokenMgr == nil {
		writeError(w, http.StatusUnauthorized, "admin jwt token manager is not configured")
		return auth.Claims{}, false
	}
	token, ok := bearerToken(w, r)
	if !ok {
		return auth.Claims{}, false
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "bearer token is required")
		return auth.Claims{}, false
	}
	claims, err := verifyAdminJWTClaims(ctx, tokenMgr, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid bearer token")
		return auth.Claims{}, false
	}
	if claims.Role != "company_admin" && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return auth.Claims{}, false
	}
	return claims, true
}

func verifyAdminJWTClaims(ctx context.Context, tokenMgr *auth.TokenManager, token string) (auth.Claims, error) {
	claims, err := tokenMgr.VerifyFull(ctx, token)
	if err == nil {
		return claims, nil
	}
	return auth.Claims{}, err
}

func handleAdminRefresh(w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if tokenMgr == nil {
		writeError(w, http.StatusUnauthorized, "admin jwt token manager is not configured")
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	claims, err := verifyAdminJWTClaims(r.Context(), tokenMgr, req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if claims.TokenType != "refresh" {
		writeError(w, http.StatusBadRequest, "refresh token is required")
		return
	}
	claims.TokenType = "access"
	accessToken, err := tokenMgr.Sign(claims, adminAccessTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": accessToken})
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request, service AdminService, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := adminBearerClaims(r.Context(), w, r, tokenMgr)
	if !ok {
		return
	}
	if _, err := service.IncrementSessionVersion(r.Context(), claims.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "logged out",
	})
}

func handleAdminVerify(w http.ResponseWriter, r *http.Request, tokenMgr *auth.TokenManager) {
	defer r.Body.Close()

	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := adminBearerClaims(r.Context(), w, r, tokenMgr)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user_id":       claims.UserID,
		"domain_id":     claims.DomainID,
		"company_id":    claims.CompanyID,
		"role":          claims.Role,
	})
}

func handleListAdminUsers(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if !rejectUnknownQueryKeys(w, r, "company_id") {
		return
	}

	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return
	}

	// company_admin may only list admins within their own company.
	if claims, hasClaims := adminClaimsFromCtx(r.Context()); hasClaims && claims.Role == "company_admin" {
		companyID = claims.CompanyID
	}

	users, _, err := service.ListAdminUsers(r.Context(), maildb.AdminUserListRequest{CompanyID: companyID})
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func sendInviteEmailAsync(ctx context.Context, cfg adminRouteConfig, service AdminService, userID, token string) {
	if cfg.systemEmailSender == nil || cfg.publicBaseURL == "" {
		return
	}
	work := func(emailCtx context.Context) {
		email, err := userPrimaryEmail(emailCtx, service, userID)
		if err != nil {
			slog.WarnContext(ctx, "send invite email lookup failed", "error", err)
			return
		}
		inviteURL := cfg.publicBaseURL + "/admin/invite/" + url.PathEscape(token) + "/accept"
		if err := cfg.systemEmailSender.SendInvite(emailCtx, email, inviteURL); err != nil {
			slog.WarnContext(ctx, "send invite email failed", "error", err)
		}
	}
	if cfg.bgTracker != nil {
		cfg.bgTracker.Track(ctx, 10*time.Second, work)
		return
	}
	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		work(emailCtx)
	}()
}

func sendWelcomeEmailAsync(ctx context.Context, cfg adminRouteConfig, service AdminService, user maildb.UserView) {
	if cfg.systemEmailSender == nil {
		return
	}
	work := func(emailCtx context.Context) {
		email, err := userEmailFromView(emailCtx, service, user)
		if err != nil {
			slog.WarnContext(ctx, "send welcome email lookup failed", "error", err)
			return
		}
		if err := cfg.systemEmailSender.SendWelcome(emailCtx, email, user.DisplayName); err != nil {
			slog.WarnContext(ctx, "send welcome email failed", "error", err)
		}
	}
	if cfg.bgTracker != nil {
		cfg.bgTracker.Track(ctx, 10*time.Second, work)
		return
	}
	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		work(emailCtx)
	}()
}

func userPrimaryEmail(ctx context.Context, service AdminService, userID string) (string, error) {
	user, err := service.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return userEmailFromView(ctx, service, user)
}

func userEmailFromView(ctx context.Context, service AdminService, user maildb.UserView) (string, error) {
	domain, err := service.GetDomain(ctx, user.DomainID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(user.Username) + "@" + strings.TrimSpace(domain.Name), nil
}

func handleCreateAdminUser(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Only system_admin may create admin users.
	if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "only system_admin may manage admin users")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	req.Role = strings.TrimSpace(req.Role)

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Role != "system_admin" && req.Role != "company_admin" {
		writeError(w, http.StatusBadRequest, "role must be system_admin or company_admin")
		return
	}

	if err := service.SetUserRole(r.Context(), req.UserID, req.Role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := service.GetUser(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "role assigned but failed to fetch user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     user.ID,
		"role":   user.Role,
		"status": user.Status,
	})
}

func handleDeleteAdminUser(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()

	if r.Method != "DELETE" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Only system_admin may remove admin roles.
	if claims, ok := adminClaimsFromCtx(r.Context()); ok && claims.Role != "system_admin" {
		writeError(w, http.StatusForbidden, "only system_admin may manage admin users")
		return
	}

	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	if err := service.ClearUserAdminRole(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "admin role removed",
		"id":     id,
	})
}

// registerAuthAndAdminUserRoutes registers auth, admin-user, health, and metrics routes.
func registerAuthAndAdminUserRoutes(mux *http.ServeMux, service AdminService, cfg adminRouteConfig, loginLimiter *AdminIPRateLimiter, adminAuthFn func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /admin/v1/auth/login", loginLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogin(w, r, service, cfg)
	})).ServeHTTP)

	mux.HandleFunc("POST /admin/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRefresh(w, r, cfg.tokenMgr)
	})

	mux.HandleFunc("POST /admin/v1/auth/setup", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		handleAdminSetup(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogout(w, r, service, cfg.tokenMgr)
	})

	mux.HandleFunc("GET /admin/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		handleAdminVerify(w, r, cfg.tokenMgr)
	})

	mux.HandleFunc("GET /admin/v1/admin-users", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		handleListAdminUsers(w, r, service)
	}))

	mux.HandleFunc("POST /admin/v1/admin-users", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		handleCreateAdminUser(w, r, service)
	}))

	mux.HandleFunc("DELETE /admin/v1/admin-users/{id}", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAdminUser(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/health", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		handleAdminHealth(w, r, service)
	}))

	mux.HandleFunc("GET /admin/v1/system/metrics", adminAuthFn(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		memPct := 0.0
		if ms.Sys > 0 {
			memPct = float64(ms.HeapInuse) / float64(ms.Sys) * 100
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"memory": map[string]any{
				"heap_inuse_bytes": ms.HeapInuse,
				"heap_sys_bytes":   ms.HeapSys,
				"sys_bytes":        ms.Sys,
				"alloc_bytes":      ms.Alloc,
				"gc_runs":          ms.NumGC,
				"usage_pct":        memPct,
			},
			"goroutines": runtime.NumGoroutine(),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
	}))
}
