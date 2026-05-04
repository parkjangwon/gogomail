# gogomail next steps

This file is the short task handoff for future coding agents.

## Read first

Before changing code, read:

1. `AGENTS.md`
2. `docs/CURRENT_STATUS.md`
3. `docs/backend-roadmap.md`
4. `docs/backend-api-contracts.md`
5. `docs/backend-release-readiness.md`
6. `docs/openapi.yaml`
7. recent `git log --oneline`

## Immediate backend priorities

### 1. Hierarchical quota ledger

Current state:

- Mailbox quota is enforced on selected mail write/delete paths.
- Company/domain/user quota read and update APIs exist.
- Mail storage growth/delete paths atomically update company, domain, and user
  quota ledgers in one transaction.
- Attachment upload metadata creation and stale upload cleanup also reserve and
  release bytes through the same company/domain/user quota ledger.
- Admin quota usage/detail views expose remaining capacity, child allocation,
  allocatable capacity, and over-allocation indicators.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with active message rows and reserved/stored attachment rows.
- Admin API can apply operator-controlled quota reconciliation corrections with
  transaction-scoped advisory locking and affected quota-row locks.
- User quota source is tracked as `default|custom`.
- Domain quota updates can apply a new default user quota to default-following
  users while preserving custom overrides.
- ADR 0003 defines company → domain → user unified storage pool semantics.

Next:

- Extend the same ledger service to future Drive writes and large-attachment
  share-link objects.

### 2. Message threading and search

Current state:

- Messages store `thread_id`, `in_reply_to`, `rfc_message_id`.
- Thread aggregation APIs exist for `GET /api/v1/threads` and
  `GET /api/v1/threads/{id}/messages`.
- New inbound and reply/forward outbound rows inherit thread IDs from local
  `References`/`In-Reply-To`/source messages.
- Reply composition writes RFC thread headers into outgoing `.eml`.
- Mail API exposes `GET /api/v1/search` backed by a small-deployment Postgres
  FTS index over active-message metadata.
- Received-message body indexing has an asynchronous boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml`, extracts
  bounded plain text through `internal/message`, and upserts
  `message_search_documents`.
- Postgres search includes indexed received body text without changing the
  existing search response envelope.
- Search clients can opt into relevance ordering, rank scores, and bounded
  headline snippets with `sort=relevance`, `include_rank=true`, and
  `include_highlights=true`; date ordering remains the default.
- `internal/searchindex` has an OpenSearch writer adapter behind the same
  indexing interface, and `search-index-worker` can select it with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- The OpenSearch writer can bootstrap a strict message index mapping for future
  query adapter work.
- `search-index-worker` can bootstrap that mapping on startup with
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
- OpenSearch query-side groundwork can return ranked message IDs scoped to a
  user.
- `maildb` can hydrate ordered message ID search hits into active
  `MessageSummary` rows.
- `mailservice` can compose OpenSearch relevance ID hits with Postgres summary
  hydration when relevance sorting is requested.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- OpenSearch indexed documents include parsed sender and attachment presence
  fields for search-filter parity.
- OpenSearch relevance search can apply folder, from, subject, and attachment
  filters before Postgres metadata hydration.
- OpenSearch relevance search can return subject/from/body highlights in the
  existing Mail API `search_highlights` shape.
- Optional OpenSearch integration coverage can validate bootstrap, indexing,
  and folder-aware relevance search against a real backend when
  `GOGOMAIL_TEST_OPENSEARCH_URL` is set.
- OpenSearch sender/subject filters use lower-cased keyword fields for
  Postgres-like case-insensitive filtering.
- OpenSearch highlight fragments are bounded before they are mapped into Mail
  API responses, and duplicate external hit IDs are deduplicated before
  Postgres summary hydration.
- Search index worker startup logs include non-secret backend diagnostics, and
  OpenSearch calls have an explicit configurable timeout.
- Postgres and OpenSearch relevance queries now share metadata-first tuning:
  subject and sender matches are weighted above indexed body text, with
  regression tests guarding both backend query shapes.
- Draft rows remain out of `GET /api/v1/search` until an explicit draft search
  API/indexing contract is introduced; this keeps Postgres and OpenSearch
  relevance semantics aligned.

Next:

- Add an explicit draft search contract only after deciding whether drafts
  should be indexed in Postgres, OpenSearch, or a separate compose-focused path.

### 3. IMAP gateway planning

Current state:

- No IMAP protocol server exists.
- Message, folder, and flag models are IMAP-compatible by design.
- `internal/imapgw` defines native gateway DTOs, backend interfaces, mailbox
  helpers, and RFC-shaped flag mapping without a TCP listener or DB adapter.
- `imap_mailbox_state` and `imap_message_uid` migrations define durable
  UIDVALIDITY, UIDNEXT, mailbox MODSEQ, message UID, and message MODSEQ storage.
- `maildb` can ensure mailbox UID state and assign stable mailbox-local message
  UIDs transactionally.
- `maildb` can list/get folders as `internal/imapgw.Mailbox` DTOs, list mailbox
  messages as `internal/imapgw.MessageSummary` DTOs, and resolve UID-addressed
  messages to stored raw body paths.
- `mailservice` can open UID-addressed IMAP messages as raw `io.ReadCloser`
  bodies for future IMAP FETCH handling.
- `mailservice` can delegate IMAP STORE flag mutations to `maildb`, where
  `\Seen`, `\Flagged`, and `\Answered` map to persisted JSON flags and MODSEQ
  advances only for actual changes.
- `maildb` can backfill missing mailbox-local UIDs for active messages in
  bounded, stable-order batches.
- `event-worker` handles committed `mail.stored` events with an IMAP UID
  assignment handler, so newly received active messages become UID-visible
  asynchronously without adding IMAP work to the SMTP hot path.
- The IMAP `mail.stored` notification handler can publish UID-bearing
  `exists` mailbox events after UID assignment when a process-local mailbox
  event publisher is wired.
- Mail API move/delete paths remove stale IMAP UID rows transactionally so moved
  messages can receive fresh mailbox-local UIDs later.
- Optional PostgreSQL integration tests cover IMAP UID backfill and move
  invalidation when a test database URL is configured.
- `internal/imapgw` includes an in-memory mailbox event broker that future IDLE
  sessions can subscribe to without blocking write paths.
- `mailservice.StoreIMAPFlags` can publish mailbox `flags` events through the
  broker boundary after repository flag mutations succeed.
- Mail API single and bulk flag mutations can publish mailbox `flags` events
  for messages that already have IMAP UID rows.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages.
- `mailservice` exposes IMAP mailbox/message listing and event subscription
  methods for a future protocol listener.
- Admin API exposes bounded IMAP UID backfill for future operator/bootstrap
  modes without enabling an IMAP protocol listener.
- IMAP mailbox event publication is best-effort after successful mutations, so
  future IDLE fan-out cannot make committed mail writes appear failed.
- `mailservice.IMAPStoreAdapter` satisfies `imapgw.Store` for future protocol
  listener wiring through the service boundary.

Next:

- Wire a process-local mailbox event broker into the future IMAP listener mode
  so `mail.stored` UID assignment can wake connected IDLE sessions.
- Plan IMAP IDLE support over the mailbox event broker for push-on-connect
  clients.
- Keep IMAP as a separate binary mode (`--mode=imap`).

### 4. Pipeline extension hooks

Current state:

- SMTP pipeline defines stages/hooks but they are not fully pluggable.
- Attachment scan hook exists as a disabled-by-default synchronous SMTP-stage
  adapter, and `GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook` wires a bounded HTTP
  scanner with an optional bounded bearer token into Edge, Inbound, and
  Submission MTA app boundaries. `docs/webhook-integrations.md` records the
  scanner JSON payload, bounded request/response behavior, and verdict
  semantics.
- Push notification enqueue now has a disabled-by-default async
  `push-notification-worker` over `mail.stored` with a replaceable sink and
  `slog` first adapter plus `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook` for
  handing raw-token targets to an external push gateway with an optional
  bounded bearer token; webhook URLs must be HTTPS in production.
  `docs/webhook-integrations.md` records the push gateway payload and attempt
  state semantics. Target resolution drops blank, CR/LF-bearing, oversized, or
  unsupported targets before candidate recording, and the webhook sink bounds
  direct-call payload metadata before JSON serialization.
- Admin push-notification attempt/stats repository filters reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- User-scoped push device storage now exists for `apns`, `fcm`, and `webpush`
  tokens through the Mail API. Responses expose only a token suffix; raw tokens
  remain write-only, and create/update rejects invalid-UTF-8, unsafe, or
  oversized user/token metadata before repository upsert.
- The worker can resolve active user devices from Postgres with
  `GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT`, then pass those targets to the
  sink without coupling SMTP or storage writes to vendor delivery. Malformed
  resolved targets with blank or CR/LF-bearing device IDs/tokens, or
  unsupported platforms, are dropped before sink handoff.
- The worker records per-device candidate attempts to
  `push_notification_attempts` before sink handoff, then marks those attempts
  `queued` only after the sink succeeds. Failed sink handoffs are marked
  `failed` with the sink error before the handler returns an error for stream
  retry.
- Admin API exposes `GET /admin/v1/push-notification-attempts` with bounded
  message/status/user/platform/device/provider-status/provider-message/since
  filters for inspecting candidate fan-out and vendor outcomes.
- Admin API exposes `GET /admin/v1/push-notification-attempts/{id}` for
  single-attempt troubleshooting.
- Admin API exposes
  `PATCH /admin/v1/push-notification-attempts/{id}/outcome` so authenticated
  operators or external push gateways can record queued/delivered/failed/
  invalid-token outcomes with bounded provider diagnostics.
- Admin API exposes `GET /admin/v1/push-notification-stats` for active-device
  and status-count summaries, with optional `message_id`, `user_id`, and
  `platform`/`device_id`/`since` scoping for per-message, per-user,
  provider-platform, per-device, and recent-window troubleshooting.
- Candidate recording returns an attempt id to the worker sink, giving future
  vendor adapters a stable row to update with delivered/failed/invalid-token
  outcomes.
- Candidate and provider-outcome diagnostics are capped at UTF-8 boundaries
  before storage so internationalized subjects and vendor messages remain valid
  in Admin API views.
- Push notification candidate recording rejects invalid-UTF-8, CR/LF-bearing,
  or oversized message/user/device/company/domain IDs before SQL insert
  dispatch, and rejects unsupported platforms at the recorder boundary.
- Existing attempts can be updated with queued, delivered, failed, or
  invalid-token outcomes through the internal recorder or the Admin API.
- Internal push worker outcome updates and authenticated Admin outcome updates
  share the same `maildb` storage path, reducing drift before vendor gateway
  callbacks are wired more deeply.
- Push notification outcome recording rejects invalid-UTF-8, CR/LF-bearing, or
  oversized attempt IDs before SQL update dispatch.
- Invalid-token outcomes soft-delete the matching user push device in the same
  Postgres transaction as the attempt update.
- `mail.stored` event payloads carry an explicit schema version; preserve this
  contract when adding fields for audit, search, push, IMAP, or future fan-out
  workers.
- Audit, search, and push consumers enforce known explicit schema versions; add
  a new accepted version before introducing incompatible event payload changes.
- Spam and vendor FCM/APNs delivery are not wired.

Next:

- Add first-party FCM/APNs/Web Push sink adapters behind `internal/pushnotify`
  when provider credentials and deployment expectations are decided.
- Use the authenticated Admin outcome endpoint for external push gateway
  callbacks until first-party provider adapters are added.
- Keep hooks disabled by default and wired only in `app/run.go`.

### 5. Attachment upload API

Current state:

- Attachment table and storage model exist.
- Attachment endpoints exist in the Mail API.
- Domain outbound policy can enforce `max_attachment_bytes` for attachment
  metadata reservation and direct multipart upload before storage writes.
- Mail API can cancel a pending user-scoped upload immediately, releasing
  reserved quota and deleting any stored upload object.
- Mail API exposes attachment upload capabilities so clients can discover
  current limits and supported modes without hard-coding them.
- ADR 0007 defines the resumable/chunked attachment upload boundary: explicit
  upload sessions, quota reservation at session creation, adapter-owned staged
  chunks, normal attachment rows after finalization, and bounded cleanup.
- `attachment_upload_sessions` migration defines the future resumable upload
  session state table, including declared/received byte counts, lifecycle
  status, expiry, storage adapter metadata, and cleanup-oriented indexes.
- `maildb` can create a resumable upload session record and reserve the
  declared size in the shared quota ledger in one transaction.
- `maildb` can cancel pending/uploading/failed upload sessions, marking them
  `canceled` and releasing the declared size without allowing duplicate quota
  release on repeated cancellation.
- `maildb` can expire stale pending/uploading/failed upload sessions in bounded
  batches, marking them `expired` and releasing declared quota reservations.
- `maildb` can count stale pending/uploading/failed upload sessions under the
  same normalized cleanup cap, enabling non-destructive operator previews.
- `mailservice` exposes resumable upload session create/cancel/expire methods
  over the repository boundary, reusing attachment metadata validation,
  max-size checks, and domain outbound attachment policy enforcement.
- Stale upload cleanup can run as `attachment-cleanup-worker` with configurable
  interval, stale age, batch size, and optional run-once mode for CronJob-style
  deployments, and now expires stale resumable upload sessions in the same
  bounded sweep.
- Mail API exposes upload session create/read/cancel endpoints, reserving declared
  quota for future resumable workflows without yet advertising chunk support.
- Upload session creation rejects already-expired or overlong `expires_at`
  values before quota reservation.
- Attachment upload capabilities advertise session create/cancel support
  separately from `resumable_chunked_uploads` and include the max session TTL.
- Upload session body storage can persist a complete body and checksum without
  finalizing it into an attachment row.
- Upload session body storage can reject checksum mismatches when clients send
  `X-Content-SHA256`.
- Attachment upload capabilities advertise checksum precondition support for
  upload session body storage.
- Upload session finalization can convert a stored session body into the normal
  pending attachment row while preserving the original quota reservation.
- Upload session finalization verifies staged object size and SHA-256 before
  creating the attachment row.
- Upload session cancellation deletes any staged session body after the
  repository marks the session canceled and releases quota.
- Upload session expiry deletes staged session bodies for expired sessions,
  keeping worker-driven cleanup aligned with quota release.
- Admin API can preview counts, list bounded candidates, and run stale upload
  cleanup on demand with an explicit non-future cutoff for operator-controlled
  maintenance. Cleanup run/dry-run responses include stale upload-session
  candidate and expired counts, matching the background worker's full cleanup
  scope.

Next:

- Decide whether to split body storage into explicit range-aware chunk commits,
  then flip `resumable_chunked_uploads` only after the retry/range semantics are
  complete.

### 6. OpenAPI/client readiness

Current state:

- Route, request body, response envelope, operationId, and component reference
  drift tests all pass.
- All schemas synchronized with Go types after platform hardening sprint.

Next:

- Keep `docs/openapi.yaml` synchronized with every HTTP route change.
- Consider generating a TypeScript client from the OpenAPI spec for future
  frontend use.

### 7. Frontend planning

Before creating or substantially implementing frontend apps, explicitly ask the
user for frontend-specific guidance.

### 8. API metering

Current state:

- Product direction is agreed: collect API usage from the beginning, keep
  billing/rate-limit enforcement policy-driven and off by default.
- A disabled-by-default API metering middleware boundary exists with async,
  fail-open event capture, a `slog` sink, and a durable outbox sink.
- A disabled-by-default `api-metering-worker` can consume `api.usage` events
  from `api.event`, write Postgres daily/monthly aggregates, and serve them
  through the Admin API.
- API usage events include an explicit schema version and deterministic
  `event_id`, preparing future idempotent accounting without making aggregates
  billing-grade yet.
- `api_usage_events` records claimed event IDs before aggregate upserts, so
  replayed usage events do not double-count daily/monthly operational totals.
- New `2026-05-04.api-usage.v2` usage events carry
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  idempotency ledger persists those dimensions, and daily/monthly aggregate
  primary keys include them so cross-tenant or cross-principal usage does not
  merge.
- Mail API metering can enrich identity from JWT claims, while Admin API
  metering classifies configured admin-token access through trimmed SHA-256
  digest comparison without coupling `internal/apimeter` to `internal/auth`.
- The worker records immutable `api_usage_ledger` rows before updating
  aggregate read models. Admin API exposes bounded ledger list, NDJSON export,
  and stats endpoints for export sanity checks without making request handling
  synchronous.
- Admin API exposes read-only API usage ledger retention readiness. Operators
  provide a non-future cutoff and optional tenant/principal filters, then
  receive candidate counts plus the covering completed export batch, artifact,
  digest, signature, and late-recorded-row evidence before any future
  archive/delete job can safely run.
- Admin API can create and list API usage export batch manifests, fetch a saved
  manifest by ID, and replay that manifest window as NDJSON. Batch manifests fix
  the filtered ledger totals used for downstream billing/warehouse jobs.
- Admin API can register and list external export artifacts for each batch,
  including object key, SHA-256, byte count, event count, and metadata. Artifact
  rows are deduplicated per batch by object key and SHA-256.
- Admin API can write a full API usage export batch to local object storage,
  register the resulting artifact metadata idempotently, clean up failed writes
  when the store supports delete, and download or verify stored NDJSON artifacts.
- Admin API can create/list/get canonical export manifest digests and verify a
  stored manifest digest. This gives operators a vendor-neutral integrity check
  over the saved batch plus registered artifact metadata before external
  signing, billing, or warehouse handoff.
- Admin API can create/list/get local-HMAC, local-Ed25519, or remote-Ed25519
  signatures for manifest digests and verify persisted signatures. The signer is
  disabled by default. HMAC uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=local-hmac`,
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID`, and
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET`; Ed25519 uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=local-ed25519` plus
  base64 raw Ed25519 private/public key env vars. Remote Ed25519 uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=remote-ed25519`,
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_URL`, optional
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN`, and the base64 raw public
  key. All signers sign the lowercase 64-character manifest digest hex string,
  and remote signatures are verified locally before they are stored.
- Admin API can report API usage export handoff readiness for a saved batch,
  summarizing artifact event coverage, latest manifest digest, latest digest
  signature, operational readiness, and separate billing readiness. Locally
  signed batches remain `billing_ready=false` with
  `production_manifest_signer_required`.
- Passing `deep=true` to the handoff readiness endpoint streams all registered
  artifacts from object storage, verifies their byte/SHA metadata, verifies the
  latest manifest digest, checks that the digest still covers current artifact
  metadata, and verifies the latest signature when a verifier is available.
  Deep mode returns `verified_billing_ready` separately so `billing_ready`
  remains a stable metadata/signer-eligibility signal.
- Manifest signature verification now goes through an
  `ExportManifestSignatureVerifier` interface. Local-HMAC and Ed25519 verifiers
  are wired today; remote Ed25519 lets an external KMS-backed signing service
  plug in without coupling gogomail to a specific vendor SDK.
- Admin API exposes API usage export capabilities, including signer backend,
  signer key ID, verifier availability, production signature readiness, and
  billing/verified-billing support flags.

Next:

- Add a concrete cloud KMS adapter, or deploy the remote-Ed25519 signer service,
  before invoices or hard Open API limits depend on completed export batches.
- Add the actual archive/delete worker for immutable API usage ledger rows after
  retention readiness is wired into an operator runbook and production storage
  target.
- Avoid synchronous writes on hot API paths.

## Do not do yet

- Do not start frontend implementation without asking the user.
- Do not build a built-in spam engine inside SMTP core.
- Do not add vendor-specific spam/filtering behavior to protocol paths.
- Do not advertise SMTP extensions before full RFC semantics exist.

## Standard finish checklist

```bash
go test ./...
go mod tidy -diff
git status --short
git push
```

Every meaningful feature should be a reviewable commit before pushing.
