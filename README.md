# GoGoMail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Self-hosted, multi-tenant mail and collaboration platform written in Go.
One static binary covers every server role — SMTP, IMAP, POP3, JMAP, CalDAV, CardDAV, WebDAV, LDAP, REST APIs, and background workers — with the role selected at startup. Pair it with PostgreSQL, Redis, and any S3-compatible store to run from a single demo host to a multi-DC enterprise deployment without changing a line of code.

Korean / 한국어: [README.ko.md](README.ko.md)

---

## What GoGoMail is

GoGoMail is a production-grade, open-source mail platform built for organizations that want to own their communication infrastructure. It ships as a single binary that speaks every major email and collaboration protocol natively, bundles a modern webmail UI and full admin console, and exposes AI automation interfaces (MCP servers) so agents can manage the service or act on behalf of individual users.

**It is for:**
- Teams self-hosting their mail stack instead of paying SaaS vendors
- Organizations that need strict multi-tenant isolation (company → domain → user) with full audit trails
- Developers building AI-native mail applications or automating mailbox workflows
- Operators who want one binary and three dependencies — not a dozen microservices

**It is not:**
- A single-user personal mail server (it is tenant-aware from day one)
- A drop-in Postfix/Dovecot replacement (it owns delivery, storage, and access in one process)
- A hosted service (you run it)

GoGoMail ships as a single static binary with 24 selectable runtime modes. Run every role in one process on a laptop for development, then promote individual modes to dedicated containers as load grows — all by changing Docker Compose profiles, environment variables, and replica counts. No code changes between topologies.

---

## Quick start

```bash
# Full dev stack: Postgres, Redis, MinIO, ClamAV, backend, workers, and monitoring
docker compose -f docker/docker-compose.dev.yml up -d
```

Once up:

| Service | URL |
|---|---|
| Backend API | `http://localhost:8080/` |
| Readiness probe | `http://localhost:8080/health/ready` |
| Grafana | `http://localhost:3000/` (admin / admin) |
| Postgres | `localhost:15432` |
| Redis | `localhost:16379` |
| MinIO console | `http://localhost:19001` |
| Web manual | `http://localhost:3005/` (run separately — see below) |

Run the frontend apps separately when working on UI:

```bash
pnpm -C apps/webmail install && pnpm -C apps/webmail dev
pnpm -C apps/console install && pnpm -C apps/console dev
pnpm -C apps/docs install && pnpm -C apps/docs dev       # web manual (port 3005)
```

### Seed dev data

Two seed datasets are available — pick the one that matches your preferred language:

```bash
bash scripts/seed_dev_beta.sh       # Korean locale (default)
bash scripts/seed_dev_beta_en.sh    # English locale
```

**Korean seed** (`parkjw.org` tenant — Korean display names, folders, and mail content):

| Account | Email | Password | Role |
|---|---|---|---|
| Admin | `admin@gogomail.io` | `admin1234` | admin |
| Demo user | `user@parkjw.org` | `pass1234` | user |

**English seed** (`acme.io` tenant — English display names, folders, and mail content):

| Account | Email | Password | Role |
|---|---|---|---|
| Admin | `admin@gogomail.io` | `admin1234` | admin |
| Demo user | `user@acme.io` | `pass1234` | user |

Both seeds include: inbox messages, custom folders, 22 contacts, and 2 CalDAV calendars. 13 co-worker accounts share password `pass1234`. Both tenants can coexist in the same database.

```bash
bash scripts/reset_dev_data.sh --yes   # wipe and reseed from scratch
```

For production or split-mode deployments:

```bash
cd docker
cp env.scale.example .env
docker compose -f docker-compose.scale.yml --profile local-infra --profile protocols --profile workers up -d
docker compose -f docker-compose.scale.yml --profile ops run --rm migrate
```

See [`docker/DEPLOYMENT.md`](docker/DEPLOYMENT.md) and [`docs/SCALING.md`](docs/SCALING.md) for production topology.

### Deployment topology

The same binary, the same image, the same config format — at every scale:

| Topology | When to use |
|---|---|
| `docker-compose.dev.yml` (all-in-one) | Local development — every role in one process |
| `docker-compose.scale.yml` + profiles | Single-site production — roles split across containers |
| Kubernetes (`k8s/` manifests or `helm/gogomail`) | Multi-node, HPA autoscaling, rolling deploys, PodDisruptionBudgets |

Each of the 24 runtime modes scales independently. Singleton workers coordinate via PostgreSQL advisory locks and Redis leases — no external coordination service required.

---

## Protocols and modules

Each module speaks a defined set of open standards. No proprietary extensions, no vendor lock-in.

### Mail transport

| Module | Standards |
|---|---|
| SMTP receive (edge MTA) | RFC 5321, RFC 5322, RFC 2045–2049, RFC 6531/6532 (SMTPUTF8) |
| SMTP submission | RFC 6409 (ports 587/465), RFC 4954 (AUTH) |
| SMTP outbound delivery | RFC 5321, RFC 7672 (DANE), RFC 7505 (null MX), RFC 3461/3464 (DSN/bounce) |
| SMTP relay / smarthost | RFC 5321 |

### Email security

| Standard | RFC |
|---|---|
| DKIM signing and verification | RFC 6376 |
| SPF | RFC 7208 |
| DMARC | RFC 7489 |
| ARC (Authenticated Received Chain) | RFC 8617 |
| MTA-STS | RFC 8461 |
| TLS-RPT | RFC 8460 |
| DNSBL | RFC 5782 |
| Milter (external spam hooks) | sendmail milter v2/v6 |

### Mailbox access

| Module | Standards |
|---|---|
| IMAP4rev2 | RFC 9051 (+ RFC 3501 fallback), IDLE, CONDSTORE, QRESYNC |
| POP3 | RFC 1939, RFC 2449 (CAPA), RFC 2595 (STLS), RFC 1734 (AUTH) |
| JMAP Core + Mail | RFC 8620, RFC 8621 — 20 methods, EventSource SSE push |

### Collaboration

| Module | Standards |
|---|---|
| CalDAV | RFC 4791, RFC 7809 (timezone), RFC 6638 (scheduling), RFC 5545 (iCalendar) |
| iMIP scheduling | RFC 6047 |
| CardDAV | RFC 6352, RFC 6350 (vCard 4.0), RFC 2426 (vCard 3.0) |
| Drive (WebDAV) | RFC 4918, RFC 3744 (ACL), RFC 4331 (quota) |
| LDAP gateway | RFC 4511, RFC 4512, RFC 4519 |

### Identity and provisioning

| Module | Standard |
|---|---|
| SCIM 2.0 | RFC 7642, RFC 7643, RFC 7644 |
| SAML 2.0 SSO | OASIS SAML 2.0 Core |
| OpenID Connect SSO | OpenID Connect Core 1.0, RFC 7636 (PKCE) |
| JWT auth | RFC 7519 |
| TOTP / HOTP MFA | RFC 6238 (TOTP), RFC 4226 (HOTP) |

### Infrastructure

| Module | Standard |
|---|---|
| DNS autodiscovery | RFC 6764 (Well-Known URIs, DNS SRV) |
| Web Push | RFC 8030 |
| TLS | RFC 8446 (TLS 1.3), RFC 5246 (TLS 1.2 minimum) |
| Real-time config push | Server-Sent Events (HTML5 EventSource) |

---

## Features

| Area | What's included |
|---|---|
| Webmail | Mail list/detail, compose, drafts, folder operations, attachments, search, spam/allowed sender settings, profile photos, contacts, Drive, calendar, encrypted DM, notification center, Web Push, MFA, localized UI (English, Korean, Japanese, Simplified Chinese) |
| Encrypted DM | Participant-only direct/group rooms with per-room encrypted storage, unread/read state, group owners, invites, text/file/Drive messages, reactions, search, media/link views |
| Admin console | Company/domain/user management, RBAC with custom roles, audit logs, mail flow logs, delivery attempts, suppression and routing rules, quota/storage views, spam-filter policy, SCIM/SSO/LDAP/RDBMS identity config, security posture, alerts, reports, analytics |
| Mail pipeline | Inbound/submission SMTP, local-domain delivery, outbound delivery workers, DSN/bounce generation, DKIM/SPF/DMARC boundaries, spam scoring hooks, retry scheduling, throttling, event fan-out |
| Anti-abuse | Built-in spam filter (SPF/DKIM/DMARC scoring, RBL/DNSBL, attachment extension scoring, phrase packs, bulk recipient limits), per-IP/per-account brute-force tracker, optional ClamAV AV scanning, milter hook |
| Auth and security | PBKDF2 password hashing with legacy auto-upgrade, TOTP MFA, refresh-token rotation with replay detection, rate limiting, IDOR isolation in every admin handler, internal header stripping |
| Observability | Prometheus metrics, structured slog JSON, X-Request-ID correlation, cleanup/rollback warning logs, SCIM sync warnings, Grafana dashboards, Loki log aggregation |
| Storage | PostgreSQL 16+, Redis 7+ (single / Sentinel / Cluster), S3 / MinIO / local FS |
| Reliability | Outbox Pattern (PG → Redis Streams), per-domain throttling, circuit breakers, graceful 30s drain, remote-signer timeouts/shutdown, crash-safe restarts |
| Deployment | Single Go binary, 24 selectable runtime modes, Docker Compose dev/scale profiles, Helm chart, Kubernetes manifests (HPA, PDB, ingress) |

---

## AI-native features

GoGoMail is built for the AI-agent era. It ships two Model Context Protocol (MCP) servers that give agents structured access to every platform capability — without mixing administrator authority with end-user data.

| Server | Audience | Tools |
|---|---|---|
| Management MCP (`apps/gogomail-manage-mcp`) | Operators, support, administrators | **50 tools** — user/domain mutations, delivery and queue diagnostics, organization membership/title management, security and spam-filter policy, admin API bridge |
| User MCP (`apps/gogomail-user-mcp`) | Individual webmail users | **123 tools** — mail send/search/bulk actions, DM rooms/messages/reactions, contacts, calendar, Drive upload/download/share, notification and Web Push, spam UX, profile/avatar |

The split is intentional: the management MCP is for running GoGoMail as a service; the user MCP lets a user connect Claude Desktop, Codex, or any other MCP-capable agent to their own mailbox and collaboration data without touching admin territory.

```
Operator agent          →  gogomail-manage-mcp  →  /admin/v1/...
Individual user agent   →  gogomail-user-mcp    →  /api/v1/... and /api/mail/...
```

All GoGoMail write actions require a human-readable `reason`; destructive operations require exact confirmation. Sensitive user actions are confirmation-gated in `basic` mode; `bypass` mode is domain-policy-controlled.

→ Management MCP: [apps/gogomail-manage-mcp/README.md](apps/gogomail-manage-mcp/README.md) / [한국어](apps/gogomail-manage-mcp/README.ko.md)
→ User MCP: [apps/gogomail-user-mcp/README.md](apps/gogomail-user-mcp/README.md) / [한국어](apps/gogomail-user-mcp/README.ko.md)

---

## Architecture principles

- **One binary, many shapes** — modular monolith. Run all 24 modes in one process for dev; split each mode into its own deployment for scale. No code changes between topologies.
- **Outbox Pattern guarantees delivery** — no event lost on Redis outage; outbox-relay drains the backlog on recovery.
- **Minimal dependency surface** — Postgres + Redis + S3. No Kafka, no ZooKeeper, no service mesh required.
- **Horizontal scale per workload** — each mode scales independently; singleton workers use PG advisory locks / Redis leases.
- **Production validator** — `internal/config/validate.go` rejects unsafe config at startup: insecure auth, HTTP S3 in prod, JWT secret < 32 bytes, localhost HELO, `sslmode=disable`, CHANGEME placeholders.
- **Multi-tenant by default** — company → domain → user boundary enforced in every query; no shared-state leakage between tenants.
- **Compose/env deployment contract** — clone the repo, keep the same image, and grow from one host to split-mode SaaS by changing Compose profiles, env vars, and replica counts.

---

## Documentation

| Topic | File |
|---|---|
| Deployment guide | [docker/DEPLOYMENT.md](docker/DEPLOYMENT.md) |
| Scaling without code changes | [docs/SCALING.md](docs/SCALING.md) |
| Backend modes (24 modes, env vars) | [docs/MODES.md](docs/MODES.md) |
| Architecture overview | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Security model | [docs/SECURITY.md](docs/SECURITY.md) |
| Security review | [docs/SECURITY_REVIEW.md](docs/SECURITY_REVIEW.md) |
| Operations / runbooks | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| Topology patterns | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI contract | [docs/openapi.yaml](docs/openapi.yaml) |
| Roadmap | [docs/backend-roadmap.md](docs/backend-roadmap.md) |
| User MCP policy notes | [docs/USER_MCP.md](docs/USER_MCP.md) |
| Web manual (VitePress, en/ko/ja/zh-CN) | [apps/docs/](apps/docs/) — `pnpm -C apps/docs dev` |

---

## Build from source

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Requires Go 1.25+. Tests: `go test ./...`.

## License

See [LICENSE](LICENSE).
