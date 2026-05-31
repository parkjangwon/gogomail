# Architecture Overview

gogomail is a unified mail + collaboration backend written in Go. It exposes
all standard protocols (SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP) and
modern REST APIs from a **single static binary** that selects its runtime role
at startup.

```
                                    +-----------------+
                                    |   gogomail bin  |
                                    +--------+--------+
                                             |
                              -mode <role>   |
                       +-----------+---------+---------+----------+
                       |           |                   |          |
                  SMTP server  HTTP API     IMAP/POP3/DAV/LDAP    Worker
                  (3 modes)   (3 modes)        (6 modes)        (12 modes)
```

---

## Design philosophy

| Decision | Rationale |
|---|---|
| **Single binary, multiple modes** | One Go module, one CI artifact. Deployment topology is a deploy-time decision, not a code-time one. Operators can collapse to one process for dev and explode to 24 deployments for scale, with **zero code change**. |
| **Stateless processes** | Every mode keeps its durable state in Postgres + Redis + object storage. Crash-restart is always safe; no local mailbox spool. |
| **Outbox Pattern for events** | Cross-component fanout (delivery, search, push, audit) goes through `mail_outbox` table → Redis Streams. Database commit is the source of truth; events are never sent before the transaction is durable. |
| **Standard Go stdlib first** | `net/http`, `database/sql`, `log/slog`. Minimal third-party surface; SDK churn minimized. |
| **PostgreSQL as the single tenant of truth** | All multi-tenant boundaries enforced in repository methods; no row-level security required, but `company_id` is in every index. |
| **RFC compliance over feature breadth** | Each protocol implementation tracks its RFC suite (see matrix below). Quirks documented; non-conformant clients accommodated only when necessary. |
| **Operationally visible failure paths** | Fail-open and best-effort paths must emit structured warning logs or retryable cleanup records so operators can reconcile orphaned objects, external IdP drift, and usage-metering gaps. |
| **Configuration via env vars** | 12-factor. Validator (`internal/config/validate.go`) hardens production. Optional YAML config file overlays env vars. |

---

## Outbox Pattern

```
+------------------+        +------------------+        +-------------------+
| HTTP / SMTP /    |  TX    | PostgreSQL       |  poll  | outbox-relay      |
| IMAP write path  +------->+ mail_outbox      +<-------+  (singleton)      |
+------------------+        | + business table |        +---------+---------+
                            +------------------+                  |
                                                                   | XADD
                                                                   v
                                                          +--------+--------+
                                                          | Redis Streams   |
                                                          | delivery.event  |
                                                          | search.event    |
                                                          | push.event      |
                                                          | api.event       |
                                                          +--------+--------+
                                                                   |
                                                                   | XREADGROUP
                                                          +--------+--------+
                                                          | Consumer workers|
                                                          |  - delivery-w   |
                                                          |  - search-idx-w |
                                                          |  - push-w       |
                                                          |  - api-meter-w  |
                                                          |  - event-w      |
                                                          +-----------------+
```

**Properties**:
- **Atomicity**: business change + outbox row in one PG transaction. If TX
  rolls back, no event is emitted.
- **At-least-once delivery**: outbox-relay marks rows `published_at` only
  after `XADD`. Consumers ack via `XACK`. Crashed consumer → message
  re-delivered after `claim_idle`.
- **Idempotency**: consumers must dedupe by outbox row id (provided as the
  Redis entry payload).
- **Backpressure**: when Redis is down, rows accumulate but are not lost;
  relay drains them when Redis recovers.
- **Singleton relay**: only one `outbox-relay` runs at a time, elected via
  Postgres advisory lock (farm coordinator). Adding replicas gives failover,
  not throughput.

---

## Multi-tenancy model

Three nested UUID-keyed entities:

```
company           (tenant — the customer account)
  id, name, plan, quotas, ...
    |
    +-- domain   (mail domain owned by the company, e.g. "acme.example")
    |     id, company_id, name, dkim_key_id, ...
    |       |
    |       +-- user
    |             id, domain_id, company_id, email_local, password_hash, ...
    |               |
    |               +-- mailbox  (per-user inbox folders)
    |               +-- drive_root
    |               +-- calendar
    |               +-- contact_book
```

**Boundaries**:
- Every row has a `company_id`. Every read-path repository function takes
  `company_id` as a parameter; queries `WHERE company_id = $1 AND ...`.
- JWTs carry `company_id` + `user_id` + scope. Middleware injects them into
  request context; repositories read from context.
- Admin scopes (`super-admin`, `company-admin`, `domain-admin`) gate which
  `company_id`/`domain_id` a token can act on.

Audit: every cross-boundary admin action is logged with `actor_id` +
`target_company_id`.

---

## Protocol coverage matrix

| Protocol | RFC | Mode | Notes |
|---|---|---|---|
| SMTP (inbound) | RFC 5321, 5322 | `edge-mta`, `inbound-mta` | + ESMTP extensions |
| SMTP submission | RFC 6409 | `outbound-mta` | port 587 STARTTLS, 465 implicit |
| SMTP delivery | RFC 5321, 7672 (DANE) | `delivery-worker` | opportunistic/required/dane TLS |
| SPF | RFC 7208 | `edge-mta` | inbound verification |
| DKIM | RFC 6376 | `edge-mta` (verify), `outbound-mta` (sign) | 2048-bit RSA default |
| DMARC | RFC 7489 | `edge-mta` | reject/quarantine/none |
| ARC | RFC 8617 | `edge-mta` | sealed on forwarded mail |
| SMTPUTF8 | RFC 6531 | optional | toggled by `*_SUPPORT_SMTPUTF8` |
| 8BITMIME | RFC 1652 | yes | always supported |
| BINARYMIME | RFC 3030 | optional | toggled |
| DSN | RFC 3461 | optional | toggled per mode |
| Milter | sendmail milter | `edge-mta` | optional content filter |
| IMAP4rev1 | RFC 3501 | `imap` | base |
| IMAP4rev2 | RFC 9051 | `imap` | preferred |
| IDLE | RFC 2177 | `imap` | push via Redis stream `imap.notify` |
| CONDSTORE | RFC 7162 | `imap` | per-mailbox modseq |
| QRESYNC | RFC 7162 | `imap` | fast resync |
| ID | RFC 2971 | `imap` | client/server identification |
| LIST-EXTENDED | RFC 5258 | `imap` | |
| ENABLE | RFC 5161 | `imap` | |
| POP3 | RFC 1939 | `pop3` | + STLS |
| CalDAV | RFC 4791 | `caldav` | |
| CalDAV scheduling | RFC 6638 | `caldav` | optional |
| CardDAV | RFC 6352 | `carddav` | |
| WebDAV | RFC 4918 | `webdav` | Drive feature |
| LDAP | RFC 4511 | `ldap-gateway` | read-only, simple bind |
| SCIM 2.0 | RFC 7644 | `admin-api` | bearer-token authed |
| MTA-STS | RFC 8461 | recommended DNS record | publish policy |
| TLS-RPT | RFC 8460 | recommended DNS record | |
| OAuth2 password flow | RFC 6749 (subset) | `auth-server` | for first-party clients |
| JWT | RFC 7519 | `auth-server` | HS256, 32-byte secret |

---

## Data flow diagrams

### Inbound mail (public)

```
sender MTA --SMTP/25--> edge-mta
                          |
                          | (SPF/DKIM/DMARC, DNSBL, milter, scan)
                          | accept
                          v
                       PG TX:
                         INSERT mail_message + mail_attachment ref + mail_outbox
                       PUT S3:
                         message body (idempotency key = outbox row id)
                          |
                          v
                  outbox-relay XADD imap.notify
                  outbox-relay XADD search.event
                  outbox-relay XADD push.event
                          |
       +------------------+------------------+
       v                  v                  v
   imap (push)     search-index-worker  push-notification-worker
                          |                  |
                          v                  v
                     OpenSearch          APNs / WebPush / webhook
```

### Outbound mail (authenticated user)

```
MUA --SMTP/587--> outbound-mta
                    |
                    | (SASL auth, DKIM sign, bulk limiter)
                    v
                 PG TX:
                   INSERT mail_outbox (kind=delivery)
                 PUT S3 message body
                    |
                    v
              outbox-relay XADD delivery.event
                    |
                    v
              delivery-worker XREADGROUP
                    |
                    | (per-domain throttle, MX lookup, TLS negotiate)
                    v
              remote MX  ---> { ok | defer | bounce }
                    |
                    +---> on bounce/defer: emit DSN via PG outbox -> delivery again
                    +---> on success: mark delivered, XACK
```

### Drive upload

```
client --HTTP PUT--> mail-api / webdav
                       |
                       | (auth, quota, MIME sniff)
                       v
                    PG TX:
                      INSERT drive_object (state=uploading)
                    PUT S3:
                      object body (multipart for large)
                    PG TX:
                      UPDATE drive_object SET state='committed', etag=...
                      INSERT mail_outbox (kind=search)
                       |
                       v
                outbox-relay XADD search.event
                       |
                       v
                search-index-worker -> OpenSearch
```

### Calendar / contact sync

```
client --PROPFIND/REPORT--> caldav / carddav
                              |
                              | (sync-token: last-seen ctag)
                              v
                           PG: changed entries since ctag
                              |
                              v
                       respond with multistatus
```

---

## Component dependency matrix

| Mode | Postgres | Redis | S3 | OpenSearch | Singleton lock |
|---|:-:|:-:|:-:|:-:|:-:|
| all-in-one | required | required | required | optional | — |
| edge-mta | required | required | — | — | — |
| inbound-mta | required | required | — | — | — |
| outbound-mta | required | required | — | — | — |
| delivery-worker | required | required | — | — | — |
| imap | required | required | required | — | — |
| pop3 | required | — | required | — | — |
| caldav | required | — | — | — | — |
| carddav | required | — | — | — | — |
| webdav | required | — | required | — | — |
| ldap-gateway | required | — | — | — | — |
| mail-api | required | required | required | optional | — |
| admin-api | required | required | — | — | — |
| auth-server | required | required | — | — | — |
| outbox-relay | required | required | — | — | **yes** |
| event-worker | required | required | — | — | — |
| search-index-worker | required | required | — | required | — |
| push-notification-worker | required | required | — | — | — |
| api-metering-worker | required | required | — | — | — |
| attachment-cleanup-worker | required | — | required | — | **yes** |
| drive-cleanup-worker | required | — | required | — | **yes** |
| dav-sync-retention-worker | required | — | — | — | **yes** |
| api-usage-retention-worker | required | — | — | — | **yes** |
| batch-worker | required | — | — | — | **per-job** |

Optional `OpenSearch` means full-text search is disabled if unset; the
matching mode is a no-op rather than an error.

---

## Major design decisions + rationale

### 1. Outbox in Postgres, not Kafka

We considered a Kafka log as the spine. Rejected because:
- One more system to run for small deployments.
- Cross-service transactions become complex (no PG-Kafka XA).
- Most deployments don't need partition-level parallelism > what Redis Streams
  with consumer groups gives us.

Outbox + Redis Streams scales to high tens of thousands of events/sec, which
covers all envisioned single-tenant deployments. Multi-tenant SaaS scale
moves Redis to a cluster; the code path doesn't change.

### 2. PostgreSQL advisory locks for singleton election

Cheaper than ZooKeeper / etcd / Consul, and we already require PG. Lease
duration is `GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL`. Trade-off: if PG is
down, no singleton runs — acceptable, since PG-down means no work either.

### 3. Per-mode env var validation

A single `Config` struct, parsed at startup, validated once. Each mode reads
the fields it needs. Validation is the same regardless of mode — easier to
diagnose misconfiguration in CI.

### 4. RFC-first protocol implementations

Each protocol implementation is a thin handler over an internal `mailservice`
/ `davservice` package. No protocol-specific business logic leaks into the
core. Adding a 25th mode (e.g. JMAP) is a new handler, not a new module.

### 5. Object storage abstraction

`internal/storage` exposes a `Backend` interface with `local`, `s3`, and
`minio` (s3-compatible) implementations. Hot path code never touches the SDK
directly. Switching providers is config-only.

### 6. No service mesh required

All inter-component communication is via Postgres + Redis. No gRPC between
modes. This means topology can change without endpoint discovery; a worker
finding its work via Redis stream is location-agnostic.

### 7. Graceful degradation

- OpenSearch down → search disabled, mail flow unaffected.
- Redis down → outbox accumulates, no data loss; ingest may slow at ratelimit
  fallback to in-process.
- Storage down → uploads fail, but mail metadata still processes.
- One worker class down → its stream backs up; ingest unaffected.

The only single-point-of-failure is Postgres; HA Postgres is the operator's
responsibility.

---

## Related documents

| Topic | Document |
|---|---|
| Per-mode env vars + replica advice | [`MODES.md`](MODES.md) |
| Topology recipes (single → multi-DC) | [`DEPLOYMENT.md`](DEPLOYMENT.md) |
| Threat model, auth, rate limits | [`SECURITY.md`](SECURITY.md) |
| Metrics, alerts, runbooks | [`OPERATIONS.md`](OPERATIONS.md) |
| ADRs | [`adr/`](adr/) |
| API contract | [`openapi.yaml`](openapi.yaml) |
| Roadmap | [`backend-roadmap.md`](backend-roadmap.md) |
