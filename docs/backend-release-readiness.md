# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Mail API exposes thread list and thread-message read models for conversation-style webmail rendering.
- Inbound and reply/forward outbound persistence assign thread IDs from RFC thread headers or source messages where possible.
- Reply composition writes RFC thread headers into outgoing `.eml`, preserving conversation threading outside gogomail.
- Mail API exposes a small-deployment Postgres-backed search endpoint for metadata and draft text, with full body indexing deferred to the indexing worker boundary.
- Received-message body indexing now has a first worker boundary: `search-index-worker` consumes `mail.stored`, reads stored `.eml` objects, extracts bounded plain text, writes Postgres search documents, and lets the existing search endpoint include received body text without changing its response envelope.
- OpenSearch has a first writer adapter behind `internal/searchindex`, and the
  search index worker can select it with explicit endpoint/index configuration;
  the worker can optionally bootstrap the index mapping on startup. API
  read-side search remains on the current contract until a query adapter is
  added.
- Search results can now opt into relevance ordering, rank scores, and bounded headline snippets without changing default newest-first behavior.
- Mail API exposes bounded bulk flag, move, and soft-delete actions for efficient webmail list operations.
- Attachment uploads now support both metadata reservation and direct multipart storage writes.
- Stale attachment uploads have a repository/service cleanup path and a partial index for efficient lifecycle sweeps.
- Direct multipart uploads write through the configured storage backend and only record metadata after the object write succeeds.
- Attachment upload size is guarded in HTTP and service layers, including multipart request caps and declared-size consistency checks.
- Draft-to-send uses the normal outbound send path, then closes the source draft and links it to the sent message.
- Draft attachment uploads move to the sent message during draft-to-send, keeping sent folder detail and attachment list views consistent.
- Mail API send responses explicitly expose queued send, pending delivery, and no-bounce status fields so generated clients can model send lifecycle state without guessing from queue internals.
- Detail reads mark unread messages as read while avoiding redundant writes for already-read messages.
- Compose and draft validation guard user id, intent/source rules, recipient presence, recipient email syntax, recipient count, subject size, text body size, attachment IDs, filename safety, MIME type, and upload size.
- API errors use a stable structured envelope with code, message, HTTP status, and HTTP status text.
- Service info exposes API and backend contract version metadata; readiness exposes a structured checks envelope.
- Readiness checks now include contract/storage/outbox boundary metadata for deployment automation.
- Admin API supports domain/user list, detail, create, and status updates plus queue, delivery-attempt, suppression, DKIM, retry, and delete operations.
- Admin API now exposes trusted relay CIDR list/create/delete operations backed by PostgreSQL, preparing inbound SMTP relay policy for auditable runtime administration.
- Admin API now exposes delivery route list/create/status/delete operations backed by PostgreSQL, preparing gateway and smart-host policy for auditable runtime administration without coupling it to SMTP core.
- Admin API can dry-run delivery route resolution for a recipient domain, improving runtime route observability without triggering SMTP delivery.
- Admin API exposes a quota usage pressure read model for company, domain, and user limits so operators can spot backpressure risks before SMTP or Mail API writes start failing.
- Admin quota read models expose remaining capacity, child allocation, allocatable capacity, and over-allocation flags.
- Admin API exposes a read-only quota reconciliation report for detecting ledger drift against message and attachment source rows.
- Admin API exposes operator-controlled quota reconciliation corrections guarded by transaction/advisory locking.
- Quota product direction is captured in ADR 0003 and partially implemented: company contracted storage pool, domain allocations, user unified storage allowance, `default|custom` user quota source, domain default user quota propagation, and atomic company/domain/user ledger updates for mail storage writes/deletes plus attachment upload/cleanup.
- API metering is recorded as a planned SaaS platform boundary: usage should be collected asynchronously for future billing/rate-limit/abuse analytics, while enforcement remains policy-driven and disabled by default in the MVP.
- API metering has a disabled-by-default fail-open middleware boundary with `slog` and outbox sinks for early operational visibility and durable usage-event collection.
- API metering has a disabled-by-default aggregation worker and daily/monthly Postgres read models exposed through `GET /admin/v1/api-usage/daily` and `GET /admin/v1/api-usage/monthly`; events carry schema versions and deterministic IDs, replayed event IDs are not double-counted, but the aggregates are operational telemetry, not a billing ledger yet.
- IMAP has a backend gateway boundary package with native DTOs/interfaces, mailbox state helpers, and RFC-shaped flag mapping; no protocol server is in release scope yet.
- IMAP UID storage has durable mailbox UIDVALIDITY/UIDNEXT/highest-MODSEQ rows and message UID/MODSEQ rows, with transactional assignment helpers, first mailbox/message list adapters, raw body fetch groundwork, MODSEQ-aware flag mutation, bounded UID backfill, and move/delete UID invalidation; no protocol server is in release scope yet.
- IMAP IDLE remains out of scope, but `internal/imapgw` now has an in-memory
  mailbox event broker for future session fan-out.
- EML parser hot-path guardrails include bounded-read truncation coverage and a
  large-body benchmark.
- Push notification enqueue has a disabled-by-default worker boundary over committed `mail.stored` events with a bounded Postgres device resolver, per-device candidate-attempt persistence, Admin API inspection/stats, replaceable sink, and `slog` first adapter; Mail API device-token registration/list/delete exists with write-only raw tokens, while vendor push delivery is still out of scope.
- Domain outbound policy can cap individual attachment uploads with `max_attachment_bytes`, enforced before quota reservation or object storage writes.
- Attachment scanner integration has a disabled-by-default hook adapter outside SMTP core.
- Admin API can persist a domain operational policy model in `domains.settings.policy`, and Mail API send/draft-send enforces outbound recipient-count and composed-size guardrails when `outbound_mode=enforce`.
- DKIM key creation derives the public DNS TXT record from the private key when omitted, reducing operator DNS setup errors while preserving private-key omission from admin list responses.
- Admin API exposes domain DNS verification for MX, SPF, DMARC, and active DKIM TXT records, and each check is persisted with an audit log entry for domain onboarding traceability before frontend implementation.
- Delivery workers can opt into PostgreSQL-backed delivery routes through `GOGOMAIL_DELIVERY_ROUTE_BACKEND=postgres`, reusing the existing delivery router boundary and falling back to direct MX delivery when no active route matches.
- Admin domain/user create validation rejects malformed domains, unsafe usernames, invalid ACE names, and mismatched primary address ownership.
- SMTP receive/submission paths now include TCP-level protocol integration coverage for inbound delivery, AUTH PLAIN submission, policy rejection, and SMTPS.
- SMTP wire coverage now exercises enabled/disabled extension advertisement, DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` propagation including `NOTIFY=NEVER`, unsupported extension rejection, STARTTLS-gated AUTH, implicit TLS, trusted relay CIDR rejection, and repeated transactions on a single connection.
- Outbound SMTP wire coverage now verifies DSN parameters are emitted only when the remote MTA advertises DSN support, preventing accidental RFC 3461 option leakage to non-DSN peers.
- Outbound SMTP controlled-sink coverage now verifies accepted DATA can coexist with per-recipient permanent and temporary RCPT failures, preserving retry/bounce classification for delivery handlers.
- DSN/bounce generation validates inbound event metadata before composing and queueing null reverse-path DSNs.
- DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext identity metadata before it can reach outbound SMTP command generation or RFC 3464 report composition.
- Delivery partial-failure handling preserves recipient-level retry/bounce decisions even when every RCPT is rejected.
- Attachment upload storage paths reject absolute, parent-traversal, backslash, and newline forms, and generated attachment object paths sanitize path segments before writing to storage.
- `docs/backend-api-contracts.md` stages the backend-only OpenAPI contract source.
- `docs/openapi.yaml` provides the first backend-only OpenAPI 3.1 draft and is guarded against backend contract version drift, registered-route drift, dangling component references, request-body omissions, response envelope reference drift, message flag enum drift, and list limit contract drift.
- OpenAPI response components now document the Mail/Admin JSON envelope keys used by generated clients, including admin queue, delivery attempt, suppression, DKIM, domain, and user read models.
- OpenAPI operations now carry stable lower-camel `operationId` values and default reusable Error responses for protected/mutable operations, reducing generated-client naming and error-decoding drift.
- HTTP list endpoints now enforce the documented `1 <= limit <= 200` boundary before reaching repository pagination, so generated clients can rely on the OpenAPI limit bounds.
- `docs/smtp-release-runbook.md` now records operator-facing SMTP soak, STARTTLS, SMTPS, trusted relay, and outbound DSN/bounce smoke procedures.
- `scripts/verify-backend-release.sh` runs the standard backend release checks (`go test ./...`, `go mod tidy -diff`, optional PostgreSQL integration tests when `GOGOMAIL_TEST_DATABASE_URL` is set, and `git status --short`).
- PostgreSQL-backed integration tests can be enabled with `GOGOMAIL_TEST_DATABASE_URL` to run migrations in a temporary schema and exercise draft-to-send/outbox/retry behavior plus IMAP UID backfill/move invalidation against real SQL.

## Must verify before release cut

- Run `go test ./...`.
- Run `go mod tidy -diff`.
- Or run `./scripts/verify-backend-release.sh` to execute the standard backend release verification bundle.
- Verify `docs/openapi.yaml` still matches Go routes through the `internal/httpapi` contract tests before generating frontend clients.
- Verify generated clients preserve the documented top-level envelope keys rather than flattening Mail/Admin response bodies.
- Run `GOGOMAIL_TEST_DATABASE_URL=... go test ./internal/maildb ./internal/outbox` against a disposable PostgreSQL database/schema.
- Run focused SMTP soak checks for repeated same-connection transactions and STARTTLS/SMTPS startup in the intended deployment environment.
- Exercise multipart attachment upload against the intended object storage adapter. Local-storage path safety, declared-size mismatch, oversize body cleanup, metadata-after-object-write behavior, and quota-exhaustion HTTP mapping are now covered in automated tests.
- Exercise outbound DSN/bounce generation against a deployment-level controlled SMTP sink. Unit and wire tests now cover `NOTIFY=NEVER`, null reverse-path queueing/suppression, DSN option suppression to non-DSN peers, and retry/bounce recipient classification for temporary/permanent recipient failures.
- Verify frontend contracts for error envelope parsing, upload endpoint naming, and draft send response handling.

## Intentionally out of scope for this release slice

- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam logic.
- IMAP/POP3.
- OpenSearch indexing, vendor push delivery adapters, Kafka, Vault, and etcd.
