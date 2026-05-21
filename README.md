# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

GoGoMail is a standards-first mail platform written in Go, with a webmail app,
an enterprise admin console, and a VitePress product guide.

The backend is built around interoperable protocol surfaces rather than
proprietary client assumptions: SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV,
LDAP, DKIM, SPF, DMARC, DSN, and OpenAPI-backed REST APIs.

[한국어 README](README.ko.md)

---

## What Is In This Repo

| Area | Path | Notes |
|---|---|---|
| Go backend | `cmd/`, `internal/`, `migrations/` | Mail protocols, REST APIs, delivery workers, storage, security policy, migrations |
| Webmail | `apps/webmail` | Next.js 16 webmail on port `3003` |
| Admin console | `apps/console` | Next.js 16 + Cloudscape console on port `3001` |
| Product guide | `apps/docs` | VitePress guide on port `3005`, localized in English, Korean, Japanese, and Simplified Chinese |
| API clients | `clients/` | Generated/shared API types |
| Operations docs | `docs/` | Current status, OpenAPI, security review, roadmap, ADRs |
| Local infrastructure | `docker/` | Docker Compose profiles for development and larger deployment sketches |

---

## Current Capabilities

### Backend

- SMTP receive, submission, outbound delivery, smart-host routing, DSN/bounce handling
- IMAP and POP3 mailbox access
- CalDAV, CardDAV, iMIP, WebDAV/Drive, and read-only LDAP gateway surfaces
- Mail and Admin REST APIs with OpenAPI documentation
- PostgreSQL metadata, Redis coordination, and local/MinIO/S3-compatible object storage
- DKIM signing, SPF/DMARC verification, DNS checks, queue/backpressure controls, audit logs, API metering
- Built-in spam policy, DNSBL/RBL checks, tenant spam-filter packs, and optional ClamAV attachment scanning
- Bulk delivery batching, batched delivery-attempt/outbox status writes, delivery route observability, and tunable parsed-message body caching
- Request-ID propagation, configurable PostgreSQL pool sizing, scheduled quota-alert email delivery, and optional retention AutoPurge jobs
- PostgreSQL backup script and Compose cron profile for scheduled `pg_dump` backups
- Company/domain/user configuration boundaries and security governance policy

### Webmail

- Mail list, reading pane, rich compose, folders, search, snooze, labels, reminders, attachments, and Drive picker flows
- Password reset UI, refresh-token based session renewal, server-synced email signatures, filter rules, and quick reply templates, Web Push service worker registration, calendar edit/delete flows, and crypto-backed browser ID generation for client-created records
- Keyboard-oriented UX: global shortcuts, app switching, Spotlight search, row focus, reading-pane navigation, and message actions
- Safer rendering for HTML mail, external images, and proxied remote content
- TOTP MFA login (two-step password → code) and in-settings enrollment with QR code, recovery codes, and disable flow

### Admin Console

- Company, domain, user, admin, role, and onboarding workflows
- SSO/SCIM, webhooks, notification templates, signatures, organization settings
- Message trace, mail-flow logs, delivery attempts, outbox, routing, relays, queues, and system health
- Security posture, MFA, auth/session/IP/rate-limit policies, DKIM, DMARC/SPF, spam filtering, API keys, retention, audit, and legal hold workflows
- Company/domain security governance for posture presets and controlled private-network webhook exceptions

### Product Guide

`apps/docs` documents Admin Console, Webmail, external integration APIs, glossary terms, shortcut behavior, security governance, and user/operator workflows.

---

## Security Posture

Recent hardening work is documented in [`docs/SECURITY_REVIEW.md`](docs/SECURITY_REVIEW.md).

Implemented controls include:

- Production bootstrap admin login is disabled.
- Backend outbound URL guards reject localhost, private/link-local, multicast, unspecified, and metadata-service targets after DNS resolution.
- Guarded outbound clients re-check redirects and cap redirect chains.
- Webhook secrets are redacted in list responses.
- Webmail HTML rendering strips active content and unsafe URL schemes.
- Image proxy rejects SVG, oversized responses, private destinations, and private redirects.
- Webmail and console API proxies strip client-supplied credentials and forward only allowlisted headers.
- Cookie-backed mutating routes require same-origin `Origin` or `Referer`.
- Production auth cookies use `__Host-` names with `HttpOnly`, `SameSite=Strict`, and `Secure`.
- Production CSP removes `unsafe-eval`; apps set `nosniff`, frame denial, COOP/CORP, HSTS, and no-store where appropriate.
- Go builds are pinned to patched toolchain `go1.26.3`; frontend apps override PostCSS to a patched line.
- Company/domain `/security/governance` controls allow typed operational exceptions while platform invariants remain fixed.
- Webmail login enforces TOTP MFA when required by company/domain `auth_policy`; enrolled users always receive a TOTP challenge. IP-based exemptions are supported via `mfa_exempt_cidrs`.
- Admin console login enforces TOTP MFA when enabled; `company_admin` MFA is controlled by the per-tenant `auth_policy` config key, and `system_admin` forced enrollment is controlled by `GOGOMAIL_ADMIN_MFA_REQUIRED`.
- Admin MFA is fully wired end-to-end (login challenge, `/admin/v1/auth/mfa/*` endpoints, security settings setup/verify/disable flow, and `console_mfa_setup_required` setup gate). Break-glass reset: `bin/gogomail admin mfa-reset --email <address>` (reads `DATABASE_URL`).

Verification commands:

```bash
./scripts/verify-backend-release.sh
GOGOMAIL_SECURITY_VERIFY=1 ./scripts/verify-backend-release.sh
pnpm --dir apps/webmail type-check
pnpm --dir apps/webmail test:security-helpers
pnpm --dir apps/webmail audit --prod
pnpm --dir apps/console type-check
pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts
pnpm --dir apps/console audit --prod
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
```

---

## Quick Start

### 1. Start local backend infrastructure

```bash
docker compose -f docker/docker-compose.dev.yml up -d
```

This starts PostgreSQL, Redis, MinIO, MinIO bucket initialization, and the Go
backend with hot reload through `air`.

Common local endpoints:

| Service | URL |
|---|---|
| Backend API | `http://localhost:8080` |
| PostgreSQL | `localhost:15432` |
| Redis | `localhost:16379` |
| MinIO API | `http://localhost:19000` |
| MinIO Console | `http://localhost:19001` |

Stop everything:

```bash
docker compose -f docker/docker-compose.dev.yml down
```

### 2. Seed development data

```bash
./scripts/seed_dev_beta.sh
```

Default development login:

```text
pjw@parkjw.org / pass1234
```

### 3. Run the frontends

```bash
pnpm --dir apps/webmail install
pnpm --dir apps/webmail dev
# http://localhost:3003
```

```bash
pnpm --dir apps/console install
pnpm --dir apps/console dev
# http://localhost:3001
```

```bash
pnpm --dir apps/docs install
pnpm --dir apps/docs dev
# http://localhost:3005
```

---

## Backend Binary

The backend is a single Go binary with multiple modes:

```bash
go build -o bin/gogomail ./cmd/gogomail

bin/gogomail --mode=all-in-one
bin/gogomail --mode=mail-api
bin/gogomail --mode=admin-api
bin/gogomail --mode=auth-server
bin/gogomail --mode=edge-mta
bin/gogomail --mode=inbound-mta
bin/gogomail --mode=outbound-mta
bin/gogomail --mode=delivery-worker
bin/gogomail --mode=outbox-relay
bin/gogomail --mode=event-worker
bin/gogomail --mode=imap
bin/gogomail --mode=pop3
bin/gogomail --mode=caldav
bin/gogomail --mode=carddav
bin/gogomail --mode=webdav
bin/gogomail --mode=ldap-gateway
bin/gogomail --migrate --mode=mail-api
```

`--mode` is the primary selector. `APP_MODE` is also honored when `--mode` is
not passed, which keeps Docker Compose environment files and direct binary
startup consistent. If both are set, `--mode` wins. Invalid modes fail fast at
startup; the accepted mode set is defined in `internal/app/mode.go`.

Core runtime dependencies:

- Go module declares `go 1.25.7` and pins toolchain `go1.26.3`.
- PostgreSQL 15+.
- Redis 7+.
- Local, MinIO, or S3-compatible object storage.

Important environment variables:

| Variable | Purpose |
|---|---|
| `GOGOMAIL_ENV` | Use `production` for stricter auth/TLS/security defaults |
| `APP_MODE` | Backend component mode fallback when `--mode` is not provided; `--mode` takes precedence |
| `GOGOMAIL_DATABASE_URL` | PostgreSQL connection string |
| `GOGOMAIL_DB_MAX_OPEN_CONNS` | PostgreSQL max open connections, default `20` |
| `GOGOMAIL_DB_MAX_IDLE_CONNS` | PostgreSQL max idle connections, default `5` |
| `GOGOMAIL_DB_CONN_MAX_LIFETIME` | PostgreSQL connection max lifetime, default `30m` |
| `GOGOMAIL_DB_CONN_MAX_IDLE_TIME` | PostgreSQL connection max idle time, default `5m` |
| `GOGOMAIL_REDIS_ADDR` | Redis host and port |
| `GOGOMAIL_REDIS_PASSWORD` | Redis password; required by the medium/large Docker profiles |
| `GOGOMAIL_REDIS_SENTINEL_ADDRS` / `GOGOMAIL_REDIS_MASTER_NAME` | Optional Redis Sentinel failover configuration |
| `GOGOMAIL_STORAGE_BACKEND` | `local`, `nfs`, `minio`, or `s3` |
| `GOGOMAIL_MAILSTORE_ROOT` / `GOGOMAIL_STORAGE_ROOT` | Local/NFS object root. `GOGOMAIL_MAILSTORE_ROOT` is primary; `GOGOMAIL_STORAGE_ROOT` is a deprecated fallback alias |
| `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS` | Compatibility labels exposed in storage capabilities during migrations |
| `GOGOMAIL_STORAGE_S3_*` | S3/MinIO endpoint, region, bucket, prefix, credentials, path-style, and TLS CA options |
| `GOGOMAIL_AUTH_JWT_SECRET` | Mail API JWT signing secret |
| `GOGOMAIL_ADMIN_TOKEN` | Admin API bearer token for token-based admin access |
| `GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL` / `GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD` | Development-only bootstrap admin credentials; blocked in production |
| `GOGOMAIL_SYSTEM_EMAIL_FROM` / `GOGOMAIL_SYSTEM_SMTP_*` | System email sender for invites, welcome mail, quota alerts, and password reset |
| `GOGOMAIL_APNS_*` / `GOGOMAIL_WEBPUSH_*` | Push notification credentials for APNs and Web Push |
| `GOGOMAIL_WEBHOOK_DISPATCH_ENABLED` | Enables tenant webhook dispatch, default `true` |
| `GOGOMAIL_CORS_ALLOWED_ORIGINS` | Comma-separated browser origins allowed by admin/mail APIs |
| `GOGOMAIL_METRICS_BACKEND` / `GOGOMAIL_METRICS_ADDR` | Metrics backend and Prometheus scrape address |
| `GOGOMAIL_PUBLIC_BASE_URL` | Public HTTPS origin used in system email links and open-tracking pixels; required to be non-local in production |
| `GOGOMAIL_OUTBOX_RELAY_*` | Outbox relay batch size, polling interval, and retry controls |
| `GOGOMAIL_DELIVERY_*` | Delivery worker stream, consumer, retry, TLS, smart-host, route, throttle, and timeout controls |
| `GOGOMAIL_DELIVERY_RECIPIENT_BATCH_SIZE` | Max recipients per same-domain SMTP delivery batch, default `100` |
| `GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES` | Parsed message body cache capacity, default `256`; set `0` to disable |
| `GOGOMAIL_MESSAGE_BODY_CACHE_TTL` | Parsed message body cache TTL, default `5m` |
| `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL` | Optional database URL used by release verification for backup/restore rehearsal |
| `GOGOMAIL_RESTORE_REHEARSAL_DB_URL` / `GOGOMAIL_RESTORE_REHEARSAL_KEEP_DB` | Optional scratch DB override and keep flag for `scripts/backup-restore-rehearsal.sh` |
| `GOGOMAIL_AUTO_PURGE_ENABLED` | Enables scheduled retention AutoPurge jobs for companies whose retention policy has `auto_purge_enabled` |
| `GOGOMAIL_AUTO_PURGE_INTERVAL` | Retention AutoPurge scheduler interval, default `24h` |
| `GOGOMAIL_AUTO_PURGE_BATCH_SIZE` | Max messages/audit rows purged per company per run, default `1000` |
| `GOGOMAIL_API_METERING_*` / `GOGOMAIL_API_USAGE_*` | API metering stream, aggregation, retention, and export signer controls |
| `GOGOMAIL_ATTACHMENT_SCAN_*` / `GOGOMAIL_ATTACHMENT_CLEANUP_*` | Attachment scanning backend, ClamAV/webhook options, and stale upload cleanup controls |
| `GOGOMAIL_PUSH_NOTIFICATION_*` | Push notification backend, webhook, consumer, and device-limit controls |
| `GOGOMAIL_BACKUP_DIR` | Directory used by `scripts/backup.sh`, default `./backups` |
| `GOGOMAIL_BACKUP_RETENTION_DAYS` | Local backup retention window, default `7` |
| `GOGOMAIL_BACKUP_S3_BUCKET` / `GOGOMAIL_BACKUP_S3_PREFIX` | Optional S3 bucket and key prefix for backup uploads |
| `GOGOMAIL_SECURITY_VERIFY` | Set to `1` to add `go vet` and `govulncheck` to backend release verification |
| `GOGOMAIL_BACKEND_URL` | Backend URL used by Next.js server routes |
| `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` | Public origin displayed in browser-facing console copy when needed |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | Browser-visible Web Push VAPID public key for webmail subscription registration |
| `GOGOMAIL_ADMIN_MFA_REQUIRED` | Require TOTP MFA enrollment for `system_admin` login; default `false` |

Full configuration details live under `internal/config/`, `internal/config/validate.go`, `configs/`, [`docker/.env.example`](docker/.env.example), [`apps/webmail/.env.example`](apps/webmail/.env.example), and [`apps/console/.env.example`](apps/console/.env.example).

---

## External Integration API

Trusted external systems can call GoGoMail server-to-server APIs to embed mail
features in portals, groupware, approval flows, or internal dashboards.

- Use `Authorization: Bearer gm_...` API keys generated in the Admin Console.
- Prefer `X-Gogomail-User-Email` or `user_email` for mailbox identity.
- Use narrow scopes: `mail:read`, `mail:send`, and `mail:manage`.
- API calls are metered for usage reporting and quota analysis.

References:

- [`docs/openapi.yaml`](docs/openapi.yaml)
- `apps/docs` page `/admin-console/external-integration`

---

## Development Notes

```bash
go test ./...
go vet ./...
go build ./...
./scripts/verify-backend-release.sh
./scripts/verify-frontend-release.sh
pnpm --dir apps/webmail type-check
pnpm --dir apps/console type-check
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
```

Release verification entrypoints:

- `./scripts/verify-backend-release.sh` runs Go tests, module tidy diff checks, optional PostgreSQL/OpenSearch integration checks, optional restore rehearsal, optional security verification, and a clean-worktree gate.
- `./scripts/verify-frontend-release.sh` runs webmail and console type checks plus helper tests by default; set `GOGOMAIL_FRONTEND_E2E=1` and `GOGOMAIL_FRONTEND_BUILD=1` for heavier browser/build gates.

The repository uses a strict project harness:

- Read `docs/ACTIVE_TASK.md` before picking implementation work.
- Keep code, tests, and docs in the same commit when behavior changes.
- The pre-commit hook runs `go test ./...`.
- Backend or migration changes require a staged `docs/` update.

See [`PROJECT_HARNESS.md`](PROJECT_HARNESS.md).

---

## Key Documents

| Document | Contents |
|---|---|
| [`docs/ACTIVE_TASK.md`](docs/ACTIVE_TASK.md) | Current development task |
| [`docs/CURRENT_STATUS.md`](docs/CURRENT_STATUS.md) | Detailed implementation status |
| [`docs/SECURITY_REVIEW.md`](docs/SECURITY_REVIEW.md) | Security hardening summary and verification commands |
| [`docs/backend-release-readiness.md`](docs/backend-release-readiness.md) | Release readiness checks, optional gates, and operational verification notes |
| [`docs/openapi.yaml`](docs/openapi.yaml) | Mail and Admin API contract |
| [`docs/backend-roadmap.md`](docs/backend-roadmap.md) | Long-form backend roadmap and completed hardening items |
| [`apps/docs/`](apps/docs/) | Product guide for Admin Console, Webmail, glossary, and integration APIs |
| [`docker/`](docker/) | Docker Compose files and deployment notes |
| [`docs/adr/`](docs/adr/) | Architecture decision records |

---

## License

[Elastic License 2.0](LICENSE). Free to use and modify internally; offering
GoGoMail as a hosted or managed service requires explicit permission.

Copyright (c) 2026 Park Jangwon.
