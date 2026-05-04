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
  FTS index over metadata and draft text.
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
  hydration when filters/highlights do not require fallback.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- OpenSearch indexed documents include parsed sender and attachment presence
  fields for search-filter parity.

Next:

- Add OpenSearch query parity for from/subject/attachment filters and rank
  preservation before broadening Mail API usage.
- Add backend-specific relevance tuning and regression tests as the corpus grows.

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
- Mail API move/delete paths remove stale IMAP UID rows transactionally so moved
  messages can receive fresh mailbox-local UIDs later.
- Optional PostgreSQL integration tests cover IMAP UID backfill and move
  invalidation when a test database URL is configured.
- `internal/imapgw` includes an in-memory mailbox event broker that future IDLE
  sessions can subscribe to without blocking write paths.

Next:

- Wire mailbox event publication from append/flag/move/delete paths.
- Plan IMAP IDLE support over the mailbox event broker for push-on-connect
  clients.
- Keep IMAP as a separate binary mode (`--mode=imap`).

### 4. Pipeline extension hooks

Current state:

- SMTP pipeline defines stages/hooks but they are not fully pluggable.
- Attachment scan hook exists as a disabled synchronous SMTP-stage adapter.
- Push notification enqueue now has a disabled-by-default async
  `push-notification-worker` over `mail.stored` with a replaceable sink and
  `slog` first adapter.
- User-scoped push device storage now exists for `apns`, `fcm`, and `webpush`
  tokens through the Mail API. Responses expose only a token suffix; raw tokens
  remain write-only.
- The worker can resolve active user devices from Postgres with
  `GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT`, then pass those targets to the
  sink without coupling SMTP or storage writes to vendor delivery.
- The worker records per-device candidate attempts to
  `push_notification_attempts` after sink enqueue succeeds, giving operators a
  trace before vendor adapters exist.
- Admin API exposes `GET /admin/v1/push-notification-attempts` with bounded
  status/user filters for inspecting candidate fan-out.
- Admin API exposes `GET /admin/v1/push-notification-stats` for active-device
  and status-count summaries.
- Candidate recording returns an attempt id to the worker sink, giving future
  vendor adapters a stable row to update with delivered/failed/invalid-token
  outcomes.
- `internal/pushnotify.PostgresRecorder` can update an existing attempt with
  queued, delivered, failed, or invalid-token outcomes.
- Invalid-token outcomes soft-delete the matching user push device in the same
  Postgres transaction as the attempt update.
- `mail.stored` event payloads carry an explicit schema version; preserve this
  contract when adding fields for audit, search, push, IMAP, or future fan-out
  workers.
- Audit, search, and push consumers enforce known explicit schema versions; add
  a new accepted version before introducing incompatible event payload changes.
- Spam and vendor FCM/APNs delivery are not wired.

Next:

- Add FCM/APNs/Web Push sink adapters behind `internal/pushnotify`.
- Extend candidate attempts with vendor delivery outcomes and invalid-token
  cleanup.
- Wire attachment scanning only when a concrete scanner backend is configured.
- Keep hooks disabled by default and wired only in `app/run.go`.

### 5. Attachment upload API

Current state:

- Attachment table and storage model exist.
- Attachment endpoints exist in the Mail API.
- Domain outbound policy can enforce `max_attachment_bytes` for attachment
  metadata reservation and direct multipart upload before storage writes.

Next:

- Add multipart upload support for large attachments.
- Add resumable/chunked upload contracts for large attachment workflows.

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

Next:

- Add async enrichment keyed by company/domain/user/api-key.
- Add billing-grade ledger semantics before aggregates drive invoices or hard
  Open API limits.
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
