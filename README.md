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
- Company/domain/user configuration boundaries and security governance policy

### Webmail

- Mail list, reading pane, rich compose, folders, search, snooze, labels, reminders, attachments, and Drive picker flows
- Keyboard-oriented UX: global shortcuts, app switching, Spotlight search, row focus, reading-pane navigation, and message actions
- Safer rendering for HTML mail, external images, and proxied remote content

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

Verification commands:

```bash
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
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

bin/gogomail --mode=api
bin/gogomail --mode=smtp-edge
bin/gogomail --mode=smtp-submission
bin/gogomail --mode=delivery-worker
bin/gogomail --mode=imap
bin/gogomail --mode=pop3
bin/gogomail --mode=caldav
bin/gogomail --mode=carddav
bin/gogomail --mode=webdav
bin/gogomail --mode=ldap-gateway
bin/gogomail --mode=migration
```

Core runtime dependencies:

- Go module declares `go 1.25.7` and pins toolchain `go1.26.3`.
- PostgreSQL 15+.
- Redis 7+.
- Local, MinIO, or S3-compatible object storage.

Important environment variables:

| Variable | Purpose |
|---|---|
| `GOGOMAIL_ENV` | Use `production` for stricter auth/TLS/security defaults |
| `GOGOMAIL_DATABASE_URL` | PostgreSQL connection string |
| `GOGOMAIL_REDIS_URL` / `REDIS_ADDR` | Redis connection |
| `GOGOMAIL_STORAGE_BACKEND` | `local`, `minio`, or `s3` |
| `GOGOMAIL_AUTH_JWT_SECRET` | Mail API JWT signing secret |
| `GOGOMAIL_ADMIN_TOKEN` | Admin API bearer token for token-based admin access |
| `GOGOMAIL_BACKEND_URL` | Backend URL used by Next.js server routes |
| `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` | Public origin displayed in browser-facing console copy when needed |

Full configuration details live under `internal/config/` and `configs/`.

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
pnpm --dir apps/webmail type-check
pnpm --dir apps/console type-check
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
```

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
