# Backend Mode Reference

gogomail ships as a single static Go binary (`gogomail`) that selects a runtime
role at startup via the `-mode` flag or `APP_MODE` env var. There are **24
modes** in total. All modes share the same configuration surface (see
`internal/config/config.go`); each mode reads only the env vars it needs.

## Quick index

| Category | Modes |
|---|---|
| All-in-one | `all-in-one` |
| SMTP | `edge-mta`, `inbound-mta`, `outbound-mta`, `delivery-worker` |
| User protocols | `imap`, `pop3`, `caldav`, `carddav`, `webdav`, `ldap-gateway` |
| API & auth | `mail-api`, `admin-api`, `auth-server` |
| Event workers | `outbox-relay`, `event-worker`, `delivery-worker`, `search-index-worker`, `push-notification-worker`, `api-metering-worker` |
| Background workers | `attachment-cleanup-worker`, `drive-cleanup-worker`, `dav-sync-retention-worker`, `api-usage-retention-worker`, `batch-worker` |

Canonical list: `internal/app/mode.go`.

## Common conventions

All modes inherit these settings from `internal/config/config.go`:

| Env var | Default | Purpose |
|---|---|---|
| `GOGOMAIL_ENV` | `development` | Must be `production` for prod runs (enables strict validation). |
| `APP_MODE` | `all-in-one` | Equivalent to `-mode` flag. |
| `GOGOMAIL_DATABASE_URL` | `postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable` | Postgres DSN. |
| `GOGOMAIL_REDIS_ADDR` | `localhost:6379` | Redis address (single-node). |
| `GOGOMAIL_REDIS_SENTINEL_ADDRS` | _(empty)_ | CSV of Sentinel addresses; enables HA Redis. |
| `GOGOMAIL_METRICS_BACKEND` | `none` | Set to `prometheus` to expose metrics. |
| `GOGOMAIL_METRICS_ADDR` | _(empty)_ | TCP address for `/metrics`; required when metrics enabled. |

Production validation (`internal/config/validate.go`) enforces:
- `GOGOMAIL_AUTH_JWT_SECRET` ≥ 32 bytes
- `GOGOMAIL_ADMIN_TOKEN` non-empty
- `GOGOMAIL_FARM_COORDINATOR_BACKEND=redis`
- `GOGOMAIL_REDIS_PASSWORD` non-empty (when farm coordinator uses Redis)
- `GOGOMAIL_SMTP_DOMAIN` and `GOGOMAIL_DELIVERY_SMTP_HELLO` not localhost/loopback
- `GOGOMAIL_STORAGE_S3_ENDPOINT` HTTPS and `INSECURE_SKIP_VERIFY=false`
- `*_ALLOW_INSECURE_AUTH=false` (IMAP, CalDAV, CardDAV, submission)

Graceful shutdown is uniform: SIGTERM/SIGINT trigger a 30s `context.WithTimeout`
drain before the process exits.

Production logs are structured slog JSON across Go modes. Protocol gateways,
workers, cleanup/rollback paths, SCIM synchronization, and fail-open metering
paths emit warning context for operator tracking; see `docs/OPERATIONS.md` and
`docs/MONITORING.md` for the current log pivots.

---

## All-in-one

### `all-in-one`

**Purpose** — Runs every HTTP role (`mail-api`, `admin-api`, `auth-server`)
inside one process. Intended for development, demo, and single-node
deployments. Does **not** run SMTP, IMAP, POP3, DAV, LDAP, or worker loops; pair
it with a worker process for end-to-end operation, or use it with an external
relay.

**Required env vars** — `GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`,
`GOGOMAIL_AUTH_JWT_SECRET` (prod), `GOGOMAIL_ADMIN_TOKEN` (prod),
`GOGOMAIL_PUBLIC_BASE_URL` (prod).

**Optional** — `GOGOMAIL_HTTP_ADDR` (default `:8080`), CORS, storage, push,
search index settings.

**Dependencies** — Postgres, Redis, S3 (or compatible).

**Replicas** — 1 for dev, 2-N behind LB for HA. Stateless.

**Scaling** — Stateless; horizontal behind nginx/HAProxy.

**Startup**
```
gogomail -mode all-in-one
```

**Health** — `GET /health/live`, `GET /health/ready`.

**Metrics** — HTTP request histogram, auth failures, DB pool stats.

---

## SMTP modes

### `edge-mta`

**Purpose** — Public-facing inbound MTA. Listens on port 25, performs SPF +
DKIM verification, DMARC enforcement, DNSBL checks, optional milter/clamav
scanning, rate limiting and connection backpressure.

**Required env vars** — `GOGOMAIL_SMTP_ADDR` (default `:2525`),
`GOGOMAIL_SMTP_DOMAIN`, `GOGOMAIL_SMTP_TLS_CERT_FILE`, `GOGOMAIL_SMTP_TLS_KEY_FILE`,
`GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`.

**Optional** — `GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED=true` (enables SPF/DKIM),
`GOGOMAIL_SMTP_DMARC_ENFORCEMENT` (`reject`/`quarantine`/`none`),
`GOGOMAIL_DNSBL_ZONES`, `GOGOMAIL_MILTER_ENABLED`,
`GOGOMAIL_ATTACHMENT_SCAN_BACKEND` (`clamav`/`webhook`),
`GOGOMAIL_SMTP_MAX_CONNECTIONS` (10000), `GOGOMAIL_SMTP_MAX_MESSAGE_BYTES`.

**Dependencies** — Postgres, Redis (rate limit, dedup, backpressure), optional
ClamAV/milter.

**Replicas** — 2-N. Each accepts a fraction of inbound traffic via LB or DNS
round-robin.

**Scaling** — Stateless. Use multiple A records or HAProxy on :25.

**Startup**
```
gogomail -mode edge-mta
```

**Health** — TCP listen check on `:25`/`:2525`. Metrics endpoint optional.

**Metrics** — SMTP session duration histogram, rejected count by reason
(`spf_fail`, `dmarc_reject`, `dnsbl`, `ratelimit`), accepted bytes.

### `inbound-mta`

**Purpose** — Internal-trusted MTA for relaying from `edge-mta` after policy
checks, or for receiving from sister sites over a trusted network. Disables
SPF/DKIM/DMARC enforcement (`EnableAuthVerification: false`).

**Required** — `GOGOMAIL_INBOUND_SMTP_ADDR` (default `:2526`),
`GOGOMAIL_INBOUND_TRUSTED_RELAYS` (CSV CIDRs; default `127.0.0.1/32,::1/128`).

**Optional** — Same SMTP knobs as `edge-mta`.

**Dependencies** — Postgres, Redis.

**Replicas** — Match upstream `edge-mta` count, or 2 minimum for HA.

**Scaling** — Stateless.

**Startup**
```
gogomail -mode inbound-mta
```

**Metrics** — Same shape as `edge-mta`; tagged `component=inbound-mta`.

### `outbound-mta`

**Purpose** — Submission server for authenticated user clients (port 587/465).
Performs SASL auth, DKIM signing, bulk-sender rate limits, and writes outbound
messages to the outbox table.

**Required** — `GOGOMAIL_SUBMISSION_ADDR` (default `:2587`), TLS cert/key,
`GOGOMAIL_DATABASE_URL`, `GOGOMAIL_AUTH_JWT_SECRET`,
`GOGOMAIL_DKIM_ENABLED=true` for outbound signing.

**Optional** — `GOGOMAIL_SUBMISSION_SMTPS_ADDR` (implicit-TLS 465),
`GOGOMAIL_SUBMISSION_MAX_CONNECTIONS`, `GOGOMAIL_SUBMISSION_BULK_SENDER_ENABLED`,
`GOGOMAIL_SUBMISSION_BULK_SENDER_RATE`.

**Dependencies** — Postgres, Redis.

**Replicas** — 2-N. Sticky session not required.

**Scaling** — Stateless.

**Startup**
```
gogomail -mode outbound-mta
```

**Metrics** — Submission count, auth failures, DKIM sign latency.

### `delivery-worker`

**Purpose** — Reads `delivery.event` Redis stream and performs SMTP delivery to
remote MX or configured smarthost. Implements per-domain throttling, circuit
breaker, exponential backoff with jitter, MX cache, DSN generation.

**Required** — `GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`,
`GOGOMAIL_DELIVERY_SMTP_HELLO` (must be a public-resolvable hostname in prod).

**Optional** — `GOGOMAIL_DELIVERY_SMARTHOST`, `GOGOMAIL_DELIVERY_TLS_MODE`
(`opportunistic`/`required`/`dane`), `GOGOMAIL_DELIVERY_RETRY_DELAYS`,
`GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_*`, `GOGOMAIL_DELIVERY_DOMAIN_CONCURRENCY`,
`GOGOMAIL_DELIVERY_THROTTLE_BACKEND=redis`.

**Dependencies** — Postgres, Redis (consumer group `gogomail.delivery-worker`),
public outbound 25 access.

**Replicas** — 2-N. Each member of the consumer group claims partitions.

**Scaling** — Stateless; consumer-group sharded.

**Startup**
```
gogomail -mode delivery-worker
```

**Metrics** — Delivery attempts, successes, deferrals, permanent failures,
per-domain backoff state, circuit breaker state.

---

## User protocol modes

### `imap`

**Purpose** — RFC 9051 IMAP4rev2 + IMAP4rev1 server with IDLE, CONDSTORE,
QRESYNC. Subscribes to `imap.notify` Redis stream for push updates.

**Required** — `GOGOMAIL_IMAP_ADDR` (default `:1143`),
`GOGOMAIL_IMAP_TLS_CERT_FILE`, `GOGOMAIL_IMAP_TLS_KEY_FILE`,
`GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`.

**Optional** — `GOGOMAIL_IMAP_MAX_CONNECTIONS` (5000),
`GOGOMAIL_IMAP_IDLE_TIMEOUT` (30m), `GOGOMAIL_IMAP_NOTIFY_CONSUMER_*`.

**Dependencies** — Postgres, Redis, S3 (for message bodies via storage backend).

**Replicas** — 2-N. Long-lived TCP connections — sticky-by-source-IP at LB
helps minimize reconnects but is not required.

**Scaling** — Stateless. Each replica is one consumer group member.

**Startup**
```
gogomail -mode imap
```

**Metrics** — Active connections, IDLE clients, fetch latency, auth failures
(`authFailureTracker`).

### `pop3`

**Purpose** — RFC 1939 POP3 server with TLS / STLS. Read-only mailbox window
view.

**Required** — `GOGOMAIL_POP3_ADDR` (default `:1110`), TLS cert/key if
`GOGOMAIL_POP3S_ADDR` is set.

**Optional** — `GOGOMAIL_POP3_MAX_CONNECTIONS` (2000),
`GOGOMAIL_POP3_IDLE_TIMEOUT`.

**Dependencies** — Postgres, S3.

**Replicas** — 2-N. Stateless.

**Scaling** — Stateless.

**Startup**
```
gogomail -mode pop3
```

**Metrics** — Sessions, auth failures, retrieval bytes.

### `caldav`

**Purpose** — RFC 4791 CalDAV server. Supports scheduling extensions (RFC
6638) when enabled.

**Required** — `GOGOMAIL_CALDAV_ADDR` (default `:8081`), `GOGOMAIL_DATABASE_URL`.

**Optional** — `GOGOMAIL_CALDAV_SCHEDULING=true`,
`GOGOMAIL_CALDAV_TRUSTED_PROXIES` (CIDRs),
`GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO`,
`GOGOMAIL_WELL_KNOWN_CALDAV_URL`.

**Dependencies** — Postgres.

**Replicas** — 2-N. Stateless.

**Scaling** — Stateless.

**Startup**
```
gogomail -mode caldav
```

**Health** — TCP listen.

**Metrics** — Request histogram by method (PROPFIND/REPORT/PUT).

### `carddav`

**Purpose** — RFC 6352 CardDAV server (contacts).

**Required** — `GOGOMAIL_CARDDAV_ADDR` (default `:8082`),
`GOGOMAIL_DATABASE_URL`.

**Optional** — `GOGOMAIL_CARDDAV_TRUSTED_PROXIES`,
`GOGOMAIL_CARDDAV_TRUST_FORWARDED_PROTO`, `GOGOMAIL_WELL_KNOWN_CARDDAV_URL`.

**Dependencies** — Postgres.

**Replicas** — 2-N. Stateless.

**Startup**
```
gogomail -mode carddav
```

### `webdav`

**Purpose** — RFC 4918 WebDAV server for the Drive feature.

**Required** — `GOGOMAIL_WEBDAV_ADDR` (default `:8083`),
`GOGOMAIL_DATABASE_URL`, S3 storage.

**Optional** — `GOGOMAIL_WEBDAV_DEPTH_INFINITY_ENABLED=false` (off by default
for DoS protection).

**Dependencies** — Postgres, S3.

**Replicas** — 2-N. Stateless.

**Startup**
```
gogomail -mode webdav
```

### `ldap-gateway`

**Purpose** — Read-only LDAP front-end (RFC 4511) for address-book and
authentication queries. Optional LDAPS.

**Required** — `GOGOMAIL_LDAP_ADDR` (default `:389`),
`GOGOMAIL_LDAP_COMPANY_ID`, `GOGOMAIL_LDAP_BASE_DOMAIN`,
`GOGOMAIL_DATABASE_URL`.

**Optional** — `GOGOMAIL_LDAPS_ADDR`, `GOGOMAIL_LDAP_TLS_CERT_FILE`,
`GOGOMAIL_LDAP_TLS_KEY_FILE`, `GOGOMAIL_LDAP_REFERRAL_URLS`.

**Dependencies** — Postgres.

**Replicas** — 2-N. Stateless.

**Startup**
```
gogomail -mode ldap-gateway
```

**Metrics** — Bind/search counts, auth failures.

---

## API & auth modes

### `mail-api`

**Purpose** — REST API for end-user mail, drive, calendar, contacts. Backs the
Webmail SPA. Implements rate limits on search (30/min/IP) and attachment
downloads (60/min/IP).

**Required** — `GOGOMAIL_HTTP_ADDR` (default `:8080`),
`GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`, `GOGOMAIL_AUTH_JWT_SECRET`,
`GOGOMAIL_PUBLIC_BASE_URL` (prod), `GOGOMAIL_CORS_ALLOWED_ORIGINS`.

**Optional** — `GOGOMAIL_HTTP_READ_TIMEOUT`, `GOGOMAIL_HTTP_WRITE_TIMEOUT`,
`GOGOMAIL_HTTP_MAX_HEADER_BYTES`,
`GOGOMAIL_MAIL_MUTATION_RATELIMIT_PER_MINUTE` (300),
`GOGOMAIL_API_METERING_BACKEND`.

**Dependencies** — Postgres, Redis, S3, optional OpenSearch (search index).

**Replicas** — 2-N behind LB. Stateless.

**Scaling** — Stateless.

**Startup**
```
gogomail -mode mail-api
```

**Health** — `GET /health/live`, `GET /health/ready`.

**Metrics** — HTTP histogram, mutation rate-limit hits, API metering counters.

### `admin-api`

**Purpose** — Tenant/company admin REST API. Backs the Admin Console. Login
limiter 5/min/IP, MFA enforced when `GOGOMAIL_ADMIN_MFA_REQUIRED=true`.

**Required** — Same as `mail-api`, plus `GOGOMAIL_ADMIN_TOKEN` (32+ bytes,
prod).

**Optional** — `GOGOMAIL_ADMIN_MFA_REQUIRED=true`,
`GOGOMAIL_SCIM_TOKEN` (enables SCIM provisioning),
`GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID`.

**Dependencies** — Postgres, Redis.

**Replicas** — 2-N. Stateless.

**Startup**
```
gogomail -mode admin-api
```

**Health** — `GET /health/live`, `GET /health/ready`.

**Metrics** — HTTP histogram, admin login failures, SCIM request counters.

### `auth-server`

**Purpose** — OAuth-style token issuance, refresh, password reset, MFA. Limits:
password reset 5/15min/IP, refresh 10/min/IP, login confirm 10/min/IP.

**Required** — `GOGOMAIL_AUTH_JWT_SECRET` (≥32 bytes prod),
`GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`.

**Optional** — Same HTTP knobs as `mail-api`.

**Dependencies** — Postgres, Redis.

**Replicas** — 2-N. Stateless.

**Startup**
```
gogomail -mode auth-server
```

**Health** — `GET /health/live`, `GET /health/ready`.

**Metrics** — Token issuance, refresh, MFA verifications.

---

## Event workers (Redis-Streams consumers)

### `outbox-relay`

**Purpose** — The PostgreSQL→Redis Streams bridge (Outbox Pattern). Polls
`mail_outbox` table and publishes events to `delivery.event`, `search.event`,
`push.event`, `api.event`. Uses pg advisory lock (`farm-coordinator`) to make
the relay singleton across replicas.

**Required** — `GOGOMAIL_DATABASE_URL`, `GOGOMAIL_REDIS_ADDR`,
`GOGOMAIL_FARM_COORDINATOR_BACKEND=redis` (prod).

**Optional** — `GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE` (default 100),
`GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL`, `GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS`.

**Dependencies** — Postgres, Redis.

**Replicas** — **Run 2+** for failover; only **one** runs at a time (advisory
lock).

**Scaling** — Singleton with leader election. Cannot scale work by adding
replicas — increase `BATCH_SIZE` and poll frequency instead.

**Startup**
```
gogomail -mode outbox-relay
```

**Metrics** — Relay batch size, lag (seconds since oldest unrelayed row).

### `event-worker`

**Purpose** — Generic event consumer for fanout to webhooks and downstream
systems.

**Required** — `GOGOMAIL_REDIS_ADDR`, `GOGOMAIL_EVENT_STREAM`,
`GOGOMAIL_EVENT_CONSUMER_GROUP`.

**Optional** — `GOGOMAIL_EVENT_CONSUMER_COUNT`,
`GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM`.

**Dependencies** — Redis, Postgres.

**Replicas** — 2-N. Sharded by consumer group.

**Startup**
```
gogomail -mode event-worker
```

**Metrics** — Events consumed, DLQ count, processing latency.

### `search-index-worker`

**Purpose** — Consumes `search.event`, writes/updates documents in OpenSearch.

**Required** — `GOGOMAIL_REDIS_ADDR`, `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`,
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT`.

**Optional** — `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX`,
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_KOREAN_ANALYZER`,
`GOGOMAIL_SEARCH_INDEX_MAX_BODY_BYTES`,
`GOGOMAIL_SEARCH_INDEX_CONSUMER_COUNT`.

**Dependencies** — Redis, Postgres, OpenSearch (or Elasticsearch).

**Replicas** — 2-N. Consumer group sharded.

**Startup**
```
gogomail -mode search-index-worker
```

**Metrics** — Documents indexed, OpenSearch latency, DLQ count.

### `push-notification-worker`

**Purpose** — Consumes `push.event`, dispatches to APNs / WebPush / generic
webhooks. Per-user device cap.

**Required** — `GOGOMAIL_REDIS_ADDR`, at least one of:
- APNs: `GOGOMAIL_APNS_KEY_ID`, `GOGOMAIL_APNS_TEAM_ID`,
  `GOGOMAIL_APNS_PRIVATE_KEY`, `GOGOMAIL_APNS_BUNDLE_ID`
- WebPush: `GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY`,
  `GOGOMAIL_WEBPUSH_VAPID_PRIVATE_KEY`, `GOGOMAIL_WEBPUSH_CONTACT_EMAIL`
- Webhook: `GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL`

**Optional** — `GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT` (200),
`GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_COUNT`.

**Dependencies** — Redis, Postgres.

**Replicas** — 2-N. Consumer group sharded.

**Startup**
```
gogomail -mode push-notification-worker
```

**Metrics** — Pushes sent by transport, failures, device pruning.

### `api-metering-worker`

**Purpose** — Consumes `api.event` and aggregates per-tenant API usage into the
`api_usage_*` tables. Optionally signs export manifests for billing.

**Required** — `GOGOMAIL_REDIS_ADDR`, `GOGOMAIL_DATABASE_URL`,
`GOGOMAIL_API_METERING_BACKEND=redis`.

**Optional** — `GOGOMAIL_API_METERING_AGGREGATE_BACKEND`,
`GOGOMAIL_API_METERING_CONSUMER_COUNT` (default 100),
`GOGOMAIL_API_USAGE_EXPORT_*` for signed manifests.

**Dependencies** — Redis, Postgres.

**Replicas** — 2-N. Consumer group sharded.

**Startup**
```
gogomail -mode api-metering-worker
```

**Metrics** — Events aggregated, manifest signatures generated.

---

## Background workers

### `attachment-cleanup-worker`

**Purpose** — Deletes orphaned attachment blobs in S3 / mailstore where the
DB row was rolled back or never written.

**Required** — `GOGOMAIL_DATABASE_URL`, storage credentials.

**Optional** — `GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL` (1h),
`GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE` (24h),
`GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE` (100),
`GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE`.

**Dependencies** — Postgres, S3, farm coordinator (advisory lock).

**Replicas** — 1 active. Use 2 for failover (only one holds the lock).

**Scaling** — Singleton with leader election.

**Startup**
```
gogomail -mode attachment-cleanup-worker
```

**Metrics** — Objects scanned, objects deleted, errors.

### `drive-cleanup-worker`

**Purpose** — Removes soft-deleted Drive objects past retention.

**Required** — `GOGOMAIL_DATABASE_URL`, storage credentials.

**Optional** — `GOGOMAIL_DRIVE_CLEANUP_INTERVAL` (15m),
`GOGOMAIL_DRIVE_CLEANUP_BATCH_SIZE` (100),
`GOGOMAIL_DRIVE_CLEANUP_RUN_ONCE`.

**Dependencies** — Postgres, S3.

**Replicas** — 1 active; 2 for failover.

**Scaling** — Singleton.

**Startup**
```
gogomail -mode drive-cleanup-worker
```

### `dav-sync-retention-worker`

**Purpose** — Trims CalDAV/CardDAV sync-token history past
`GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE` (default 90d). Defaults to **dry-run**;
requires explicit `GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN=false` and
`GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY=true` to actually delete.

**Required** — `GOGOMAIL_DATABASE_URL`,
`GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY=true` to enable real runs.

**Optional** — `GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL` (24h),
`GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE` (1000),
`GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN` (defaults `true`),
`GOGOMAIL_DAV_SYNC_RETENTION_RUN_ONCE`.

**Dependencies** — Postgres.

**Replicas** — 1 active.

**Scaling** — Singleton.

**Startup**
```
gogomail -mode dav-sync-retention-worker
```

### `api-usage-retention-worker`

**Purpose** — Prunes `api_usage_*` history. Dry-run by default.

**Required** — `GOGOMAIL_DATABASE_URL`,
`GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY=true` to enable real runs.

**Optional** — `GOGOMAIL_API_USAGE_RETENTION_INTERVAL` (24h),
`GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE` (90d),
`GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE`,
`GOGOMAIL_API_USAGE_RETENTION_DRY_RUN`,
`GOGOMAIL_API_USAGE_RETENTION_TENANT_ID`,
`GOGOMAIL_API_USAGE_RETENTION_PRINCIPAL_ID`.

**Dependencies** — Postgres.

**Replicas** — 1 active.

**Scaling** — Singleton.

**Startup**
```
gogomail -mode api-usage-retention-worker
```

### `batch-worker`

**Purpose** — Periodic housekeeping tasks: scheduled-mail flusher, quota alert
emails, MFA grace-period expiry, expired token pruning, optional auto-purge.

**Required** — `GOGOMAIL_DATABASE_URL`.

**Optional** — `GOGOMAIL_AUTO_PURGE_ENABLED=true`,
`GOGOMAIL_AUTO_PURGE_INTERVAL`, `GOGOMAIL_AUTO_PURGE_BATCH_SIZE`,
alert SMTP env vars (`GOGOMAIL_ALERT_EMAIL_*`).

**Dependencies** — Postgres, alert SMTP host (if alerts configured).

**Replicas** — 1 active. Each registered job holds its own advisory lock.

**Scaling** — Singleton per job. Multiple replicas safe (only one wins per
job).

**Startup**
```
gogomail -mode batch-worker
```

**Metrics** — Per-job last run, success/failure counters.

---

## Mode × dependency matrix

| Mode | PG | Redis | S3 | OpenSearch | Lock |
|---|:-:|:-:|:-:|:-:|:-:|
| all-in-one | ✓ | ✓ | ✓ | opt | — |
| edge-mta | ✓ | ✓ | — | — | — |
| inbound-mta | ✓ | ✓ | — | — | — |
| outbound-mta | ✓ | ✓ | — | — | — |
| delivery-worker | ✓ | ✓ | — | — | — |
| imap | ✓ | ✓ | ✓ | — | — |
| pop3 | ✓ | — | ✓ | — | — |
| caldav | ✓ | — | — | — | — |
| carddav | ✓ | — | — | — | — |
| webdav | ✓ | — | ✓ | — | — |
| ldap-gateway | ✓ | — | — | — | — |
| mail-api | ✓ | ✓ | ✓ | opt | — |
| admin-api | ✓ | ✓ | — | — | — |
| auth-server | ✓ | ✓ | — | — | — |
| outbox-relay | ✓ | ✓ | — | — | ✓ |
| event-worker | ✓ | ✓ | — | — | — |
| search-index-worker | ✓ | ✓ | — | ✓ | — |
| push-notification-worker | ✓ | ✓ | — | — | — |
| api-metering-worker | ✓ | ✓ | — | — | — |
| attachment-cleanup-worker | ✓ | — | ✓ | — | ✓ |
| drive-cleanup-worker | ✓ | — | ✓ | — | ✓ |
| dav-sync-retention-worker | ✓ | — | — | — | ✓ |
| api-usage-retention-worker | ✓ | — | — | — | ✓ |
| batch-worker | ✓ | — | — | — | ✓ |

`Lock` = uses Postgres advisory lock or Redis lease for singleton election.
