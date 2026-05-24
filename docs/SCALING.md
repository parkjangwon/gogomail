# Scaling Without Code Changes

GoGoMail is intended to scale by changing deployment topology, not application
code. A clone of this repository can run from a single local Compose stack to a
split SaaS deployment by changing Docker Compose files and environment values.

The contract is:

- Build or pull one backend image.
- Run the same image with different `-mode` / `APP_MODE` values.
- Keep durable state in shared Postgres, Redis, S3/MinIO, and optional
  OpenSearch.
- Scale stateless modes by adding replicas.
- Run singleton workers with multiple replicas only for failover.

## Recommended Starting Point

Use the split-mode template when you want a topology that can grow without a
rewrite:

```bash
cd docker
cp env.scale.example .env
docker compose -f docker-compose.scale.yml --profile local-infra --profile protocols --profile workers up -d
```

Run migrations as an explicit one-shot step:

```bash
docker compose -f docker-compose.scale.yml --profile ops run --rm migrate
```

Scale stateless roles with Compose:

```bash
docker compose -f docker-compose.scale.yml up -d --scale mail-api=4 --scale delivery-worker=3
```

The template publishes range-based host ports for scaleable listener services
so multiple replicas do not collide on one fixed host port. In production, put
nginx/HAProxy or a cloud load balancer in front of those published ports.

For production, point `.env` at managed or externally operated Postgres, Redis,
and S3/OpenSearch, then run without the `local-infra` profile.

## What Scales Horizontally

These modes are stateless from the application's point of view and can be
replicated behind a load balancer or Redis consumer group:

| Mode | Scale trigger | Coordination |
|---|---|---|
| `mail-api` | HTTP RPS, latency | Shared Postgres/Redis/S3 |
| `admin-api` | Admin/API RPS | Shared Postgres/Redis |
| `auth-server` | Login/refresh RPS | Shared Postgres/Redis |
| `edge-mta` | SMTP sessions/sec | Shared Postgres/Redis |
| `outbound-mta` | Submission sessions/sec | Shared Postgres/Redis |
| `imap` | Active connections | Shared Postgres/Redis/S3 |
| `pop3` | Active sessions | Shared Postgres/S3 |
| `caldav`, `carddav`, `webdav` | Protocol RPS | Shared Postgres/S3 as needed |
| `delivery-worker` | `delivery.event` lag | Redis consumer group |
| `event-worker` | Event stream lag | Redis consumer group |
| `search-index-worker` | `search.event` lag | Redis consumer group |
| `push-notification-worker` | `push.event` lag | Redis consumer group |
| `api-metering-worker` | `api.event` lag | Redis consumer group |

These modes are singleton or per-job singleton. Extra replicas are useful for
failover, not linear throughput:

| Mode | Behavior |
|---|---|
| `outbox-relay` | One active relay elected by farm coordinator; replicas are standby. |
| `attachment-cleanup-worker` | One active cleanup loop. |
| `drive-cleanup-worker` | One active cleanup loop. |
| `dav-sync-retention-worker` | One active retention loop. |
| `api-usage-retention-worker` | One active retention loop. |
| `batch-worker` | One active runner per registered job. |

## Shared Backbone

Backend containers do not exchange state through direct RPC. They coordinate
through the shared backbone:

```text
HTTP / SMTP / IMAP / DAV write
  -> Postgres transaction
  -> optional S3/MinIO object write
  -> mail_outbox row
  -> outbox-relay
  -> Redis Streams
  -> delivery/search/push/metering/event workers
```

This lets a load balancer route the next request to any healthy API container.
The next container sees the same state through Postgres/S3/Redis.

## Consumer Names In Scaled Containers

Redis Streams work best when every worker process has a stable, unique consumer
name. The config loader expands these placeholders in consumer and node-id env
vars:

- `{hostname}`
- `${HOSTNAME}`
- `$HOSTNAME`

The split Compose template uses values such as:

```env
GOGOMAIL_DELIVERY_CONSUMER_NAME=delivery-{hostname}
GOGOMAIL_EVENT_CONSUMER_NAME=event-{hostname}
GOGOMAIL_FARM_COORDINATOR_NODE_ID=node-{hostname}
```

That keeps `docker compose --scale delivery-worker=3` safe without manually
editing per-container env values.

## Database Load Strategy

Postgres remains the source of truth, so backend scale must be paired with DB
discipline.

Start with:

- PgBouncer in transaction pooling mode when connection count grows.
- Conservative `GOGOMAIL_DB_MAX_OPEN_CONNS` per replica.
- Redis-backed mutation/share/delivery throttles for cluster-wide limits.
- OpenSearch for full-text search at larger sizes.
- S3/MinIO for blobs; do not use local filesystem storage for scaled runs.

Watch:

- `gogomail_db_pool_in_use / gogomail_db_pool_open`
- Postgres CPU, IO wait, lock waits, and slow queries.
- `gogomail_outbox_lag_seconds`
- Redis stream lag per consumer group.
- S3/MinIO request latency.

Important sizing rule:

```text
total DB connections ~= replicas * GOGOMAIL_DB_MAX_OPEN_CONNS
```

If `mail-api=8` and `GOGOMAIL_DB_MAX_OPEN_CONNS=40`, that one role can open up
to 320 DB connections. Keep that below Postgres/PgBouncer capacity.

## SaaS Cell Path

For very large SaaS, keep one control-plane deployment and add tenant cells.
Each cell owns its own Postgres, Redis, S3 prefix/bucket, OpenSearch index, and
mode replicas. The control plane maps `company_id` / `domain_id` to the target
cell.

Cell-ready rules for future changes:

- Every tenant data query must remain scoped by company/domain/user identity.
- Avoid cross-tenant joins in hot paths.
- Keep object storage paths prefixed by tenant/cell boundaries.
- Emit events through outbox records with tenant identifiers.
- Prefer read models and cache invalidation over repeated aggregate scans.

The current split-mode Compose template is intentionally compatible with this
future: a cell is just another copy of the same services pointed at a different
backbone.
