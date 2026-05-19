# Console MFA Design

Date: 2026-05-19

## Summary

Add TOTP-based MFA to the admin console for both `company_admin` and `system_admin` roles. Reuses the existing `MFAStore` infrastructure. Includes a CLI break-glass command for system admin recovery.

---

## 1. Enrollment Model

Voluntary opt-in, enforced once enrolled — same model as webmail.

- Enrolled admin → always challenged for TOTP at login
- Unenrolled admin + forced policy → login succeeds but `mfa_setup_required: true` returned; console blocks access to all pages except `/settings/security`
- Unenrolled admin + no forced policy → login proceeds normally

---

## 2. Forced-Setup Policy

### company_admin
Reads the domain's existing `auth_policy` from configstore (`mfa_required: true`). Same policy object already used for end-users.

### system_admin
Controlled by server config / environment variable:

```
GOGOMAIL_ADMIN_MFA_REQUIRED=false   # default: disabled
```

Parsed at startup into `AppConfig`. `false` by default — no forced setup unless explicitly enabled.

---

## 3. Backend Changes

### 3a. Admin login flow (`handleAdminLogin`)

After password verification, before issuing a full token:

```
1. GetUserMFAStatus(userID)
2. If enabled:
     issue pending_token (mfa_pending type, 5 min TTL)
     return {mfa_required: true, pending_token}
3. If not enabled:
     check forced-setup condition (policy or GOGOMAIL_ADMIN_MFA_REQUIRED)
     if forced: issue full token + return {mfa_setup_required: true}
     else: issue full token normally
```

Bootstrap system admin (`admin@system`) skips MFA — dev/test only, blocked in production.

### 3b. New admin MFA endpoints

All require admin JWT auth middleware.

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/v1/auth/mfa/verify` | pending_token + TOTP/recovery → admin JWT pair (access_token + refresh_token, same as normal login response) |
| `GET` | `/admin/v1/auth/mfa/status` | own enrollment status |
| `POST` | `/admin/v1/auth/mfa/setup` | start enrollment, returns `secret`, `qr_image`, `recovery_codes` |
| `POST` | `/admin/v1/auth/mfa/setup/confirm` | confirm first code → activate |
| `DELETE` | `/admin/v1/auth/mfa` | disable own MFA |

`qr_image` is a `data:image/png;base64` URI generated server-side (same as webmail, uses `go-qrcode`).

### 3c. Config

```go
// AppConfig
AdminMFARequired bool // GOGOMAIL_ADMIN_MFA_REQUIRED, default false
```

Passed into `handleAdminLogin` via existing config struct.

---

## 4. CLI Break-Glass

New subcommand in `cmd/gogomail/main.go`:

```bash
gogomail admin mfa-reset --email <email>
```

- Connects to DB using existing env vars (`POSTGRES_HOST`, etc.)
- Calls `DisableMFA(ctx, userID)` on the repository
- Prints result with timestamp to stdout
- Exits with code 1 on failure

**Container usage:**
```bash
docker exec gogomail-backend gogomail admin mfa-reset --email sysadmin@company.com
# → [2026-05-19T12:00:00Z] MFA reset successful for sysadmin@company.com
```

**Kubernetes:**
```bash
kubectl exec -it <pod> -- gogomail admin mfa-reset --email sysadmin@company.com
```

Requires direct container/server access — the access itself is the authorization.

---

## 5. Frontend Changes (Console)

### 5a. Login page (`apps/console/src/app/login/page.tsx`)

Add `step` state: `'password' | 'mfa'`

- On login response with `mfa_required: true` → store `pending_token`, switch to `'mfa'` step
- On login response with `mfa_setup_required: true` → set localStorage flag `console_mfa_setup_required`, proceed to dashboard
- MFA step UI: 6-digit input + submit, recovery code toggle

### 5b. Console API proxy routes

New Next.js API routes under `apps/console/src/app/api/admin/auth/mfa/`:

- `POST /api/admin/auth/mfa/verify`
- `GET /api/admin/auth/mfa/status`
- `POST /api/admin/auth/mfa/setup`
- `POST /api/admin/auth/mfa/setup/confirm`
- `DELETE /api/admin/auth/mfa`

Each proxies to `GOGOMAIL_BACKEND_URL/admin/v1/auth/mfa/...` with the admin JWT cookie forwarded.

### 5c. Setup gate

In the root layout (or middleware):
- Read `console_mfa_setup_required` from localStorage
- If set and current path is not `/settings/security`: redirect to `/settings/security`
- Clear flag after MFA is successfully confirmed

### 5d. Settings security page

Add MFA section to `apps/console/src/app/settings/security/page.tsx` (create if absent):
- Same UX as webmail `SettingsSecuritySection`
- QR scan → 6-digit confirm → recovery codes display
- Uses console admin MFA proxy routes

---

## 6. Data Flow Summary

```
[Login form] --password--> [/api/admin/auth/login]
                                    |
                            [handleAdminLogin]
                                    |
                    ┌───────────────┼───────────────┐
                enrolled        not enrolled      not enrolled
                    |           + forced           + no policy
                    ↓               ↓                   ↓
             pending_token    full token           full token
             mfa_required     mfa_setup_required
                    |               |
            [MFA input step]  [/settings/security]
                    |           (gate active)
            [/admin/v1/auth/mfa/verify]
                    |
               full token
               → dashboard
```

---

## 7. Out of Scope

- WebAuthn / hardware key support
- Per-admin MFA policy (all admins share the same policy per role)
- MFA audit log (separate feature)
- Admin resetting another admin's MFA via UI (only CLI for system_admin recovery)
