# ADR 0013: Mail Flow Hybrid Storage Architecture

## Status

Accepted.

## Context

Mail flow logs (`mail.stored`, `mail.delivered`, `mail.bounced`, `mail.delivery_failed`, `mail.delivery_exhausted` events) must support:

1. **Audit compliance**: ACID transactions, referential integrity, forensic traceability
2. **High-volume aggregation**: Daily time-series breakdowns, delivery rates, size distributions
3. **Low-latency queries**: Admin dashboards, operational forensics

PostgreSQL provides ACID guarantees but aggregation queries on large tables cause index bloat and vacuum overhead. OpenSearch provides distributed aggregation pipelines but lacks ACID semantics.

## Decision

Use a hybrid storage architecture:

- **PostgreSQL `mail_flow_logs` table**: Primary write target for audit integrity. Foreign keys to `companies`, `domains`, `users` tables. `MailFlowLogWriter.InsertInbound/InsertOutbound` methods handle all writes.

- **OpenSearch `mail_flow` index**: Secondary indexing target via `MailFlowIndexer.IndexMailFlow`. Enables scalable aggregation queries via `MailFlowStatsSearcher.GetStats` and `GetDailyStats`.

### Implementation

- `internal/mailflow/Handler` supports optional OpenSearch indexer via `NewHandlerWithIndexer(db, indexer)`. When indexer is nil, falls back to PostgreSQL-only mode.
- `internal/searchindex.MailFlowIndexer` manages the OpenSearch index with keyword fields (direction, company_id, domain_id, user_id, from_addr, to_addr, flow_status) and a date field (created_at) for time-series aggregation.
- App startup wires `MailFlowIndexer` when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch` and `GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP=true`.

### Trade-offs

- Write path complexity: events are written to both stores. Failure in OpenSearch indexing does not fail the PostgreSQL write (audit integrity is preserved, stats may be delayed).
- Consistency: OpenSearch is eventually consistent. Stats queries may briefly reflect stale data.
- Operational overhead: Requires managing OpenSearch cluster for mail flow stats.

## Consequences

- Positive: Stats aggregation queries no longer tax PostgreSQL; scales to national-scale deployments.
- Positive: Audit integrity preserved via PostgreSQL ACID.
- Negative: Dual-write complexity; requires monitoring for indexing failures.
- Negative: Additional OpenSearch infrastructure requirement.
