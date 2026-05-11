# gogomail

<img width="1456" height="720" alt="1777874812592" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

A standards-first mail platform built in Go. SMTP, IMAP, CalDAV, CardDAV — all implemented against their RFCs so any compliant client works out of the box, without proprietary plugins or vendor-specific extensions.

---

## Philosophy

Most webmail platforms accumulate years of proprietary APIs and vendor lock-in. gogomail takes the opposite approach: every protocol surface maps to a published RFC, and every design decision is filtered through interoperability first. If a feature can't be expressed in a standard, it waits until it can.

This matters in practice. When your mail client, calendar app, and contacts sync work via open standards, you can replace any component — the MTA, the storage backend, the identity provider — without rewriting integrations.

---

## What's implemented

### Backend

| Component | Standards | Status |
|---|---|---|
| SMTP receive (edge MTA) | RFC 5321, 5322, 2045–2049 | Production-ready |
| SMTP submission (outbound MTA) | RFC 6409, AUTH RFC 4954 | Production-ready |
| SMTP delivery + smart-host relay | RFC 5321, RFC 7505 (null MX) | Production-ready |
| DKIM signing | RFC 6376 | Production-ready |
| SPF / DMARC verification | RFC 7208, RFC 7489 | Production-ready |
| DSN / bounce handling | RFC 3461, RFC 3464 | Production-ready |
| IMAP | RFC 9051 (IMAP4rev2), RFC 3501 | Production-ready |
| CalDAV + iCalendar | RFC 4791, RFC 5545, RFC 6638 | Advanced |
| iMIP scheduling | RFC 6047 | Complete |
| Timezone support | RFC 7809 | Complete |
| CardDAV + vCard | RFC 6352, RFC 6350, RFC 2426 | Advanced |
| Mail API (REST) | OpenAPI | Production-ready |
| Admin API | OpenAPI | Production-ready |
| Drive / file storage | S3-compatible backend | Advanced |
| Mail flow logs + analytics | PostgreSQL + OpenSearch | Advanced |

### Webmail (Next.js 15)

A keyboard-first webmail client built with Next.js 15, Tailwind v4, and shadcn/ui. Inspired by Notion Mail / Superhuman UX principles.

**Mail**
- 3-pane layout — sidebar, message list, reading pane
- Keyboard shortcuts: Gmail-style (`g i`, `g s`, `e`, `r`, `a`, `f`, `#`, …) + Korean IME support
- Spotlight search (Cmd+K) with Gmail-style operators (`from:`, `to:`, `subject:`, `has:attachment`)
- Inline reply editor (TipTap v2) with rich text, slash commands, inline images
- Compose with TipTap — slash commands, inline image paste, attachment upload
- Send delay (undo send countdown)
- Snooze, pin, follow-up reminders for sent mail
- Inbox category tabs + smart auto-classification chips
- Compact view toggle
- Inbox zero celebration state
- Unsubscribe link auto-detection
- ICS calendar invite detection with Add to Calendar

**Filters & Automation**
- Multi-condition filter rules with AND/OR logic
- 9 condition fields: from, to, cc, subject, body, has attachment, is unread, size larger/smaller
- 6 match types: contains, not contains, equals, starts with, ends with, regex
- 9 actions: label color, move to folder, mark read/unread, star, mark important, skip inbox, delete, forward
- Enabled toggle + stop processing flag per rule
- Blocked senders, vacation responder

**Virtual folders**
- Unread, Snoozed, Pinned, Important, Tasks — all sidebar shortcuts

**Calendar**
- Month/week/day/agenda views
- Multiple calendars with color-coding — add, edit, delete calendars
- Recurring events (RFC 5545 RRULE: daily/weekly/monthly/yearly, interval, day selection, end by count or date)
- ICS import via email

**Contacts**
- CardDAV-backed contact list with search
- Contact hover card in message headers

**Drive**
- S3-backed file manager with folder tree, upload, download, share link, trash

**Settings**
- General, appearance, notifications, reading, compose, signature/auto-reply, filters, blocked senders, vacation responder, templates, privacy, accessibility, enterprise security, storage/backup, about
- Per-folder mailbox stats (message count, unread, starred, estimated size)
- Async EML / ZIP backup per folder (non-blocking, progress tracking)
- Mailbox restore from EML/MBOX file
- Settings import / export (JSON)
- Focus mode, DND-aware browser notifications
- High contrast, reduced motion, font family, screen reader mode

---

## Architecture

Single binary, multiple modes. Each mode runs one component independently, so you can deploy them on separate nodes or compose them into a single process for development.

```
gogomail --mode=smtp-edge          # inbound SMTP (port 25)
gogomail --mode=smtp-submission    # authenticated submission (port 587)
gogomail --mode=imap               # IMAP server (port 143 / 993)
gogomail --mode=caldav             # CalDAV server
gogomail --mode=carddav            # CardDAV server
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

**Requirements**: Go 1.25+, PostgreSQL 15+, Redis 7+

```bash
# Build
go build -o bin/gogomail ./cmd/gogomail

# Run migrations
GOGOMAIL_DATABASE_URL=postgres://... bin/gogomail --mode=migration

# Start API server (development)
GOGOMAIL_DATABASE_URL=postgres://... \
GOGOMAIL_REDIS_URL=redis://localhost:6379 \
GOGOMAIL_STORAGE_BACKEND=local \
GOGOMAIL_STORAGE_LOCAL_PATH=/tmp/gogomail \
bin/gogomail --mode=api
```

Key environment variables:

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

Full configuration reference: `internal/config/`.

### Webmail

**Requirements**: Node.js 20+, pnpm 9+

```bash
cd apps/webmail
pnpm install
pnpm dev       # http://localhost:3000
pnpm build     # production build
```

---

## Development

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/scheduling/...

# Build check
go build ./...

# Webmail type check
tsc --noEmit -p apps/webmail/tsconfig.json
```

The pre-commit hook enforces two rules automatically:

1. `go test ./...` must pass before any commit.
2. Production code changes (`internal/`, `migrations/`) require at least one `docs/` file staged in the same commit.

Development workflow is driven by `docs/ACTIVE_TASK.md` — one task at a time, TDD, docs and code committed together. See `PROJECT_HARNESS.md` for the full contract.

---

## Roadmap

| Phase | Focus | Status |
|---|---|---|
| 0–1 | SMTP, IMAP, CalDAV, CardDAV, Mail/Admin API, delivery, DKIM/SPF/DMARC | ✓ Complete |
| 2 | Webmail frontend — keyboard-first, settings, filters, calendar, contacts, drive | ✓ Complete |
| 3 | Runtime config store · company→domain→user settings hierarchy · 2FA/TOTP | Planned |
| 4 | Enterprise identity: LDAP (RFC 4511) · SCIM 2.0 · SAML/OIDC | Planned |
| 5 | Drive WebDAV gateway (RFC 4918) · CalDAV/CardDAV production hardening | Planned |
| 6 | Mail security: milter adapter · DNSBL (RFC 5782) | Planned |
| 7 | POP3 (RFC 1939) | Planned |
| 8 | Push notifications: FCM / APNs / Web Push (RFC 8030) | Planned |

Full roadmap: `docs/backend-roadmap.md`.

---

## Key documents

| Document | Contents |
|---|---|
| `docs/ACTIVE_TASK.md` | Current development task |
| `docs/backend-roadmap.md` | Full phase-by-phase roadmap with RFC references |
| `docs/CURRENT_STATUS.md` | Detailed current implementation status |
| `docs/DESIGN.md` | Frontend design language and component patterns |
| `docs/openapi.yaml` | OpenAPI spec for Mail + Admin APIs |
| `docs/backend-api-contracts.md` | API contract documentation |
| `docs/adr/` | Architecture decision records |
| `PROJECT_HARNESS.md` | Development loop contract for autonomous agents |

---

## License

[Elastic License 2.0](LICENSE) — free to use internally (commercial or non-commercial), fork and customize. Selling or hosting gogomail as a product or managed service requires explicit permission from the copyright holder.

Copyright (c) 2026 Park Jangwon.
