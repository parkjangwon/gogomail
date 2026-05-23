# Operations Runbook

Day-2 operations for gogomail: health, metrics, alerts, shutdown, backup,
migration, and incident response.

See also: [`MODES.md`](MODES.md), [`DEPLOYMENT.md`](DEPLOYMENT.md),
[`SECURITY.md`](SECURITY.md), `docker/alert-rules.yml`,
`docker/prometheus.yml`.

---

## Health check endpoints

HTTP modes (`mail-api`, `admin-api`, `auth-server`, `all-in-one`):

| Path | Purpose | LB usage |
|---|---|---|
| `GET /health/live` | Process responsive (no deps probed) | k8s liveness |
| `GET /health/ready` | Postgres + Redis reachable | k8s readiness + LB |

Both return `200` with JSON `{"status":"ok",...}` on success. `/health/ready`
returns `503` when DB ping fails or Redis is unreachable.

Non-HTTP modes (SMTP/IMAP/POP3/LDAP) — use TCP listen probe on the bound
port. There is no protocol-level health URI. For deep probing run a periodic
synthetic SMTP HELO or IMAP LOGIN with a monitoring account.

Workers — no HTTP server by default. Add `GOGOMAIL_METRICS_ADDR=:9090` to
expose Prometheus metrics (which doubles as a TCP liveness probe target).

---

## Prometheus metrics

Enabled when `GOGOMAIL_METRICS_BACKEND=prometheus` and `GOGOMAIL_METRICS_ADDR`
is set (e.g. `:9090`). Endpoint: `GET /metrics`.

### Core metrics

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `gogomail_http_request_duration_seconds` | histogram | `method`, `route`, `status` | One per HTTP route |
| `gogomail_http_requests_in_flight` | gauge | `route` | |
| `gogomail_smtp_session_duration_seconds` | histogram | `component`, `outcome` | `component ∈ {edge-mta, inbound-mta, outbound-mta}` |
| `gogomail_smtp_sessions_total` | counter | `component`, `outcome` | `outcome ∈ {accepted, rejected_spf, rejected_dmarc, rejected_dnsbl, rejected_ratelimit, error}` |
| `gogomail_smtp_bytes_received_total` | counter | `component` | |
| `gogomail_imap_active_connections` | gauge | | |
| `gogomail_imap_idle_clients` | gauge | | |
| `gogomail_pop3_active_connections` | gauge | | |
| `gogomail_auth_failures_total` | counter | `protocol`, `reason` | `protocol ∈ {http, imap, pop3, smtp, ldap, caldav, carddav}` |
| `gogomail_redis_stream_lag_seconds` | gauge | `stream`, `group` | One per consumer group |
| `gogomail_redis_stream_dlq_total` | counter | `stream` | Messages routed to dead-letter |
| `gogomail_delivery_attempts_total` | counter | `outcome`, `domain_class` | `outcome ∈ {success, defer, bounce}` |
| `gogomail_delivery_circuit_state` | gauge | `domain` | `0=closed, 1=open, 2=half_open` |
| `gogomail_outbox_lag_seconds` | gauge | | Oldest unrelayed row age |
| `gogomail_db_pool_open` / `_in_use` / `_idle` | gauge | | sql.DBStats |
| `gogomail_db_query_duration_seconds` | histogram | `op` | |
| `gogomail_search_index_documents_total` | counter | `result` | |
| `gogomail_push_notifications_sent_total` | counter | `transport`, `result` | `transport ∈ {apns, webpush, webhook}` |
| `gogomail_api_metering_events_total` | counter | `tenant`, `event_kind` | High-cardinality — sample if needed |
| `gogomail_attachment_scan_total` | counter | `verdict` | |
| `gogomail_backpressure_state` | gauge | `level` | `0=ok, 1=warn, 2=danger, 3=critical` |

Standard Go runtime metrics (`go_*`) and process metrics (`process_*`) are
also exported.

---

## Recommended alerts

Starter rules in `docker/alert-rules.yml`. Tune thresholds per traffic.

```yaml
groups:
- name: gogomail-critical
  rules:
  - alert: HighHTTPErrorRate
    expr: |
      sum by (route) (rate(gogomail_http_request_duration_seconds_count{status=~"5.."}[5m]))
        / sum by (route) (rate(gogomail_http_request_duration_seconds_count[5m])) > 0.05
    for: 10m
    annotations:
      summary: ">5% 5xx on {{ $labels.route }}"

  - alert: OutboxLagHigh
    expr: gogomail_outbox_lag_seconds > 300
    for: 5m
    annotations:
      summary: "Outbox relay lag > 5m — delivery delayed"

  - alert: StreamLagHigh
    expr: gogomail_redis_stream_lag_seconds > 60
    for: 10m
    annotations:
      summary: "{{ $labels.stream }} consumer lag > 60s"

  - alert: DLQGrowing
    expr: rate(gogomail_redis_stream_dlq_total[10m]) > 0
    for: 15m
    annotations:
      summary: "{{ $labels.stream }} dead-lettering events"

  - alert: DBConnectionPoolExhausted
    expr: gogomail_db_pool_in_use / gogomail_db_pool_open > 0.9
    for: 5m

  - alert: SMTPAuthFailureSpike
    expr: rate(gogomail_auth_failures_total{protocol="smtp"}[5m]) > 50
    for: 5m
    annotations:
      summary: "Possible credential-stuffing on submission"

  - alert: DeliveryCircuitOpen
    expr: gogomail_delivery_circuit_state == 1
    for: 15m
    annotations:
      summary: "Delivery to {{ $labels.domain }} circuit open >15m"

  - alert: BackpressureCritical
    expr: gogomail_backpressure_state == 3
    for: 2m
    annotations:
      summary: "Backpressure CRITICAL — rejecting SMTP"

- name: gogomail-warning
  rules:
  - alert: ReplicationLagHigh
    # check on the Postgres exporter, not on gogomail metrics
    expr: pg_replication_lag_seconds > 30
    for: 10m

  - alert: AttachmentScanFailures
    expr: rate(gogomail_attachment_scan_total{verdict="error"}[10m]) > 1
    for: 15m
```

---

## Graceful shutdown sequence

On SIGTERM/SIGINT, gogomail enters a 30-second drain window
(`context.WithTimeout(context.Background(), 30*time.Second)` — see
`internal/app/run.go:689`, 749, 810, 3101, 3464).

Per-mode behavior:

| Mode | Drain behavior |
|---|---|
| HTTP modes | `Server.Shutdown(ctx)` — stop accepting; finish in-flight; close idle |
| SMTP modes | Stop `Accept()`; existing sessions get reply 421 on next command after timeout |
| IMAP / POP3 / DAV / LDAP | Stop `Accept()`; existing sessions allowed to finish current command, then forcefully closed at 30s |
| Stream-consumer workers | Stop `XREADGROUP`; finish in-flight handler; `XACK` everything claimed; release consumer-group lease |
| Singleton workers | Finish current iteration; release advisory lock |
| `outbox-relay` | Finish current batch; release lock; standby takes over within ~5s |

**Operator action**: set k8s `terminationGracePeriodSeconds: 45` (30s app drain
+ 15s buffer) and PreStop `sleep 5` to let LB drain first.

---

## Migration runbook (zero-downtime)

Migrations are forward-only SQL in `migrations/`.

```bash
# 1. Pre-flight: ensure new schema is backward-compatible with running code
#    (no DROP/RENAME of columns still referenced)

# 2. Run migrations against primary
gogomail -migrate -mode all-in-one  # exits after migrations

# 3. Rolling-restart app/worker replicas
kubectl rollout restart deployment/gogomail-app
kubectl rollout restart deployment/gogomail-worker

# 4. Verify
kubectl rollout status deployment/gogomail-app --timeout=5m
```

Rules:
- **Additive only** during traffic-up window: new tables, new nullable
  columns, new indexes (`CONCURRENTLY`).
- **Destructive** changes (DROP, NOT NULL on populated column, type change)
  go in a follow-up release after the field is unreferenced.
- Long-running index builds: always `CREATE INDEX CONCURRENTLY`.
- Backfills: chunked in `batch-worker` or a one-shot script, not the
  migration itself.

---

## Backup & restore

### Postgres

```bash
# Daily logical
pg_dump --format=custom --no-owner --no-acl \
  --file=gogomail-$(date +%F).dump "$DATABASE_URL"

# Continuous WAL archive (preferred for RPO < 1m)
# Use managed RDS / Patroni / pgBackRest
```

Recovery:
```bash
createdb gogomail
pg_restore --dbname=gogomail --no-owner --no-acl gogomail-2026-05-23.dump
```

Test the restore quarterly into a scratch namespace.

### Object storage

- S3: enable **versioning** + **lifecycle to delete versions > 90d**.
- MinIO: enable site replication.
- Verify monthly with a sample restore (download a known object from a prior
  version).

### Redis

Redis state is **ephemeral** for gogomail — streams + rate-limit counters.
Recovery: bring up a fresh Redis; `outbox-relay` re-publishes any events that
were not yet acknowledged. AOF is recommended for `delivery.event` to
minimize re-delivery.

---

## Common failure scenarios

### Outbox lag growing

**Symptom** — `gogomail_outbox_lag_seconds` > 300; outbound mail delayed.

**Diagnose** —
1. Is `outbox-relay` running? `kubectl logs deploy/outbox-relay`.
2. Holding the advisory lock? `SELECT pg_locks.* FROM pg_locks WHERE locktype='advisory'`.
3. Redis stream backed up? `XLEN delivery.event`.

**Fix** —
- If relay is OOM-looping: increase memory.
- If Redis is saturated: scale Redis, increase
  `DELIVERY_CONSUMER_COUNT`.
- If PG is the bottleneck: check `pg_stat_activity` for blocking queries.

### Delivery stuck on a single remote domain

**Symptom** — `gogomail_delivery_circuit_state{domain=...}` == 1 for hours.

**Fix** — Confirm the remote MX is broken; the circuit will auto half-open
after `GOGOMAIL_DELIVERY_CIRCUIT_BREAKER_TIMEOUT`. Force-close by reducing
the timeout temporarily, or pause the worker and re-queue.

### IMAP gateway CPU/memory exhaustion

**Symptom** — `gogomail_imap_active_connections` near
`IMAP_MAX_CONNECTIONS`; replicas OOM.

**Fix** — Scale IMAP replicas; reduce
`GOGOMAIL_IMAP_NOTIFY_CONSUMER_COUNT` per replica; tighten
`IMAP_IDLE_TIMEOUT`.

### Brute-force on SMTP submission

**Symptom** — `rate(gogomail_auth_failures_total{protocol="smtp"}[5m]) > 50`.

**Fix** — `authFailureTracker` already throttles per-IP. If sustained, add an
upstream firewall rule. Check `audit_log` for the targeted accounts and
force-reset their passwords.

### Database connection exhausted

**Symptom** — `gogomail_db_pool_in_use / gogomail_db_pool_open > 0.9`;
HTTP 503s.

**Fix** — Increase `GOGOMAIL_DB_MAX_OPEN_CONNS` if PG has headroom; otherwise
scale PG vertically or add PgBouncer (transaction-pooling) in front. Check
for runaway queries with `pg_stat_activity`.

### Replication lag (PG)

**Symptom** — Stale reads, `pg_replication_lag_seconds` > 30.

**Fix** — Reduce write rate (throttle workers temporarily); check WAL disk
saturation; verify replica is not running heavy ad-hoc queries.

### JWT secret leaked

**Procedure** —
1. Generate new 32-byte secret.
2. Deploy app with `GOGOMAIL_AUTH_JWT_SECRET=<new>` and
   `GOGOMAIL_AUTH_JWT_SECRET_PREVIOUS=<old>` (accepted for verify only).
3. Force token rotation via admin endpoint (revokes all refresh families).
4. After max refresh-token TTL elapses, remove the previous secret.

---

## Log fields reference

All logs are slog JSON in production. Common keys:

| Key | Type | Meaning |
|---|---|---|
| `time` | RFC3339 | Timestamp |
| `level` | string | `info`/`warn`/`error` |
| `msg` | string | Human message |
| `mode` | string | Runtime mode (`edge-mta`, ...) |
| `env` | string | `production` / `development` |
| `component` | string | Sub-component (`outbox-relay`, `delivery-worker`) |
| `request_id` | UUID | Per-HTTP/SMTP-session correlation id |
| `tenant_id` | UUID | `company_id` of the request |
| `user_id` | UUID | Acting user |
| `actor_id` | UUID | Admin actor (for admin-api) |
| `route` | string | HTTP route template |
| `status` | int | HTTP status |
| `duration_ms` | int | Total request time |
| `remote_ip` | string | Source IP (post trusted-proxy resolution) |
| `error` | string | Error string (only on `level=error/warn`) |
| `stream` | string | Redis stream name |
| `consumer_group` | string | Consumer-group name |
| `message_id` | string | Redis stream entry id |
| `domain` | string | Target mail domain (delivery worker) |
| `mailbox_id` | UUID | IMAP/POP3 mailbox id |
| `outbox_id` | UUID | mail_outbox row id |
| `audit_action` | string | Admin audit action name |
| `lock_holder` | string | Singleton lease holder node id |

Sensitive keys (`password`, `token`, `secret`, `key`, `private_key`,
`authorization`, `cookie`) are **never logged** — handler-level redaction.

---

## Capacity planning rules of thumb

These are starting points — measure your workload.

| Resource | Per ... | Allocation |
|---|---|---|
| `mail-api` CPU | 1k active users | 0.5 vCPU |
| `mail-api` RAM | 1k active users | 500 MB |
| `imap` CPU | 1k connected clients | 0.5 vCPU |
| `imap` RAM | 1k connected clients | 800 MB (idle buffer + parser state) |
| `edge-mta` CPU | 100 sessions/sec | 1 vCPU |
| `edge-mta` RAM | 1000 concurrent sessions | 1 GB |
| `delivery-worker` CPU | 100 deliveries/sec | 1 vCPU |
| `outbox-relay` CPU | 10k events/sec | 1 vCPU (singleton; vertical scale only) |
| `search-index-worker` CPU | 1k indexed docs/sec | 1 vCPU |
| Postgres CPU | 1k active mailboxes | 0.5 vCPU |
| Postgres RAM | 1k mailboxes | 1 GB (shared_buffers heuristic) |
| Postgres storage | 1 user-year of mail | ~5 GB metadata (bodies in S3) |
| Redis RAM | 10k connected IMAP clients | 200 MB (state + streams) |
| Object storage | 1k users avg mailbox | ~5 GB/user typical |

Validation tests should drive numbers — run `scripts/load-test.sh` against a
staging environment before committing to a sizing.

---

## Routine maintenance schedule

| Cadence | Action |
|---|---|
| Daily | Verify backup completed; spot-check alert log; review `audit_log` for suspicious admin actions |
| Weekly | Review error-rate trends; check disk usage on PG and storage; sample restore test (rotating) |
| Monthly | Patch base images; rotate non-critical credentials; review capacity vs trend; full DR drill in staging |
| Quarterly | Rotate `AUTH_JWT_SECRET`, `ADMIN_TOKEN`; rotate DKIM keys; review and prune audit log per policy |
| Annually | Full restore exercise from cold backup; reassess threat model |
