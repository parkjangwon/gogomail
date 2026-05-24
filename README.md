# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

A self-hosted, multi-tenant mail and collaboration platform written in Go.
One static binary serves SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP,
REST APIs, and event workers — each role selectable at startup. Pair it
with PostgreSQL, Redis, and any S3-compatible store to run anything from a
single demo host to a multi-DC enterprise deployment with **zero code
change**.

Korean / 한국어: [README.ko.md](README.ko.md)

Current baseline: **2026-05-25**. The repo is focused on SaaS pre-launch
hardening: webmail usability, notification safety, local-domain delivery,
spam controls, MCP automation, and split-mode deployment readiness.

## What it is

- Self-hosted mail platform: SMTP receive/submission/delivery + IMAP + POP3
- Bundled webmail (Next.js 16) and admin console
- Calendar / Contacts / Drive via CalDAV, CardDAV, and WebDAV
- LDAP directory front-end + SCIM 2.0 provisioning
- Multi-tenant: **company → domain → user** boundary in every query
- Single Go binary, 24 selectable runtime modes (see [`docs/MODES.md`](docs/MODES.md))

## Features

| Area | Features |
|---|---|
| Mail server | RFC 5321/5322 SMTP, RFC 6409 submission (587/465), RFC 5321/7672 outbound delivery with DANE |
| Mailbox protocols | IMAP4rev2 (RFC 9051) with IDLE/CONDSTORE/QRESYNC, POP3 (RFC 1939) |
| Collaboration | CalDAV (RFC 4791), CardDAV (RFC 6352), WebDAV (RFC 4918), LDAP (RFC 4511) |
| APIs | Mail API, Admin API, Auth server (JWT + refresh + MFA), SCIM 2.0 |
| Webmail / admin | Next.js 16 webmail SPA and admin console (`apps/webmail`, `apps/console`) |
| Email security | SPF (RFC 7208), DKIM (RFC 6376), DMARC (RFC 7489), ARC (RFC 8617), MTA-STS (RFC 8461), TLS-RPT (RFC 8460) |
| Auth | JWT (HS256, ≥32-byte secret), TOTP MFA, refresh-token rotation with replay detection, PBKDF2 password hashes |
| Anti-abuse | Per-IP/per-account brute-force tracker, configurable rate limits, DNSBL, milter, optional ClamAV |
| Observability | Prometheus metrics, slog JSON logs with secret redaction |
| Storage | PostgreSQL 16+, Redis 7+ (single / Sentinel / Cluster), S3 / MinIO / local FS |
| Reliability | Outbox Pattern (PG → Redis Streams), per-domain throttling, circuit breakers, graceful 30s drain |

## Current product surface

- **Webmail** — mail list/detail, compose, drafts, folder operations,
  attachments, search, All Mail, spam/blocked/allowed sender settings,
  profile photos, contacts, Drive, calendar, notification center, Web Push,
  MFA, refresh-token sessions, and localized settings in English, Korean,
  Japanese, and Simplified Chinese.
- **Admin console** — company/domain/user administration, audit logs,
  delivery attempts, suppression and routing controls, quota/storage views,
  security posture, SCIM/SSO/organization settings, and broad mocked E2E
  coverage for launch-readiness UI.
- **Mail pipeline** — inbound/submission SMTP, local-domain delivery without
  MX fallback, outbound delivery workers, DSN/bounce generation, DKIM/SPF/DMARC
  boundaries, spam relay hooks, retry scheduling, throttling, and event fan-out.
- **Agent automation** — separate management and user MCP servers so operators
  can manage the service while users can safely automate their own mailbox,
  contacts, Drive, calendar, and preferences.

## Strengths

- **One binary, many shapes** — modular monolith. Run all 24 modes in one
  process for dev; split each mode into its own deployment for scale.
- **Outbox Pattern guarantees event delivery** — no message lost on Redis
  outage; outbox-relay drains the backlog on recovery.
- **RFC-first protocols** — `5321`, `5322`, `9051`, `1939`, `4791`, `6352`,
  `4918`, `4511`, plus DKIM/SPF/DMARC/ARC/MTA-STS.
- **Production validator** — `internal/config/validate.go` rejects unsafe
  config at startup (insecure auth, HTTP S3 in prod, JWT secret < 32 bytes,
  localhost HELO, sslmode=disable in prod, …).
- **Minimal dependency surface** — Postgres + Redis + S3. No Kafka, no
  ZooKeeper, no service mesh.
- **Horizontal scale per workload** — each mode is independently
  scalable; singleton workers use PG advisory locks / Redis leases.
- **Compose/env deployment contract** — clone the repo, keep the same image,
  and grow from one host to split-mode SaaS by changing Compose profiles,
  env vars, and replica counts.
- **Single source of truth** — Postgres holds tenant, mailbox, and outbox
  state. No local spool, crash-safe restarts.
- **Local-first smoke path** — the dev Compose stack now starts the HTTP
  backend plus event, outbox relay, and delivery workers so webmail send/receive
  paths run without manual worker startup.

## Quick start

```bash
# Local development stack (Postgres + Redis + MinIO + ClamAV + backend + workers)
cd docker
docker compose -f docker-compose.dev.yml up -d --build \
  backend event-worker outbox-relay delivery-worker
```

Once up:

- Backend API: `http://localhost:8080/`
- Readiness: `http://localhost:8080/health/ready`
- Postgres / Redis / MinIO: `localhost:15432`, `localhost:16379`,
  `localhost:19000` (`localhost:19001` console)

Run the frontend apps separately when you are working on UI:

```bash
pnpm -C apps/webmail install
pnpm -C apps/webmail dev

pnpm -C apps/console install
pnpm -C apps/console dev
```

For production-like or split-mode deployments, start from the no-code scaling
template:

```bash
cd docker
cp env.scale.example .env
docker compose -f docker-compose.scale.yml --profile local-infra --profile protocols --profile workers up -d
docker compose -f docker-compose.scale.yml --profile ops run --rm migrate
```

Production deployments should follow
[`docker/DEPLOYMENT.md`](docker/DEPLOYMENT.md) and
[`docs/SCALING.md`](docs/SCALING.md).

## AI Agent Automation (MCP Servers)

GoGoMail has two separate [Model Context Protocol](https://modelcontextprotocol.io/) servers so agents can operate the platform without mixing administrator authority with end-user data access.

| Server | Directory | Audience | Scope |
|---|---|---|---|
| Management MCP | `apps/gogomail-manage-mcp` | Operators, support, administrators | 50 admin tools for Admin API, queue/health inspection, user/domain operations, organization membership/title metadata, security/spam policies, and optional Suppo/GitHub integrations |
| User MCP | `apps/gogomail-user-mcp` | Individual webmail users | 105 user tools for mail, drafts, folders, threads, contacts, directory, Drive, calendars, preferences, notifications/Web Push, spam UX, profile/avatar, and account context through user-scoped `gmu_` keys |

The split is intentional: the management MCP is for running GoGoMail as a service, while the user MCP lets a user connect Codex, Claude Desktop, or another agent to their own mailbox and collaboration data.

```
Operator request
    → AI agent
        → gogomail-manage-mcp
            → /admin/v1/...

User request
    → AI agent
        → gogomail-user-mcp
            → /api/v1/... and /api/mail/...
```

`gogomail-manage-mcp` currently exposes **50 admin tools**, including audited user/domain mutations, delivery and queue diagnostics, organization membership/title management, security and spam-filter policy helpers, and a guarded `gogomail_admin_api_request` bridge for documented admin-console routes without dedicated wrappers. All GoGoMail write actions require a human-readable `reason`; destructive operations also require exact confirmation.

`gogomail-user-mcp` currently exposes **105 user tools**, including mail send/drafts/search, bulk message and thread actions, notification preference and Web Push subscription/device helpers, spam report/not-spam and sender allow/block helpers, profile/avatar helpers, contact and calendar CRUD helpers, Drive upload/download/share tools, and an exact-manifest `gogomail_api_request` bridge for documented user APIs. Sensitive actions stay confirmation-gated in `basic` mode; `bypass` mode is allowed only when both user settings and domain policy permit it.

→ Management MCP documentation: [apps/gogomail-manage-mcp/README.md](apps/gogomail-manage-mcp/README.md) / [한국어](apps/gogomail-manage-mcp/README.ko.md)
→ User MCP documentation: [apps/gogomail-user-mcp/README.md](apps/gogomail-user-mcp/README.md) / [한국어](apps/gogomail-user-mcp/README.ko.md)

## Documentation

| Topic | File |
|---|---|
| Deployment guide (agent-friendly) | [docker/DEPLOYMENT.md](docker/DEPLOYMENT.md) |
| Scaling without code changes | [docs/SCALING.md](docs/SCALING.md) |
| Backend modes (24 modes, env vars) | [docs/MODES.md](docs/MODES.md) |
| Current implementation status | [docs/CURRENT_STATUS.md](docs/CURRENT_STATUS.md) |
| Architecture overview | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Security model and threat model | [docs/SECURITY.md](docs/SECURITY.md) |
| Operations / runbooks | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| Topology patterns | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI contract | [docs/openapi.yaml](docs/openapi.yaml) |
| Roadmap | [docs/backend-roadmap.md](docs/backend-roadmap.md) |
| AI Agent management MCP server | [apps/gogomail-manage-mcp/README.md](apps/gogomail-manage-mcp/README.md) |
| AI Agent user MCP server | [apps/gogomail-user-mcp/README.md](apps/gogomail-user-mcp/README.md) |
| User MCP security and policy notes | [docs/USER_MCP.md](docs/USER_MCP.md) |

## Build from source

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Requires Go 1.25+. Tests: `go test ./...`.

## License

See [LICENSE](LICENSE).
