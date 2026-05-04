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

Retention and archival must remain gated by explicit export evidence. The Admin
API exposes a read-only retention readiness report for a cutoff and optional
tenant/principal filters. It marks rows ready only when no candidates exist or a
completed export batch with matching filters covers the candidate time range
through the cutoff, was completed after the latest candidate row was recorded,
and has artifact, manifest digest, and signature evidence. Admin API also
exposes an operator-controlled bounded retention run that reuses the same
readiness gate, requires explicit destructive confirmation, and deletes only a
normalized batch of ready immutable ledger rows. Each blocked, dry-run, or
destructive retention attempt is persisted as an audit row with the filters,
counts, deleted rows, and readiness snapshot used for the decision. Scheduled
archive/delete workers remain deferred until production export storage and
signer policy are settled.

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
before signing. Manifest digests can be signed through disabled-by-default
local-HMAC, local-Ed25519, or remote-Ed25519 signers with explicit key IDs,
persisted signature rows, and Admin API verification. Signing and verification
are separate interfaces. Signers sign the lowercase 64-character manifest digest
hex string. Remote Ed25519 calls an HTTPS signer endpoint and verifies the
returned signature locally, letting a KMS-backed service replace local keys
without changing the export handoff shape.

The Admin API also exposes a read-only handoff readiness report for a saved
batch. The report summarizes whether the batch is complete, artifact event
counts cover the saved batch total, at least one manifest digest exists, and the
latest digest has a signature. It intentionally separates operational readiness
from billing readiness: locally signed batches can be operationally ready for
warehouse handoff checks, but remain billing-blocked until a production signer
such as KMS-backed remote Ed25519 signing is configured.

When operators explicitly request deep verification, the report may stream
registered artifacts from object storage, verify artifact byte/SHA metadata,
verify the latest manifest digest, check that the latest digest manifest still
covers the current registered artifact metadata, and verify the latest
signature when a verifier for that signer backend exists. These deep checks add
`deep_ready` and `verified_billing_ready` evidence without changing the
metadata-only readiness fields.

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
  manifest records rather than daily/monthly aggregates alone. The handoff
  readiness report is an operator summary of those records, not a replacement
  for production-grade signing or deep artifact verification.
- Route cardinality must stay bounded by stable HTTP route patterns rather than
  raw URLs.
- Additional plan or product-policy dimensions can be added without changing the
  middleware/worker separation.
