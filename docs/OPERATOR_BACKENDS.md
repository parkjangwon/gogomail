# Optional backends: Kafka & OpenSearch

GoGoMail runs on three dependencies out of the box — PostgreSQL, Redis, and an
S3-compatible object store. Two workloads can optionally be delegated to
dedicated infrastructure when an operator already runs it or needs its scale
characteristics:

| Workload | Built-in (default) | Optional dedicated backend |
|---|---|---|
| Event bus (platform event fan-out) | Redis Streams | **Apache Kafka** |
| Full-text mail search | PostgreSQL FTS | **OpenSearch** |

Both are strictly opt-in. The built-in backends are production-grade; choose a
dedicated backend only when you have a concrete reason below.

---

## When to choose each

### Event bus: Redis Streams vs Kafka

**Stay on Redis Streams (default) if:**
- You are running a single site or a handful of nodes.
- You do not already operate Kafka.
- Your event volume is comfortably handled by Redis (typical for most
  deployments — events are small JSON envelopes).

**Choose Kafka (`GOGOMAIL_EVENT_BUS_BACKEND=kafka`) if:**
- You already run Kafka and want GoGoMail events on the same bus as the rest of
  your platform (analytics, SIEM, data lake ingestion).
- You need long retention / replay of the raw event stream beyond what Redis
  Streams is sized for.
- You want multiple independent downstream consumer groups outside GoGoMail
  consuming mail/delivery/audit events.

Kafka replaces **only** the outbox-relay's publish transport. GoGoMail's own
internal consumers (search-index-worker, push, delivery, etc.) continue to read
from Redis Streams. In other words, enabling Kafka *adds* an external event
sink; it does not re-plumb GoGoMail's internal workers.

### Search: PostgreSQL vs OpenSearch

**Stay on PostgreSQL FTS (default) if:**
- Mailbox sizes are moderate and `tsvector`/GIN search latency is acceptable.
- You want zero extra infrastructure.

**Choose OpenSearch (`GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`) if:**
- You have large mailboxes / high message volume and need sub-second full-text
  search with relevance ranking and highlighting.
- You want language-aware analysis (e.g. the Nori Korean analyzer via
  `GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER=true`).

---

## Architecture: why mail flow is never at risk

GoGoMail uses the **Outbox Pattern**. When mail is accepted it is durably
written to PostgreSQL (message + `mail_outbox` row) in the same transaction as
the business change. A separate **outbox-relay** worker later reads pending
outbox rows and publishes them to the event bus.

```
SMTP/IMAP/JMAP accept ──▶ Postgres (message + outbox row, one txn)
                                   │
                          outbox-relay (poll)
                                   │
                    ┌──────────────┴───────────────┐
             Redis Streams (default)          Kafka (optional)
                    │
      internal workers: search-index, push, delivery, event
```

Because publishing happens in the relay — **after** mail is already durably
stored and **outside** the accept path — an event-bus outage can never block or
lose mail. It only delays downstream fan-out until the bus recovers.

---

## Configuration reference

### Kafka event bus

Set on the **outbox-relay** process (the only producer):

| Env var | Default | Notes |
|---|---|---|
| `GOGOMAIL_EVENT_BUS_BACKEND` | `redis` | `redis` or `kafka`. |
| `GOGOMAIL_EVENT_BUS_KAFKA_BROKERS` | — | Comma-separated `host:port`. **Required** when backend=kafka. |
| `GOGOMAIL_EVENT_BUS_KAFKA_TOPIC_PREFIX` | *(empty)* | Prefix prepended to the event topic (e.g. `gogomail.`). |
| `GOGOMAIL_EVENT_BUS_KAFKA_CLIENT_ID` | `gogomail-outbox-relay` | Kafka client id. |
| `GOGOMAIL_EVENT_BUS_KAFKA_BATCH_SIZE` | `100` | Max records buffered per producer batch. |
| `GOGOMAIL_EVENT_BUS_KAFKA_BATCH_TIMEOUT` | `100ms` | Max linger before a partial batch is flushed. |
| `GOGOMAIL_EVENT_BUS_KAFKA_WRITE_TIMEOUT` | `10s` | Per-write deadline. |
| `GOGOMAIL_EVENT_BUS_KAFKA_MAX_ATTEMPTS` | `5` | Producer retry attempts per write. |
| `GOGOMAIL_EVENT_BUS_KAFKA_REQUIRED_ACKS` | `all` | `none`\|`leader`\|`all`. `none` is rejected in production. |
| `GOGOMAIL_EVENT_BUS_KAFKA_TLS` | `false` | Enable TLS. **Required `true` in production.** |
| `GOGOMAIL_EVENT_BUS_KAFKA_TLS_INSECURE_SKIP_VERIFY` | `false` | Skip cert verification. Rejected in production. |
| `GOGOMAIL_EVENT_BUS_KAFKA_SASL_MECHANISM` | `none` | `none`\|`plain`\|`scram-sha-256`\|`scram-sha-512`. |
| `GOGOMAIL_EVENT_BUS_KAFKA_SASL_USERNAME` | — | Required when a SASL mechanism is set. |
| `GOGOMAIL_EVENT_BUS_KAFKA_SASL_PASSWORD` | — | Required when a SASL mechanism is set. |

The event topic is the outbox event's topic (optionally prefixed); the outbox
event's partition key is used as the Kafka message key so per-entity ordering
is preserved via hash partitioning. Each message carries an `outbox_id` header
for idempotent downstream de-duplication.

### OpenSearch search backend

Set on all processes that load the shared config (validation requires the
endpoint whenever `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`), and on the
**search-index-worker**:

| Env var | Default | Notes |
|---|---|---|
| `GOGOMAIL_SEARCH_INDEX_BACKEND` | `disabled` | `disabled`\|`postgres`\|`opensearch`. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT` | — | `http(s)://host:port`. Required when backend=opensearch. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX` | `gogomail-messages` | Index (or write-alias) name. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_USERNAME` | *(empty)* | Basic-auth user. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_PASSWORD` | *(empty)* | Basic-auth password. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP` | `false` | Create the index/mapping at worker startup. |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT` | `10s` | HTTP client timeout for index/search calls. |
| `GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER` | `false` | Use the Nori analyzer (requires the `analysis-nori` plugin). |

**Production readiness features:**

- **Health check** — the search-index-worker performs a `_cluster/health`
  probe (`wait_for_status=yellow`) at startup. A failure is logged as a warning
  and the worker keeps retrying via the event stream, so a cluster that is
  still coming up does not crash the worker.
- **Mapping versioning** — the index mapping is stamped with
  `_meta.gogomail_mapping_version`. At startup the worker reads it back and
  warns if the running index predates the current mapping, signalling that a
  rollover/reindex is due.
- **Server-side query timeout** — search requests carry a `timeout` so a slow
  cluster returns partial results instead of holding the webmail search request
  open (in addition to the HTTP client timeout).
- **Bulk indexing with backpressure** — `BulkIndexMessages` chunks large
  batches into bounded `_bulk` requests so a single request can never grow
  unbounded, and surfaces per-item failures for at-least-once retry.

**Index lifecycle / rollover.** GoGoMail writes to the single index (or alias)
named by `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX`. For retention and rollover,
point that name at an OpenSearch **write alias** managed by an ISM
(Index State Management) rollover policy on the OpenSearch side. Bump the
mapping version and roll to a fresh backing index when the mapping changes; the
startup warning tells you when a running index is stale.

---

## Failure modes

### "What happens when Kafka is down?"

1. Mail continues to be accepted and durably stored — the accept path never
   touches Kafka.
2. The outbox-relay's `WriteMessages` call fails; the relay marks the affected
   outbox rows failed (incrementing their attempt counter) and logs a warning.
3. On the next poll the relay re-attempts the unpublished rows. Once the broker
   recovers, the backlog drains automatically.
4. No event is lost: rows stay in `mail_outbox` until successfully published or
   until `GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS` is exhausted (then they are
   marked failed for operator inspection rather than silently dropped).

Downstream impact while Kafka is down: external Kafka consumers see delayed
events. GoGoMail's own internal features (search, push, delivery) are unaffected
because they consume from Redis Streams, not Kafka.

### "What happens when OpenSearch is down?"

1. Mail continues to be accepted and stored.
2. The search-index-worker's index calls fail; the event stays pending on the
   Redis stream and is retried (at-least-once). Poison messages that exceed the
   max delivery count are dead-lettered for inspection.
3. Webmail search requests against OpenSearch fail fast (bounded by the query
   and client timeouts). Operators can fall back to
   `GOGOMAIL_SEARCH_INDEX_BACKEND=postgres` to restore search without a cluster.
4. Once OpenSearch recovers, pending index events drain and the index catches
   up.

---

## Docker Compose profiles

Overlay files extend any base stack:

```bash
# OpenSearch (already bundled in docker-compose.dev.yml; overlay for small/medium)
docker compose -f docker/docker-compose.small.yml \
               -f docker/docker-compose.opensearch.yml up -d

# Kafka event bus
docker compose -f docker/docker-compose.small.yml \
               -f docker/docker-compose.kafka.yml up -d

# Both together
docker compose -f docker/docker-compose.small.yml \
               -f docker/docker-compose.opensearch.yml \
               -f docker/docker-compose.kafka.yml up -d
```

The `docker-compose.kafka.yml` overlay runs a single-node KRaft broker (dev/CI
only) and flips the outbox-relay to `GOGOMAIL_EVENT_BUS_BACKEND=kafka`. For
production, point `GOGOMAIL_EVENT_BUS_KAFKA_BROKERS` at your managed cluster and
enable TLS/SASL.

---

## Testing

- Unit tests use in-process fakes and `httptest` servers — **no real Kafka or
  OpenSearch is required** (`go test -short ./...`).
- The Kafka producer integration test is env-gated and skipped under `-short`;
  run it against a real broker with:

  ```bash
  GOGOMAIL_KAFKA_INTEGRATION=localhost:9092 go test ./internal/outbox/ -run Integration
  ```

- The OpenSearch integration test is likewise env-gated (see
  `internal/searchindex/opensearch_integration_test.go`).
