# Deployment Patterns

Four reference topologies for gogomail. Each is a starting point — adjust
replicas based on traffic and the capacity rules in
[`OPERATIONS.md`](OPERATIONS.md).

See also: [`MODES.md`](MODES.md) for per-mode env vars,
[`docker/`](../docker/) for ready-to-run compose files.

---

## 1. Single-node (all-in-one)

**Use case** — development, demos, very small deployments (< 50 mailboxes,
internal-only).

**Sizing** — 1 host, 4 vCPU / 8 GB RAM / 100 GB SSD.

### Topology

```
                +------------------------+
   Internet --> | host                   |
                |  - gogomail (all-in-one)|
                |  - postgres             |
                |  - redis                |
                |  - minio / local fs     |
                +------------------------+
```

### Infra

| Component | Version | Notes |
|---|---|---|
| PostgreSQL | 16+ | Local install or container |
| Redis | 7+ | Single instance |
| Storage | MinIO 1-node or local FS | `GOGOMAIL_STORAGE_BACKEND=local` for dev |

### Mode-to-instance mapping

| Process | Mode | Replicas |
|---|---|---:|
| `gogomail` | `all-in-one` | 1 |
| `gogomail` | `delivery-worker` (separate process) | 1 |
| `gogomail` | `outbox-relay` (separate process) | 1 |

### Reference compose

See [`docker/docker-compose.dev.yml`](../docker/docker-compose.dev.yml).

```bash
cd docker
cp .env.example .env  # edit secrets
docker compose -f docker-compose.dev.yml up -d
gogomail -migrate -mode all-in-one
```

### Failure modes

| Failure | Effect | Recovery |
|---|---|---|
| Host down | Total outage | Restore from backup, re-launch |
| Postgres corruption | Data loss | Restore from `pg_dump` + WAL |
| Redis lost | In-flight events lost | Outbox-relay re-publishes from PG |

---

## 2. Small (2-tier, ~1k users)

**Use case** — small business, up to ~1000 mailboxes, single AZ. Single host
each for app and worker, external managed PG/Redis/S3.

**Sizing** — 2 app hosts (4 vCPU / 8 GB) + 1 worker host (2 vCPU / 4 GB) +
managed PG (4 vCPU / 16 GB) + managed Redis + S3.

### Topology

```
            (TLS, :443/:25/:587/:993/:995)
                       |
                +------+-------+
                |   nginx LB   |
                +------+-------+
                       |
        +--------------+--------------+
        |                             |
   +----+----+                   +----+----+
   | app-1   |                   | app-2   |     (all-in-one + edge-mta + imap)
   +---------+                   +---------+
                       |
                +------+-------+
                | worker-1     |     (delivery + outbox-relay + batch + cleanup)
                +--------------+
                       |
        +--------------+--------------+
        |              |              |
   +----+----+   +-----+-----+   +----+----+
   | Postgres|   |  Redis    |   | S3 /MinIO|
   +---------+   +-----------+   +----------+
```

### Infra

| Component | Spec |
|---|---|
| PostgreSQL | 16+, managed, 4 vCPU / 16 GB, daily snapshots |
| Redis | 7+, managed, 2 GB |
| Object storage | S3 or MinIO 4-node |
| LB | nginx (L7) for HTTP + L4 stream for SMTP/IMAP/POP3 |

### Mode-to-instance mapping

| Host | Modes | Count |
|---|---|---:|
| app | `all-in-one`, `edge-mta`, `outbound-mta`, `imap`, `pop3`, `caldav`, `carddav`, `webdav` | 2 |
| worker | `outbox-relay`, `delivery-worker`, `event-worker`, `search-index-worker` (opt), `push-notification-worker` (opt), `api-metering-worker`, `attachment-cleanup-worker`, `drive-cleanup-worker`, `batch-worker` | 1 |

### Reference compose

[`docker/docker-compose.small.yml`](../docker/docker-compose.small.yml).

### k8s sketch

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gogomail-app
spec:
  replicas: 2
  selector: { matchLabels: { app: gogomail-app } }
  template:
    metadata: { labels: { app: gogomail-app } }
    spec:
      containers:
      - name: gogomail
        image: gogomail:latest
        args: ["-mode", "all-in-one"]
        envFrom: [{ secretRef: { name: gogomail-env } }]
        ports:
        - { name: http, containerPort: 8080 }
        readinessProbe:
          httpGet: { path: /health/ready, port: 8080 }
        livenessProbe:
          httpGet: { path: /health/live, port: 8080 }
```

### Failure modes

| Failure | Effect | Recovery |
|---|---|---|
| 1 app host down | Continues on remaining; LB removes failed host | Auto-heal / replace |
| Worker host down | Delivery + cleanup paused | Restart; outbox catches up |
| PG primary down | Total outage | Failover to managed replica |

---

## 3. Medium (4-role split, ~50k users)

**Use case** — mid-size SaaS, ~50k mailboxes, multi-AZ in one region.

**Sizing** — Roles split: edge / app / worker / admin.

### Topology

```
                              Internet
                                  |
              +-------------------+--------------------+
              |                   |                    |
          :25/465              :443                 :993/995
              |                   |                    |
       +------+------+       +----+----+         +-----+-----+
       |  HAProxy /  |       | nginx   |         | nginx     |
       |  Postfix LB |       | (HTTP)  |         | (IMAP/POP)|
       +------+------+       +----+----+         +-----+-----+
              |                   |                    |
       +------+------+      +-----+------+       +-----+-----+
       | edge-mta x3 |      | mail-api x3|       | imap x3   |
       | outbound-   |      | admin-api  |       | pop3 x2   |
       | mta x2      |      |   x2       |       +-----------+
       +-------------+      | auth-server|
                            |   x2       |
                            +------------+
              |                   |                    |
       +------+-------------------+--------------------+
       |              shared backbone                  |
       +------+-------------------+--------------------+
              |                   |                    |
       +------+-----+      +------+-----+      +-------+-----+
       | worker     |      | search-idx |      |  push-worker|
       | (delivery, |      | x2         |      |  x2         |
       |  outbox,   |      +------------+      +-------------+
       |  cleanup)  |
       |   x2-3     |
       +------------+
              |                   |
       +------+-----+      +------+------+
       | PG primary |      | Redis       |
       | + 2 replica|      | Sentinel x3 |
       +------------+      +-------------+
              |                   |
       +------+-------------------+----+
       |  MinIO erasure-coded x4-8     |
       +-------------------------------+
              |
       +------+-------------+
       | OpenSearch x3      |
       +--------------------+
```

### Infra

| Component | Spec |
|---|---|
| PostgreSQL | Primary + 2 streaming replicas, 8 vCPU / 32 GB each |
| Redis | Sentinel HA (1 master + 2 replicas + 3 sentinels), 4 GB |
| Object storage | MinIO 4-8 nodes erasure coded, or managed S3 |
| OpenSearch | 3-node cluster, 8 vCPU / 16 GB each |
| LB | HAProxy for SMTP (L4), nginx for HTTP |

### Mode-to-instance mapping

| Role | Modes | Replicas |
|---|---|---:|
| edge | `edge-mta`, `outbound-mta` | 3 + 2 |
| app | `mail-api`, `admin-api`, `auth-server` | 3 + 2 + 2 |
| user-proto | `imap`, `pop3`, `caldav`, `carddav`, `webdav`, `ldap-gateway` | 3 + 2 + 2 + 2 + 2 + 2 |
| worker | `outbox-relay`, `delivery-worker`, `event-worker`, `api-metering-worker` | 2 + 3 + 2 + 2 |
| index | `search-index-worker`, `push-notification-worker` | 2 + 2 |
| singleton | `attachment-cleanup-worker`, `drive-cleanup-worker`, `dav-sync-retention-worker`, `api-usage-retention-worker`, `batch-worker` | 2 each (1 active) |

### Reference compose

[`docker/docker-compose.medium.yml`](../docker/docker-compose.medium.yml).

### Failure modes

| Failure | Effect | Recovery |
|---|---|---|
| 1 edge-mta down | LB removes, others absorb | Auto-heal |
| PG primary failover | ~10-30s outage | Sentinel-style promote replica |
| Redis master failover | <10s outage | Sentinel promote, app reconnects |
| Outbox-relay leader dies | Standby takes over after lease expiry (~30s) | Automatic |
| OpenSearch down | Search degraded; mail flow unaffected | Restart cluster |

---

## 4. Large (full mode-split, multi-DC)

**Use case** — enterprise, 100k+ mailboxes, multi-region active-active or
active-passive.

**Sizing** — Every mode runs as its own deployment behind k8s or VM groups,
with cross-DC PG replication and multi-region S3.

### Topology

```
                       Geo DNS / Anycast
                               |
              +----------------+----------------+
              |                                 |
        +-----v-----+                     +-----v-----+
        |  DC-A     |                     |  DC-B     |
        |           |                     |           |
        | edge-mta  |                     | edge-mta  |
        | imap/pop3 |                     | imap/pop3 |
        | mail-api  |                     | mail-api  |
        | admin-api |                     | admin-api |
        | auth      |                     | auth      |
        | caldav/.. |                     | caldav/.. |
        |           |                     |           |
        | delivery  |                     | delivery  |
        | search-idx|                     | search-idx|
        | event/push|                     | event/push|
        |           |                     |           |
        | outbox-   |   (cross-DC lock,   | outbox-   |
        | relay x2  |    1 active global) | relay x2  |
        |           |                     |           |
        | PG replica|<--- streaming ----->| PG primary|
        | Redis     |    replication      | Redis     |
        |           |                     |           |
        +-----------+                     +-----------+
                  \                         /
                   \                       /
                    +-- S3 cross-region --+
                    +-- OpenSearch xDC  --+
```

### Infra

- PostgreSQL: 1 global primary, N read replicas per DC, synchronous commit
  optional. Failover via Patroni or managed RDS multi-AZ.
- Redis: Sentinel per DC for local state; for global locks
  (`outbox-relay`, cleanup workers), use a single coordinator DC.
- Storage: S3 with cross-region replication, or MinIO multi-site.
- OpenSearch: cross-cluster replication.
- LB: anycast + GeoDNS + per-DC HAProxy/nginx.

### Mode-to-instance mapping

Every mode is a separate k8s `Deployment` per DC, with HPA based on metrics:

| Mode | HPA signal |
|---|---|
| edge-mta | SMTP sessions/sec |
| mail-api | HTTP RPS |
| imap | active connections |
| delivery-worker | Redis stream lag (`delivery.event`) |
| search-index-worker | stream lag (`search.event`) |
| push-notification-worker | stream lag (`push.event`) |
| api-metering-worker | stream lag (`api.event`) |

Singleton workers (`outbox-relay`, `*-cleanup-worker`, `batch-worker`,
`*-retention-worker`): 2-3 replicas globally — only one runs at a time via
the configured farm coordinator. **Pin singletons to the PG-primary DC** to
reduce coordinator round-trips.

### Reference compose

[`docker/docker-compose.large.yml`](../docker/docker-compose.large.yml).

### Failure modes

| Failure | Effect | Recovery |
|---|---|---|
| DC-A loss | DNS shifts traffic to DC-B | Promote DC-B PG; restart singletons there |
| Network partition | Split-brain risk — `outbox-relay` lock prevents | Lock holder wins; other DC reads-only |
| PG primary loss | Promote replica (Patroni); ~30-60s outage | Automatic |
| S3 region loss | Reads from replica region, writes paused until promote | Manual switch |
| Cross-DC link saturated | Replication lag grows | Throttle non-critical workers |

---

## Choosing a pattern

| Pattern | Mailboxes | Hosts | Operational complexity |
|---|---:|---:|---|
| Single-node | < 50 | 1 | Trivial |
| Small | ~1k | 3 + managed services | Low |
| Medium | ~50k | 15-25 | Medium |
| Large | 100k+ | 50+ k8s pods/region | High |

Start with the smallest pattern that meets your SLO. Always test failover
playbooks before going live (see [`OPERATIONS.md`](OPERATIONS.md)).
