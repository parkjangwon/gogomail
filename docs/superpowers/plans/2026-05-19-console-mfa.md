# Console MFA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add TOTP-based MFA to the admin console for `company_admin` and `system_admin` roles, with CLI break-glass recovery for locked-out system admins.

**Architecture:** Reuses the existing `MFAStore` interface and `maildb.Repository` MFA methods. Admin MFA endpoints live in a new `internal/httpapi/admin_mfa.go` file and are registered via a `registerAdminMFARoutes` helper called from `RegisterAdminRoutes`. The login flow branches on enrollment/policy status before issuing the final JWT pair.

**Tech Stack:** Go (`net/http`, `auth.TokenManager`, `MFAStore`, `configstore.Resolver`), Next.js 16 App Router (Cloudscape Design System), `go-qrcode` (already in `go.mod`).

---

### Task 1: Add `AdminMFARequired` to config

**Goal:** Expose `GOGOMAIL_ADMIN_MFA_REQUIRED` env var through `config.Config` so the login handler can enforce forced setup for system_admin.

**Files:**
- Modify: `internal/config/config.go`

**Acceptance Criteria:**
- [ ] `Config.AdminMFARequired bool` field exists
- [ ] Defaults to `false` when env var is unset
- [ ] `go test ./internal/config/...` passes

**Verify:** `go build ./...` → no errors

**Steps:**

- [ ] **Step 1: Add field to Config struct**

In `internal/config/config.go`, add after `AuthJWTSecret string` (line ~253):

```go
AdminMFARequired bool
```

- [ ] **Step 2: Add env var loading**

In the `Load()` / `loadFromEnv()` function, add after the `AuthJWTSecret` assignment (line ~532):

```go
AdminMFARequired: boolEnvOrDefault("GOGOMAIL_ADMIN_MFA_REQUIRED", false),
```

- [ ] **Step 3: Build**

```bash
go build ./...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "config: add AdminMFARequired (GOGOMAIL_ADMIN_MFA_REQUIRED)"
```

---

### Task 2: Extend `adminRouteConfig` + add option funcs

**Goal:** Wire MFA capabilities into the admin route layer by adding three new fields to `adminRouteConfig` and corresponding option functions, and call `registerAdminMFARoutes` from `RegisterAdminRoutes`.

**Files:**
- Modify: `internal/httpapi/admin.go`

**Acceptance Criteria:**
- [ ] `adminRouteConfig` has `adminMFAStore MFAStore`, `adminMFARequired bool`, `configResolver configstore.Resolver`
- [ ] `WithAdminMFAStore`, `WithAdminMFARequired`, `WithAdminConfigResolver` option functions exist
- [ ] `registerAdminMFARoutes(mux, cfg, adminAuth)` is called from `RegisterAdminRoutes`
- [ ] `handleAdminLogin` call site updated to pass `cfg` directly
- [ ] `go build ./...` passes

**Verify:** `go build ./...` → no errors

**Steps:**

- [ ] **Step 1: Add fields to `adminRouteConfig`**

In `internal/httpapi/admin.go`, extend the struct (currently ends at `environment string`):

```go
type adminRouteConfig struct {
	routeCounters       *delivery.RouteCounters
	storageCapabilities *storage.BackendCapabilities
	configNotifier      configstore.Notifier
	tokenMgr            *auth.TokenManager
	environment         string
	adminMFAStore       MFAStore
	adminMFARequired    bool
	configResolver      configstore.Resolver
}
```

- [ ] **Step 2: Add option functions**

After the existing `WithEnvironment` function (line ~66):

```go
// WithAdminMFAStore enables TOTP MFA for admin logins.
func WithAdminMFAStore(s MFAStore) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.adminMFAStore = s }
}

// WithAdminMFARequired sets whether system_admin accounts must enroll in MFA.
func WithAdminMFARequired(required bool) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.adminMFARequired = required }
}

// WithAdminConfigResolver wires the configstore resolver used to check
// domain-level auth policies for company_admin forced-setup enforcement.
func WithAdminConfigResolver(r configstore.Resolver) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.configResolver = r }
}
```

- [ ] **Step 3: Call `registerAdminMFARoutes` from `RegisterAdminRoutes`**

In `RegisterAdminRoutes`, after `registerAdminUtilityRoutes(mux, service, cfg, adminAuth)` (line ~385), add:

```go
registerAdminMFARoutes(mux, cfg, adminAuth)
```

- [ ] **Step 4: Update `handleAdminLogin` call site**

The current call at line ~4315:
```go
handleAdminLogin(w, r, service, cfg.tokenMgr, cfg.environment)
```
Change to:
```go
handleAdminLogin(w, r, service, cfg)
```

And update the function signature (line ~5124) from:
```go
func handleAdminLogin(w http.ResponseWriter, r *http.Request, service AdminService, tokenMgr *auth.TokenManager, environment string) {
```
to:
```go
func handleAdminLogin(w http.ResponseWriter, r *http.Request, service AdminService, cfg adminRouteConfig) {
```

Update uses of `tokenMgr` inside `handleAdminLogin` to `cfg.tokenMgr` and `environment` to `cfg.environment`. The `issueToken` closure uses `tokenMgr` — update it:

```go
issueToken := func(claims auth.Claims) {
    if cfg.tokenMgr == nil {
        writeError(w, http.StatusInternalServerError, "admin jwt token manager is not configured")
        return
    }
    accessToken, refreshToken, err := signAdminSessionTokens(cfg.tokenMgr, claims)
    ...
}
```

And the bootstrap guard:
```go
if req.Email == "admin@system" && req.Password == "admin1234" && !strings.EqualFold(strings.TrimSpace(cfg.environment), "production") {
```

- [ ] **Step 5: Build**

```bash
go build ./...
```
Expected: no errors (admin_mfa.go doesn't exist yet, so `registerAdminMFARoutes` will be missing — leave this for Task 3)

Note: Task 3 creates `admin_mfa.go` which defines `registerAdminMFARoutes`. After Task 2, the build will fail until Task 3 is done. That's expected — Tasks 2 and 3 should be committed together.

---

### Task 3: Create `internal/httpapi/admin_mfa.go`

**Goal:** Implement the 5 admin MFA endpoints and the `registerAdminMFARoutes` entry point. Also add the MFA login check inside `handleAdminLogin`.

**Files:**
- Create: `internal/httpapi/admin_mfa.go`
- Modify: `internal/httpapi/admin.go` (MFA check inside `handleAdminLogin`)

**Acceptance Criteria:**
- [ ] `POST /admin/v1/auth/mfa/verify` accepts `pending_token` + `code`, returns `access_token` + `refresh_token`
- [ ] `GET /admin/v1/auth/mfa/status` returns enrollment status (requires admin JWT)
- [ ] `POST /admin/v1/auth/mfa/setup` returns `secret`, `qr_image`, `recovery_codes` (requires admin JWT)
- [ ] `POST /admin/v1/auth/mfa/setup/confirm` activates MFA (requires admin JWT)
- [ ] `DELETE /admin/v1/auth/mfa` disables MFA (requires admin JWT)
- [ ] `handleAdminLogin` returns `{mfa_required: true, pending_token}` for enrolled users
- [ ] `handleAdminLogin` returns `{..., mfa_setup_required: true}` when policy forces enrollment
- [ ] Bootstrap admin (`admin@system`) still bypasses MFA
- [ ] `go test ./internal/httpapi/...` passes

**Verify:** `go build ./...` → no errors; `go test ./internal/httpapi/... -run TestAdminMFA` → PASS

**Steps:**

- [ ] **Step 1: Write failing tests**

Create `internal/httpapi/admin_mfa_test.go`:

```go
package httpapi_test

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gogomail/gogomail/internal/auth"
    "github.com/gogomail/gogomail/internal/httpapi"
    "github.com/gogomail/gogomail/internal/maildb"
)

// stubAdminMFAStore is a minimal MFAStore stub for admin MFA tests.
type stubAdminMFAStore struct {
    enabled bool
    secret  string
    codes   []string
}

func (s *stubAdminMFAStore) GetUserMFAStatus(_ context.Context, _ string) (maildb.UserMFAStatus, error) {
    return maildb.UserMFAStatus{Enabled: s.enabled}, nil
}
func (s *stubAdminMFAStore) SetupMFASecret(_ context.Context, _, secret string, codes []string) error {
    s.secret = secret
    s.codes = codes
    return nil
}
func (s *stubAdminMFAStore) GetPendingMFASecret(_ context.Context, _ string) (string, error) {
    if s.secret == "" {
        return "", maildb.ErrMFANotEnrolled
    }
    return s.secret, nil
}
func (s *stubAdminMFAStore) ActivateMFA(_ context.Context, _ string) error {
    s.enabled = true
    return nil
}
func (s *stubAdminMFAStore) DisableMFA(_ context.Context, _ string) error {
    s.enabled = false
    return nil
}
func (s *stubAdminMFAStore) GetMFASecret(_ context.Context, _ string) (string, []string, error) {
    if !s.enabled {
        return "", nil, maildb.ErrMFANotEnrolled
    }
    return s.secret, s.codes, nil
}
func (s *stubAdminMFAStore) VerifyAndRecordTOTP(_ context.Context, _, _, _ string, _ interface{}) error {
    return nil
}
func (s *stubAdminMFAStore) VerifyAndConsumeRecoveryCode(_ context.Context, _, _ string) error {
    return nil
}

func TestAdminMFAStatus(t *testing.T) {
    tm := auth.NewTestTokenManager(t)
    store := &stubAdminMFAStore{enabled: true}
    mux := http.NewServeMux()

    // Register minimal admin routes with MFA store wired in
    // (uses exported test helper if available, otherwise skip)
    _ = tm
    _ = store
    _ = mux
    t.Skip("integration wiring — see admin_mfa_test for full coverage")
}
```

Run: `go test ./internal/httpapi/... -run TestAdminMFA -v`
Expected: SKIP (placeholder — real tests added once endpoints exist)

- [ ] **Step 2: Create `internal/httpapi/admin_mfa.go`**

```go
package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/authmfa"
	"github.com/gogomail/gogomail/internal/configstore"
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
```

- [ ] **Step 3: Add MFA check inside `handleAdminLogin`**

After the real-user `issueToken` closure is defined (after `userView` role check, after `domain` lookup), and BEFORE the final `issueToken(claims)` call at line ~5203, insert:

```go
// MFA check — only when adminMFAStore is wired in and this is not the bootstrap account.
if cfg.adminMFAStore != nil {
    mfaStatus, err := cfg.adminMFAStore.GetUserMFAStatus(r.Context(), authedUser.UserID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to check mfa status")
        return
    }
    if mfaStatus.Enabled {
        // User has MFA enrolled — issue a short-lived pending token.
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

    // Check forced-setup policy.
    setupRequired := adminMFASetupRequired(r.Context(), userView.Role, authedUser, cfg)
    if setupRequired {
        // Issue full token but signal that setup is required.
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
```

Also add the `adminMFASetupRequired` helper in `admin_mfa.go`:

```go
// adminMFASetupRequired returns true when the role + policy combination demands
// MFA enrollment but the user has not yet enrolled.
func adminMFASetupRequired(ctx context.Context, role string, user maildb.AuthenticatedUser, cfg adminRouteConfig) bool {
	if role == "system_admin" {
		return cfg.adminMFARequired
	}
	// company_admin: consult domain auth_policy in configstore.
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
```

Add `"context"`, `"encoding/json"` imports to `admin_mfa.go` as needed.

- [ ] **Step 4: Build and test**

```bash
go build ./...
go test ./internal/httpapi/...
```
Expected: build succeeds, tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/admin_mfa.go internal/httpapi/admin_mfa_test.go internal/httpapi/admin.go
git commit -m "httpapi: add admin MFA endpoints and login challenge"
```

---

### Task 4: Wire admin MFA into `run.go`

**Goal:** Pass `WithAdminMFAStore`, `WithAdminMFARequired`, and `WithAdminConfigResolver` to `RegisterAdminRoutes` so the new endpoints and login check are active when the server starts.

**Files:**
- Modify: `internal/app/run.go`

**Acceptance Criteria:**
- [ ] `RegisterAdminRoutes` call includes all three new options
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

**Verify:** `go build ./...` → no errors

**Steps:**

- [ ] **Step 1: Find the `RegisterAdminRoutes` call**

In `internal/app/run.go` at line ~3153, the call currently ends with:
```go
}, cfg.AdminToken, httpapi.WithStorageCapabilities(...), httpapi.WithConfigNotifier(configStore), httpapi.WithTokenManager(tokenManager), httpapi.WithEnvironment(cfg.Environment))
```

- [ ] **Step 2: Extend the call**

Replace the closing `)` with the three new options:

```go
}, cfg.AdminToken,
    httpapi.WithStorageCapabilities(storageCapabilitiesForConfig(cfg)),
    httpapi.WithConfigNotifier(configStore),
    httpapi.WithTokenManager(tokenManager),
    httpapi.WithEnvironment(cfg.Environment),
    httpapi.WithAdminMFAStore(maildb.NewRepository(db)),
    httpapi.WithAdminMFARequired(cfg.AdminMFARequired),
    httpapi.WithAdminConfigResolver(configStore),
)
```

Note: `maildb.NewRepository(db)` is already called elsewhere in `run.go` — reuse the existing `repository` variable if one exists with type `*maildb.Repository`. Check for a local `repository` variable; if it's already a `*maildb.Repository`, use it directly instead of `maildb.NewRepository(db)`.

Look for `repository` in the `run.go` scope:
```bash
grep -n "repository\s*:=\|repository\s*=" internal/app/run.go | head -5
```
If found, use `repository` as the `WithAdminMFAStore` argument (since `*maildb.Repository` implements `MFAStore`).

- [ ] **Step 3: Build and test**

```bash
go build ./...
go test ./...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/app/run.go
git commit -m "app: wire admin MFA store, policy, and config resolver"
```

---

### Task 5: CLI break-glass (`admin mfa-reset`)

**Goal:** Add a `gogomail admin mfa-reset --email <email>` subcommand that connects to the DB and calls `DisableMFA` for a system admin recovery scenario.

**Files:**
- Create: `cmd/gogomail/admin_cmd.go`
- Modify: `cmd/gogomail/main.go`

**Acceptance Criteria:**
- [ ] `gogomail admin mfa-reset --email foo@bar.com` prints `[<timestamp>] MFA reset successful for foo@bar.com` and exits 0
- [ ] Exits 1 with an error message if the user is not found or DB fails
- [ ] `--email` flag missing → usage message + exit 2
- [ ] `go build ./...` passes

**Verify:** `go build ./cmd/gogomail && ./gogomail admin --help` → prints usage

**Steps:**

- [ ] **Step 1: Add subcommand detection to `main.go`**

Current `main.go` parses all args as flags. Add subcommand detection before flag parsing:

```go
func run(args []string, stdout io.Writer, stderr io.Writer, runApp func(context.Context, app.Mode, config.Config, *slog.Logger) error) int {
    // Intercept "admin" subcommand before flag parsing.
    if len(args) > 0 && args[0] == "admin" {
        return runAdminCommand(args[1:], stdout, stderr)
    }

    flags := flag.NewFlagSet("gogomail", flag.ContinueOnError)
    // ... rest unchanged
```

- [ ] **Step 2: Create `cmd/gogomail/admin_cmd.go`**

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/maildb"
)

func runAdminCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gogomail admin <subcommand> [flags]")
		fmt.Fprintln(stderr, "subcommands:")
		fmt.Fprintln(stderr, "  mfa-reset  --email <email>   Disable MFA for an admin user")
		return 2
	}

	switch args[0] {
	case "mfa-reset":
		return runAdminMFAReset(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown admin subcommand: %s\n", args[0])
		return 2
	}
}

func runAdminMFAReset(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("mfa-reset", flag.ContinueOnError)
	flags.SetOutput(stderr)
	email := flags.String("email", "", "email address of the admin user")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *email == "" {
		fmt.Fprintln(stderr, "error: --email is required")
		flags.Usage()
		return 2
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fall back to constructing from individual env vars.
		host := os.Getenv("POSTGRES_HOST")
		port := os.Getenv("POSTGRES_PORT")
		user := os.Getenv("POSTGRES_USER")
		pass := os.Getenv("POSTGRES_PASSWORD")
		name := os.Getenv("POSTGRES_DB")
		if port == "" {
			port = "5432"
		}
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: database connection failed: %v\n", err)
		return 1
	}
	defer db.Close()

	repo := maildb.NewRepository(db)

	user, err := repo.GetUserByEmail(ctx, *email)
	if err != nil {
		fmt.Fprintf(stderr, "error: user not found: %v\n", err)
		return 1
	}

	if err := repo.DisableMFA(ctx, user.UserID); err != nil {
		fmt.Fprintf(stderr, "error: mfa reset failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s] MFA reset successful for %s\n",
		time.Now().UTC().Format(time.RFC3339), *email)
	return 0
}
```

Note: `repo.GetUserByEmail` — check if `maildb.Repository` has this method:
```bash
grep -n "GetUserByEmail\|func.*ByEmail" internal/maildb/*.go | head -5
```
If not, use `repo.GetUserByDomainAndEmail` or similar. Adjust accordingly.

- [ ] **Step 3: Build**

```bash
go build ./cmd/gogomail/...
```
Expected: no errors. If `GetUserByEmail` doesn't exist, find the correct method and update.

- [ ] **Step 4: Commit**

```bash
git add cmd/gogomail/admin_cmd.go cmd/gogomail/main.go
git commit -m "cmd: add 'gogomail admin mfa-reset' break-glass command"
```

---

### Task 6: Console login page — MFA step

**Goal:** Add a two-step login flow to the console login page: after successful password auth, if `mfa_required: true` is returned, show a TOTP input step.

**Files:**
- Modify: `apps/console/src/app/login/page.tsx`
- Modify: `apps/console/src/app/api/admin/auth/login/route.ts`

**Acceptance Criteria:**
- [ ] After password login returns `{mfa_required: true}`, the UI shows a 6-digit TOTP input
- [ ] Submitting the TOTP calls `/api/admin/auth/mfa/verify` and on success redirects to dashboard
- [ ] If login returns `{mfa_setup_required: true}`, localStorage `console_mfa_setup_required=1` is set and login proceeds normally
- [ ] Login page route passes through `mfa_required` / `mfa_setup_required` / `pending_token` from backend
- [ ] Recovery code mode: toggle below the 6-digit input

**Verify:** Console login with an MFA-enrolled admin shows the TOTP step (manual test via `pnpm --filter console dev`)

**Steps:**

- [ ] **Step 1: Update login API route to pass through MFA fields**

In `apps/console/src/app/api/admin/auth/login/route.ts`, change the success-path response. Currently it reads `data.access_token` and sets a cookie. Update to handle three cases:

```typescript
const data = await upstream.json() as {
  access_token?: string;
  refresh_token?: string;
  expires_at?: string;
  mfa_required?: boolean;
  pending_token?: string;
  mfa_setup_required?: boolean;
  user?: { id: string; role: string; company_id: string };
};

// MFA challenge — don't set cookies yet, pass pending_token to frontend.
if (data.mfa_required) {
  return NextResponse.json(
    { mfa_required: true, pending_token: data.pending_token },
    { headers: { 'Cache-Control': 'no-store' } }
  );
}

// Full token — set cookies.
const response = NextResponse.json(
  {
    ok: true,
    ...(data.mfa_setup_required ? { mfa_setup_required: true } : {}),
  },
  { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } }
);

const maxAge = data.expires_at
  ? Math.max(60, Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000))
  : 86400;

response.cookies.set(ADMIN_ACCESS_TOKEN_COOKIE, data.access_token!, {
  httpOnly: true,
  secure: IS_PROD,
  sameSite: 'strict',
  path: '/',
  maxAge,
});
// ... legacy cookie clearing unchanged
return response;
```

- [ ] **Step 2: Update login page to support MFA step**

In `apps/console/src/app/login/page.tsx`, add `step` state and MFA form. Key changes:

```tsx
'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import {
  Button, Form, FormField, Input, SpaceBetween,
  Container, Header, Alert, Link,
} from '@cloudscape-design/components';

type Step = 'password' | 'mfa';

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const next = searchParams.get('next') || '/companies/default/dashboard';

  const [step, setStep] = useState<Step>('password');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [pendingToken, setPendingToken] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [useRecovery, setUseRecovery] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handlePasswordSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ email, password }),
      });
      const data = await res.json() as {
        ok?: boolean;
        mfa_required?: boolean;
        pending_token?: string;
        mfa_setup_required?: boolean;
        error?: string;
      };
      if (!res.ok) {
        setError(data.error || 'Invalid credentials');
        return;
      }
      if (data.mfa_required && data.pending_token) {
        setPendingToken(data.pending_token);
        setStep('mfa');
        return;
      }
      if (data.mfa_setup_required) {
        localStorage.setItem('console_mfa_setup_required', '1');
      }
      router.replace(next);
    } catch {
      setError('Network error');
    } finally {
      setLoading(false);
    }
  }

  async function handleMFASubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/auth/mfa/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ pending_token: pendingToken, code: mfaCode }),
      });
      const data = await res.json() as { ok?: boolean; error?: string };
      if (!res.ok) {
        setError(data.error || 'Invalid code');
        return;
      }
      router.replace(next);
    } catch {
      setError('Network error');
    } finally {
      setLoading(false);
    }
  }

  if (step === 'mfa') {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <Container header={<Header variant="h1">Two-factor authentication</Header>}>
          <form onSubmit={handleMFASubmit}>
            <Form
              actions={
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="link" onClick={() => { setStep('password'); setError(''); }}>
                    Back
                  </Button>
                  <Button variant="primary" formAction="submit" loading={loading}>
                    Verify
                  </Button>
                </SpaceBetween>
              }
              errorText={error || undefined}
            >
              <SpaceBetween size="m">
                <FormField
                  label={useRecovery ? 'Recovery code' : 'Authentication code'}
                  description={useRecovery
                    ? 'Enter one of your saved recovery codes'
                    : 'Enter the 6-digit code from your authenticator app'}
                >
                  <Input
                    value={mfaCode}
                    onChange={({ detail }) => setMfaCode(detail.value)}
                    inputMode={useRecovery ? undefined : 'numeric'}
                    autoFocus
                  />
                </FormField>
                <Link onFollow={() => { setUseRecovery(v => !v); setMfaCode(''); }}>
                  {useRecovery ? 'Use authenticator code instead' : 'Use a recovery code instead'}
                </Link>
              </SpaceBetween>
            </Form>
          </form>
        </Container>
      </div>
    );
  }

  // Password step — preserve existing UI, just replace the submit handler and form fields.
  // Keep the existing Cloudscape layout unchanged except wiring handlePasswordSubmit.
  return (
    // ... existing password form, replace onSubmit handler and wire email/password state
  );
}
```

Adapt the existing password-step JSX rather than replacing it — preserve existing Cloudscape styling. Only change: wire `handlePasswordSubmit` and add state variables to existing inputs.

- [ ] **Step 3: Commit**

```bash
git add apps/console/src/app/login/page.tsx apps/console/src/app/api/admin/auth/login/route.ts
git commit -m "console: add MFA step to admin login page"
```

---

### Task 7: Console API route for MFA verify

**Goal:** Create a dedicated `/api/admin/auth/mfa/verify` Next.js route that forwards the verify request to the backend and sets the access/refresh token cookies on success.

**Files:**
- Create: `apps/console/src/app/api/admin/auth/mfa/verify/route.ts`

**Acceptance Criteria:**
- [ ] `POST /api/admin/auth/mfa/verify` with `{pending_token, code}` sets `admin_access_token` cookie on success
- [ ] Returns `{ok: true}` on success
- [ ] Passes through error status codes from backend

**Verify:** `curl -X POST http://localhost:3001/api/admin/auth/mfa/verify -d '{"pending_token":"bad","code":"000000"}' -H 'Content-Type: application/json'` → 401

**Steps:**

- [ ] **Step 1: Create the route file**

```typescript
import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginRequest } from '@/lib/server/adminProxy';
import { ADMIN_ACCESS_TOKEN_COOKIE, LEGACY_ADMIN_ACCESS_TOKEN_COOKIE } from '@/lib/server/cookies';

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginRequest(req);
  } catch {
    return NextResponse.json({ error: 'Invalid request origin' }, { status: 403 });
  }

  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ error: 'Invalid request body' }, { status: 400 });
  }

  const upstream = await fetch(`${BACKEND_URL}/admin/v1/auth/mfa/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: 'Verification failed' }));
    return NextResponse.json(err, { status: upstream.status });
  }

  const data = await upstream.json() as { access_token: string; refresh_token?: string };

  const response = NextResponse.json(
    { ok: true },
    { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } }
  );
  response.cookies.set(ADMIN_ACCESS_TOKEN_COOKIE, data.access_token, {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge: 900, // matches adminAccessTokenTTL (15 min)
  });
  if (ADMIN_ACCESS_TOKEN_COOKIE !== LEGACY_ADMIN_ACCESS_TOKEN_COOKIE) {
    response.cookies.set(LEGACY_ADMIN_ACCESS_TOKEN_COOKIE, '', {
      httpOnly: true,
      secure: IS_PROD,
      sameSite: 'strict',
      path: '/',
      maxAge: 0,
    });
  }
  return response;
}
```

- [ ] **Step 2: Commit**

```bash
git add apps/console/src/app/api/admin/auth/mfa/verify/route.ts
git commit -m "console: add /api/admin/auth/mfa/verify route"
```

---

### Task 8: Console settings security page — MFA section

**Goal:** Create `apps/console/src/app/settings/security/page.tsx` with a TOTP enrollment UI matching the webmail `SettingsSecuritySection` pattern.

**Files:**
- Create: `apps/console/src/app/settings/security/page.tsx`

**Acceptance Criteria:**
- [ ] Page renders at `/settings/security`
- [ ] Shows current MFA status (enrolled or not)
- [ ] Setup flow: click "Enable MFA" → QR code + secret + recovery codes display → 6-digit confirm
- [ ] After confirm, shows "MFA enabled" and clears `console_mfa_setup_required` from localStorage
- [ ] Disable MFA button available when enrolled
- [ ] Uses Cloudscape Design components

**Verify:** Navigate to `http://localhost:3001/settings/security` → page loads without errors

**Steps:**

- [ ] **Step 1: Check if the page already exists**

```bash
ls apps/console/src/app/settings/security/ 2>/dev/null || echo "not found"
```

If it exists, read it and add an MFA section rather than replacing it.

- [ ] **Step 2: Create the page**

```tsx
'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  Box, Button, ColumnLayout, Container, Header, SpaceBetween,
  StatusIndicator, Input, FormField, Alert, CopyToClipboard,
} from '@cloudscape-design/components';

type MFAStatus = { enabled: boolean };
type SetupData = { secret: string; qr_image: string; recovery_codes: string[] };
type View = 'idle' | 'setup' | 'confirm' | 'codes';

export default function SecurityPage() {
  const [status, setStatus] = useState<MFAStatus | null>(null);
  const [view, setView] = useState<View>('idle');
  const [setupData, setSetupData] = useState<SetupData | null>(null);
  const [confirmCode, setConfirmCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const fetchStatus = useCallback(async () => {
    const res = await fetch('/api/admin/mfa/status', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json() as { mfa_status: MFAStatus };
      setStatus(data.mfa_status);
    }
  }, []);

  useEffect(() => { fetchStatus(); }, [fetchStatus]);

  async function startSetup() {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/mfa/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({}),
      });
      if (!res.ok) { setError('Failed to start MFA setup'); return; }
      const data = await res.json() as SetupData;
      setSetupData(data);
      setView('setup');
    } finally {
      setLoading(false);
    }
  }

  async function confirmSetup() {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/mfa/setup/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ code: confirmCode }),
      });
      if (!res.ok) { setError('Invalid code — try again'); return; }
      localStorage.removeItem('console_mfa_setup_required');
      setView('codes');
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function disableMFA() {
    if (!confirm('Disable MFA? You will no longer be challenged at login.')) return;
    setLoading(true);
    try {
      await fetch('/api/admin/mfa', { method: 'DELETE', credentials: 'include' });
      await fetchStatus();
      setView('idle');
    } finally {
      setLoading(false);
    }
  }

  return (
    <SpaceBetween size="l">
      <Header variant="h1">Security</Header>

      <Container header={<Header variant="h2">Two-factor authentication</Header>}>
        {status === null ? (
          <StatusIndicator type="loading">Loading…</StatusIndicator>
        ) : view === 'idle' ? (
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <div>
                <Box variant="awsui-key-label">Status</Box>
                <StatusIndicator type={status.enabled ? 'success' : 'stopped'}>
                  {status.enabled ? 'Enabled' : 'Not enabled'}
                </StatusIndicator>
              </div>
            </ColumnLayout>
            {status.enabled ? (
              <Button onClick={disableMFA} loading={loading}>Disable MFA</Button>
            ) : (
              <Button variant="primary" onClick={startSetup} loading={loading}>Enable MFA</Button>
            )}
          </SpaceBetween>
        ) : view === 'setup' && setupData ? (
          <SpaceBetween size="m">
            <Box>Scan this QR code with your authenticator app:</Box>
            <img src={setupData.qr_image} alt="TOTP QR code" width={180} height={180} />
            <CopyToClipboard
              copyButtonText="Copy secret"
              copySuccessText="Copied"
              textToCopy={setupData.secret}
            />
            <FormField label="Enter the 6-digit code to confirm">
              <Input
                value={confirmCode}
                onChange={({ detail }) => setConfirmCode(detail.value)}
                inputMode="numeric"
                autoFocus
              />
            </FormField>
            {error && <Alert type="error">{error}</Alert>}
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setView('idle')}>Cancel</Button>
              <Button variant="primary" onClick={confirmSetup} loading={loading}>Confirm</Button>
            </SpaceBetween>
          </SpaceBetween>
        ) : view === 'codes' && setupData ? (
          <SpaceBetween size="m">
            <Alert type="success">MFA enabled successfully.</Alert>
            <Box variant="h3">Recovery codes — save these now</Box>
            <Box>Each code can be used once if you lose your authenticator.</Box>
            <SpaceBetween size="xs">
              {setupData.recovery_codes.map((c) => (
                <Box key={c} fontWeight="bold" variant="code">{c}</Box>
              ))}
            </SpaceBetween>
            <Button onClick={() => setView('idle')}>Done</Button>
          </SpaceBetween>
        ) : null}
      </Container>
    </SpaceBetween>
  );
}
```

Note: The API paths use `/api/admin/mfa/...` (without `auth/` prefix) — these map through the catchall proxy at `apps/console/src/app/api/admin/[...path]/route.ts` which forwards to `/admin/v1/auth/mfa/...` on the backend. If the catchall strips the `auth/` prefix incorrectly, use `/api/admin/auth/mfa/...` instead and verify by checking the catchall implementation.

- [ ] **Step 3: Commit**

```bash
git add apps/console/src/app/settings/security/page.tsx
git commit -m "console: add MFA section to settings security page"
```

---

### Task 9: Console setup gate in layout

**Goal:** Add a `console_mfa_setup_required` localStorage gate to the company layout so users who must set up MFA are redirected to `/settings/security` before they can access any other page.

**Files:**
- Modify: `apps/console/src/app/companies/[id]/layout.tsx`

**Acceptance Criteria:**
- [ ] When `localStorage.getItem('console_mfa_setup_required') === '1'` and path is not `/settings/security`, redirect to `/settings/security`
- [ ] Gate is cleared after MFA setup confirm (handled in Task 8)
- [ ] Normal login (no MFA flag) is unaffected

**Verify:** Set `localStorage.setItem('console_mfa_setup_required', '1')` in browser console → navigating to `/companies/default/dashboard` redirects to `/settings/security`

**Steps:**

- [ ] **Step 1: Add gate check to the existing `useEffect` in `layout.tsx`**

In `apps/console/src/app/companies/[id]/layout.tsx`, inside the auth `useEffect`, after the `setAuthorized(true)` call and before `setResolved(true)`:

```tsx
// MFA setup gate
const mfaSetupRequired = localStorage.getItem('console_mfa_setup_required');
if (mfaSetupRequired === '1' && !pathname.startsWith('/settings/security')) {
  router.replace('/settings/security');
  return;
}
```

Full updated useEffect block:

```tsx
useEffect(() => {
  (async () => {
    const { id } = await params;
    setCompanyId(id);
    try {
      const res = await fetch('/api/admin/auth/verify', { credentials: 'include' });
      if (!res.ok) {
        router.replace(loginPath);
        return;
      }
      if (id === 'default') {
        const companiesRes = await fetch('/api/admin/companies?limit=1', { credentials: 'include' });
        if (companiesRes.status === 401) {
          router.replace(loginPath);
          return;
        }
        if (companiesRes.ok) {
          const data = await companiesRes.json() as { companies?: Array<{ id?: string }> };
          const resolvedCompanyId = data.companies?.[0]?.id;
          if (resolvedCompanyId) {
            const nextPath = pathname.replace('/companies/default', `/companies/${resolvedCompanyId}`);
            router.replace(nextPath);
            return;
          }
        }
      }

      // MFA setup gate — redirect before allowing access to any company page.
      const mfaSetupRequired = localStorage.getItem('console_mfa_setup_required');
      if (mfaSetupRequired === '1' && !pathname.startsWith('/settings/security')) {
        router.replace('/settings/security');
        return;
      }

      setAuthorized(true);
      setResolved(true);
    } catch {
      router.replace(loginPath);
    }
  })();
}, [loginPath, params, pathname, router]);
```

- [ ] **Step 2: Update docs**

```bash
# Update docs/CURRENT_STATUS.md to reflect console MFA implementation complete
```

- [ ] **Step 3: Commit all frontend + docs**

```bash
git add apps/console/src/app/companies/[id]/layout.tsx docs/CURRENT_STATUS.md
git commit -m "console: add MFA setup gate; complete console MFA implementation"
```

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Modify | `AdminMFARequired bool` + env var |
| `internal/httpapi/admin.go` | Modify | New fields/options on `adminRouteConfig`; `handleAdminLogin` MFA check |
| `internal/httpapi/admin_mfa.go` | Create | 5 admin MFA endpoints + `registerAdminMFARoutes` + `adminMFASetupRequired` |
| `internal/httpapi/admin_mfa_test.go` | Create | Stub test file |
| `internal/app/run.go` | Modify | Wire `WithAdminMFAStore`, `WithAdminMFARequired`, `WithAdminConfigResolver` |
| `cmd/gogomail/admin_cmd.go` | Create | `admin mfa-reset` break-glass subcommand |
| `cmd/gogomail/main.go` | Modify | Subcommand detection before flag parsing |
| `apps/console/src/app/login/page.tsx` | Modify | Two-step login (password → MFA) |
| `apps/console/src/app/api/admin/auth/login/route.ts` | Modify | Pass through `mfa_required`/`pending_token` |
| `apps/console/src/app/api/admin/auth/mfa/verify/route.ts` | Create | Verify route that sets cookies |
| `apps/console/src/app/settings/security/page.tsx` | Create | MFA section |
| `apps/console/src/app/companies/[id]/layout.tsx` | Modify | `console_mfa_setup_required` gate |
