# ADR 0004: API metering aggregation boundary

Date: 2026-05-04

## Status

Accepted

## Context

gogomail needs API usage visibility for future SaaS billing, abuse analysis,
rate-limit planning, support investigations, and operations dashboards. HTTP
request handling should stay fail-open and should not synchronously perform
high-cardinality aggregate writes on the hot path.

The existing API metering middleware can emit `api.usage` events to the outbox
topic `api.event`. That gives the platform a durable event source, but operators
also need a compact read model that can be queried without replaying events.

## Decision

API usage accounting is handled by a separate `api-metering-worker` component.
It consumes `api.usage` events from the `api.event` stream, records immutable
event-level rows in `api_usage_ledger`, and writes daily and monthly Postgres
aggregates into `api_usage_daily` and `api_usage_monthly`.

The aggregate dimensions are:

- day
- method
- route
- status
- tenant ID
- company ID
- domain ID
- user ID
- API key ID
- principal ID
- auth source

The aggregate tracks request count, request bytes, response bytes, total
latency, maximum latency, and first/last seen timestamps. Admin API exposes the
read models through `GET /admin/v1/api-usage/daily` and
`GET /admin/v1/api-usage/monthly`.

The immutable ledger is exposed through bounded Admin API list, NDJSON export,
and stats endpoints so billing or warehouse jobs can consume event-level data
instead of operational aggregates.

Operators can also create persisted export batch manifests over a bounded
ledger window. A saved batch fixes the filter window and totals, and can be
replayed as NDJSON by batch ID.

External export jobs can register artifact metadata against a batch, including
object key, SHA-256, byte count, event count, and JSON metadata. This keeps the
core boundary vendor-neutral while still making completed export handoff
auditable.

The Admin API can also write a saved batch as an NDJSON artifact through the
configured object store. The writer streams ledger rows, computes byte count and
SHA-256 during the write, registers artifact metadata idempotently, and exposes
stored artifact download and object-body verification for audit handoff. Export
scans use stable `(event_timestamp, event_id)` ordering and dedicated indexes so
large saved windows are not silently constrained by bounded Admin API list
limits.

Export batches can also produce canonical SHA-256 manifest digest records over
the saved batch metadata and registered artifacts. The Admin API exposes digest
creation, listing, detail, and verification so operators can audit integrity
before signing. Manifest digests can be signed through a disabled-by-default
local-HMAC signer with explicit key IDs, persisted signature rows, and Admin API
verification. The signer boundary stays vendor-neutral so a future KMS or
asymmetric backend can replace local HMAC without changing the export handoff
shape.

The HTTP middleware remains fail-open. The worker is disabled by default through
`GOGOMAIL_API_METERING_AGGREGATE_BACKEND=disabled` and can be enabled with the
Postgres backend when operators want persisted aggregates.

## Consequences

- API handlers do not take a synchronous dependency on aggregate writes.
- Redis Stream redelivery of the same deterministic event ID does not
  double-count aggregates, because the worker claims `api_usage_events` before
  upserting daily/monthly totals.
- Aggregates remain operational read models, not a financial ledger. Future
  money movement should use the immutable ledger plus explicit billing batch
  manifests/checkpoints, export artifacts, verified manifest digests, and signed
  manifest records rather than daily/monthly aggregates alone.
- Route cardinality must stay bounded by stable HTTP route patterns rather than
  raw URLs.
- Additional plan or product-policy dimensions can be added without changing the
  middleware/worker separation.
