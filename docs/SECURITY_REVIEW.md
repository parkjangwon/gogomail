# gogomail Security Review

Last updated: 2026-05-17

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

- Production bootstrap admin login using `admin@system / admin1234` is disabled
  unless the admin route environment is explicitly non-production.
- Backend outbound URL guard rejects non-HTTP(S), localhost, loopback, private,
  link-local, multicast, unspecified, and metadata-service addresses after DNS
  resolution; guarded clients re-check redirects and cap redirect chains.
- Attachment scan webhooks use the outbound URL guard by default. Unit tests may
  opt into private-network endpoints for local `httptest` servers only.
- Admin company webhooks reject private URLs and do not expose stored webhook
  secrets in list responses; only a suffix is returned after storage.
- Webmail image proxy rejects SVG, private destinations, oversized images, and
  redirects to private destinations.
- Webmail HTML email rendering removes high-risk active content tags and strips
  unsafe URL schemes before inserting sanitized HTML.
- Console admin proxies are consolidated into a shared server helper that
  encodes path segments, checks same-origin mutating requests, forwards only
  allowlisted request headers, and returns `no-store` plus `nosniff`.
- Webmail mail proxy now encodes backend path segments, checks same-origin
  mutating requests, strips client-supplied credentials, and forwards only the
  required upload/download headers.
- Login/logout proxy responses set `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`; console demo credentials are hidden in
  production builds.
- Go builds are pinned to patched toolchain `go1.26.3`, and both frontend apps
  override `postcss` to `^8.5.14` so production dependency audits are clean.
- Enterprise cookie posture uses `__Host-` token cookie names in production,
  with legacy cookie cleanup during migration.
- Company/domain security governance is explicit via
  `/security/governance`; platform invariants remain fixed, while approved
  operational exceptions such as private-network webhook targets are deny by
  default and can be enabled per tenant policy.
- Cookie-backed mutating API routes now require same-origin `Origin` or
  `Referer`; requests without browser provenance are rejected instead of treated
  as implicitly trusted.
- Frontend server routes use server-only `GOGOMAIL_BACKEND_URL`; public browser
  configuration should use purpose-specific public origins such as
  `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` for displayed SCIM endpoints.
- Production CSP removes `unsafe-eval`, adds `upgrade-insecure-requests`, and
  both apps now set COOP/CORP plus DNS prefetch disabling.
- Built-in SMTP spam filtering now supports strict SPF/DKIM/DMARC scoring,
  policy-managed RBL/DNSBL zone registration, reject-on-listed-IP behavior,
  dangerous attachment extension scoring, and policy-driven bulk recipient
  thresholds. RBL lookup failures fail open to preserve receive availability,
  while positive listings reject by default when enabled.
- SMTP receive parsing keeps body extraction bounded to 64KB for spam scoring
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
  overrides, so operators can tighten or relax filtering without crossing
  tenant boundaries.

## Verification Commands

- `go test ./...`
- `go vet ./...`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `pnpm --dir apps/webmail test:security-helpers`
- `pnpm --dir apps/webmail type-check`
- `pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts`
- `pnpm --dir apps/console type-check`
- `pnpm --dir apps/webmail audit --prod`
- `pnpm --dir apps/console audit --prod`

## Remaining Follow-Ups

- Move the current TypeScript helper tests into a broader frontend security
  suite if the repo later standardizes on one runner for webmail.
- Add deployment-specific allowlists for intentional internal webhook targets
  behind the governance setting if operators need narrower controls than the
  current tenant-level allow/deny; the default remains private-network deny.
- Evaluate nonce-based inline theme/style bootstrapping so production CSP can
  remove `unsafe-inline` without visual flicker.
- Add centralized security event logging for rejected same-origin, private URL,
  and oversized proxy attempts once the audit pipeline is finalized.
- Add operator-facing ClamAV health and signature freshness monitoring in
  `apps/console` so administrators can see stale signatures or scanner outages
  before mail acceptance depends on them.
- Expand filter-pack lifecycle controls with signed import/export bundles,
  staged rollout, hit-rate analytics, and emergency disable once production
  telemetry volume is available.
