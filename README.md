# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

A standards-first mail platform built in Go. SMTP, IMAP, CalDAV, CardDAV — implemented against their RFCs so any compliant client works without proprietary plugins.

---

## Philosophy

Every protocol surface maps to a published RFC, and every design decision is filtered through interoperability first. If a feature can't be expressed in a standard, it waits until it can — so any component (MTA, storage, identity provider) can be replaced without rewriting integrations.

---

## What's implemented

### Backend

| Component | Standards | Status |
|---|---|---|
| SMTP receive (edge MTA) | RFC 5321, 5322, 2045–2049 | Production |
| SMTP submission | RFC 6409, 4954 | Production |
| SMTP delivery + smart-host relay | RFC 5321, 7505 | Production |
| DKIM signing | RFC 6376 | Production |
| SPF / DMARC verification | RFC 7208, 7489 | Production |
| DSN / bounce handling | RFC 3461, 3464 | Production |
| IMAP | RFC 9051, 3501 | Production |
| POP3 | RFC 1939 | Production |
| CalDAV + iCalendar | RFC 4791, 5545, 6638, 7809, 3744 | Production |
| iMIP scheduling | RFC 6047 | Production |
| CardDAV + vCard | RFC 6352, 6350, 2426, 3744 | Production |
| WebDAV / Drive gateway | RFC 4918 | Production |
| LDAP directory gateway | RFC 4511, 4512, 4519 | Advanced |
| Mail + Admin REST API | OpenAPI | Production |
| Drive / file storage | S3-compatible | Advanced |
| Mail flow logs + analytics | PostgreSQL + OpenSearch | Advanced |

Current implementation detail: [`docs/CURRENT_STATUS.md`](docs/CURRENT_STATUS.md).

### Webmail (Next.js 15)

Keyboard-first webmail built with Next.js 15, Tailwind v4, and shadcn/ui.

- **Mail** — 3-pane layout, Gmail-style shortcuts (`g i`, `e`, `r`, `#`, …) with Korean IME, Spotlight search (Cmd+K) with operators, TipTap rich-text compose with slash commands and inline images, send delay, snooze, pin, follow-up reminders, inbox category tabs, unsubscribe detection, ICS invite detection.
- **Filters** — multi-condition rules (AND/OR), 9 condition fields, 6 match types including regex, 9 actions, blocked senders, vacation responder.
- **Calendar** — month/week/day/agenda views, multiple color-coded calendars, RFC 5545 recurring events, ICS import.
- **Contacts** — CardDAV-backed list, hover cards, hierarchical org chart, group-based recipient picker.
- **Drive** — S3-backed file manager with folder tree, share links, trash.
- **Settings** — per-folder mailbox stats, async EML/ZIP backup, restore from EML/MBOX, JSON settings import/export, focus mode, accessibility (high contrast, reduced motion, screen reader).

---

## Architecture

Single binary, multiple modes. Each mode runs one component independently — deploy on separate nodes or compose into a single process for development.

```
gogomail --mode=smtp-edge          # inbound SMTP (port 25)
gogomail --mode=smtp-submission    # authenticated submission (port 587)
gogomail --mode=imap               # IMAP server (port 143 / 993)
gogomail --mode=pop3               # POP3 server (port 110 / 995)
gogomail --mode=caldav             # CalDAV server
gogomail --mode=carddav            # CardDAV server
gogomail --mode=ldap-gateway       # read-only LDAP v3 directory gateway
gogomail --mode=webdav             # WebDAV gateway (RFC 4918)
gogomail --mode=api                # Mail + Admin REST API
gogomail --mode=delivery-worker    # outbound SMTP delivery
gogomail --mode=outbox-relay       # outbox → event stream relay
gogomail --mode=event-worker       # event stream consumer
gogomail --mode=migration          # run database migrations
```

**Infrastructure**: PostgreSQL, Redis, S3-compatible object storage (local, MinIO, or AWS S3).

---

## Quick start

### Backend

Requirements: Go 1.25+, PostgreSQL 15+, Redis 7+

```bash
go build -o bin/gogomail ./cmd/gogomail

GOGOMAIL_DATABASE_URL=postgres://... bin/gogomail --mode=migration

GOGOMAIL_DATABASE_URL=postgres://... \
GOGOMAIL_REDIS_URL=redis://localhost:6379 \
GOGOMAIL_STORAGE_BACKEND=local \
GOGOMAIL_STORAGE_LOCAL_PATH=/tmp/gogomail \
bin/gogomail --mode=api
```

| Variable | Description |
|---|---|
| `GOGOMAIL_DATABASE_URL` | PostgreSQL connection string |
| `GOGOMAIL_REDIS_URL` | Redis connection string |
| `GOGOMAIL_STORAGE_BACKEND` | `local` / `minio` / `s3` |
| `GOGOMAIL_AUTH_JWT_SECRET` | HS256 secret for Mail API JWT auth |
| `GOGOMAIL_ADMIN_TOKEN` | Bearer token for Admin API |
| `GOGOMAIL_DKIM_ENABLED` | `true` to enable DKIM signing on delivery |
| `GOGOMAIL_DELIVERY_TLS_MODE` | `opportunistic` (default) / `require` / `disable` |
| `GOGOMAIL_ENV` | `production` enforces stricter TLS and auth guards |

Full reference: `internal/config/`.

### Webmail

Requirements: Node.js 20+, pnpm 9+

```bash
cd apps/webmail
pnpm install
pnpm dev       # http://localhost:3000
pnpm build
```

### Seed data

```bash
docker compose -f docker/docker-compose.dev.yml up -d postgres
./scripts/seed_dev_beta.sh
```

Default login: `pjw@parkjw.org` / `pass1234`.

---

## Development

```bash
go test ./...                                # all tests
go build ./...                               # build check
tsc --noEmit -p apps/webmail/tsconfig.json   # webmail type check
```

The pre-commit hook enforces:

1. `go test ./...` must pass.
2. Changes under `internal/` or `migrations/` require at least one `docs/` file in the same commit.

Workflow is driven by `docs/ACTIVE_TASK.md` — one task at a time, TDD, docs and code committed together. See `PROJECT_HARNESS.md`.

---

## Roadmap

| Phase | Focus | Status |
|---|---|---|
| 0–1 | SMTP, IMAP, CalDAV, CardDAV, Mail/Admin API, delivery, DKIM/SPF/DMARC | ✓ Complete |
| 2 | Webmail frontend | ✓ Complete |
| 3 | Runtime config store · company→domain→user hierarchy · 2FA/TOTP | Planned |
| 4 | Enterprise identity: LDAP · SCIM 2.0 · SAML/OIDC | LDAP advanced, SCIM/SSO planned |
| 5 | WebDAV gateway · CalDAV/CardDAV hardening | ✓ Complete |
| 6 | Mail security: milter adapter · DNSBL (RFC 5782) | Planned |
| 7 | POP3 | ✓ Complete |
| 8 | Push notifications: FCM / APNs / Web Push (RFC 8030) | Planned |

Full roadmap: [`docs/backend-roadmap.md`](docs/backend-roadmap.md).

---

## Key documents

| Document | Contents |
|---|---|
| `docs/ACTIVE_TASK.md` | Current development task |
| `docs/backend-roadmap.md` | Full phase-by-phase roadmap with RFC references |
| `docs/CURRENT_STATUS.md` | Detailed implementation status |
| `docs/openapi.yaml` | OpenAPI spec for Mail + Admin APIs |
| `docs/adr/` | Architecture decision records |
| `PROJECT_HARNESS.md` | Development loop contract for autonomous agents |

---

## License

[Elastic License 2.0](LICENSE). Free to use and modify internally; offering gogomail as a hosted or managed service requires explicit permission.

Copyright (c) 2026 Park Jangwon.
