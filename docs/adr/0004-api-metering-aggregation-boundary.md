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

The first aggregate dimensions are:

- day
- method
- route
- status
- user ID when available

The aggregate tracks request count, request bytes, response bytes, total
latency, maximum latency, and first/last seen timestamps. Admin API exposes the
read models through `GET /admin/v1/api-usage/daily` and
`GET /admin/v1/api-usage/monthly`.

The HTTP middleware remains fail-open. The worker is disabled by default through
`GOGOMAIL_API_METERING_AGGREGATE_BACKEND=disabled` and can be enabled with the
Postgres backend when operators want persisted aggregates.

## Consequences

- API handlers do not take a synchronous dependency on aggregate writes.
- Redis Stream redelivery may double-count events after worker failure; the
  aggregate is eventually useful for operations, not a financial ledger.
- Future billing-grade metering should add immutable event IDs or a dedicated
  usage ledger before money movement depends on it.
- Route cardinality must stay bounded by stable HTTP route patterns rather than
  raw URLs.
- Additional tenant dimensions such as company, domain, API key, and plan can be
  added without changing the middleware/worker separation.
