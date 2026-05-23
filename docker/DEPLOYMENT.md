# gogomail Deployment Guide

> **Audience** — Ops engineers and AI agents generating docker-compose
> (or k8s) manifests for gogomail. This document is the single source of
> truth for env vars, ports, sizing, and DNS records. Cross-reference
> [`docs/MODES.md`](../docs/MODES.md) for per-mode details and
> [`docs/SECURITY.md`](../docs/SECURITY.md) for the threat model.
>
> Korean / 한국어: [DEPLOYMENT.ko.md](DEPLOYMENT.ko.md)

---

## Table of contents

1. [Introduction](#1-introduction)
2. [Required infrastructure](#2-required-infrastructure)
3. [Sizing decision tree](#3-sizing-decision-tree)
4. [Mode-to-container mapping](#4-mode-to-container-mapping)
5. [Required env vars](#5-required-env-vars)
6. [Compose recipes](#6-compose-recipes)
7. [Network exposure matrix](#7-network-exposure-matrix)
8. [DNS setup](#8-dns-setup)
9. [TLS / certificates](#9-tls--certificates)
10. [Initial setup](#10-initial-setup)
11. [Operations](#11-operations)
12. [For AI agents](#12-for-ai-agents)

---

## 1. Introduction

gogomail ships as one static Go binary that selects its runtime role at
startup. There are **24 modes** (see [`docs/MODES.md`](../docs/MODES.md)).
This guide explains how to compose those modes into a working production
deployment using Docker Compose. The same env vars and ports map cleanly
to Kubernetes Deployments — substitute `replicas:` for Compose scaling and
`Service` for the LB.

Scope:
- Hard infra requirements and version floors
- A decision tree for picking a topology
- A complete mode-to-container reference table
- A complete env-var reference
- Four ready-to-customize compose recipes (single, small, medium, large)
- Required DNS, TLS, and first-time bootstrap steps
- Explicit instructions for AI agents at the end

Out of scope: k8s manifests (use the same env vars), managed-service
specifics (RDS, ElastiCache — substitute as the operator), client-side
mail apps.

---

## 2. Required infrastructure

Hard requirements. Everything below the floor is unsupported.

| Component | Min version | Notes |
|---|---|---|
| **PostgreSQL** | 16+ | Extensions: `pg_trgm`, `uuid-ossp`, `pgcrypto` (created by migrations). HA: streaming replication or Patroni. |
| **Redis** | 7+ | Single, Sentinel, or Cluster. KeyDB / Dragonfly with Redis 7 protocol also work. `requirepass` mandatory in prod. |
| **S3-compatible store** | MinIO 8+, AWS S3, Backblaze B2, Cloudflare R2, Wasabi | HTTPS endpoint required in prod (validator rejects HTTP in prod). Server-side encryption (SSE) recommended. |
| **Reverse proxy / LB** | nginx 1.24+, Caddy 2.7+, HAProxy 2.8+, Traefik 3+ | TLS termination, HSTS, `X-Forwarded-*` headers. HAProxy preferred for L4 SMTP fan-out. |
| **Runtime** | Docker 24+ or k8s 1.28+ | Container runs as non-root distroless. Needs CAP\_NET\_BIND for ports < 1024 — or remap on host. |
| **OpenSearch** (optional) | OpenSearch 2.11+ / Elasticsearch 8+ | Only needed when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`. |
| **ClamAV** (optional) | clamd 1.0+ | Only when `GOGOMAIL_ATTACHMENT_SCAN_BACKEND=clamav`. |

**Server resources (per role, recommended floor)**:

| Role | CPU | RAM | Disk |
|---|---|---|---|
| all-in-one | 2 vCPU | 2 GiB | 10 GiB + mailstore |
| edge-mta / inbound / outbound | 0.5 vCPU | 512 MiB | 1 GiB |
| imap | 1 vCPU | 1 GiB | 1 GiB (long TCP) |
| mail-api / admin-api / auth-server | 1 vCPU | 1 GiB | 1 GiB |
| delivery-worker | 0.5 vCPU | 512 MiB | 1 GiB |
| outbox-relay (singleton) | 0.5 vCPU | 256 MiB | 1 GiB |
| Postgres (medium) | 4 vCPU | 16 GiB | 100+ GiB SSD |
| Redis (medium) | 2 vCPU | 4 GiB | persistent AOF |
| MinIO node (medium) | 2 vCPU | 4 GiB | 1+ TiB |

---

## 3. Sizing decision tree

```
Q1: < 50 mailboxes, internal-only, single host OK?
    yes -> Pattern A: Single-node (docker-compose.small.yml as base)
    no  -> Q2

Q2: < 5k mailboxes, < 50k mail/day, single AZ acceptable?
    yes -> Pattern B: Small (2 app hosts + 1 worker host + managed PG/Redis/S3)
    no  -> Q3

Q3: < 50k mailboxes, < 500k mail/day, multi-AZ one region?
    yes -> Pattern C: Medium (role-split: edge / app / worker / index)
    no  -> Pattern D: Large (full mode-split, multi-DC, k8s)
```

| Pattern | Mailboxes | Mail/day | Hosts | Compose file |
|---|---:|---:|---:|---|
| A single-node | < 500 | < 5k | 1 | `docker-compose.small.yml` |
| B small | < 5k | < 50k | 3-5 | derived from `small.yml` |
| C medium | < 50k | < 500k | 15-25 | `docker-compose.medium.yml` |
| D large | 50k+ | 500k+ | k8s | `docker-compose.large.yml` (k8s in practice) |

If unsure, start one tier smaller and scale out. Every pattern is
upgradeable in-place by adding container instances and switching env vars;
no schema or code changes are needed.

---

## 4. Mode-to-container mapping

The 24 modes, sortable into containers. `APP_MODE` is the canonical
selector (equivalent to `-mode` flag).

| Mode | `APP_MODE` value | Replicas (min/rec) | Memory | CPU | Ports exposed | Hard deps | Scaling rule | Health |
|---|---|---|---|---|---|---|---|---|
| All-in-one | `all-in-one` | 1 / 2 | 1 GiB | 1 | 8080 (HTTP) | PG, Redis, S3 | stateless | `GET /health/ready` |
| Edge MTA | `edge-mta` | 2 / 3+ | 512 MiB | 0.5 | 25 (or 2525) | PG, Redis | stateless | TCP probe `:25` |
| Inbound MTA (trusted) | `inbound-mta` | 2 / 2 | 256 MiB | 0.25 | 2526 (internal) | PG, Redis | stateless | TCP probe |
| Outbound MTA (submission) | `outbound-mta` | 2 / 3 | 512 MiB | 0.5 | 587 + 465 | PG, Redis | stateless | TCP probe |
| Delivery worker | `delivery-worker` | 2 / 3+ | 512 MiB | 0.5 | — (outbound 25) | PG, Redis | consumer-group sharded | log/metric heartbeat |
| IMAP | `imap` | 2 / 3+ | 1 GiB | 1 | 143 + 993 | PG, Redis, S3 | stateless, sticky-by-IP optional | TCP probe |
| POP3 | `pop3` | 2 / 2 | 512 MiB | 0.5 | 110 + 995 | PG, S3 | stateless | TCP probe |
| CalDAV | `caldav` | 2 / 2 | 512 MiB | 0.5 | 8081 | PG | stateless | TCP probe |
| CardDAV | `carddav` | 2 / 2 | 512 MiB | 0.5 | 8082 | PG | stateless | TCP probe |
| WebDAV (Drive) | `webdav` | 2 / 2 | 512 MiB | 0.5 | 8083 | PG, S3 | stateless | TCP probe |
| LDAP gateway | `ldap-gateway` | 2 / 2 | 512 MiB | 0.5 | 389 + 636 (LDAPS) | PG | stateless | TCP probe |
| Mail API | `mail-api` | 2 / 3+ | 1 GiB | 1 | 8080 | PG, Redis, S3 | stateless | `GET /health/ready` |
| Admin API | `admin-api` | 2 / 2 | 1 GiB | 1 | 8080 | PG, Redis | stateless | `GET /health/ready` |
| Auth server | `auth-server` | 2 / 2 | 512 MiB | 0.5 | 8080 | PG, Redis | stateless | `GET /health/ready` |
| Outbox relay | `outbox-relay` | 2 / 2 | 256 MiB | 0.25 | — | PG, Redis | **singleton (PG lock)** — replicas for failover only | metric `outbox_lag` |
| Event worker | `event-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group sharded | metric heartbeat |
| Search index worker | `search-index-worker` | 2 / 2 | 1 GiB | 1 | — | PG, Redis, OpenSearch | consumer-group sharded | metric heartbeat |
| Push notification worker | `push-notification-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group sharded | metric heartbeat |
| API metering worker | `api-metering-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group sharded | metric heartbeat |
| Attachment cleanup | `attachment-cleanup-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG, S3 | **singleton (advisory lock)** | metric heartbeat |
| Drive cleanup | `drive-cleanup-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG, S3 | **singleton** | metric heartbeat |
| DAV sync retention | `dav-sync-retention-worker` | 1 / 1 | 256 MiB | 0.25 | — | PG | **singleton**, dry-run by default | metric heartbeat |
| API usage retention | `api-usage-retention-worker` | 1 / 1 | 256 MiB | 0.25 | — | PG | **singleton**, dry-run by default | metric heartbeat |
| Batch worker | `batch-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG | **singleton per job** | metric heartbeat |

**Notes**:
- Singletons elected via Postgres advisory lock (or Redis lease when
  `GOGOMAIL_FARM_COORDINATOR_BACKEND=redis`). Run 2 replicas for failover;
  only one performs work at a time.
- Each mode reads **only** the env vars it needs; you can pass the full
  superset to every container and let the validator ignore unused fields.
- Default internal ports for cleartext SMTP/IMAP/POP3 are `2525 / 1143 /
  1110` so non-root containers can bind without `CAP_NET_BIND_SERVICE`.
  Map host ports to internet-standard numbers (25 / 143 / 110) at publish.
- `outbox-relay` is global. In multi-DC, run it only in the DC that hosts
  the PG primary; the standby DC's replicas wait on the advisory lock.

### Dependency graph (start order)

```
postgres ──┐
redis ─────┼──> outbox-relay ──> delivery/search/push/api-metering workers
s3/minio ──┘    │
                ├──> edge-mta, outbound-mta, mail-api, admin-api, auth-server
                ├──> imap, pop3, caldav, carddav, webdav, ldap-gateway
                └──> cleanup / retention / batch (singletons)
```

Compose `depends_on:` with `condition: service_healthy` covers infra
dependencies; the app-layer order above is enforced by validators (the
process exits with a clear error if a required dep is unreachable).

---

## 5. Required env vars

Every env var read by the validator. **R** = required for the listed
scope; **O** = optional (default shown). "prod" means
`GOGOMAIL_ENV=production`.

### 5.1 Core

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_ENV` | R | `development` | enum `development/test/production` | `production` | Enables strict validator. |
| `APP_MODE` | R | `all-in-one` | enum (see §4) | `mail-api` | Equivalent to `-mode` CLI flag. |
| `GOGOMAIL_PUBLIC_BASE_URL` | R prod | _empty_ | absolute URL | `https://mail.example.com` | Must be https in prod, no localhost. |
| `GOGOMAIL_HTTP_ADDR` | O | `:8080` | host:port | `:8080` | API/admin/auth listener. |
| `GOGOMAIL_CORS_ALLOWED_ORIGINS` | O | _empty_ | CSV | `https://webmail.example.com` | Set for browser apps. |

### 5.2 Database

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_DATABASE_URL` | R | `postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable` | DSN | `postgres://gogomail:secret@pg.internal:5432/gogomail?sslmode=require` | Must **not** be `sslmode=disable` in prod. |
| `GOGOMAIL_DB_MAX_OPEN_CONNS` | O | `20` | int | `40` | Per replica. |
| `GOGOMAIL_DB_MAX_IDLE_CONNS` | O | `5` | int | `10` | |
| `GOGOMAIL_DB_CONN_MAX_LIFETIME` | O | `30m` | duration | `30m` | |
| `GOGOMAIL_DB_CONN_MAX_IDLE_TIME` | O | `5m` | duration | `5m` | |
| `GOGOMAIL_MIGRATION_DIR` | O | `/app/migrations` | path | _default in image_ | Set by Dockerfile. |

### 5.3 Redis

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_REDIS_ADDR` | R (single) | `localhost:6379` | host:port | `redis:6379` | Or use Sentinel. |
| `GOGOMAIL_REDIS_PASSWORD` | R prod | _empty_ | string | `…32+ random…` | Required when farm coordinator=redis. |
| `GOGOMAIL_REDIS_SENTINEL_ADDRS` | O | _empty_ | CSV host:port | `s1:26379,s2:26379,s3:26379` | Enables Sentinel HA. |
| `GOGOMAIL_REDIS_MASTER_NAME` | O | `mymaster` | string | `mymaster` | Sentinel master name. |
| `GOGOMAIL_FARM_COORDINATOR_BACKEND` | R prod | `noop` | enum `noop/redis` | `redis` | **Must be `redis` in prod.** |
| `GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL` | O | `30s` | duration | `30s` | Lease TTL for singleton election. |
| `GOGOMAIL_FARM_COORDINATOR_JOB_VISIBILITY_TIMEOUT` | O | `5m` | duration | `5m` | |

### 5.4 Storage (S3 / MinIO / local)

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_STORAGE_BACKEND` | R | `local` | enum `local/nfs/s3/minio` | `s3` | |
| `GOGOMAIL_STORAGE_S3_ENDPOINT` | R for s3/minio | _empty_ | URL | `https://minio.internal:9000` | **HTTPS required in prod.** |
| `GOGOMAIL_STORAGE_S3_REGION` | R for s3/minio | `us-east-1` | string | `us-east-1` | |
| `GOGOMAIL_STORAGE_S3_BUCKET` | R for s3/minio | _empty_ | bucket name | `gogomail` | Must exist; see §10. |
| `GOGOMAIL_STORAGE_S3_PREFIX` | O | _empty_ | path | `prod/` | |
| `GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID` | R for s3/minio | _empty_ | string | `AKIA…` | |
| `GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY` | R for s3/minio | _empty_ | string | _secret_ | |
| `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE` | O | `false` | bool | `true` | Set `true` for MinIO. |
| `GOGOMAIL_STORAGE_S3_CA_CERT_FILE` | O | _empty_ | path | `/certs/ca.pem` | Self-signed MinIO CA. |
| `GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY` | _forbidden prod_ | `false` | bool | `false` | Must be false in prod. |
| `GOGOMAIL_MAILSTORE_ROOT` | R for local/nfs | `var/mailstore` | path | `/var/lib/gogomail/mailstore` | |

### 5.5 Auth + admin

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_AUTH_JWT_SECRET` | R prod | _empty_ | string ≥32 bytes | `openssl rand -base64 32` | **HS256 secret; ≥32 bytes in prod.** |
| `GOGOMAIL_ADMIN_TOKEN` | R prod | _empty_ | string ≥32 bytes | _random_ | Admin automation bearer. |
| `GOGOMAIL_ADMIN_MFA_REQUIRED` | O | `false` | bool | `true` | Enforce TOTP for admins. |
| `GOGOMAIL_SCIM_TOKEN` | O | _empty_ | string | _bearer_ | Enables SCIM 2.0. |
| `GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID` | O if SCIM | _empty_ | UUID | `…` | |

### 5.6 SMTP / submission / delivery

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_SMTP_ADDR` | R for edge | `:2525` | host:port | `:2525` | Map 25→2525 at host. |
| `GOGOMAIL_SMTP_DOMAIN` | R | `localhost` | hostname | `mail.example.com` | **Must not be localhost in prod.** |
| `GOGOMAIL_SMTP_TLS_CERT_FILE` | R for prod TLS | _empty_ | path | `/certs/smtp.pem` | STARTTLS. |
| `GOGOMAIL_SMTP_TLS_KEY_FILE` | R for prod TLS | _empty_ | path | `/certs/smtp.key` | |
| `GOGOMAIL_SMTP_DMARC_ENFORCEMENT` | O | `reject` | enum `monitor/quarantine/reject` | `reject` | |
| `GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED` | O | `false` | bool | `true` | Enables SPF/DKIM/DMARC verification. |
| `GOGOMAIL_SMTP_MAX_CONNECTIONS` | O | `10000` | int | `10000` | |
| `GOGOMAIL_SMTP_MAX_MESSAGE_BYTES` | O | `26214400` (25 MiB) | int64 | `52428800` | |
| `GOGOMAIL_SUBMISSION_ADDR` | R for outbound | `:2587` | host:port | `:2587` | Maps to 587. |
| `GOGOMAIL_SUBMISSION_SMTPS_ADDR` | O | _empty_ | host:port | `:2465` | Implicit-TLS 465; needs TLS files. |
| `GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH` | _forbidden prod_ | `false` | bool | `false` | |
| `GOGOMAIL_DELIVERY_SMTP_HELLO` | R | _empty_ | hostname | `mail.example.com` | Public hostname; not localhost in prod. |
| `GOGOMAIL_DELIVERY_TLS_MODE` | O | `opportunistic` | enum `opportunistic/require/disable` | `opportunistic` | `disable` only for relays. |
| `GOGOMAIL_DELIVERY_SMARTHOST` | O | _empty_ | host:port | `smtp.sendgrid.net:587` | Optional smarthost relay. |
| `GOGOMAIL_DELIVERY_RETRY_DELAYS` | O | _embedded defaults_ | CSV durations | `1m,5m,15m,1h,6h,24h` | |
| `GOGOMAIL_DKIM_ENABLED` | O | `false` | bool | `true` | Outbound DKIM signing. |
| `GOGOMAIL_DNSBL_ZONES` | O | _empty_ | CSV | `zen.spamhaus.org,b.barracudacentral.org` | |
| `GOGOMAIL_INBOUND_TRUSTED_RELAYS` | O | `127.0.0.1/32,::1/128` | CSV CIDRs | `10.0.0.0/8` | Only for `inbound-mta`. |
| `GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE` | O | `60` | int >0 | `60` | |
| `GOGOMAIL_RATELIMIT_BACKEND` | O | `none` | enum `none/redis` | `redis` | Set redis in prod. |
| `GOGOMAIL_BACKPRESSURE_BACKEND` | O | `none` | enum `none/redis` | `redis` | |

### 5.7 IMAP / POP3 / DAV / LDAP

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_IMAP_ADDR` | R for imap | `:1143` | host:port | `:1143` | Map 143→1143. |
| `GOGOMAIL_IMAP_TLS_CERT_FILE` / `_KEY_FILE` | R for STARTTLS | _empty_ | path | `/certs/imap.pem` | |
| `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH` | _forbidden prod_ | `false` | bool | `false` | |
| `GOGOMAIL_IMAP_MAX_CONNECTIONS` | O | `5000` | int | `5000` | |
| `GOGOMAIL_IMAP_IDLE_TIMEOUT` | O | `30m` | duration | `30m` | |
| `GOGOMAIL_POP3_ADDR` | R for pop3 | `:1110` | host:port | `:1110` | |
| `GOGOMAIL_POP3S_ADDR` | O | _empty_ | host:port | `:1995` | Implicit TLS. |
| `GOGOMAIL_POP3_MAX_CONNECTIONS` | O | `2000` | int | `2000` | |
| `GOGOMAIL_CALDAV_ADDR` | R for caldav | `:8081` | host:port | `:8081` | |
| `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH` | _forbidden prod_ | `false` | bool | `false` | |
| `GOGOMAIL_CALDAV_TRUSTED_PROXIES` | O | _empty_ | CSV CIDRs | `10.0.0.0/8` | For `X-Forwarded-For`. |
| `GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO` | O | `false` | bool | `true` | Behind TLS-terminating LB. |
| `GOGOMAIL_CALDAV_SCHEDULING` | O | `false` | bool | `true` | RFC 6638. |
| `GOGOMAIL_CARDDAV_ADDR` | R for carddav | `:8082` | host:port | `:8082` | |
| `GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH` | _forbidden prod_ | `false` | bool | `false` | |
| `GOGOMAIL_WEBDAV_ADDR` | R for webdav | `:8083` | host:port | `:8083` | |
| `GOGOMAIL_WEBDAV_DEPTH_INFINITY_ENABLED` | O | `false` | bool | `false` | DoS guard; leave off. |
| `GOGOMAIL_LDAP_ADDR` | R for ldap | `:389` | host:port | `:1389` | |
| `GOGOMAIL_LDAPS_ADDR` | O | _empty_ | host:port | `:1636` | Requires LDAP TLS files. |
| `GOGOMAIL_LDAP_COMPANY_ID` | R for ldap | _empty_ | UUID | _company id_ | |
| `GOGOMAIL_LDAP_BASE_DOMAIN` | R for ldap | _empty_ | string | `example.com` | |

### 5.8 Workers / events

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE` | O | `100` | int >0 | `200` | |
| `GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL` | O | `1s` | duration | `1s` | |
| `GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS` | O | (positive) | int >0 | `10` | |
| `GOGOMAIL_DELIVERY_STREAM` | O | `delivery.event` | string | `delivery.event` | |
| `GOGOMAIL_DELIVERY_CONSUMER_GROUP` | O | `gogomail.delivery-worker` | string | _default_ | |
| `GOGOMAIL_DELIVERY_CONSUMER_COUNT` | O | positive | int >0 | `4` | |
| `GOGOMAIL_DELIVERY_THROTTLE_BACKEND` | O | `local` | enum `local/redis` | `redis` | Cluster-wide throttle. |
| `GOGOMAIL_SEARCH_INDEX_BACKEND` | O | `disabled` | enum `disabled/postgres/opensearch` | `opensearch` | |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT` | R for opensearch | _empty_ | URL | `https://opensearch:9200` | |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX` | R for opensearch | _empty_ | string | `gogomail-mail` | |
| `GOGOMAIL_PUSH_NOTIFICATION_BACKEND` | O | `none` | enum `none/slog/webhook` | `webhook` | Plus APNs / WebPush envs. |
| `GOGOMAIL_APNS_KEY_ID` / `_TEAM_ID` / `_PRIVATE_KEY` / `_BUNDLE_ID` | O (APNs) | _empty_ | string | _APNs creds_ | |
| `GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY` / `_PRIVATE_KEY` / `_CONTACT_EMAIL` | O (WebPush) | _empty_ | string | _VAPID_ | |
| `GOGOMAIL_API_METERING_BACKEND` | O | `none` | enum `none/slog/outbox` | `outbox` | |
| `GOGOMAIL_EVENT_STREAM` | O | _default_ | string | `gogomail.event` | |
| `GOGOMAIL_EVENT_CONSUMER_GROUP` | O | _default_ | string | _default_ | |
| `GOGOMAIL_EVENT_CONSUMER_COUNT` | O | positive | int >0 | `4` | |

### 5.9 Observability / rate limits / misc

| Env var | Scope | Default | Format | Example | Notes |
|---|---|---|---|---|---|
| `GOGOMAIL_METRICS_BACKEND` | O | `none` | enum `none/slog/prometheus` | `prometheus` | |
| `GOGOMAIL_METRICS_ADDR` | O | _empty_ | host:port | `:9090` | Required when prometheus. |
| `GOGOMAIL_MAIL_MUTATION_RATELIMIT_BACKEND` | O | `none` | enum `none/redis` | `redis` | |
| `GOGOMAIL_MAIL_MUTATION_RATELIMIT_PER_MINUTE` | O | `300` | int >0 | `300` | |
| `GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE` | O | `120` | int >0 | `120` | |
| `GOGOMAIL_ATTACHMENT_SCAN_BACKEND` | O | `none` | enum `none/webhook/clamav` | `clamav` | |
| `GOGOMAIL_ATTACHMENT_SCAN_CLAMAV_ADDR` | R for clamav | `127.0.0.1:3310` | host:port | `clamav:3310` | |
| `GOGOMAIL_AUTO_PURGE_ENABLED` | O | `false` | bool | `false` | Trash auto-purge. |
| `GOGOMAIL_HTTP_MAX_HEADER_BYTES` | O | `65536` | int 4096..1048576 | `65536` | |

Full list: `grep envOrDefault internal/config/config.go`. Validator
behavior: `internal/config/validate.go`.

---

## 6. Compose recipes

Each recipe is a starting point. Adjust replicas with `docker compose up
-d --scale <service>=N` or set `deploy.replicas` in Swarm.

### 6.1 Pattern A — single-node (demo / very small)

**Topology**

```
                 +---------------------------------+
   Internet ---> | host: docker-compose.small.yml  |
                 |  - nginx (TLS terminator)        |
                 |  - gogomail (all-in-one)         |
                 |  - postgres 16                   |
                 |  - redis 7                        |
                 |  - minio (single node)           |
                 +---------------------------------+
```

**Compose** — use the shipped [`docker-compose.small.yml`](docker-compose.small.yml)
as-is. Critical edits to `.env`:

```bash
GOGOMAIL_ENV=production
GOGOMAIL_AUTH_JWT_SECRET=$(openssl rand -base64 32)
GOGOMAIL_ADMIN_TOKEN=$(openssl rand -base64 32)
GOGOMAIL_PUBLIC_BASE_URL=https://mail.example.com
GOGOMAIL_DATABASE_URL=postgres://gogomail:STRONGPW@postgres:5432/gogomail?sslmode=require
GOGOMAIL_REDIS_PASSWORD=$(openssl rand -base64 24)
GOGOMAIL_STORAGE_BACKEND=minio
GOGOMAIL_STORAGE_S3_ENDPOINT=https://minio:9000
GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true
GOGOMAIL_FARM_COORDINATOR_BACKEND=redis
GOGOMAIL_SMTP_DOMAIN=mail.example.com
GOGOMAIL_DELIVERY_SMTP_HELLO=mail.example.com
POSTGRES_PASSWORD=STRONGPW
MINIO_ROOT_PASSWORD=$(openssl rand -base64 24)
```

For an end-to-end deployment, also run the worker modes alongside. Add
two extra services to the compose file:

```yaml
  outbox-relay:
    image: ${BACKEND_IMAGE:-gogomail:latest}
    restart: always
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    environment:
      APP_MODE: outbox-relay
    env_file: .env
    networks: [gogomail]

  delivery-worker:
    image: ${BACKEND_IMAGE:-gogomail:latest}
    restart: always
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    environment:
      APP_MODE: delivery-worker
    env_file: .env
    networks: [gogomail]
```

**nginx** — shipped [`nginx-single.conf`](nginx-single.conf) handles HTTP.
For SMTP / IMAP on port 25 / 143 / 110 expose them directly from the
gogomail container (`backend.ports: ["25:2525", "587:2587", "143:1143",
"993:1993"]`) or layer an HAProxy in front.

**Verify**:

```bash
docker compose -f docker-compose.small.yml up -d
docker compose logs -f backend         # check validation
docker compose exec backend gogomail -migrate
curl -k https://localhost/health/ready
```

### 6.2 Pattern B — small (~1k mailboxes)

**Topology**

```
                 (TLS @ :443/:25/:587/:993)
                            |
                     +------+------+
                     |    nginx    |
                     +------+------+
                            |
              +-------------+-------------+
              |                           |
        +-----+-----+               +-----+-----+
        |  app-1    |               |  app-2    |    (all-in-one)
        +-----------+               +-----------+
                            |
                     +------+------+
                     |  worker-1   |     (outbox-relay + delivery-worker +
                     +-------------+      cleanup workers + batch-worker)
                            |
        +-------------------+--------------------+
        |              |              |
   +----+-----+   +----+----+   +-----+----+
   | Postgres |   | Redis   |   | S3/MinIO |   (managed or self-hosted)
   +----------+   +---------+   +----------+
```

**Compose recipe** — extend `docker-compose.small.yml`: duplicate the
`backend` service as `backend-1`, `backend-2` (different `INSTANCE_ID`),
keep nginx as LB upstream, add worker services from §6.1. Use managed PG
and Redis when possible (cut `postgres` and `redis` from compose, point
`GOGOMAIL_DATABASE_URL` and `GOGOMAIL_REDIS_ADDR` at the managed
endpoints).

Required additions to `.env`:

```bash
GOGOMAIL_RATELIMIT_BACKEND=redis
GOGOMAIL_BACKPRESSURE_BACKEND=redis
GOGOMAIL_MAIL_MUTATION_RATELIMIT_BACKEND=redis
GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND=redis
GOGOMAIL_DELIVERY_THROTTLE_BACKEND=redis
GOGOMAIL_METRICS_BACKEND=prometheus
GOGOMAIL_METRICS_ADDR=:9090
GOGOMAIL_DKIM_ENABLED=true
GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED=true
```

### 6.3 Pattern C — medium (~50k mailboxes)

Use [`docker-compose.medium.yml`](docker-compose.medium.yml) as a
starting point. It already includes PG primary+replica, Redis with
Sentinel, MinIO 3-node, Prometheus, and two backend instances.

**Required customizations**:

1. **Split modes**: instead of two `all-in-one` instances, define one
   Deployment per role group:
   - `edge` ×3 (`APP_MODE=edge-mta`)
   - `outbound` ×2 (`APP_MODE=outbound-mta`)
   - `mail-api` ×3
   - `admin-api` ×2
   - `auth` ×2
   - `imap` ×3
   - `pop3` ×2
   - `caldav` ×2 / `carddav` ×2 / `webdav` ×2
   - `ldap` ×2
   - `outbox-relay` ×2 (singleton)
   - `delivery-worker` ×3
   - `search-index-worker` ×2 (needs OpenSearch — add the
     `elasticsearch` service block from the large compose)
   - `push-notification-worker` ×2
   - `api-metering-worker` ×2
   - cleanup / retention / batch ×1 each (with one cold-standby for
     failover)

2. **HAProxy or DNS round-robin** for SMTP fan-out: replace nginx for
   SMTP/IMAP/POP3 with HAProxy in L4 mode. Sample stub:

```haproxy
frontend fe_smtp_25
    bind *:25
    mode tcp
    default_backend be_edge
backend be_edge
    mode tcp
    balance leastconn
    server edge-1 edge-1:2525 check
    server edge-2 edge-2:2525 check
    server edge-3 edge-3:2525 check
```

3. **nginx** for HTTP — [`nginx-backend.conf`](nginx-backend.conf)
   already supports multi-upstream. Add HSTS, gzip, and rate-limit
   directives:

```nginx
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
```

### 6.4 Pattern D — large (multi-DC, 100k+ mailboxes)

[`docker-compose.large.yml`](docker-compose.large.yml) gives a reference
for the local-DC stack: PG 3-node + etcd, Redis 3-node cluster, MinIO
6-node distributed, ELK + Prometheus + Grafana, HAProxy.

In practice production at this scale runs on Kubernetes; the compose file
is a topology reference. Translate each compose `service` into a
`Deployment` + `Service`:

- One Deployment per mode (24 deployments per DC if you use every mode).
- `HorizontalPodAutoscaler` driven by metric:
  - `edge-mta`: SMTP sessions/sec
  - `mail-api` / `admin-api`: HTTP RPS
  - `imap`: active connections
  - `delivery-worker`: `delivery.event` Redis-stream lag
  - `search-index-worker` / `push-notification-worker` /
    `api-metering-worker`: their respective stream lag
- Singletons: 2-3 replicas globally; pin to the PG-primary DC.
- Pod anti-affinity for `outbox-relay` replicas across nodes; deliberately
  one DC at a time.

Cross-DC topology:

```
                  Geo DNS / Anycast
                          |
        +-----------------+-----------------+
        |                                   |
   +----+----+                         +----+----+
   |  DC-A    |  <----  WAL stream --> |  DC-B    |
   | (active) |  <-- redis sentinel  > | (standby)|
   +----+----+                         +----+----+
        \                                   /
         \---- S3 cross-region replication-/
```

---

## 7. Network exposure matrix

```
PUBLIC (internet):
  25/tcp   SMTP inbound (edge-mta)
  465/tcp  SMTPS (outbound-mta implicit TLS)
  587/tcp  Submission (outbound-mta STARTTLS)
  443/tcp  HTTPS — webmail, admin console, mail-api, admin-api, auth-server
  993/tcp  IMAPS (imap)
  995/tcp  POP3S (pop3) — optional, restrict to VPN if not needed
  636/tcp  LDAPS — only if remote LDAP clients

INTERNAL (cluster / VPN only):
  80/tcp   HTTP (LB → backend, never on internet without TLS)
  143/tcp  IMAP STARTTLS (terminate at LB if needed)
  110/tcp  POP3 STARTTLS
  389/tcp  LDAP (use LDAPS in prod)
  2525/tcp gogomail SMTP listener (mapped to 25)
  2526/tcp inbound-mta (trusted relays only)
  2587/tcp submission listener (mapped to 587)
  1143/tcp IMAP listener (mapped to 143)
  1110/tcp POP3 listener (mapped to 110)
  1389/tcp LDAP listener (mapped to 389)
  1995/tcp POP3S listener (mapped to 995)
  8080/tcp gogomail HTTP API (behind LB)
  8081/tcp CalDAV
  8082/tcp CardDAV
  8083/tcp WebDAV
  9000/tcp MinIO API
  9001/tcp MinIO console (internal only)
  9090/tcp Prometheus metrics (`GOGOMAIL_METRICS_ADDR=:9090`)
  5432/tcp Postgres
  6379/tcp Redis
  26379/tcp Redis Sentinel
  9200/tcp OpenSearch (if used)
  3310/tcp ClamAV (if used)
```

**Firewall guidance**: deny all by default; allowlist the PUBLIC block
above. INTERNAL ports must be reachable only from gogomail nodes; for
Postgres / Redis / MinIO restrict to the backend security group.

---

## 8. DNS setup

For domain `example.com` with mail host `mail.example.com`:

| Type | Name | Value | Required | Notes |
|---|---|---|---|---|
| A / AAAA | `mail.example.com` | _public IP_ | yes | Used by `SMTP_DOMAIN` and `DELIVERY_SMTP_HELLO`. |
| A / AAAA | `webmail.example.com` | _public IP_ | yes | Webmail SPA / mail-api. |
| A / AAAA | `admin.example.com` | _public IP_ | yes | Admin console. |
| A / AAAA | `autodiscover.example.com` | _public IP_ | recommended | Outlook autodiscover. |
| MX | `example.com` | `10 mail.example.com.` | yes | Inbound mail. |
| TXT (SPF) | `example.com` | `v=spf1 mx -all` | yes | |
| TXT (DKIM) | `default._domainkey.example.com` | `v=DKIM1; k=rsa; p=…` | yes | `gogomail admin dkim:print --domain example.com` outputs the value. Selector defaults to `default`. |
| TXT (DMARC) | `_dmarc.example.com` | `v=DMARC1; p=reject; rua=mailto:dmarc@example.com; adkim=s; aspf=s` | yes | Start with `p=quarantine` while testing. |
| TXT (MTA-STS) | `_mta-sts.example.com` | `v=STSv1; id=20260101000000;` | recommended | Publish `https://mta-sts.example.com/.well-known/mta-sts.txt` (version: STSv1, mode: enforce, mx: mail.example.com). |
| TXT (TLS-RPT) | `_smtp._tls.example.com` | `v=TLSRPTv1; rua=mailto:tlsrpt@example.com` | recommended | |
| TLSA | `_25._tcp.mail.example.com` | _3 1 1 \<sha256(spki)\>_ | optional | Enables DANE; pair with `GOGOMAIL_DELIVERY_TLS_MODE=require` or DANE-aware peers. |
| CAA | `example.com` | `0 issue "letsencrypt.org"` | recommended | Restrict cert issuance. |

**Reverse DNS (PTR)**: the public IP that emits mail **must** PTR back to
`mail.example.com`. Without correct PTR, major receivers (Gmail, Outlook)
will reject or junk your mail.

---

## 9. TLS / certificates

### 9.1 ACME (Let's Encrypt) via Caddy

Easiest path for single-node and small deployments. Replace nginx with
Caddy:

```caddyfile
mail.example.com, webmail.example.com, admin.example.com {
    reverse_proxy backend:8080
    encode gzip zstd
    header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
}
```

For SMTP/IMAP TLS, mount the Caddy-managed certs into the gogomail
container:

```yaml
backend:
  volumes:
    - caddy-data:/data:ro
  environment:
    GOGOMAIL_SMTP_TLS_CERT_FILE: /data/caddy/certificates/.../mail.example.com.crt
    GOGOMAIL_SMTP_TLS_KEY_FILE:  /data/caddy/certificates/.../mail.example.com.key
    GOGOMAIL_IMAP_TLS_CERT_FILE: /data/caddy/certificates/.../mail.example.com.crt
    GOGOMAIL_IMAP_TLS_KEY_FILE:  /data/caddy/certificates/.../mail.example.com.key
```

### 9.2 Certbot + nginx

```bash
certbot certonly --standalone -d mail.example.com -d webmail.example.com -d admin.example.com
# certs land in /etc/letsencrypt/live/mail.example.com/
```

Mount `/etc/letsencrypt` read-only into both nginx and backend.

### 9.3 Cert paths expected by nginx config

[`nginx-backend.conf`](nginx-backend.conf) looks for certs under
`/etc/nginx/certs/`. Convention:

```
/etc/nginx/certs/
  fullchain.pem    # cert + intermediates
  privkey.pem      # private key
  dhparam.pem      # optional, 2048-bit
```

### 9.4 mTLS for IMAP / POP3 (optional)

Not enabled by default. To require client cert auth, terminate TLS in a
sidecar (envoy or nginx stream) with `ssl_verify_client on` and forward
plaintext on the loopback to the gogomail container; the user identity
header is mapped to the SASL auth via a custom milter (not shipped).

---

## 10. Initial setup

Step-by-step bootstrap for a fresh deployment.

```bash
# 1. Clone
git clone https://github.com/gogomail/gogomail.git
cd gogomail/docker

# 2. Configure .env
cp .env.example .env
# Edit .env — at minimum:
#   GOGOMAIL_ENV=production
#   GOGOMAIL_PUBLIC_BASE_URL=https://mail.example.com
#   POSTGRES_PASSWORD, MINIO_ROOT_PASSWORD, REDIS_PASSWORD

# 3. Generate secrets
echo "GOGOMAIL_AUTH_JWT_SECRET=$(openssl rand -base64 32)" >> .env
echo "GOGOMAIL_ADMIN_TOKEN=$(openssl rand -base64 32)" >> .env

# 4. Start infra-only first to bootstrap
docker compose -f docker-compose.small.yml up -d postgres redis minio minio-init

# 5. Apply database migrations
docker compose run --rm backend gogomail -migrate

# 6. Verify the configuration validates
docker compose run --rm backend gogomail -validate-config

# 7. Bring up the backend and LB
docker compose -f docker-compose.small.yml up -d backend nginx

# 8. Create the bootstrap super-admin
docker compose exec backend gogomail admin create-admin \
    --email admin@example.com --scope super-admin

# 9. Generate the first DKIM key for the domain
docker compose exec backend gogomail admin dkim:rotate \
    --domain example.com --selector default
# Publish the printed TXT record at default._domainkey.example.com

# 10. Smoke tests
curl -fsS https://mail.example.com/health/ready
swaks --to test@example.com --from probe@external.tld \
    --server mail.example.com --tls
```

---

## 11. Operations

Pointers; the runbooks live in [`docs/OPERATIONS.md`](../docs/OPERATIONS.md).

| Topic | Practice |
|---|---|
| Backup | Nightly `pg_dump` + WAL archiving to S3; enable bucket versioning on the mail blob bucket. |
| Restore drill | Quarterly. `gogomail tools restore-rehearsal` runs a non-destructive PG restore against `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL`. |
| Schema migration | `docker compose run --rm backend gogomail -migrate`. Migrations are forward-only and idempotent. |
| Scale-out trigger | `delivery.event` lag > 30s for 5 min → add `delivery-worker`. `mail-api` p95 > 500 ms → add API replicas. |
| Failure: PG primary | Promote replica; restart any singleton workers pinned to old primary. |
| Failure: Redis | Sentinel/Cluster auto-failover. Outbox drains on recovery. |
| Failure: S3 | Mail flow continues for metadata-only ops; body fetches and uploads fail until restored. |
| Secret rotation | Rotate `GOGOMAIL_AUTH_JWT_SECRET` by deploying with both old and new for one refresh-token lifetime, then dropping old. |
| Log retention | slog JSON to stdout; ship via Docker logging driver. Secret keys (`password`, `token`, `secret`, `key`, `private_key`) auto-redacted. |

---

## 12. For AI agents

**When asked to generate a docker-compose or k8s manifest for gogomail,
follow this procedure deterministically.**

### Procedure

1. **Gather inputs** from the user:
   - Approx number of mailboxes
   - Approx mail/day
   - High-availability requirement (single AZ / multi-AZ / multi-DC)
   - Existing managed services (managed PG? managed Redis? S3 vs MinIO?)
   - Public hostname(s)
   - Whether OpenSearch search + push notifications are needed
2. **Select the pattern** using the §3 decision tree.
3. **Start from the matching compose recipe in §6**:
   - Pattern A → [`docker-compose.small.yml`](docker-compose.small.yml)
   - Pattern B → derive from `small.yml`, scale `backend` to 2,
     add `outbox-relay` + `delivery-worker` services
   - Pattern C → [`docker-compose.medium.yml`](docker-compose.medium.yml)
     with per-role split per §6.3
   - Pattern D → [`docker-compose.large.yml`](docker-compose.large.yml)
     as DC template; translate to k8s for production
4. **Configure env vars** from §5. Apply this minimum production set:
   ```
   GOGOMAIL_ENV=production
   GOGOMAIL_AUTH_JWT_SECRET=<openssl rand -base64 32>       # ≥32 bytes
   GOGOMAIL_ADMIN_TOKEN=<openssl rand -base64 32>           # ≥32 bytes
   GOGOMAIL_REDIS_PASSWORD=<random>                         # non-empty
   GOGOMAIL_FARM_COORDINATOR_BACKEND=redis                  # required in prod
   GOGOMAIL_DATABASE_URL=postgres://…?sslmode=require       # NOT sslmode=disable
   GOGOMAIL_STORAGE_S3_ENDPOINT=https://…                    # HTTPS
   GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY=false
   GOGOMAIL_PUBLIC_BASE_URL=https://<hostname>              # https, not localhost
   GOGOMAIL_SMTP_DOMAIN=<public hostname>                   # not localhost
   GOGOMAIL_DELIVERY_SMTP_HELLO=<public hostname>           # not localhost
   GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_METRICS_BACKEND=prometheus
   GOGOMAIL_METRICS_ADDR=:9090
   GOGOMAIL_RATELIMIT_BACKEND=redis
   GOGOMAIL_BACKPRESSURE_BACKEND=redis
   GOGOMAIL_DKIM_ENABLED=true
   GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED=true
   GOGOMAIL_SMTP_DMARC_ENFORCEMENT=reject
   ```
5. **Configure DNS** per §8: at minimum A/AAAA, MX, SPF, DKIM, DMARC, PTR.
6. **Configure nginx / Caddy / HAProxy** per §6 and §9. Always:
   - Terminate TLS at the LB
   - Send HSTS `max-age=63072000; includeSubDomains; preload`
   - Set `X-Forwarded-Proto` and `X-Real-IP`
   - Add the LB CIDR to `GOGOMAIL_CALDAV_TRUSTED_PROXIES`,
     `GOGOMAIL_CARDDAV_TRUSTED_PROXIES`
7. **Verify** before declaring done:
   ```bash
   docker compose run --rm backend gogomail -validate-config
   docker compose up -d
   docker compose run --rm backend gogomail -migrate
   curl -fsS https://<hostname>/health/ready
   ```
8. **Report back** with:
   - Chosen pattern and rationale (cite §3)
   - All deviations from the recipe and why
   - Outstanding manual steps (DNS records to publish, certs to issue,
     bootstrap admin to create)

### Constraints (hard rules)

- **Never** emit `GOGOMAIL_ENV=production` without all of `JWT_SECRET ≥
  32 bytes`, non-empty `ADMIN_TOKEN`, non-empty `REDIS_PASSWORD`,
  `FARM_COORDINATOR_BACKEND=redis`, `sslmode=require` in PG DSN, HTTPS S3
  endpoint, `INSECURE_SKIP_VERIFY=false`. The validator will refuse to
  start otherwise.
- **Never** expose Postgres, Redis, or MinIO API ports on the internet.
- **Never** map IMAP/POP3/SMTP to plaintext-only on the internet — always
  pair with STARTTLS or implicit TLS.
- **Never** set `*_ALLOW_INSECURE_AUTH=true` in prod.
- **Never** point `GOGOMAIL_SMTP_DOMAIN` or `GOGOMAIL_DELIVERY_SMTP_HELLO`
  to `localhost`, `127.0.0.1`, or an unspecified address in prod.
- **Never** run more than one `outbox-relay` actively in the same farm —
  use 2 replicas for failover only; lock election handles activation.
- **Always** prefer named volumes over bind mounts for Postgres / Redis /
  MinIO data, except when the operator explicitly requests a host path.

### Reference summary

- Compose recipes: §6 above plus the three checked-in
  `docker-compose.{small,medium,large}.yml` files.
- Env var reference: §5 above, source of truth `internal/config/config.go`.
- Mode reference: §4 above, source of truth
  [`docs/MODES.md`](../docs/MODES.md) and `internal/app/mode.go`.
- Validator: `internal/config/validate.go`. Behavior must match this doc.
