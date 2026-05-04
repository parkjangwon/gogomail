# gogomail current status

Last updated: 2026-05-04 (updated after platform hardening sprint)

## Current phase

gogomail is in the backend platform hardening phase.

The project has moved beyond SMTP-only development. SMTP remains a critical
RFC-sensitive core, but current work should balance:

- tenant/domain operations
- Admin API
- Mail API contracts
- delivery routing and observability
- DNS/DKIM/domain onboarding
- quota and policy enforcement
- OpenAPI drift prevention

Actual Next.js frontend implementation has not started. Before creating or
substantially implementing frontend apps, ask the user for frontend-specific
guidance.

## Completed or materially advanced

- SMTP receive engine with real TCP integration coverage.
- Authenticated Submission MTA with STARTTLS and SMTPS support.
- Outbound SMTP delivery with direct MX, smart-host, TLS policy, retry, and
  partial recipient failure handling.
- DSN/bounce handling with RFC 3461/3464-oriented metadata, null reverse-path,
  `NOTIFY=NEVER`, deterministic outbox dedupe, and loop-risk reduction.
- Shared high-performance-minded EML parsing boundary under `internal/message`.
- PostgreSQL metadata model for companies, domains, users, folders, messages,
  attachments, outbox, audit logs, DKIM keys, trusted relays, delivery routes,
  domain DNS checks, and policy-bearing domain settings.
- Admin APIs for domains, users, quotas, DKIM keys, trusted relays, delivery
  routes, delivery route resolution, queue stats, delivery attempts,
  suppression list, quota usage, domain DNS checks/history, backpressure
  inspection/update, domain policy, per-domain stats, DKIM DNS verification,
  delivery route runtime counters, and exhausted delivery attempts.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, thread lists/thread messages, plus user-scoped sent-message
  delivery/bounce status.
- Inbound parsing now extracts RFC `In-Reply-To`/`References`; inbound and
  reply/forward outbound persistence inherit local thread IDs when matching
  source messages exist.
- Reply composition writes RFC `In-Reply-To`/`References` headers into outgoing
  `.eml` messages.
- Mail API exposes a first Postgres-backed search endpoint for message metadata
  and draft text, with an FTS index for small deployments.
- Received-message body search now has an asynchronous indexing boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml` objects,
  extracts bounded text through the shared parser, and upserts Postgres search
  documents used by the existing search endpoint.
- Search responses can now opt into relevance sorting, rank scores, and bounded
  Postgres headline snippets while preserving date-sorted results by default.
- Mail API send/draft-send applies domain outbound policy in enforce mode for
  recipient-count and composed-message-size guardrails.
- Mail API attachment reservation/direct upload applies enforced domain
  `max_attachment_bytes` policy before quota reservation or object storage
  writes.
- Per-domain inbound policy enforced at SMTP receive and Submission MTA (max
  recipients, max message size, inbound mode).
- Hierarchical quota ledger enforced at mail storage write/delete boundaries:
  company, domain, and user usage counters are updated atomically in the same
  PostgreSQL transaction. User quota source is tracked as `default|custom`, and
  domain default user quota updates apply to default-following users while
  preserving custom overrides.
- Attachment upload metadata creation reserves bytes from the same
  company/domain/user quota ledger, stale upload cleanup releases them, and the
  Mail API returns HTTP 507 `insufficient_storage` for quota exhaustion.
- Admin quota views now expose runtime remaining capacity, child-allocation
  usage, allocatable capacity, and over-allocation indicators for
  company/domain/user operations.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with message and attachment source rows.
- Admin API can run operator-controlled quota reconciliation corrections with
  transaction/advisory locking.
- Product quota direction is company pool → domain allocation → user unified
  storage allowance. User quota should cover mailbox, attachments, future Drive,
  and other user-owned storage features.
- API metering direction is agreed for future SaaS operations: collect usage
  dimensions early through an async middleware/event boundary, but keep
  billing/rate-limit enforcement policy-driven and disabled by default.
- API metering has a first disabled-by-default middleware boundary with a
  `slog` sink for low-risk operational visibility and an outbox sink for durable
  `api.usage` event emission.
- API metering now has an aggregation worker boundary: `api-metering-worker`
  consumes `api.usage` events from `api.event`, upserts Postgres daily
  and monthly aggregates, and exposes `GET /admin/v1/api-usage/daily` plus
  `GET /admin/v1/api-usage/monthly` for operations.
- API metering events now use `2026-05-04.api-usage.v2` payloads with
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  worker stores those dimensions in the idempotency ledger and keys daily/monthly
  aggregates by identity so usage from different tenants or principals does not
  merge.
- API metering now records immutable `api_usage_ledger` rows before aggregate
  upserts. Admin API exposes bounded ledger list, NDJSON export, and stats
  endpoints for billing/export preparation while keeping HTTP request handling
  fail-open.
- API usage exports now have persisted batch manifests/checkpoints. Admin API
  can create/list/get manifest rows and replay a saved manifest window as NDJSON
  by batch ID.
- API usage export batches can now carry external artifact metadata rows with
  object key, content type, byte count, SHA-256, event count, and JSON metadata.
  Artifacts are deduplicated per batch by object key and SHA-256.
- Admin API can now write API usage export batch artifacts to the configured
  object store, register the resulting byte count/SHA-256 metadata, and download
  or verify stored NDJSON artifacts for handoff verification.
- API usage export batches now have canonical manifest digest rows and
  verification endpoints. Operators can generate SHA-256 digests over the saved
  batch plus registered artifacts, list/fetch digest records, and re-check the
  stored manifest against its canonical digest before billing handoff.
- API usage export manifest digests can now be signed through disabled-by-
  default local HMAC or local Ed25519 signers. Admin API exposes signature
  create/list/detail and verification endpoints while keeping the signer
  backend pluggable for future KMS integrations.
- Admin API exposes API usage export handoff readiness by batch. The report
  summarizes artifact coverage, latest digest/signature state, operational
  readiness, and a separate billing readiness grade so local signers are not
  mistaken for invoice-grade exports.
- Handoff readiness can now opt into `deep=true`, which streams registered
  artifacts from object storage for byte/SHA verification and verifies the
  latest manifest digest/signature in one operator report while keeping
  metadata-only readiness fields stable.
- Manifest signature verification now sits behind an
  `apimeter.ExportManifestSignatureVerifier` boundary parallel to signing. The
  current wired verifiers are local-HMAC and local-Ed25519, leaving a clean
  replacement point for KMS-backed production verification.
- Admin API exposes API usage export capabilities so operators can see the
  configured signer backend, signer key ID, verifier availability, and whether
  production/verified billing readiness is supported before creating handoff
  batches.
- Push notification enqueue now has an async worker boundary:
  `push-notification-worker` consumes `mail.stored` events, resolves active
  user devices from PostgreSQL, and can emit disabled-by-default `slog`
  notification candidates with Postgres candidate-attempt audit rows without
  touching SMTP hot paths or committing to FCM/APNs SDKs.
- Admin API exposes `GET /admin/v1/push-notification-attempts` for inspecting
  push notification candidate fan-out by status or user.
- Admin API exposes `GET /admin/v1/push-notification-stats` for a compact
  active-device and attempt-status summary.
- Push notification sinks receive the persisted candidate attempt id with each
  target, preparing clean vendor outcome updates later.
- `internal/pushnotify` can update attempt outcomes to queued, delivered,
  failed, or invalid-token without exposing that mutation as a public API.
- Invalid-token outcomes automatically soft-delete the affected push device in
  the same Postgres transaction.
- `mail.stored` events now carry an explicit
  `2026-05-04.mail-stored.v1` schema version for downstream audit, search, and
  push workers.
- Audit, search indexing, and push notification consumers reject unsupported
  explicit `mail.stored` schema versions while accepting versionless legacy
  events.
- Mail API now has user-scoped push device registration/list/delete contracts
  for `apns`, `fcm`, and `webpush`; raw device tokens are accepted only on
  write and are not returned in API JSON responses.
- DKIM key DNS verification workflow with `dns_verified_at` persistence.
- Delivery route runtime counters (`RouteCounters`) with Admin API exposure.
- Retry exhaustion hook: `mail.delivery_exhausted` outbox event emitted and
  `delivery_attempts` row with status `exhausted` written when all retries fail.
- DMARC reject policy enforcement at SMTP receive (`DMARCEnforce` flag).
- SMTPUTF8 declared correctly on outbound MAIL FROM for all internationalized
  addresses (RFC 6531 compliance fix).
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests.  All schemas kept in sync with Go types.
- Backend release verification script and SMTP release runbook.
- Public GitHub repository:
  <https://github.com/parkjangwon/gogomail>

## Explicitly not started

- Next.js shell/webmail/admin frontend implementation.
- Built-in spam scoring or pattern filtering.
- IMAP/POP3 protocol servers. A dependency-light `internal/imapgw` boundary now
  records native IMAP gateway DTOs, mailbox helpers, and flag semantics; durable
  UIDVALIDITY/UIDNEXT/MODSEQ storage and first `maildb` mailbox/message adapters
  exist, but no TCP protocol server is enabled.
- OpenSearch indexing.
- Kafka migration.
- etcd/Vault production control plane.
- Vendor push notification delivery adapters.

## Important guardrails

- Implemented SMTP features must strictly follow the relevant email RFCs.
- Do not advertise SMTP extensions until end-to-end semantics are implemented
  and tested.
- Do not turn SMTP core into a spam engine. Spam relay/filtering belongs behind
  explicit hooks/adapters.
- Keep hot paths streaming and allocation-aware.
- Preserve domain-as-tenant isolation.
- Commit by feature and push after completed work.

## Latest direction

The platform hardening sprint completed the following:

- Mailbox quota enforcement (receive, send, delete)
- Per-domain SMTP inbound policy (max recipients, max message size)
- DKIM DNS verification workflow
- Delivery route runtime counters
- Retry exhaustion events and Admin API exposure
- SMTPUTF8 outbound RFC 6531 fix
- DMARC reject policy enforcement hook
- Domain aggregate stats endpoint
- OpenAPI schema expansion (DKIMKey, DeliveryAttempt, DKIMKeyDNSVerification)
- Hierarchical quota ledger first implementation: company/domain/user Admin
  quota APIs, user quota source, domain default user quota propagation, and
  aggregate quota enforcement for mail writes/deletes.
- Attachment upload quota integration: upload metadata reserves quota, stale
  upload cleanup releases quota, and API quota exhaustion maps to 507.
- Search indexing boundary: bounded received body extraction runs in
  `search-index-worker` and stores Postgres search documents outside SMTP hot
  paths.
- OpenSearch indexing has a first `internal/searchindex` writer adapter behind
  the same indexing interface, and `search-index-worker` can select it with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- The OpenSearch writer can bootstrap a strict message index mapping for
  message IDs, tenant/user filters, subject/body text, timestamps, and bounded
  body metadata.
- `search-index-worker` can optionally bootstrap the OpenSearch index on startup
  with `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
- OpenSearch query-side groundwork can search user-scoped documents and return
  ranked gogomail message IDs for later metadata hydration.
- `maildb` can hydrate ordered message ID search hits back into active
  `MessageSummary` rows without changing the Mail API response envelope.
- `mailservice` can compose OpenSearch relevance ID hits with Postgres summary
  hydration when the current API search contract can be preserved; unsupported
  filter/highlight combinations fall back to Postgres search.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, enabling safe relevance-search
  read-side rollout while preserving fallback behavior.
- OpenSearch indexed documents now include parsed sender and attachment
  presence fields needed for Mail API search-filter parity.
- OpenSearch relevance search can apply folder, from, subject, and attachment
  filters before Postgres metadata hydration; sender and subject filtering use
  lower-cased keyword fields to preserve case-insensitive filter behavior.
- OpenSearch relevance search can return subject/from/body highlights and map
  them into the existing Mail API `search_highlights` response field with
  bounded UTF-8-safe fragments.
- Mail API OpenSearch hydration deduplicates repeated external hit IDs before
  loading Postgres summaries while preserving the first rank/highlight result.
- Optional OpenSearch integration coverage can create a disposable index and
  verify indexing plus folder-aware relevance search when
  `GOGOMAIL_TEST_OPENSEARCH_URL` is available.
- Search index worker startup logs include non-secret backend diagnostics,
  including OpenSearch index name and bootstrap state when that backend is
  selected.
- OpenSearch writer/searcher HTTP calls use a configurable timeout through
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`, defaulting to 10 seconds.
- Search contract expansion: clients can request `sort=relevance`,
  `include_rank=true`, and `include_highlights=true` without changing the
  default message list shape.
- Quota operations read models: capacity fields and reconciliation reporting
  show ledger pressure and drift without mutating counters.
- Quota correction actions: operators can explicitly apply reconciliation
  results to company/domain/user ledgers after reviewing drift.
- IMAP gateway planning: native backend interfaces, RFC-shaped flag/mailbox
  helpers, and durable UID/MODSEQ storage exist without starting a TCP protocol
  server.
- The first IMAP adapter path can list/get mailboxes, list mailbox messages,
  resolve messages by UID, stream raw stored message bodies, and mutate
  persisted IMAP-visible flags with MODSEQ advancement as `internal/imapgw`
  DTOs while ensuring UID state.
- Existing active mailbox contents can be backfilled with stable mailbox-local
  IMAP UIDs in bounded batches before any live IMAP listener is enabled.
- Mail API move/delete operations invalidate stale IMAP UID rows in the same
  transaction, keeping mailbox-local UID state from leaking across folders.
- Optional PostgreSQL integration coverage now exercises IMAP UID backfill and
  move invalidation when `GOGOMAIL_TEST_DATABASE_URL` is available.
- `internal/imapgw` has a small in-memory mailbox event broker for future IDLE
  fan-out without introducing a protocol listener yet; broker delivery is
  scoped by both user and mailbox to preserve tenant isolation.
- `mailservice.StoreIMAPFlags` can publish IMAP mailbox `flags` events through
  an optional event publisher after repository flag mutations succeed.
- Mail API single and bulk flag mutations can look up existing IMAP UID rows and
  publish mailbox `flags` events for UID-visible messages after the database
  update succeeds.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages after the database move succeeds.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages after soft-delete succeeds.
- `mailservice` exposes IMAP mailbox/message listing and mailbox-event
  subscription methods, keeping the future protocol listener pointed at the
  service boundary instead of `maildb` internals.
- `mailservice` exposes bounded IMAP UID backfill through the same service
  boundary for future operator/bootstrap modes.
- IMAP mailbox event publication from service mutations is best-effort, so a
  fan-out failure does not turn an already-committed mail mutation into a client
  error.
- `mailservice` has an `IMAPStoreAdapter` that satisfies `imapgw.Store`, so a
  future protocol listener can depend on the gateway interface while still
  routing through service methods.
- EML parser guardrails include a truncation-probe test and benchmark for the
  bounded text-body reader on large bodies.
- Push notification worker boundary: `mail.stored` can be consumed by a
  dedicated notification worker with a replaceable sink and a bounded Postgres
  device-target resolver plus candidate-attempt persistence.
- Push notification attempts are inspectable through the Admin API without
  introducing vendor push delivery as a required runtime dependency.
- Push notification device storage: authenticated users can register, list, and
  delete active device tokens through the Mail API while responses expose only a
  short token suffix.
- API metering boundary: HTTP middleware can emit fail-open usage events to
  logs or the durable outbox, while the disabled-by-default aggregation worker
  can build daily/monthly Postgres read models for operations.
- API metering events now carry an explicit schema version and deterministic
  event ID groundwork for future idempotent billing-grade aggregation.
- API metering aggregation now has an `api_usage_events` idempotency ledger so
  duplicate `event_id` deliveries do not increment daily/monthly counters again.
- API metering Admin API responses now expose tenant/company/domain/user/API-key,
  principal, and auth-source dimensions for daily/monthly aggregates.
- API metering Admin API now exposes immutable ledger list/export/stats endpoints
  so future billing and warehouse jobs can consume event-level usage instead of
  operational aggregates.
- API usage export batch manifests now capture fixed event/request/byte/latency
  totals for a bounded ledger window, preparing idempotent downstream export
  workflows.
- API usage export artifact metadata is now persisted and inspectable through
  Admin API endpoints, preparing object-store handoff without adding a vendor
  dependency to the core service.
- API usage export manifests now have canonical SHA-256 digest generation,
  local-HMAC/local-Ed25519 signing, and verification Admin API endpoints,
  tightening the audit trail before external KMS-backed signers are added.
- API usage export artifact writing now has a local object-store adapter path
  through Admin API, including full-batch streaming, retry-friendly artifact
  registration, stored artifact download, and object body byte/SHA verification.
- API usage export handoff readiness now has a compact Admin API report that
  shows whether a batch has artifact coverage, a latest manifest digest, and a
  signature for that digest while keeping local signatures billing-blocked until
  production signing is wired.
- API usage export handoff readiness can now run an explicit deep verification
  mode for release/warehouse checks, returning artifact, digest, and signature
  verification evidence plus `verified_billing_ready` without turning the
  default readiness read into an object-store streaming operation.
- Attachment policy hardening: domain outbound policy can cap individual
  attachment upload sizes.

Next focus areas:

1. Add backend-specific search relevance tuning/regression fixtures and decide
   how drafts should participate in non-Postgres search.
2. Extend the quota ledger to future Drive writes and large share-link objects.
3. Wire mailbox event publication from append/flag/move/delete paths behind the
   IMAP gateway boundary.
4. Add FCM/APNs/Web Push sink adapters and invalid-token cleanup behind the push
   notification worker.
5. Add external KMS-backed signing before using API usage batches for
   invoices or hard limits.
6. Frontend planning and API contract review before webmail implementation.
