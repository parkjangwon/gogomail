# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Mail API exposes thread list and thread-message read models for conversation-style webmail rendering.
- `gogomail --mode=all-in-one` registers Mail API and Admin API routes in the
  same HTTP process for small-deployment and local release smoke coverage.
- Inbound and reply/forward outbound persistence assign thread IDs from RFC thread headers or source messages where possible.
- Reply composition writes RFC thread headers into outgoing `.eml`, preserving conversation threading outside gogomail.
- Mail API exposes a small-deployment Postgres-backed search endpoint for active-message metadata, with full received-body indexing handled by the indexing worker boundary. Draft rows stay out of active-message search and use the separate compose-focused `GET /api/v1/drafts/search` contract for subject, sender, recipient, body, and attachment-state lookup.
- Received-message body indexing now has a first worker boundary: `search-index-worker` consumes `mail.stored`, rejects ambiguous storage object paths before opening `.eml` objects, caps event `References` metadata, extracts bounded plain text, writes Postgres search documents, and lets the existing search endpoint include received body text without changing its response envelope.
- OpenSearch has writer and query adapters behind `internal/searchindex`; the
  search index worker can select the writer with explicit endpoint/index
  configuration, configurable HTTP timeout, and optional index bootstrap on
  startup. Mail API relevance search can use OpenSearch for ranked IDs,
  folder/from/subject/attachment filters, and highlights, then hydrate the
  existing response shape from Postgres summaries. OpenSearch document
  metadata and search/filter text are bounded at UTF-8 boundaries, and wildcard
  metacharacters in sender/subject filters are escaped before submission.
  Index document IDs are rejected when blank, CR/LF-bearing, or oversized
  before `_doc/{id}` URL construction. Returned hit IDs are cleaned and
  CR/LF-bearing IDs are dropped before Postgres hydration. Search response
  decoding rejects oversized bodies and trailing JSON tokens before hits are
  accepted.
- Search results can now opt into relevance ordering, rank scores, and bounded headline snippets without changing default newest-first behavior.
- Mail API exposes bounded bulk flag, move, and soft-delete actions for efficient webmail list operations.
- Attachment uploads now support both metadata reservation and direct multipart storage writes.
- Pending attachment uploads can be canceled immediately, releasing quota and
  deleting any stored upload object without waiting for stale cleanup.
- Attachment upload capability discovery exposes current limits and supported
  modes for future clients without hard-coded constants.
- ADR 0007 captures the resumable/chunked upload boundary before implementation,
  keeping session state, quota reservation, staged chunks, final attachment
  rows, and cleanup responsibilities explicit.
- `attachment_upload_sessions` migration prepares resumable upload state with
  lifecycle status, expiry, declared/received byte counters, checksum, and
  storage adapter metadata.
- Repository support can create upload sessions and reserve declared bytes in
  the shared quota ledger transactionally.
- Repository support can cancel resumable upload sessions and release the
  declared quota reservation exactly once.
- Repository support can expire stale resumable upload sessions in bounded
  batches and release declared quota reservations.
- `mailservice` wraps resumable upload session create/cancel/expire operations,
  preserving validation and domain attachment policy enforcement.
- Mail API exposes upload session create/read/cancel endpoints while keeping
  `resumable_chunked_uploads=false` until chunk receive and finalize routes are
  implemented; capabilities advertise session support separately, and session
  creation rejects already-expired or overlong expiries before quota
  reservation.
- Upload session body storage can persist a complete session body, record
  received bytes and SHA-256, while rejecting terminal sessions before storage
  writes.
- Upload session body storage can verify optional client-provided SHA-256
  digests before recording staged bodies.
- Attachment upload capabilities expose checksum precondition support for
  generated clients.
- OpenAPI contract tests now lock the upload session body checksum header so
  generated clients do not lose the integrity precondition.
- Upload session body storage explicitly rejects `Content-Range` requests while
  `resumable_chunked_uploads=false`, keeping complete-body storage distinct
  from future range-aware chunk commits.
- Upload session body replacement records retries through distinct staged object
  paths, preserving the previously recorded body if repository metadata updates
  fail and best-effort cleaning old staged bodies after successful replacement.
- Upload-session staged object paths must stay relative under the
  `upload-sessions/` prefix before repository persistence and before service
  storage reads/deletes, so corrupted rows cannot redirect cleanup or
  finalization to ambiguous object keys.
- Upload session finalization can create the normal pending attachment row from
  a ready stored session body without double-reserving quota.
- Upload session finalization verifies the staged object still exists and
  matches recorded size/SHA-256 before creating the attachment row.
- Optional PostgreSQL integration coverage verifies upload session finalization
  creates an attachment row while preserving the original quota reservation.
- Optional PostgreSQL integration coverage also rejects duplicate upload
  session finalization without changing quota or creating extra attachment rows.
- Optional PostgreSQL integration coverage rejects finalization before a session
  body is stored, preserving quota and avoiding empty attachment rows.
- Upload session cancellation deletes staged session bodies when present,
  aligning storage cleanup with quota release.
- Upload session expiry deletes staged session bodies when present, so cleanup
  workers do not leave abandoned session objects behind.
- Stale attachment uploads have a repository/service cleanup path, partial
  index, and `attachment-cleanup-worker` mode for efficient lifecycle sweeps,
  including stale resumable session expiry, optional run-once execution for
  scheduler-driven deployments, and Admin API direct upload-session inspection,
  dry-run previews, plus candidate listing before on-demand cleanup. Admin
  cleanup run responses now include
  stale upload-session candidate and expired counts so operator previews cover
  the same quota reservations that the worker can release, and candidate
  previews include bounded upload-session rows for row-level operator review.
- Direct multipart uploads write through the configured storage backend and only record metadata after the object write succeeds.
- Attachment upload size is guarded in HTTP and service layers, including
  multipart request caps that return 413 for over-limit direct upload envelopes
  and upload session bodies, plus declared-size consistency checks.
- Draft-to-send uses the normal outbound send path, then closes the source draft and links it to the sent message.
- Draft attachment uploads move to the sent message during draft-to-send, keeping sent folder detail and attachment list views consistent.
- Mail API send responses explicitly expose queued send, pending delivery, and no-bounce status fields so generated clients can model send lifecycle state without guessing from queue internals.
- Detail reads mark unread messages as read while avoiding redundant writes for already-read messages.
- Compose and draft validation guard user id, intent/source rules, recipient presence, recipient email syntax, recipient count, subject size, text body size, attachment IDs, filename safety, MIME type, upload size, and outbound RFC 5322 header injection values.
- Mail API path identifiers and direct-upload `draft_id` form values are trimmed
  at the HTTP boundary before service dispatch, and direct multipart uploads
  reject repeated `draft_id` or `file` parts before storage work begins.
- Mail and Admin API JSON request bodies reject trailing JSON tokens and
  unknown object fields instead of accepting drifted payloads, and shared JSON
  decoding is capped at 1 MiB before parsing.
- Attachment downloads expose a safe ASCII `filename` fallback plus UTF-8
  `filename*` in `Content-Disposition`, bound stored filename length before
  response headers are written, keep responses private `no-store`, and fall
  back to `application/octet-stream` for unsafe or media-type-invalid stored
  MIME types. OpenAPI documents the binary media type and download response
  headers.
- API usage artifact downloads sanitize stored content types and SHA-256
  response headers before streaming handoff objects, including media-type
  validation before writing `Content-Type`.
- API usage NDJSON exports and stored export artifact downloads return
  `Cache-Control: no-store`, documented in OpenAPI for generated clients.
- Attachment downloads, usage NDJSON exports, and stored export artifact
  downloads return `X-Content-Type-Options: nosniff`, documented in OpenAPI for
  generated clients.
- API errors use a stable structured envelope with code, message, HTTP status,
  and HTTP status text, and return `Cache-Control: no-store` plus
  `X-Content-Type-Options: nosniff`.
- Service info exposes API and backend contract version metadata; readiness exposes a structured checks envelope.
- Readiness checks now include contract/storage/outbox boundary metadata and
  runtime-injected database/Redis probes for HTTP modes that depend on those
  services, returning a degraded 503 response when a required dependency probe
  fails.
- Mail/Admin HTTP readiness now includes a real local storage write/read/delete
  probe, and unsupported HTTP storage backends fail fast instead of silently
  falling back to local storage wiring.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, and HTTP runtimes now share storage backend validation, preventing
  unsupported object-storage settings from silently using the local adapter.
- The shared HTTP server now has configurable and validated read, write, idle,
  read-header, and maximum-header guardrails for Mail/Admin/API-metered modes.
- Admin API supports domain/user list, detail, create, and status updates plus queue, outbox-event metadata, delivery-attempt, suppression, DKIM, retry, and delete operations. Company lists support lifecycle status filters for tenant triage. Domain lists support company, status, and latest DNS-status filters for onboarding triage, and DNS-check history supports summary-status plus recent-window filters. Delivery-route lists support status, farm, and domain-pattern filters for route audits. Trusted-relay lists support CIDR and description filters for inbound relay-policy audits. Delivery-attempt lists and stats support status, recipient-domain, message-id, farm, sender, and recent-window filters for bounded retry/bounce triage; exhausted-attempt lists support the same incident filters for terminal retry triage. Suppression-list reads support domain, email, and reason filters for bounce triage. Attempt rows retain sender, enhanced-status, and RFC 3461 DSN metadata for operator diagnostics. Attempt list ordering uses a stable ID tie-breaker after timestamp ordering.
- Admin user creation and password-hash rotation can persist validated
  `password_hash` values for SMTP Submission authentication, rejecting
  unsupported, CR/LF-bearing, or oversized hash strings before database
  storage. User read models expose `password_configured` without leaking stored
  password hashes, and user listing can filter by status plus that readiness
  flag for operator triage.
- Admin API now exposes trusted relay CIDR list/create/delete operations backed by PostgreSQL, preparing inbound SMTP relay policy for auditable runtime administration.
- Admin API now exposes delivery route list/create/status/delete operations backed by PostgreSQL, preparing gateway and smart-host policy for auditable runtime administration without coupling it to SMTP core.
- Admin API can dry-run delivery route resolution for a recipient domain, improving runtime route observability without triggering SMTP delivery.
- Admin queue stats distinguish ready pending work, delayed pending work, and stale processing locks so operators can tell backlog from scheduled retry delay.
- Admin API exposes a quota usage pressure read model for company, domain, and user limits, with scope/domain/over-limit/over-allocation filters so operators can spot targeted backpressure risks before SMTP or Mail API writes start failing.
- Admin quota read models expose remaining capacity, child allocation, allocatable capacity, and over-allocation flags.
- Admin API exposes a read-only quota reconciliation report for detecting ledger drift against message and attachment source rows.
- Admin API exposes operator-controlled quota reconciliation corrections guarded by transaction/advisory locking, with bounded audit-log detail for dry-run and applied correction attempts.
- Admin API exposes bounded audit-log list/detail reads with category, action, result, target-type, company/domain/user, and recent-window filters so operator-visible audit trails no longer require direct database access.
- Domain DNS check, quota reconciliation correction, trusted relay create/delete, delivery route create/status/delete, DKIM key lifecycle, domain/user status, company/domain/user quota, domain policy, and domain/user provisioning audit writes reuse the shared hash-chain writer, so their newly visible audit rows carry non-empty tamper-evidence hashes while excluding relay, password-hash, and private-key secrets from audit detail.
- Admin API exposes a bounded audit-log integrity check that recomputes recent row hashes and reports hash or in-window `prev_hash` breaks without mutating audit rows.
- Quota product direction is captured in ADR 0003 and partially implemented: company contracted storage pool, domain allocations, user unified storage allowance, `default|custom` user quota source, domain default user quota propagation, and atomic company/domain/user ledger updates for mail storage writes/deletes plus attachment upload/cleanup.
- API metering is recorded as a planned SaaS platform boundary: usage should be collected asynchronously for future billing/rate-limit/abuse analytics, while enforcement remains policy-driven and disabled by default in the MVP.
- API metering has a disabled-by-default fail-open middleware boundary with `slog` and outbox sinks for early operational visibility and durable usage-event collection; configured admin-token identity classification uses fixed-length SHA-256 digest comparison for bearer and `X-Admin-Token` values.
- API metering has a disabled-by-default aggregation worker and daily/monthly Postgres read models exposed through `GET /admin/v1/api-usage/daily` and `GET /admin/v1/api-usage/monthly`; v2 events carry tenant/company/domain/user/API-key/principal/auth-source dimensions plus deterministic IDs, and aggregate reads support bounded dimension, route/method/status, and time-window filters for scoped billing and operational triage. Request identity dimensions are whitespace-normalized, CR/LF-bearing or oversized default request dimensions are dropped, auth-source values are normalized to a fixed known set, blank or unsafe bearer headers are not classified as bearer traffic, route keys and HTTP-like status values are required, CR/LF-bearing or oversized middleware route keys are dropped before sink dispatch, CR/LF-bearing method/route/event-id/identity dimensions are rejected before outbox insertion and ledger storage, aggregate store direct calls also validate event IDs, schema versions, identity dimensions, and HTTP-like status before writes, middleware falls back to `METHOD /path` when no ServeMux pattern is available, negative byte/latency metrics are clamped to zero before outbox payload generation and aggregate storage, replayed event IDs are not double-counted, and aggregates are keyed by identity. The worker also records immutable `api_usage_ledger` rows with bounded Admin API list, NDJSON export, stats, non-future-cutoff retention readiness enforced at HTTP and repository boundaries, bounded confirm-gated retention runs with persisted audit rows and Admin API list/detail inspection, a dry-run-by-default `api-usage-retention-worker` for interval or once-and-exit retention sweeps whose destructive configuration requires `confirm_ready` plus a `remote-ed25519` export manifest signer backend, CR/LF/size-bounded tenant/principal query filters, persisted export batch manifests/checkpoints that require explicit `from`/`to` windows and can be listed by tenant/principal/status/window filters, local object-store artifact writing/download/verification, external artifact metadata handoff, canonical manifest digest verification, disabled-by-default local-HMAC/local-Ed25519/remote-Ed25519 manifest signing and verification behind signer/verifier interfaces, export capability inspection that advertises retention-run/worker support and destructive-worker remote-key requirements, and a handoff readiness report that separates operational readiness from billing readiness. Optional PostgreSQL integration coverage verifies retention runs preserve blocked candidates, keep dry-runs read-only, persist run audit rows, expose list/detail reads, and delete only bounded ready rows while preserving newer ledger rows. The handoff report can opt into `deep=true` to stream artifacts, check latest digest artifact coverage, and verify latest digest/signature evidence in one operator response with separate `verified_billing_ready`. Invoice/hard-limit use should rely on `remote-ed25519` only when backed by an approved KMS signing service, or on a future direct cloud KMS adapter.
- API usage export artifact object keys reject ambiguous path-cleaning changes,
  backslashes, and parent-traversal segments before handoff objects are written.
- API usage export manifest digesting rejects unsupported explicit manifest
  schema versions before canonical digest evidence is generated.
- API usage export manifest signing rejects blank, CR/LF-bearing, or oversized
  key IDs for local and remote signer metadata before signature evidence is
  returned.
- API usage export batch, artifact, manifest-digest, and manifest-signature path
  identifiers reject blank, CR/LF-bearing, or oversized values before service
  dispatch.
- Search relevance has backend-specific regression coverage for Postgres weighted `tsvector` ranking and OpenSearch field boosts, keeping subject/sender matches ahead of indexed body matches while preserving date-sorted defaults.
- IMAP has a backend gateway boundary package with native DTOs/interfaces, mailbox state helpers, and RFC-shaped flag mapping; no protocol server is in release scope yet.
- IMAP UID storage has durable mailbox UIDVALIDITY/UIDNEXT/highest-MODSEQ rows and message UID/MODSEQ rows, with transactional assignment helpers, first mailbox/message list adapters, raw body fetch groundwork, MODSEQ-aware flag mutation, bounded UID backfill, and move/delete UID invalidation; no protocol server is in release scope yet.
- `mailservice` now exposes IMAP mailbox/message listing, raw fetch, flag store,
  UID backfill, and mailbox-event subscription through service methods plus an
  `IMAPStoreAdapter` satisfying `imapgw.Store`, keeping future protocol wiring
  off direct `maildb` internals.
- Admin API exposes bounded IMAP UID backfill by user/mailbox for future
  operator/bootstrap runs without enabling an IMAP protocol listener.
- IMAP IDLE remains out of scope, but `internal/imapgw` now has an in-memory
  mailbox event broker for future session fan-out. The broker is scoped by
  user+mailbox, and service-side flag/move/delete mutations publish best-effort
  `flags`/`expunge` events for UID-visible messages.
- `gogomail --mode=imap` now starts an IMAP gateway scaffold that wires the
  service-backed IMAP store adapter and process-local mailbox event broker
  without advertising or enabling a TCP IMAP listener.
- The shared event worker now ensures IMAP UID rows for committed `mail.stored`
  receive events, moving received messages toward UID-visible state without
  coupling SMTP receive to future IMAP listener work; IMAP UID assignment event
  decoding rejects CR/LF-bearing or oversized message/user/folder IDs before
  UID work or mailbox event fan-out.
- Redis-backed event/search/API-metering/push/delivery workers reclaim idle
  pending stream messages with configurable claim-idle windows so crashed
  consumers do not strand at-least-once work indefinitely.
- Event routing trims registered and payload event names and rejects
  CR/LF-bearing or oversized event names before worker dispatch.
- Redis stream event decoding trims outbox id, partition key, and payload
  fields and rejects blank, CR/LF-bearing, or oversized metadata before handler
  dispatch.
- Push notification `mail.stored` event decoding rejects CR/LF-bearing or
  oversized message/user IDs before target resolution or candidate fan-out.
- Search indexing `mail.stored` event decoding rejects oversized message/user
  IDs and storage paths before stored EML objects are opened.
- Mail receive audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery status audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery `mail.queued` decoding rejects oversized message identities and
  storage paths, and rejects ambiguous, absolute, parent-traversal,
  backslash-bearing, or non-`.eml` storage object keys before SMTP transport or
  message storage access.
- Delivery `mail.queued` DSN option decoding rejects oversized
  `original_recipient` values before retry/delivery attempt recording.
- Delivery `mail.queued` decoding rejects oversized recipient and DSN-recipient
  arrays before normalization, routing, or retry bookkeeping.
- Attachment scanner hook rejection/tempfail reasons are CR/LF-stripped and
  UTF-8 safely bounded before they are surfaced as SMTP hook errors.
- Redis duplicate-message detection uses fixed-length hashed dedup keys so raw
  message IDs or recipient addresses cannot create oversized Redis keys.
- Redis outbox publishing trims event id, topic, partition key, and payload
  metadata and rejects invalid topics or non-JSON payloads before stream writes.
- EML parser hot-path guardrails include bounded-read truncation coverage, a
  MIME part-count cap with `PartsTruncated` signaling, retained subject,
  address, message-id, and reference metadata caps with UTF-8-safe truncation or
  oversized-ID dropping, and a large-body plus metadata-only benchmark. Retained
  address-list and `References` counts are also capped with truncation flags so
  oversized headers cannot expand downstream storage and search metadata without
  bound.
- EML attachment metadata detection includes inline filename parts and non-text
  inline parts without reading attachment bodies.
- The audit `mail.stored` consumer trims event, tenant, recipient, subject,
  storage, and timestamp fields and rejects CR/LF-bearing message identifiers
  before audit-log persistence.
- Delivery-status audit consumers trim event, tenant, sender, recipient, farm,
  status, error, and timestamp fields and reject CR/LF-bearing message
  identifiers before audit-log persistence.
- Delivery outcome and exhausted outbox event payloads trim message, tenant,
  farm, sender, recipient, error, and DSN metadata before event persistence.
- Push notification enqueue has a disabled-by-default worker boundary over committed `mail.stored` events with a bounded Postgres device resolver that drops malformed targets, including blank, CR/LF-bearing, oversized, or unsupported device IDs/tokens/platforms, before candidate recording and sink handoff; optional target labels and token suffixes are UTF-8 safely bounded. The worker has per-device candidate-attempt persistence with UTF-8-safe diagnostic caps, queued outcome updates after successful sink handoff, failed outcome updates after sink errors, Admin API inspection/detail/stats including message-, user-, platform-, device-, and recent-window-scoped views, an authenticated Admin outcome update endpoint for operator/provider handoff callbacks, replaceable sink, `slog` first adapter, and a webhook sink for handing raw-token targets to an external push gateway with an optional bounded bearer token. Candidate recording rejects invalid-UTF-8, CR/LF-bearing, or oversized message/user/device/company/domain IDs before SQL insert dispatch, and rejects unsupported platforms at the recorder boundary. Worker and Admin outcome updates share the same `maildb` storage path, so validation, diagnostic bounds, attempted timestamps, and invalid-token device deletion cannot drift between internal sink handling and operator/provider callbacks. Attempt and stats inspection both support bounded `message_id` filters for per-message fan-out troubleshooting. Outcome recording rejects invalid-UTF-8, CR/LF-bearing, or oversized attempt IDs before SQL update dispatch. The webhook sink also bounds and normalizes direct-call payload metadata before JSON serialization and drops malformed targets; `docs/webhook-integrations.md` documents the JSON payload, authentication, HTTPS requirement, and attempt-state semantics. Mail API device-token registration/list/delete exists with write-only raw tokens, and delete rejects blank, CR/LF-bearing, or oversized device identifiers before repository dispatch, while first-party vendor push delivery remains out of scope.
- Domain outbound policy can cap individual attachment uploads with `max_attachment_bytes`, enforced before quota reservation or object storage writes.
- Attachment scanner integration has a disabled-by-default hook adapter outside
  SMTP core and a configured HTTP webhook backend that can be wired into Edge,
  Inbound, and Submission MTA app boundaries with an optional bounded bearer
  token; webhook URLs must be HTTPS in production, and
  `docs/webhook-integrations.md` documents the scanner request, bounded
  response, and accept/reject/tempfail verdict contract. Scanner webhook
  requests bound and normalize message, address, subject, recipient, and
  attachment metadata before JSON serialization.
- Admin API can persist a domain operational policy model in `domains.settings.policy`, and Mail API send/draft-send enforces outbound recipient-count and composed-size guardrails when `outbound_mode=enforce`.
- DKIM key creation derives the public DNS TXT record from the private key when omitted, reducing operator DNS setup errors while preserving private-key omission from admin list responses.
- Admin API exposes domain DNS verification for MX, SPF, DMARC, and active DKIM TXT records, and each check is persisted with an audit log entry for domain onboarding traceability before frontend implementation.
- Delivery workers can opt into PostgreSQL-backed delivery routes through `GOGOMAIL_DELIVERY_ROUTE_BACKEND=postgres`, reusing the existing delivery router boundary and falling back to direct MX delivery when no active route matches.
- Admin domain/user create validation rejects malformed domains, unsafe usernames, invalid ACE names, and mismatched primary address ownership.
- SMTP receive/submission paths now include TCP-level protocol integration coverage for inbound delivery, AUTH PLAIN submission, policy rejection, and SMTPS.
- SMTP wire coverage now exercises enabled/disabled extension advertisement, DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` propagation including `NOTIFY=NEVER`, unsupported extension rejection, STARTTLS-gated AUTH, implicit TLS, trusted relay CIDR rejection, and repeated transactions on a single connection.
- Outbound SMTP wire coverage now verifies DSN parameters are emitted only when the remote MTA advertises DSN support, preventing accidental RFC 3461 option leakage to non-DSN peers. Outbound EAI addresses fail closed with a permanent SMTPUTF8 error when the remote MTA does not advertise SMTPUTF8.
- Outbound SMTP controlled-sink coverage now verifies accepted DATA can coexist with per-recipient permanent and temporary RCPT failures, preserving retry/bounce classification for delivery handlers.
- DSN/bounce generation validates inbound event metadata before composing and queueing null reverse-path DSNs.
- DSN/bounce generation now honors RFC 3461 `RET=HDRS` by attaching bounded,
  sanitized original message headers as a `text/rfc822-headers` report part
  when delivery events carry a safe original `.eml` storage path.
- DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext identity metadata before it can reach outbound SMTP command generation or RFC 3464 report composition.
- Delivery partial-failure handling preserves recipient-level retry/bounce decisions even when every RCPT is rejected.
- Attachment upload storage paths reject absolute, parent-traversal, backslash, and newline forms, and generated attachment object paths sanitize path segments before writing to storage.
- Migration file guardrails now require every SQL migration to declare explicit
  goose Up/Down sections, including the legacy API-usage, push, IMAP, and
  audit-index migration range.
- Runtime database readiness now checks the applied goose migration version
  against the latest local SQL migration, causing stale schemas to return
  degraded `/health/ready` status instead of passing on ping alone.
- Runtime storage readiness now probes the local mailstore through the storage
  adapter before Mail/Admin HTTP modes report healthy.
- Admin backpressure updates now write bounded hash-chain audit rows with
  previous/current SMTP pressure state after Redis updates, so receive-throttle
  overrides are durable operational evidence.
- Admin suppression-list deletes now write hash-chain audit rows in the same
  transaction as the delete, keeping deliverability-control removals
  forensically inspectable through audit APIs.
- Admin outbox retry now writes a hash-chain audit row in the same transaction
  as the retry reset, preserving previous event state before operator replay.
- Admin push-notification outcome updates now write hash-chain audit rows in the
  same transaction as provider-status changes and invalid-token device cleanup,
  while keeping raw push tokens and token suffixes out of audit detail.
- Admin attachment cleanup runs now write bounded hash-chain audit rows for
  stale upload and upload-session expiry sweeps, recording cutoff, normalized
  limit, expired counts, and ID samples without storage paths.
- Admin IMAP UID backfill now writes a hash-chain audit row in the same
  transaction as UID assignment, keeping mailbox bootstrap operations
  inspectable before enabling a full IMAP listener.
- Admin API-usage export batch creation now writes a hash-chain audit row in the
  same transaction as the batch, keeping invoice/retention export boundaries
  inspectable before artifact handoff.
- Admin API-usage export artifact creation/upsert now writes a hash-chain audit
  row in the same transaction as artifact persistence, keeping object key,
  byte/event counts, and SHA-256 digest evidence inspectable.
- Admin API-usage export manifest digest and signature creation now write
  hash-chain audit rows in the same transaction as the evidence rows, keeping
  canonical digest and signer evidence inspectable without copying raw
  manifests, metadata, or full signature material into audit detail.
- Admin API-usage ledger retention runs now write hash-chain audit rows in the
  same transaction as run records and destructive deletes, keeping dry-run,
  blocked, no-op, and completed retention outcomes inspectable with bounded
  readiness evidence.
- Domain policy service lookups trim domain and user identifiers before
  repository policy reads for outbound and attachment enforcement.
- Attachment upload reservation and direct-upload service requests normalize
  user, draft, filename, MIME type, and storage-path metadata before quota,
  storage, and repository work, and reject CR/LF-bearing or oversized draft
  identifiers before quota reservation or object writes.
- Stale attachment-upload cleanup validates its time window and limit at the
  service boundary before repository cleanup/object deletion work, and the
  worker's interval, stale age, and batch size are configuration-validated.
  Stored-object delete failures are reported to the worker/operator, while
  missing objects are treated as idempotently cleaned. Cleanup batch sizes use
  an attachment-specific 1000-row cap rather than message-list pagination caps.
  Admin API can also run stale upload cleanup on demand with an explicit
  non-future cutoff.
- Attachment list/download and draft-delete service methods trim user, message,
  attachment, and draft identifiers before repository/storage work; attachment
  reads reject blank, CR/LF-bearing, or oversized message/attachment
  identifiers before repository/storage dispatch.
- `docs/backend-api-contracts.md` stages the backend-only OpenAPI contract source.
- `docs/openapi.yaml` provides the first backend-only OpenAPI 3.1 draft and is guarded against backend contract version drift, registered-route drift, dangling component references, request-body omissions, response envelope reference drift, message flag enum drift, list limit contract drift, and thread-list parameter leakage.
- OpenAPI response components now document the Mail/Admin JSON envelope keys used by generated clients, including admin queue, IMAP UID backfill, delivery attempt, exhausted-attempt, suppression, DKIM, domain, and user read models.
- OpenAPI operations now carry stable lower-camel `operationId` values and default reusable Error responses for protected/mutable operations, reducing generated-client naming and error-decoding drift.
- OpenAPI now documents and tests the API usage ledger `tenant_id`, `principal_id`, `from`, and `to` filters that runtime handlers already accept, keeping generated billing/export clients aligned with Admin API behavior.
- OpenAPI contract tests pin the push-device list `limit` query parameter so
  generated clients keep pagination controls for device management.
- Mail search requests trim query/folder/sender/subject filters at the HTTP and
  service boundaries, then reject CR/LF-bearing or oversized query/filter fields
  before Postgres or OpenSearch dispatch.
- Draft search requests trim query/sender/subject filters at the HTTP and
  service boundaries, then reject CR/LF-bearing or oversized query/filter fields
  before bounded compose-store lookup.
- Mail API bearer JWT verification rejects unsupported `alg` values and
  non-JWT `typ` headers before accepting signed claims; token/header/payload/
  signature segments are size-bounded before base64 decoding; `user_id`/`sub`
  identities are whitespace-normalized and blank, CR/LF-bearing, or oversized
  identities are rejected during signing and verification; and future `iat`
  values beyond a one-minute skew are rejected.
- Mail/Admin authentication headers are size-bounded before bearer parsing,
  JWT decoding, or admin-token comparison.
- Password hash verification rejects oversized stored hashes, excessive PBKDF2
  iteration counts, and oversized salt/key metadata before expensive derivation
  or decoded allocation.
- Mail API search control query values and direct multipart attachment
  `draft_id` fields reject CR/LF-bearing or oversized values at the HTTP
  boundary before service dispatch.
- VERP return-path parsing rejects oversized addresses, local parts, tokens,
  and encoded recipients before base64 decoding DSN recipient metadata.
- API usage export Ed25519 signer/verifier key configuration rejects oversized
  base64 public/private keys before decoding.
- API usage export manifest signer configuration rejects CR/LF-bearing or
  oversized key IDs and remote signer tokens, and local HMAC signing rejects
  oversized secrets before MAC generation.
- API usage export HMAC and Ed25519 signature verification rejects incorrectly
  sized signature hex before decoding.
- Remote Ed25519 manifest signer responses reject oversized bodies and trailing
  JSON tokens before signature evidence is accepted.
- Admin API domain query identifiers for user listing, DKIM key listing, and
  delivery-route resolution are trimmed before service dispatch; DKIM key
  listing can filter active and inactive key lifecycle states.
- Admin API DKIM key deactivate and DNS-verify path identifiers are trimmed
  before service dispatch and response envelopes.
- Admin API suppression-list and trusted-relay delete path identifiers are
  trimmed before service dispatch and response envelopes.
- Admin API company, domain, and user quota/status/policy mutation path
  identifiers are trimmed before service dispatch and response envelopes.
- Admin API outbox event topic, partition key, and status filters are trimmed
  before operational queue inspection, and CR/LF-bearing or oversized filter
  values are rejected before service dispatch.
- Admin API delivery-attempt status and recipient-domain filters are trimmed
  before retry/bounce inspection, and CR/LF-bearing or oversized filter values
  are rejected before service dispatch.
- Admin API push-notification attempt and stats filters are trimmed before
  device/provider troubleshooting queries, and CR/LF-bearing or oversized
  filter values are rejected before service dispatch.
- Admin push-notification attempt/stats repository filters also reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- Admin API user-list, IMAP UID backfill, DKIM key-list, and delivery-route
  resolution query filters are trimmed, and CR/LF-bearing or oversized values
  are rejected before service dispatch.
- Admin API company, domain, and user detail/mutation path identifiers reject
  blank, CR/LF-bearing, or oversized values before service dispatch.
- Admin API IMAP UID backfill mailbox IDs, outbox event/retry IDs, DKIM key
  IDs, suppression IDs, trusted-relay IDs, and delivery-route IDs reject blank,
  CR/LF-bearing, or oversized values before service dispatch.
- Mail API development `user_id` query fallback values are trimmed and reject
  CR/LF-bearing or oversized identifiers before service dispatch.
- Mail API folder, thread, message, draft, attachment, and push-device path
  identifiers reject blank, CR/LF-bearing, or oversized values before service
  dispatch.
- Mail API message-list `folder_id` and search text/filter query parameters
  reject CR/LF-bearing or oversized values before service dispatch.
- Mail API draft-search text/filter query parameters reject CR/LF-bearing or
  oversized values before service dispatch.
- Mail API push-device registration normalizes user, platform, token, and label
  fields before validation/storage while responses keep raw tokens write-only.
- Push-device list and delete service methods trim user and device identifiers
  before repository work.
- Mail compose draft/save/send requests normalize user/source/from/address and
  attachment identifier fields before repository, storage, suppression, and
  outbound composition work, reject CR/LF-bearing from/subject values and
  recipient display names/emails before draft persistence or header
  composition, and draft saves enforce the same attachment-count cap as
  immediate sends.
- Draft save/delete/send and reply/forward compose validation reject blank,
  CR/LF-bearing, or oversized draft/source-message identifiers before
  repository dispatch.
- Single-message flag, move, and delete service methods trim user/message/flag
  and folder identifiers before repository mutation and IMAP event fan-out, and
  reject blank, CR/LF-bearing, or oversized message/folder identifiers before
  repository or IMAP UID lookup work.
- Bulk flag, move, and delete service methods trim user/message/flag and folder
  identifiers before repository mutation, IMAP UID lookup, and mailbox event
  fan-out, while rejecting CR/LF-bearing or oversized bulk resource IDs.
- Folder, message-list, thread-list, and message-detail service reads trim
  user, folder, thread, message, and folder-name inputs before repository work;
  user folder create/rename rejects blank, path-bearing, CR/LF-bearing, or
  oversized names, and rename/delete reject unsafe folder identifiers before
  repository dispatch. Folder-scoped message lists and thread-message reads
  also reject unsafe folder/thread identifiers before repository work.
- Message, thread, and push-device list service methods normalize list limits
  to the documented message-list bounds before repository work.
- Message-list cursor decoding rejects oversized opaque cursor strings before
  base64 decode and JSON parsing.
- IMAP service methods trim user/mailbox identifiers and normalize list/backfill
  limits before repository, storage, broker, or mailbox-event work.
- Message and attachment body reads/deletes validate DB-returned storage object
  paths before calling the storage adapter, failing closed on absolute,
  traversal, newline, backslash-bearing, or empty stored keys where a body is
  required.
- Local storage enforces the same strict object-key contract at the adapter
  boundary, rejecting absolute, traversal, newline, backslash-bearing,
  duplicate-separator, dot-segment, and otherwise non-canonical keys before
  reads, writes, or deletes.
- Mail search service queries normalize user, text, folder, sender, subject,
  and sort inputs before Postgres or OpenSearch dispatch.
- Draft search service queries normalize user, text, sender, subject, and list
  limits before repository dispatch.
- Message delivery-status and reply source-thread service lookups trim user,
  message, and source-message identifiers before repository work.
- Push-device create/update validation rejects invalid-UTF-8, CR/LF-bearing,
  or oversized user and token metadata before repository upsert, while
  preserving write-only raw token responses.
- Mail API search query, folder, sender, and subject filters are trimmed before
  search backend dispatch, reducing accidental UI/client whitespace drift.
- Mail/Admin scalar query parameters reject duplicate values before dispatch,
  preventing HTTP parameter pollution ambiguity for user IDs, limits, boolean
  flags, timestamps, and operational filters.
- HTTP list endpoints now enforce the documented `1 <= limit <= 200` boundary before reaching repository pagination, so generated clients can rely on the OpenAPI limit bounds.
- `docs/smtp-release-runbook.md` now records operator-facing SMTP soak, STARTTLS, SMTPS, trusted relay, and outbound DSN/bounce smoke procedures.
- `docs/api-usage-export-runbook.md` records the operator-facing API usage export, deep handoff verification, signer capability, and retention-readiness sequence.
- `scripts/verify-backend-release.sh` runs the standard backend release checks (`go test ./...`, `go mod tidy -diff`, optional PostgreSQL integration tests when `GOGOMAIL_TEST_DATABASE_URL` is set, optional OpenSearch integration coverage when `GOGOMAIL_TEST_OPENSEARCH_URL` is set, and a clean-worktree gate that fails on pending repository changes).
- PostgreSQL-backed integration tests can be enabled with `GOGOMAIL_TEST_DATABASE_URL` to run migrations in a temporary schema and exercise draft-to-send/outbox/retry behavior plus IMAP UID backfill/move invalidation against real SQL.
- OpenSearch integration tests can be enabled with `GOGOMAIL_TEST_OPENSEARCH_URL` to create a disposable index and verify bootstrap mapping, idempotent indexing, folder-aware relevance filters, and query-side hydration IDs against a real backend.

## Must verify before release cut

- Run `go test ./...`.
- Run `go mod tidy -diff`.
- Or run `./scripts/verify-backend-release.sh` to execute the standard backend release verification bundle.
- Verify `docs/openapi.yaml` still matches Go routes through the `internal/httpapi` contract tests before generating frontend clients.
- Verify generated clients preserve the documented top-level envelope keys rather than flattening Mail/Admin response bodies.
- Run `GOGOMAIL_TEST_DATABASE_URL=... go test ./internal/maildb ./internal/outbox` against a disposable PostgreSQL database/schema.
- Run `GOGOMAIL_TEST_OPENSEARCH_URL=... go test ./internal/searchindex` against a disposable OpenSearch backend before enabling the OpenSearch search path in production.
- Run `go test ./internal/imapgw ./internal/mailservice ./internal/maildb` before
  changing IMAP gateway boundaries, and enable `GOGOMAIL_TEST_DATABASE_URL` for
  the optional UID backfill/move invalidation integration coverage.
- For an OpenSearch rollout smoke, set
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`,
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT`,
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX`,
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`, and an explicit
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`; then start
  `gogomail --mode=search-index-worker` and confirm logs report the expected
  backend, index, bootstrap state, and max body bytes without credentials.
- Run focused SMTP soak checks for repeated same-connection transactions and STARTTLS/SMTPS startup in the intended deployment environment.
- Exercise multipart attachment upload against the intended object storage adapter. Local-storage path safety, declared-size mismatch, oversize body cleanup, metadata-after-object-write behavior, and quota-exhaustion HTTP mapping are now covered in automated tests.
- Exercise outbound DSN/bounce generation against a deployment-level controlled SMTP sink. Unit and wire tests now cover `NOTIFY=NEVER`, null reverse-path queueing/suppression, DSN option suppression to non-DSN peers, and retry/bounce recipient classification for temporary/permanent recipient failures.
- Verify frontend contracts for error envelope parsing, upload endpoint naming, and draft send response handling.

## Intentionally out of scope for this release slice

- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam logic.
- IMAP/POP3.
- OpenSearch as the default/mandatory search backend, vendor push delivery
  adapters, Kafka, Vault, and etcd.
