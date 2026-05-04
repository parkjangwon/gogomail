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

API usage aggregation is handled by a separate `api-metering-worker` component.
It consumes `api.usage` events from the `api.event` stream and writes daily and
monthly Postgres aggregates into `api_usage_daily` and `api_usage_monthly`.

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

The HTTP middleware remains fail-open. The worker is disabled by default through
`GOGOMAIL_API_METERING_AGGREGATE_BACKEND=disabled` and can be enabled with the
Postgres backend when operators want persisted aggregates.

## Consequences

- API handlers do not take a synchronous dependency on aggregate writes.
- Redis Stream redelivery of the same deterministic event ID does not
  double-count aggregates, because the worker claims `api_usage_events` before
  upserting daily/monthly totals.
- Aggregates remain operational read models, not a financial ledger. Future
  billing-grade metering should add an immutable billing/export ledger before
  money movement depends on it.
- Route cardinality must stay bounded by stable HTTP route patterns rather than
  raw URLs.
- Additional plan or product-policy dimensions can be added without changing the
  middleware/worker separation.
