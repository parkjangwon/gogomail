# gogomail Security Review

Last updated: 2026-05-31

## Baseline

This review tracks OWASP Top 10 2025 oriented hardening across the Go backend,
admin console, and webmail frontend.

Primary risk areas covered in this pass:

- Broken access control and insecure defaults
- XSS in rendered email content
- SSRF through image proxy, webhooks, and remote HTTP integrations
- CSRF against cookie-backed Next.js API proxies
- Header, path, and download-response injection
- Dependency and static security checks

## Implemented Controls

### Access Control

- Production bootstrap admin login using `admin@system / admin1234` is disabled
  unless the admin route environment is explicitly non-production.
  `GOGOMAIL_ENV` defaults to `"production"` so omitting the variable is always
  safe (`internal/config/config.go:372`).
- IDOR sweep is complete across all admin handlers. Every list and stats
  endpoint now enforces company/domain scoping before returning data. Files
  covered: `admin_storage.go`, `admin_roles.go`, `admin_bulk.go`,
  `admin_domain_apikeys.go`, `admin_api_settings.go`, `admin_policy.go`,
  `admin_quota.go`, `admin_mail_flow_logs.go`, `admin_api_usage.go`.
- Company/domain security governance is explicit via `/security/governance`;
  platform invariants remain fixed, while approved operational exceptions such
  as private-network webhook targets are deny by default and can be enabled per
  tenant policy.

### Trusted Header Stripping

- `StripInternalHeadersMiddleware` removes every `X-Gogomail-*` request header
  before any handler runs (`internal/httpapi/admin_middleware.go:51`). The
  middleware is wired at `internal/app/run.go:489`, which means external
  clients cannot spoof resolved user identity, tenant context, or any other
  internal-only header regardless of the path they hit.

### SSRF and Outbound URL Safety

- Backend outbound URL guard rejects non-HTTP(S), localhost, loopback, private,
  link-local, multicast, unspecified, and metadata-service addresses after DNS
  resolution; guarded clients re-check redirects and cap redirect chains.
- Attachment scan webhooks use the outbound URL guard by default. Unit tests may
  opt into private-network endpoints for local `httptest` servers only.
- Admin company webhooks reject private URLs and do not expose stored webhook
  secrets in list responses; only a suffix is returned after storage.
- Webmail image proxy rejects SVG, private destinations, oversized images, and
  redirects to private destinations.

### XSS / Content Safety

- Webmail HTML email rendering removes high-risk active content tags and strips
  unsafe URL schemes before inserting sanitized HTML.
- Production CSP removes `unsafe-eval`, adds `upgrade-insecure-requests`, and
  both apps now set COOP/CORP plus DNS prefetch disabling.
- CSP nonce is generated per-request by `apps/webmail/src/middleware.ts` and
  `apps/console/src/middleware.ts`; the nonce value is applied to inline
  `<script>` tags so script-src no longer requires `'unsafe-inline'` in
  production. `style-src 'unsafe-inline'` remains (see Accepted Risk).

### CSRF

- Cookie-backed mutating API routes now require same-origin `Origin` or
  `Referer`; requests without browser provenance are rejected instead of treated
  as implicitly trusted.
- Enterprise cookie posture uses `__Host-` token cookie names in production,
  with legacy cookie cleanup during migration.

### Proxy Security

- Console admin proxies are consolidated into a shared server helper that
  encodes path segments, checks same-origin mutating requests, forwards only
  allowlisted request headers, and returns `no-store` plus `nosniff`.
- Webmail mail proxy encodes backend path segments, checks same-origin mutating
  requests, strips client-supplied credentials, and forwards only the required
  upload/download headers.
- Login/logout proxy responses set `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`; console demo credentials are hidden in
  production builds.
- Frontend server routes use server-only `GOGOMAIL_BACKEND_URL`; public browser
  configuration uses purpose-specific public origins such as
  `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` for displayed SCIM endpoints.

### Secrets and Deployment

- Helm `requireNotChangeme` helper (defined in
  `helm/gogomail/templates/_helpers.tpl`) causes `helm install` / `helm upgrade`
  to fail immediately if `GOGOMAIL_DM_MASTER_KEY`, `GOGOMAIL_AUTH_JWT_SECRET`,
  or `GOGOMAIL_ADMIN_TOKEN` still contain the placeholder value `CHANGEME`.
- `docker-compose.scale.yml` sets `sslmode=require` for all database connections
  (line 27), ensuring TLS is enforced in multi-node deployments.
- `GOGOMAIL_ENV` defaults to `"production"` (`internal/config/config.go:372`),
  so omitting the environment variable never accidentally enables development
  paths.

### Authentication

- JWT: uses `golang-jwt/jwt/v5` (`internal/auth/jwt.go:13`) with a custom
  `iat`-in-future guard that rejects tokens issued after the current clock with
  a configurable skew tolerance.
- Password hashing: PBKDF2-SHA256 is the required storage format in production;
  plain-text and raw-SHA256 hashes are rejected at login time. Legacy hashes
  are automatically upgraded to PBKDF2-SHA256 on the next successful login
  (`internal/maildb/user_auth.go:98`), so migration is zero-downtime.
- RDBMS IdP: `validateSourceQuery` (`internal/idprovider/rdbms/provider.go`)
  enforces SELECT-only queries — DML and DDL keywords are rejected, queries are
  capped at 4096 bytes, and internal semicolons are not permitted — preventing
  operator-supplied queries from mutating the identity database.

### SMTP / Mail Security

- Built-in SMTP spam filtering supports strict SPF/DKIM/DMARC scoring,
  policy-managed RBL/DNSBL zone registration, reject-on-listed-IP behavior,
  dangerous attachment extension scoring, and policy-driven bulk recipient
  thresholds. RBL lookup failures fail open to preserve receive availability
  (see Accepted Risk), while positive listings reject by default when enabled.
- SMTP receive parsing keeps body extraction bounded to 64 KB for spam scoring
  and milter BodyChunk input, so content checks have useful signal without
  unbounded memory growth on bulk inbound traffic.
- Attachment byte scanning supports a separate ClamAV `clamd` service through
  the INSTREAM protocol. The backend streams the spooled MIME message to ClamAV
  over TCP and treats `FOUND` as a reject verdict, while keeping the AV engine,
  signature database, and update lifecycle outside the backend container.
- ClamAV scan admission is bounded: only parsed messages with attachments are
  sent to `clamd`, concurrent scans are capped, oversized scan streams tempfail,
  scan calls have deadlines, and repeated scanner failures open a short circuit.
  Under load or outage the SMTP path tempfails affected messages instead of
  accumulating unbounded blocked goroutines behind the AV service.
- Spam filter packs are tenant-scoped inside company/domain policy config rather
  than managed as global mutable files. Custom pack IDs, text fields, rule
  patterns, counts, and scores are normalized before storage, and reserved
  `gogomail-core-*` system pack IDs cannot be overridden by tenant input.
- Admin console spam-filter management exposes built-in pack toggles and
  tenant-owned custom phrase packs for both company defaults and domain
  overrides, so operators can tighten or relax filtering without crossing tenant
  boundaries.

### Dependencies

- Go builds are pinned by `go.mod` to Go `1.25.7`, and both frontend apps
  override `postcss` to `^8.5.14` so production dependency audits are clean.

### Operational Observability

- Cleanup and rollback delete failures are warning-logged across attachment,
  Drive, SMTP, IMAP APPEND, outbound-send, DSN enqueue, API usage export, and
  storage readiness paths instead of being silently discarded. Drive workflows
  record retryable cleanup failures where a recorder exists.
- SCIM soft-delete/deactivate/active paths warning-log failed external IdP
  `UpdateUserStatus` calls with operation, user id, desired status, and error
  context.
- API metering middleware remains fail-open for availability, but sink errors
  now warning-log route, method, status, and user context for later
  reconciliation.
- `cmd/remote-signer` now uses structured JSON logs, startup config
  validation, HTTP timeout/max-header policy, graceful signal shutdown, and
  lifecycle tests for cancellation after listener bind.

## Accepted Risk

- `style-src 'unsafe-inline'` is still present in the Content Security Policy.
  Removing it requires nonce-based style bootstrapping, which causes visual
  flicker during initial page load. Accepted until the theme/style layer is
  refactored to support nonce injection.
- RBL lookup failures fail open to preserve receive availability. A failed DNS
  lookup during spam scoring does not cause message rejection, accepting the
  risk that a listed sender could get through during a DNS outage.

## Post-Release Hardening

- Move the current TypeScript helper tests into a broader frontend security
  suite if the repo later standardizes on one runner for webmail.
- Add deployment-specific allowlists for intentional internal webhook targets
  behind the governance setting if operators need narrower controls than the
  current tenant-level allow/deny; the default remains private-network deny.
- Evaluate nonce-based inline theme/style bootstrapping so production CSP can
  remove `unsafe-inline` without visual flicker.
- Add centralized security event logging for rejected same-origin, private URL,
  and oversized proxy attempts once the audit pipeline is finalized. Cleanup,
  SCIM sync, and API metering fail-open warning paths are already covered by
  structured logs.
- Add operator-facing ClamAV health and signature freshness monitoring in
  `apps/console` so administrators can see stale signatures or scanner outages
  before mail acceptance depends on them.
- Expand filter-pack lifecycle controls with signed import/export bundles,
  staged rollout, hit-rate analytics, and emergency disable once production
  telemetry volume is available.

## Verification Commands

- `go test ./...`
- `go vet ./...`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `GOGOMAIL_SECURITY_VERIFY=1 ./scripts/verify-backend-release.sh` (uses installed `govulncheck`, or `go run golang.org/x/vuln/cmd/govulncheck@latest` as fallback)
- `pnpm --dir apps/webmail test:security-helpers`
- `pnpm --dir apps/webmail type-check`
- `pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts`
- `pnpm --dir apps/console type-check`
- `pnpm --dir apps/webmail audit --prod`
- `pnpm --dir apps/console audit --prod`
