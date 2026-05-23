# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

A self-hosted, multi-tenant mail and collaboration platform written in Go.
One static binary serves SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP,
REST APIs, and event workers — each role selectable at startup. Pair it
with PostgreSQL, Redis, and any S3-compatible store to run anything from a
single demo host to a multi-DC enterprise deployment with **zero code
change**.

Korean / 한국어: [README.ko.md](README.ko.md)

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
- **Single source of truth** — Postgres holds tenant, mailbox, and outbox
  state. No local spool, crash-safe restarts.

## Quick start

```bash
# All-in-one demo (Postgres + Redis + MinIO + gogomail)
cd docker
cp .env.example .env   # edit secrets
docker compose -f docker-compose.small.yml up -d
```

Once up:

- Webmail / API: `https://localhost/` (via the bundled nginx)
- Admin console: behind the same nginx, `/admin` route
- Metrics: `:9090/metrics` (when `GOGOMAIL_METRICS_BACKEND=prometheus`)

For production deployments, follow
[`docker/DEPLOYMENT.md`](docker/DEPLOYMENT.md) — the agent-friendly
deployment guide.

## Documentation

| Topic | File |
|---|---|
| Deployment guide (agent-friendly) | [docker/DEPLOYMENT.md](docker/DEPLOYMENT.md) |
| Backend modes (24 modes, env vars) | [docs/MODES.md](docs/MODES.md) |
| Architecture overview | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Security model and threat model | [docs/SECURITY.md](docs/SECURITY.md) |
| Operations / runbooks | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| Topology patterns | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI contract | [docs/openapi.yaml](docs/openapi.yaml) |
| Roadmap | [docs/backend-roadmap.md](docs/backend-roadmap.md) |

## Build from source

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Requires Go 1.25+. Tests: `go test ./...`.

## License

See [LICENSE](LICENSE).
