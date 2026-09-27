# MCP Security Review — Admin & User MCP Servers

Date: 2026-09-27
Scope: `apps/gogomail-manage-mcp` (admin MCP), `apps/gogomail-user-mcp` (user MCP),
and the backend API-key auth surface they depend on
(`internal/apikeys`, `internal/httpapi`).

Threat model: an MCP server is a confused-deputy. A malicious or compromised AI
client, a prompt-injection embedded in tool output, or (for the SSE transport) a
network attacker must not be able to escalate beyond the principal the server is
supposed to represent.

## TL;DR

The MCP surface is, overall, well-hardened. Input is validated with Zod
everywhere, destructive tools are confirmation-gated, admin paths are
allow-listed, secrets are never logged, outbound calls have timeouts, and the
backend independently re-enforces user binding, scope, and confirmation for
user-scoped API keys (defense in depth — the TypeScript clients are not trusted
as the authority).

Two issues were found and fixed:

| # | Severity | Area | Status |
|---|---|---|---|
| 1 | HIGH | Backend DM routes lacked API-key user binding (dev-mode cross-user IDOR; latent) | **Fixed** |
| 2 | MEDIUM | User MCP Drive download could write to arbitrary local host paths | **Fixed** |

No critical findings. Remaining lower-severity observations are listed with
rationale for deferral.

---

## What was checked

### Admin MCP (`apps/gogomail-manage-mcp`)

- **`src/index.ts` (transports)** — SSE `Authorization: Bearer` auth using a
  constant-time compare (`crypto.timingSafeEqual` with length pre-check);
  `MCP_SECRET` required and enforced `>= 32` bytes for SSE; `Origin` allow-list
  (`MCP_ALLOWED_ORIGINS`); per-session sliding-window rate limit; idle session
  TTL eviction and `MAX_SESSIONS` cap (DNS-rebinding / resource-exhaustion
  defense); 1 MB request-body cap; `Content-Type` must be `application/json`;
  duplicate `Authorization`/`Origin`/`Content-Type`/`sessionId` headers
  rejected; `sessionId` shape-validated before Map lookup; security response
  headers on every path; error messages sanitized and truncated; tool-argument
  logging emits **field names only**, never values.
- **`src/config.ts`** — URL scheme validation (https required off-loopback
  unless explicitly opted in), rejection of embedded URL credentials, single-
  line validation to prevent header/log injection, `GITHUB_REPO` format check,
  host validation.
- **`src/tools/shared.ts`** — `requireConfirm` (exact-match), `withAudit`,
  `writeAuditComment` (audit trail to Suppo ticket or stderr fallback), the
  `ADMIN_API_ALLOWLIST` regex set, and `normalizeAdminPath` (blocks `..`,
  `://`, query, fragment, CRLF; must match the documented allow-list).
- **`src/tools/*.ts`** — every mutating tool requires a `reason`; deletes
  require exact `confirm`; all inputs parsed through Zod; the guarded
  `gogomail_admin_api_request` bridge validates method, normalizes+allow-lists
  the path, requires `reason` for non-reads and `confirm` for `DELETE`.
- **`src/clients/*.ts`** — admin key sent as Bearer, never logged; 30s
  `AbortController` timeout on all fetches; 5xx upstream bodies hidden from the
  agent (logged internally, generic message returned); control chars stripped
  from 4xx bodies. `github.ts` strips user-supplied `repo:`/`org:`/`user:`
  qualifiers so the configured repo remains the trust boundary (no cross-repo
  SSRF via search).

**IDOR note (by design):** the admin MCP authenticates with a single
full-admin operator key. It is *not* tenant-isolated and is not intended to be —
`company_id` / `domain_id` / `user_id` parameters are operator-supplied targets,
not authorization boundaries. This is the correct model for an operator tool and
is not an IDOR.

### User MCP (`apps/gogomail-user-mcp`)

- **`src/config.ts` / `src/client.ts`** — the user access key is sent as Bearer
  and the model is **never** given a way to override the user identity; the
  backend resolves identity from the key.
- **`src/tools/*.ts`** — mutations are confirmation-gated in `basic` mode with
  an exact-match `confirm`, and the confirmation string is forwarded to the
  backend as `X-Gogomail-MCP-Confirm`. `bypass` mode is only honored when the
  server domain policy allows it.
- **Generic bridge (`gogomail_api_request`)** — has a method+path route manifest
  and blocks `/api/v1/auth/*`, `/admin/*`, MCP key-management, password, and
  session-revoke routes. This client-side gate is a convenience; the backend is
  the real authority.

### Backend API-key auth (`internal/apikeys`, `internal/httpapi`)

- **`internal/apikeys/postgres.go` `Verify`** — binds the key to its user,
  enforces the CIDR allow-list, and gates on domain MCP policy (`enabled`,
  `allow_user_access_keys`, `allow_bypass_mode`), `scopesAllowedByDomainPolicy`,
  and the user's own `webmail.mcp.enabled` flag.
- **`internal/httpapi/mail_helpers.go`** — `userScopedAPIKeyUserIDFromRequest`
  binds every request to `info.UserID` and rejects any `user_id` supplied via
  query, header, or body that differs; enforces the required scope
  (`requiredUserAPIKeyScope`) and confirmation (`userAPIKeyConfirmationOK`).
- **`internal/httpapi/admin_middleware.go` + `internal/app/run.go`** —
  `StripInternalHeadersMiddleware` deletes inbound internal `X-Gogomail-*`
  headers (including `X-Gogomail-Resolved-User-ID`) and is wired to run *before*
  `apikeys.Middleware` sets the resolved header, so a client cannot spoof
  attribution or identity headers.

The mail, drafts, threads, attachments, Drive, calendar, contacts, and
notification handlers all bind the user through `userIDFromRequest` /
`bindRequestUserID` → `userScopedAPIKeyUserIDFromRequest`.

---

## Findings

### 1. HIGH — Encrypted DM routes lacked API-key user binding (cross-user IDOR in tokenless deployments)

**Location:** `internal/httpapi/dm.go`, `dmPrincipalFromRequest`.

Every user-facing route family binds an API-key request to the key's owning user
via `userScopedAPIKeyUserIDFromRequest`. The DM route family was the sole
exception: `dmPrincipalFromRequest` had only two branches —

1. `tokenManager != nil` → require a valid JWT bearer token; else
2. fallback → trust `user_id` / `company_id` / `domain_id` **query parameters**
   with no binding.

It never consulted `KeyInfoFromContext`. Consequences:

- In a **tokenless deployment** (`tokenManager == nil`, i.e. `GOGOMAIL_ENV` is
  `development`/`test` with API keys enabled), a valid user MCP key bound to
  user A that also passes `?user_id=B&company_id=…&domain_id=…` would be
  authenticated by `apikeys.Middleware` and then acted upon as **user B** by the
  DM handler — a cross-user IDOR against encrypted DM (read/send/delete
  messages, manage rooms).
- In **production** the config validator requires `GOGOMAIL_AUTH_JWT_SECRET`
  (`internal/config/validate.go`), so `tokenManager != nil` and an API key
  (not a JWT) fails closed with `401`. The safe behavior there was *incidental*,
  not intentional, and the user MCP's DM tools were silently non-functional with
  an API key.

**Fix:** `dmPrincipalFromRequest` now explicitly rejects any request carrying a
user-scoped API key (`KeyInfoFromContext` with a non-empty `UserID`) with
`403 Forbidden` **before** the JWT/query-param branches. This makes DM
fail-closed by design in every deployment and eliminates the tokenless IDOR. A
full API-key binding for DM was intentionally *not* added: encrypted DM needs a
company scope that user-scoped keys do not carry, and adding it would require a
principal-resolution change to `DMService` — out of scope for a minimal security
fix and explicitly not an auth-architecture redesign.

**Regression tests** (`internal/httpapi/dm_test.go`):
- `TestDMHandlerRejectsUserScopedAPIKey` — a key bound to `victim` posting to a
  DM room with `?user_id=attacker-target` gets `403` and the DM service is never
  invoked.
- `TestDMHandlerAllowsAPIKeylessTokenlessRequest` — control: a tokenless,
  key-less request with query-param identity still reaches the service.

### 2. MEDIUM — User MCP Drive download could write to arbitrary local host paths

**Location:** `apps/gogomail-user-mcp/src/tools/drive.ts`, `saveDownloadIfRequested`
(used by `gogomail_drive_download` and `gogomail_drive_download_share_link`).

The tool accepted `save_to_path` and wrote the downloaded bytes to
`resolve(save_to_path)` with no restriction on the destination. On the machine
running the MCP server, a malicious or prompt-injected agent could write
attacker-controlled bytes to any path the process can write — e.g.
`~/.ssh/authorized_keys`, shell rc files, or cron directories — a local
code-execution / persistence primitive. The `basic`-mode confirmation was **not**
an effective safeguard because the required confirmation string embeds the same
attacker-chosen path (`save download <path>`), so a confused agent auto-satisfies
it.

**Fix:** local saves are now confined to an operator-configured allow-list root:

- New optional env var `GOGOMAIL_MCP_DOWNLOAD_DIR`.
- When **unset**, local saving is **disabled** — the tool still returns the file
  bytes in-band (`body_base64` / `body_text`) but refuses to write to disk.
- When **set**, `save_to_path` must resolve inside that directory subtree;
  absolute paths and `..` traversal that escape the root are rejected.

The env var is read lazily inside `drive.ts` (rather than through the `config`
singleton) so that importing a tool module does not trigger full startup env
validation — preserving the existing test-loading behavior.

**Regression tests** (`apps/gogomail-user-mcp/src/test.ts`):
- refuses a local save when `GOGOMAIL_MCP_DOWNLOAD_DIR` is unset;
- rejects a `save_to_path` that escapes the configured directory;
- allows a `save_to_path` (including a nested subdir) inside the configured
  directory.

Documented in `apps/gogomail-user-mcp/README.md`.

---

## Lower-severity observations (not fixed — rationale)

- **LOW — User MCP client-side allow-list can diverge from backend routing.**
  The `gogomail_api_request` route manifest and `confirmationForUserAPI` map in
  the user MCP are maintained separately from the backend's
  `requiredUserAPIKeyConfirmation` / `requiredUserAPIKeyScope`. If the two drift,
  the *client* might not require a confirmation the backend does (or vice-versa).
  This is not exploitable: the backend independently enforces scope and
  confirmation for user-scoped keys, so a client that under-confirms is simply
  rejected server-side. Kept as-is to avoid coupling; worth a periodic
  cross-check when routes are added.

- **LOW — SSE rate-limit / session state is in-process.** The admin MCP's
  rate limiter and session maps are per-process. For the intended single-process
  operator deployment this is fine; a multi-replica SSE deployment would want a
  shared store. Not applicable to the current documented topology.

- **INFO — Admin MCP is a full-admin principal.** By design, not tenant-scoped.
  Operators must treat `GOGOMAIL_ADMIN_KEY` and `MCP_SECRET` as high-value
  secrets and only expose the SSE transport on a trusted network with the
  `Origin` allow-list configured.

---

## Verification

- **User MCP** (`apps/gogomail-user-mcp`): `npm run type-check` clean;
  `npm test` 26/26 pass (was 23 — +3 new); `npm run build` (tsc) clean.
  (`dist/` build artifact removed after verification.)
- **Admin MCP** (`apps/gogomail-manage-mcp`): `npm test` 63/63 pass (unchanged).
- **Backend**: `go build ./...` clean;
  `go test -short ./internal/httpapi ./internal/apikeys` pass, including the new
  DM regression tests.

No commits or pushes were made; all changes are left uncommitted. The dev
database and the `:8080` dev server were not touched.

## Files changed

- `internal/httpapi/dm.go` — reject user-scoped API keys in `dmPrincipalFromRequest`.
- `internal/httpapi/dm_test.go` — DM API-key rejection regression tests.
- `apps/gogomail-user-mcp/src/tools/drive.ts` — confine local download saves.
- `apps/gogomail-user-mcp/src/test.ts` — download containment regression tests.
- `apps/gogomail-user-mcp/README.md` — document `GOGOMAIL_MCP_DOWNLOAD_DIR`.
