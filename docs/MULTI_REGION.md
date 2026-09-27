# Multi-region failover runbook

GoGoMail runs **active-passive** across two regions. One region is *primary*
(accepts writes and all SMTP delivery); the other is *standby* (warm, read-only
until promoted). This document is the operator runbook for standing the topology
up, observing its health, and executing a failover.

Scope: GoGoMail's control plane and operator experience. The data-plane
replication itself is performed by PostgreSQL streaming replication and S3
Cross-Region Replication (CRR) — GoGoMail does not re-implement either; it
consumes them and exposes routing, health, and readiness signals.

---

## 1. Architecture

```
                                 ┌──────────────────────────────────────┐
                                 │                DNS / MX                │
                                 │  mail.example.com  MX → both regions   │
                                 │  api.example.com   A/AAAA → active LB   │
                                 └───────────────┬──────────────┬─────────┘
                                                 │              │
                      (write + read, active)     │              │  (standby, warm)
                                                 ▼              ▼
        ┌────────────────────── REGION A (PRIMARY) ──────────┐  ┌────────── REGION B (STANDBY) ──────────┐
        │                                                    │  │                                        │
        │   ┌────────────┐      ┌──────────────────────┐     │  │  ┌──────────────────────┐              │
        │   │ GoGoMail    │     │ PostgreSQL PRIMARY    │     │  │  │ PostgreSQL REPLICA    │              │
        │   │ all-in-one  │────▶│ (read/write)          │─────┼──┼─▶│ (hot standby, RO)     │              │
        │   │  or split   │     │                       │ WAL │  │  │  streaming replication │             │
        │   │  modes      │  ┌─▶│ pg_stat_replication   │     │  │  └──────────────────────┘              │
        │   └─────┬───────┘  │  └──────────────────────┘     │  │  ┌──────────────────────┐              │
        │         │  reads   │                               │  │  │ GoGoMail (standby)    │              │
        │         └──────────┘  Router.Reader() routes read- │  │  │ same image/config     │              │
        │            (replica  only queries to the replica    │  │  │ DATABASE_URL → RegionB │             │
        │             when     when healthy & within         │  │  └──────────────────────┘              │
        │             healthy) staleness bound                │  │                                        │
        │   ┌──────────────┐                                  │  │  ┌──────────────────────┐              │
        │   │ S3 bucket A   │──── S3 CRR replication rule ─────┼──┼─▶│ S3 bucket B (replica) │              │
        │   │ (primary)     │                                  │  │  │ (destination)         │              │
        │   └──────────────┘                                  │  │  └──────────────────────┘              │
        │   ┌──────────────┐                                  │  │  ┌──────────────────────┐              │
        │   │ Redis A       │  (per-region; NOT replicated)    │  │  │ Redis B (independent) │              │
        │   └──────────────┘                                  │  │  └──────────────────────┘              │
        └────────────────────────────────────────────────────┘  └────────────────────────────────────────┘
```

Notes on the diagram:

- **PostgreSQL** — single writable primary. The standby is a physical streaming
  replica (`hot_standby=on`). GoGoMail's `database.Router` sends writes to the
  primary always, and read-only queries to the replica when it is healthy and
  within the configured staleness bound (`Router.Reader()`), otherwise it falls
  back to the primary.
- **S3** — object storage is replicated one-way by an S3 CRR rule from bucket A
  to bucket B. GoGoMail records the replica target (region + bucket) but does
  not copy objects itself.
- **Redis** — treated as per-region ephemeral state (rate-limit windows, outbox
  streams, leases). It is **not** cross-region replicated; the standby has its
  own Redis. The Outbox Pattern in PostgreSQL is the source of truth, so a cold
  Redis in Region B is repopulated from the replicated database on promotion.

---

## 2. Configuration

Read-replica routing (Region A app pointing at its local replica, or the app
generally):

| Env var | Meaning | Default |
|---|---|---|
| `GOGOMAIL_DATABASE_URL` | Primary DSN (writes + read fallback) | required |
| `GOGOMAIL_DATABASE_REPLICA_URL` | Read-replica DSN (optional) | empty (disabled) |
| `GOGOMAIL_DB_REPLICA_MAX_STALENESS` | Lag above which reads fall back to primary (`0` disables the check) | `10s` |
| `GOGOMAIL_DB_REPLICA_FALLBACK_TO_PRIMARY` | Route reads to primary when the replica is unhealthy/stale | `true` |

S3 cross-region replication target (surfaced in the storage capabilities
endpoint; the replication itself is configured on the bucket):

| Env var | Meaning | Default |
|---|---|---|
| `GOGOMAIL_STORAGE_S3_REPLICA_REGION` | CRR destination region | empty |
| `GOGOMAIL_STORAGE_S3_REPLICA_BUCKET` | CRR destination bucket | empty |

Validation (in `internal/config/validate.go`):

- In production, neither `GOGOMAIL_DATABASE_URL` nor
  `GOGOMAIL_DATABASE_REPLICA_URL` may use `sslmode=disable`.
- The replica DSN must differ from the primary DSN.
- `GOGOMAIL_DB_REPLICA_MAX_STALENESS` must not be negative.
- An S3 replica whose region **and** bucket both equal the primary is rejected
  (nothing to replicate to).

---

## 3. Observability

When `GOGOMAIL_METRICS_BACKEND=prometheus`, the metrics endpoint
(`GOGOMAIL_METRICS_ADDR`, default `:9090`, path `/metrics`) exposes:

| Metric | Type | Meaning |
|---|---|---|
| `gogomail_database_replica_lag_seconds` | gauge | Last measured replica apply lag (from `pg_last_xact_replay_timestamp()`) |
| `gogomail_database_replica_healthy` | gauge | `1` healthy, `0` unhealthy (last probe) |
| `gogomail_database_replica_fallback_total{reason=...}` | counter | Reads served by primary instead of replica; `reason` ∈ `no_replica`, `unhealthy`, `stale` |

The readiness probe (`/health/ready`) reports `read_replica=configured|disabled`
in the `mail_database` check detail, and continues to pass even if the replica
is down (the service can run primary-only).

Alerts to configure:

- `gogomail_database_replica_lag_seconds` sustained above your RPO budget.
- `gogomail_database_replica_healthy == 0` for more than one probe interval.
- `rate(gogomail_database_replica_fallback_total{reason="stale"}[5m]) > 0`
  sustained — indicates the replica cannot keep up.

Also monitor from Postgres directly on the primary:

```sql
SELECT client_addr, state, sync_state,
       pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) AS replay_lag_bytes
FROM pg_stat_replication;
```

---

## 4. RPO / RTO expectations

| Component | Mechanism | Typical RPO | Typical RTO |
|---|---|---|---|
| PostgreSQL | Async streaming replication | Seconds of WAL (bounded by replica lag; **0** if you run `synchronous_commit=remote_apply` at a latency cost) | Minutes (promote replica + repoint app) |
| S3 objects | S3 CRR (asynchronous) | Minutes (AWS SLA: 99.99% of objects within 15 min) | 0 for reads once app points at bucket B |
| Redis | Not replicated | N/A (ephemeral; rebuilt from PG outbox) | Seconds (cold start in Region B) |

RPO for the platform as a whole is dominated by PostgreSQL replica lag at the
moment of the outage. Watch `gogomail_database_replica_lag_seconds` and treat it
as your live RPO estimate. RTO is dominated by the human/automation steps in
section 5 — practice them to keep it under 15 minutes.

---

## 5. Failover procedure (Region A → Region B)

**Trigger:** Region A primary PostgreSQL (or the whole region) is unavailable
and not expected to recover within RTO.

> Do these in order. Steps marked ⚠ are irreversible or can cause split-brain —
> read section 6 first.

1. **Declare the incident.** Announce failover start; freeze routine changes.
2. **Stop writes to Region A** ⚠ — if Region A is partially alive, fence it:
   stop the GoGoMail write modes (`backend`, `edge-mta`, `submission`,
   delivery/outbox workers) in Region A so it cannot accept new mail or write to
   the old primary. If the region is fully down, this is already true.
3. **Confirm replica drain.** On the Region B replica, verify it has replayed
   all WAL it received:
   ```sql
   SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();
   ```
   When they are equal (and no new WAL is arriving), the replica is as current
   as it can be. Note the gap as your realized RPO.
4. **Promote the replica** ⚠:
   ```sh
   pg_ctl promote -D /var/lib/postgresql/data
   # or: SELECT pg_promote();
   ```
   The Region B Postgres is now a writable primary.
5. **Repoint GoGoMail in Region B.** Set `GOGOMAIL_DATABASE_URL` to the newly
   promoted local instance and clear/point `GOGOMAIL_DATABASE_REPLICA_URL`
   (there is no downstream replica yet). Start the write modes in Region B.
6. **Fail S3 over.** Point `GOGOMAIL_STORAGE_S3_REGION` /
   `GOGOMAIL_STORAGE_S3_BUCKET` at bucket B. Because CRR is one-way A→B, treat
   bucket B as authoritative now; new writes go to B.
7. **Switch DNS / MX** (see section 7). Lower TTLs *before* the incident so this
   is fast. Update `api.example.com` A/AAAA to the Region B load balancer.
8. **Verify.** `GET /health/ready` in Region B returns `ok`; send a test message
   through SMTP; confirm login and mailbox read. Check metrics for errors.
9. **Rebuild redundancy.** Once stable, provision a new replica (old Region A or
   a fresh region) streaming from the new primary, reverse the S3 CRR direction,
   and update config so you are active-passive again.
10. **Close the incident.** Record realized RPO/RTO and any deviations.

### Failback

Failback (B → A) is the same procedure in reverse and is **not** automatic. Only
fail back after Region A is fully healthy, has been re-seeded as a replica of B,
and has caught up. Never assume the old primary is safe to reactivate as a
writer without rebuilding it from the current primary — see split-brain below.

---

## 6. Split-brain avoidance

Split-brain = two nodes both believing they are the writable primary, accepting
divergent writes. It is the worst outcome; avoiding it beats fast recovery.

- **Single-writer invariant.** There is exactly one PostgreSQL primary at any
  time. GoGoMail's `database.Router` never writes to a replica — writes always
  go to `GOGOMAIL_DATABASE_URL`. Do not point two regions' apps at two writable
  databases.
- **Fence before promote** ⚠. Before `pg_promote`, ensure the old primary
  cannot accept writes: stop its GoGoMail write modes, and ideally stop or
  network-isolate the old Postgres. If the old primary might still be reachable
  by any client, fencing is mandatory.
- **Never auto-promote on a network blip.** Promotion is an operator decision
  (or a consensus-based tool like Patroni/etcd). A brief partition must not
  trigger promotion, or you risk two primaries when the partition heals.
- **Rebuild, don't rejoin blindly.** After failover, the old primary's timeline
  has diverged. Re-add it as a replica via `pg_rewind` or a fresh
  `pg_basebackup` from the new primary — do not just restart it as a primary.
- **S3 CRR is one-way.** Do not enable bidirectional replication between buckets
  A and B; that reintroduces split-brain for objects. Pick one authoritative
  bucket at a time.

---

## 7. DNS and MX considerations for SMTP

Mail delivery is more forgiving than HTTP here because SMTP has built-in retry,
but you still need correct records:

- **MX records:** publish MX for both regions with **different priorities**,
  e.g. `10 mx1.regionA.example.com` and `20 mx2.regionB.example.com`. Sending
  MTAs try the lowest-priority (highest-preference) host first and fall back to
  the next on connection failure. This gives inbound-mail failover *without* a
  DNS change: if Region A's edge MTA is down, senders retry Region B. Ensure the
  Region B edge MTA is running (warm) and accepts the same domains.
- **A/AAAA for the API/webmail:** these have no built-in failover, so keep TTLs
  low (60–300s) and update them during failover, or front both regions with a
  global load balancer / anycast that health-checks each region.
- **SPF / DKIM / DMARC:** the SPF record must authorize the sending IPs of
  *both* regions (`include:` or `ip4:`/`ip6:` for each). DKIM keys can be shared
  across regions (same selector/private key) or per-region selectors both
  published — either works as long as the signing key's selector resolves.
  DMARC alignment is unaffected as long as SPF/DKIM pass in whichever region
  sends.
- **PTR / reverse DNS:** each region's outbound IPs need matching PTR records or
  receivers will penalize/reject; set these up in advance for Region B.
- **MTA-STS / TLS-RPT:** if you publish an MTA-STS policy, its `mx:` list must
  include both regions' MX hostnames, and each region must present a valid
  certificate for those names, or STS-enforcing senders will refuse Region B.
- **TTL discipline:** lower TTLs on failover-relevant records *before* you need
  them. A 3600s TTL means senders may cache a dead endpoint for an hour.

---

## 8. Failover checklist

Pre-flight (do continuously, before any incident):

- [ ] Region B Postgres replica is streaming and lag is within RPO budget
      (`gogomail_database_replica_lag_seconds`).
- [ ] Region B GoGoMail is warm (same image, same config except DSNs).
- [ ] S3 CRR rule A→B is enabled and objects are replicating (check S3 metrics).
- [ ] DNS TTLs on `api.*` A/AAAA are low (≤300s).
- [ ] MX records for both regions published with distinct priorities.
- [ ] SPF authorizes both regions' outbound IPs; DKIM selectors resolve; PTR set
      for Region B IPs.
- [ ] Region B outbound IPs have reverse DNS and are not on DNSBLs.
- [ ] Failover has been rehearsed with `docker-compose.multiregion.yml`.

During failover:

- [ ] Incident declared; change freeze in effect.
- [ ] Region A writes fenced (write modes stopped / region isolated). ⚠
- [ ] Replica drain confirmed (`receive_lsn == replay_lsn`); RPO recorded.
- [ ] Replica promoted (`pg_promote`). ⚠
- [ ] Region B app repointed to the new primary; replica URL cleared/repointed.
- [ ] S3 config points at bucket B.
- [ ] DNS `api.*` A/AAAA updated to Region B; MX verified.
- [ ] `/health/ready` green in Region B; test message sent and read.

Post-failover:

- [ ] New replica provisioned from the new primary (redundancy restored).
- [ ] S3 CRR direction reversed (B→A or B→new region).
- [ ] Old primary rebuilt via `pg_rewind`/`pg_basebackup`, not restarted as a
      writer. ⚠
- [ ] Realized RPO/RTO recorded; runbook deviations noted.

---

## 9. Practicing locally

`docker/docker-compose.multiregion.yml` spins up a primary + streaming replica
Postgres, Redis, and a GoGoMail backend whose reads are routed to the replica
with fallback to primary. Use it to rehearse promotion without touching
production. See the header comments in that file for the exact commands.

Concrete tool versions used in the practice stack (pin the same in production):

- PostgreSQL **16.4** (streaming replication, `pg_basebackup`, `pg_rewind`,
  `pg_promote()`)
- Redis **7.4**
- Optional HA supervisor for automated, consensus-based promotion in
  production: **Patroni 3.x** with **etcd 3.5** (out of scope for the practice
  stack, which uses manual `pg_ctl promote`).
