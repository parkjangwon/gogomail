# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Mail API exposes cursor-paginated thread list and thread-message read models for conversation-style webmail rendering, and draft search uses the same opaque cursor envelope for compose-scale draft lists.
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
  OpenSearch endpoint configuration is validated as an HTTP(S) URL with a host
  during startup config validation, so malformed search backend endpoints fail
  before worker/search adapter construction.
  OpenSearch index names are also validated during startup config validation
  using the same unsafe-character and reserved-prefix guardrails as the
  adapter, so invalid index configuration fails before worker/search setup.
  OpenSearch writer/searcher construction trims usernames while preserving
  password bytes, and rejects CR/LF-bearing or oversized endpoint credentials
  before BasicAuth request headers can be generated.
  OpenSearch username/password configuration is also CR/LF-rejected and
  size-bounded during startup config validation when the OpenSearch backend is
  selected, surfacing credential formatting mistakes before worker/search setup.
  OpenSearch writer construction rejects CR/LF-bearing direct endpoint values
  before URL parsing, keeping adapter calls aligned with startup config endpoint
  validation.
  OpenSearch index/bootstrap/search status-error diagnostics collapse backend
  response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing
  backend errors from leaking into logs or API diagnostics.
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
- Upload session body storage rejects repeated `Content-Range` or
  `X-Content-SHA256` control headers before body storage begins.
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
- Detail reads mark unread messages as read while avoiding redundant writes for already-read messages, and successful auto-read mutations publish best-effort IMAP `flags` events for UID-visible messages.
- Compose and draft validation guard user id, intent/source rules, recipient presence, recipient email syntax, recipient count, subject size, text body size, attachment IDs, filename safety, MIME type, upload size, and outbound RFC 5322 header injection values.
- Mail API path identifiers and direct-upload `draft_id` form values are trimmed
  at the HTTP boundary before service dispatch, and direct multipart uploads
  reject repeated `draft_id` or `file` parts before storage work begins.
- Mail and Admin API JSON request bodies reject trailing JSON tokens and
  unknown object fields instead of accepting drifted payloads, and shared JSON
  decoding is capped at 1 MiB before parsing.
- Mail and Admin API JSON mutation bodies require `Content-Type:
  application/json` exactly once, while allowing normal media-type parameters
  such as `charset=utf-8`.
- Mail API read and bodyless mutation routes reject request bodies and
  `Content-Type` headers before dispatch, preventing ignored JSON or multipart
  metadata on resource reads, deletes, draft-send, upload-session finalization,
  capability discovery, downloads, and push-device list/delete operations.
- Admin GET/DELETE routes and bodyless Admin POST commands reject request
  bodies and `Content-Type` headers before dispatch, preventing ignored payloads
  on operator reads, deletes, route verification, retry, IMAP UID backfill,
  API-usage export-batch creation, and manifest digest/signature creation.
- Health and service-info GET routes reject request bodies and `Content-Type`
  headers before returning probe or contract metadata responses, keeping
  unauthenticated release probes aligned with bodyless read semantics.
- Health and service-info GET routes reject unknown query parameter names, so
  release probe and metadata endpoint typos fail visibly instead of being
  ignored.
- Admin bodyless command/delete routes for IMAP UID backfill, DKIM DNS verify,
  outbox retry, DKIM deactivation, suppression deletion, trusted-relay deletion,
  and delivery-route deletion reject unknown query parameter names before
  dispatch, preventing ignored `dry_run`/`force`-style operator flags.
- Admin JSON mutation routes for tenant quotas, domain/user lifecycle and
  policy, backpressure, attachment cleanup, quota correction, push outcomes,
  trusted relays, delivery routes, and DKIM keys reject unknown query parameter
  names before dispatch.
- Mail JWT and Admin token authentication reject repeated credential headers,
  and Admin routes reject mixed `X-Admin-Token` plus bearer credentials before
  dispatch.
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
- Successful Mail/Admin JSON, health, and service-info responses return
  `X-Content-Type-Options: nosniff`, keeping browser-visible envelopes aligned
  with error, NDJSON, and download hardening.
- Successful Mail/Admin JSON envelopes return `Cache-Control: no-store` through
  the shared writer so message, audit, usage, and admin-control responses are
  not cached by browsers or intermediaries.
- Service info exposes API and backend contract version metadata; readiness exposes a structured checks envelope.
- Readiness checks now include contract/storage/outbox boundary metadata and
  runtime-injected database/Redis probes for HTTP modes that depend on those
  services, returning a degraded 503 response when a required dependency probe
  fails.
- Mail/Admin HTTP readiness now includes a real configured-storage
  write/read/delete probe, and unsupported HTTP storage backends fail fast
  instead of silently falling back to local storage wiring.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, and HTTP runtimes now share storage backend validation and factory
  wiring for local filesystem/NFS-style storage plus S3-compatible object
  storage. `GOGOMAIL_STORAGE_BACKEND=s3` can target AWS S3, while
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same S3-compatible adapter with
  path-style requests for local MinIO-style deployments. Both paths use endpoint,
  region, bucket, prefix, credential, and session-token settings.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, and the development compose stack includes `minio-init` to
  create the default local `gogomail` bucket.
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
- The shared audit writer now bounds audit scalar metadata and JSON detail size
  before hash computation or database insertion, keeping every audit producer on
  the same persistence guardrails.
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
- IMAP has a backend gateway boundary package with native DTOs/interfaces,
  mailbox state helpers, RFC-shaped flag mapping, and a bounded protocol server
  shell for the first authenticated mailbox/read commands.
- IMAP UID storage has durable mailbox UIDVALIDITY/UIDNEXT/highest-MODSEQ rows
  and message UID/MODSEQ rows, with transactional assignment helpers, first
  mailbox/message list adapters, raw body fetch groundwork, RFC-correct sequence
  numbers for `UID FETCH`/`UID STORE` untagged `FETCH` responses, MODSEQ-aware
  flag mutation, bounded UID backfill, move/delete UID invalidation, and
  same-active-mailbox idempotency checks.
- `mailservice` now exposes IMAP mailbox/message listing, raw fetch, flag store,
  COPY, MOVE, EXPUNGE, UID backfill, and mailbox-event subscription through service
  methods plus an `IMAPStoreAdapter` satisfying `imapgw.Store`, keeping
  protocol wiring off direct `maildb` internals.
- `IMAPStoreAdapter` now satisfies `imapgw.MailboxSessionStore` for mailbox
  selection, service-backed COPY/MOVE/EXPUNGE, and event subscription.
- Admin API exposes bounded IMAP UID backfill by user/mailbox for future
  operator/bootstrap runs without enabling an IMAP protocol listener.
- IMAP IDLE remains out of scope, but `internal/imapgw` now has an in-memory
  mailbox event broker for future session fan-out. The broker is scoped by
  user+mailbox, and service-side flag/move/delete mutations publish best-effort
  `flags`/`expunge` events for UID-visible messages. Mail API detail reads
  that auto-mark unread messages as read also publish `flags` events after a
  successful read-flag write.
- `gogomail --mode=imap` now starts an IMAP gateway that wires the
  service-backed IMAP store adapter, process-local mailbox event broker, and
  configured TCP IMAP listener.
- Runtime config now loads and validates `GOGOMAIL_IMAP_ADDR` as required TCP
  listener metadata for the protocol listener.
- Runtime config also loads and validates IMAP TLS certificate/key paths plus
  `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH`, preventing production IMAP auth from
  being enabled with cleartext credential policy.
- IMAP runtime TLS helper groundwork can load IMAP-specific certificate/key
  files with TLS 1.2 minimum and derive the server name from the IMAP listener
  host before falling back to `GOGOMAIL_SMTP_DOMAIN`.
- ADR 0008 records the IMAP authentication/session contract: protocol auth uses
  a dedicated adapter over local user password hashes, JWT remains HTTP-only,
  production auth requires TLS policy review, `\Deleted` is a protocol flag
  separate from gogomail soft-delete status, and MOVE is modeled as source
  expunge semantics plus a destination folder transition with fresh mailbox UIDs.
- `mailservice.NewIMAPAuthenticatorAdapter` maps the existing Submission/local
  password authentication boundary into `imapgw.Session` values, giving the
  listener a protocol-native authenticator without coupling IMAP to JWT
  middleware.
- `mailservice.NewIMAPBackendAdapter` composes the protocol authenticator with
  the service-backed store/session adapter, so the TCP listener can take one
  `imapgw.Backend` boundary.
- IMAP runtime now builds server options containing address, backend, TLS
  config, and insecure-auth policy for the TCP protocol server.
- `internal/imapgw.NewServer` now provides a protocol-server lifecycle shell
  with listener option validation, backend requirement checks, and TLS/insecure
  auth policy enforcement before the IMAP command parser is wired.
- The IMAP server shell can serve an initial connection greeting plus
  unauthenticated `CAPABILITY`, `NOOP`, `LOGIN`, and `LOGOUT` responses, giving
  TCP clients a bounded RFC-shaped handshake/auth surface before mailbox
  commands are enabled.
- Authenticated IMAP `SELECT` now maps to `imapgw.MailboxSessionStore`, returning
  permanent flags, `EXISTS`, `UIDVALIDITY`, `UIDNEXT`, and read-write completion
  metadata from the service-backed mailbox state.
- Authenticated IMAP `LIST` now maps to the service-backed mailbox list and
  returns sanitized quoted mailbox names with hierarchy delimiters.
- Authenticated IMAP `STATUS` now maps to service-backed mailbox state and
  returns `MESSAGES`, `UIDNEXT`, `UIDVALIDITY`, and `UNSEEN` metadata.
- IMAP command parsing now supports basic quoted strings with backslash escapes,
  allowing common quoted `LOGIN` credentials and mailbox atoms while rejecting
  malformed quoted controls and unsupported non-synchronizing literal tokens.
  Bounded synchronizing command literals are consumed with a continuation
  response so future `APPEND` support can preserve connection framing.
- IMAP `CAPABILITY` now advertises `AUTH=PLAIN` only before authentication,
  aligning the first command surface with RFC client state expectations.
- IMAP `AUTHENTICATE PLAIN` now supports the standard continuation response and
  SASL PLAIN credential decoding, so the advertised `AUTH=PLAIN` mechanism has
  a real protocol implementation.
- IMAP advertises `SASL-IR` before authentication and accepts
  `AUTHENTICATE PLAIN` initial responses to reduce compatible client auth
  round trips.
- Authenticated selected-mailbox `UID FETCH` can now return UID, flags,
  RFC822 size metadata, and `BODY[]` literals streamed from the service-backed
  raw message fetch boundary. Untagged `FETCH` responses use IMAP sequence
  numbers, and `RFC822.SIZE` alone is not treated as a body-fetch request.
- `UID FETCH` now accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]`, improving compatibility with clients that batch mailbox reads
  and avoid implicit read-flag side effects.
- Non-UID `FETCH` now accepts bounded sequence sets, including `*`, and resolves
  them through the selected mailbox list before returning RFC-shaped untagged
  `FETCH` responses.
- IMAP `EXAMINE` now exposes read-only mailbox selection and rejects `UID STORE`
  while the selected mailbox is read-only.
- IMAP `EXAMINE` now passes read-only selection intent through the backend
  `SelectMailboxRequest`, letting service adapters distinguish read-only
  sessions from writable `SELECT`.
- IMAP `SELECT`/`EXAMINE` now establish mailbox event subscriptions before
  emitting selected-mailbox response data, avoiding ambiguous partial selection
  state when subscription setup fails.
- IMAP `CHECK` and `CLOSE` now provide safe selected-mailbox lifecycle handling,
  with `CLOSE` silently expunging `\Deleted` messages for writable selections
  before clearing selected state, while read-only selections only clear state.
- IMAP `STATUS` now validates requested status data items and emits only those
  requested fields, including `RECENT`.
- IMAP mailbox lookup now resolves wire names such as `INBOX` and
  `Archive/2026` to the stored mailbox ID before selected-mailbox state is used
  by follow-up commands.
- IMAP `LIST` now applies exact, `*`, and `%` mailbox pattern matching before
  returning sanitized quoted mailbox names.
- IMAP `CAPABILITY` now advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`;
  `LIST` includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy
  attributes plus RFC 6154 special-use attributes for system folders such as
  Drafts, Sent, Trash, Junk, Archive, All, and Flagged when those folder roles
  are present in storage metadata, and extended
  `LIST (SPECIAL-USE)` / `RETURN (SPECIAL-USE)` forms are accepted.
- IMAP `CAPABILITY` now advertises RFC 5819 `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits requested `STATUS` metadata
  immediately after each matching selectable mailbox so compatible clients can
  avoid per-folder `STATUS` round trips.
- IMAP `SELECT`/`EXAMINE` now emit `[PERMANENTFLAGS]` response codes for
  writable versus read-only selected-mailbox state.
- IMAP `SELECT`/`EXAMINE` now emit RFC-shaped untagged `RECENT` counts
  alongside `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints,
  `UIDVALIDITY`, `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from
  durable mailbox UID state.
- IMAP `UID STORE` now supports `.SILENT` mutation modes while applying the same
  flag changes through the service-backed flag boundary.
- IMAP `FETCH`/`UID FETCH` now include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes when requested, enabling standard mailbox list metadata reads.
- IMAP `CAPABILITY` now advertises `CONDSTORE` and `ENABLE` after the RFC
  4551-shaped mod-sequence fetch/search/status/select/store paths were wired
  through durable mailbox/message state; RFC 5161-shaped `ENABLE CONDSTORE`
  marks sessions CONDSTORE-aware before mailbox selection.
- IMAP `FETCH`/`UID FETCH` now include RFC 4551-shaped `MODSEQ (n)` attributes
  when requested, surfacing durable per-message mod-sequences.
- IMAP `SEARCH`/`UID SEARCH` now support RFC 4551-shaped `MODSEQ` criteria and
  append the highest matched mod-sequence to non-empty SEARCH responses.
- IMAP `FETCH`/`UID FETCH` now support RFC 4551-shaped `CHANGEDSINCE`
  modifiers, filtering responses to messages with greater per-message
  mod-sequences and implicitly returning `MODSEQ` attributes.
- IMAP sessions now become CONDSTORE-aware after implemented mod-sequence
  enabling commands, causing subsequent flag `FETCH` event/STORE echo
  responses to include `MODSEQ` attributes.
- IMAP `STORE`/`UID STORE` now supports RFC 4551-shaped `(UNCHANGEDSINCE n)`
  modifiers, checking message mod-sequences transactionally, applying passing
  updates, and returning `[MODIFIED uid-set]` / `[MODIFIED sequence-set]` for
  stale messages. Conditional store response/event paths filter modified stale
  UIDs out of successful `FETCH` echoes and mailbox flag notifications.
- IMAP `SELECT`/`EXAMINE` now accept the RFC 4551-shaped `(CONDSTORE)`
  parameter and mark the session CONDSTORE-aware.
- IMAP `FETCH`/`UID FETCH` now return a conservative single-part
  `BODYSTRUCTURE` response for clients that require structure metadata before
  fetching message bodies.
- IMAP single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
  parameters, content-transfer-encoding, ID, and description from bounded raw
  message headers instead of always reporting text/plain defaults.
- IMAP metadata-only `BODYSTRUCTURE` fetches now use the streaming
  MIME-structure parser to return multipart child order, subtype, parameters,
  transfer encodings, dispositions, body octets, and text line counts without
  retaining attachment payloads.
- IMAP combined `BODYSTRUCTURE` plus literal body/header fetches can reopen the
  raw message for MIME metadata while preserving the original reader for
  literal streaming, so common preview/header fetch batches keep rich structure
  responses.
- IMAP `FETCH`/`UID FETCH` now supports standard `FAST`, `ALL`, and `FULL`
  macros, including the non-extensible `BODY` attribute for `FULL`.
- IMAP `FETCH`/`UID FETCH` now support bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- IMAP non-UID `FETCH` now uses the same bounded header literal path as
  `UID FETCH` for `BODY[HEADER]` and `RFC822.HEADER`.
- IMAP `FETCH`/`UID FETCH` now support `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and
  `RFC822.TEXT` section literals without returning the message headers.
- IMAP `FETCH`/`UID FETCH` now supports conservative single-part text literals
  for `BODY[1]` and `BODY.PEEK[1]`.
- IMAP `FETCH`/`UID FETCH` now answers conservative single-part MIME header
  requests for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
- IMAP `UID STORE` now accepts bounded UID sets/ranges and returns per-message
  flag updates unless `.SILENT` is requested.
- IMAP non-UID `STORE` now accepts bounded sequence sets/ranges and maps them
  to the same service-backed flag mutation boundary as `UID STORE`.
- IMAP non-UID `STORE` now supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- IMAP `NOOP` now drains queued mailbox events into untagged `EXISTS`,
  `EXPUNGE`, and flag `FETCH` updates for selected mailboxes, suppressing
  stale or duplicate exact-count `EXISTS` events.
- Mail API move/delete expunge notifications now carry mailbox sequence numbers
  from IMAP UID lookup, allowing selected `NOOP`/`IDLE` clients to receive
  renderable untagged `EXPUNGE` updates.
- IMAP now advertises and accepts `IDLE`, entering continuation mode and
  streaming selected-mailbox events while the client waits for `DONE`.
- IMAP `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` now return
  selected-mailbox sequence numbers or UIDs for basic client indexing flows.
- IMAP `SEARCH`/`UID SEARCH` now accepts sequence-set criteria such as `2:*`,
  letting clients intersect standard search predicates with selected mailbox
  sequence ranges.
- IMAP `SEARCH`/`UID SEARCH` can combine supported criteria with RFC default
  AND semantics, including `ALL` plus flag, date, size, address, and UID
  filters.
- IMAP `SEARCH`/`UID SEARCH` supports RFC `NOT` and binary `OR` criteria
  composition over the supported search predicate set.
- IMAP `SEARCH`/`UID SEARCH` now accepts parenthesized search-key groups,
  combining grouped predicates with RFC default AND semantics and allowing
  grouped operands inside `OR`.
- IMAP `FETCH`/`UID FETCH` now streams bounded partial full-body literals for
  `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`.
- IMAP `FETCH`/`UID FETCH` now streams bounded partial section literals for
  common `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]` requests.
- IMAP `FETCH`/`UID FETCH` now streams bounded top-level multipart
  body-section literals such as `BODY[1]` and `BODY[2]`, letting clients read
  individual MIME parts without fetching the full message.
- IMAP `FETCH`/`UID FETCH` now streams bounded nested multipart body-section
  literals such as `BODY[1.2]` with a capped MIME part path depth.
- IMAP `FETCH`/`UID FETCH` now streams bounded partial windows over multipart
  body-section literals such as `BODY.PEEK[2]<4.4>`.
- IMAP `FETCH`/`UID FETCH` now streams actual multipart child MIME headers for
  `BODY[n.MIME]` and `BODY.PEEK[n.MIME]` when the selected part exists.
- IMAP `SEARCH`/`UID SEARCH` now support common flag criteria including
  `SEEN`, `UNSEEN`, `FLAGGED`, `UNFLAGGED`, `ANSWERED`, `UNANSWERED`,
  `DRAFT`, and `UNDRAFT`.
- IMAP `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag
  separately from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose
  that flag through `FLAGS`, `DELETED`, and `UNDELETED`.
- IMAP `SEARCH`/`UID SEARCH` now supports `RECENT`, `OLD`, and `NEW`, returning
  no recent/new matches while durable recent-state semantics remain deferred and
  treating active messages as old.
- IMAP `SEARCH`/`UID SEARCH` now supports `KEYWORD` and `UNKEYWORD` criteria
  with validated keyword atoms, returning no custom-keyword matches until
  durable user keyword storage exists and treating active messages as
  unkeyworded.
- IMAP `FETCH`/`UID FETCH` now supports bounded `BODY[HEADER.FIELDS (...)]`
  and `BODY.PEEK[HEADER.FIELDS (...)]` literals for lightweight header reads.
- IMAP `FETCH`/`UID FETCH` now supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  literals.
- IMAP `FETCH`/`UID FETCH` now supports bounded
  `BODY[HEADER.FIELDS.NOT (...)]` and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  literals for exclude-style header reads.
- IMAP `SEARCH`/`UID SEARCH` now supports `SINCE`, `BEFORE`, and `ON` over
  message `INTERNALDATE`, plus `SENTSINCE`, `SENTBEFORE`, and `SENTON` over
  envelope dates.
- IMAP `SEARCH`/`UID SEARCH` now supports basic `FROM`, `TO`, `CC`, `BCC`,
  and `SUBJECT` substring criteria over selected-mailbox summaries.
- IMAP `SEARCH`/`UID SEARCH` now supports bounded `BODY` and `TEXT`
  raw-message criteria scans, with `BODY` excluding the RFC 5322 header block.
- IMAP `SEARCH`/`UID SEARCH` now supports bounded RFC
  `HEADER <field> <value>` criteria scans over the raw message header block.
- IMAP `SEARCH`/`UID SEARCH` now supports RFC 3501 `LARGER` and `SMALLER`
  criteria over message `RFC822.SIZE` metadata.
- IMAP `SEARCH`/`UID SEARCH` now accepts `CHARSET US-ASCII` and
  `CHARSET UTF-8` prefixes and returns an RFC-shaped `[BADCHARSET]` response
  for unsupported search charsets.
- IMAP now supports authenticated `NAMESPACE`, exposing the personal namespace
  and `/` hierarchy delimiter.
- IMAP `CAPABILITY` now advertises `NAMESPACE` alongside the implemented
  namespace command so client discovery matches the supported command surface.
- IMAP now persists authenticated `SUBSCRIBE`/`UNSUBSCRIBE` mailbox
  subscriptions and returns the saved set from `LSUB` instead of every visible
  mailbox.
- IMAP `LIST "" ""` and `LSUB "" ""` now return the hierarchy root with
  `\Noselect` and `/` delimiter metadata for clients that probe namespace
  delimiters through LIST-compatible commands.
- IMAP `LSUB` retains subscribed names after mailbox deletion with `\Noselect`
  and covers the RFC 3501 `%` hierarchy parent response case.
- IMAP now advertises and supports RFC 2971 `ID`, validating `NIL` or bounded
  field/value parameter lists before returning a bounded server identity
  response for compatibility diagnostics.
- IMAP now advertises and supports `UNSELECT`, clearing selected-mailbox state
  and event subscriptions without invoking `CLOSE`/EXPUNGE semantics.
- IMAP `EXPUNGE` and `UID EXPUNGE` now delete only messages marked with the
  IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
  `EXPUNGE` responses, remove stale mailbox UID rows, and publish best-effort
  expunge events.
- IMAP `COPY` and `UID COPY` now resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, duplicate active message
  metadata and attachment rows transactionally, assign fresh destination
  mailbox UIDs, return UIDPLUS `[COPYUID ...]` response codes when destination
  UIDs are available, return `[TRYCREATE]` when the destination mailbox is
  missing, and publish best-effort destination `EXISTS` events.
- IMAP `MOVE` and `UID MOVE` now resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, move active messages
  transactionally, assign fresh destination UIDs, and allow moves back into the
  selected mailbox by creating a fresh same-mailbox message before expunging
  the source UID. Responses return UIDPLUS `[COPYUID ...]` mappings when
  destination UIDs are available, advance and return source mailbox
  `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients, emit RFC-shaped
  source `EXPUNGE` responses,
  return `[TRYCREATE]` when the destination mailbox is missing, and publish
  best-effort source expunge events.
- IMAP `APPEND` now has a protocol-to-backend request boundary for mailbox,
  optional flag-list, optional internal date-time, literal body, and size after
  bounded literal framing. The boundary now returns UIDPLUS-ready append
  metadata so successful storage can emit `[APPENDUID uidvalidity uid]`; the
  service layer now spools and size-checks the literal body, parses the RFC
  message, writes the raw `.eml` through the configured storage backend, asks
  `maildb` to insert metadata, quota, outbox, and mailbox UID state in one
  transaction, publishes best-effort destination `EXISTS` events, and returns
  `[TRYCREATE]` when the destination mailbox is missing or `[OVERQUOTA]` when
  the quota ledger rejects the append. Commands without an RFC-shaped literal
  are rejected as syntax `BAD` responses instead of being reported as
  unsupported. Successful append results include the appended message sequence
  number, which is used as the precise `EXISTS` event count when available.
- IMAP `CREATE`, `DELETE`, and `RENAME` now delegate to the service folder
  boundary for authenticated flat user-mailbox management, resolving wire names
  before destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- IMAP now supports `STARTTLS` on plaintext listeners with configured TLS,
  advertising it before authentication and removing it after upgrade.
- IMAP `STARTTLS` completion now includes an updated `[CAPABILITY ...]`
  response code for the post-TLS command surface.
- IMAP plaintext sessions advertise `LOGINDISABLED` and reject
  `LOGIN`/`AUTHENTICATE` with `[PRIVACYREQUIRED]` when insecure auth is disabled
  before STARTTLS.
- Authenticated selected-mailbox `UID STORE` now maps `FLAGS`, `+FLAGS`, and
  `-FLAGS` for supported system flags to the service-backed flag mutation
  boundary and returns updated flag metadata.
- `gogomail --mode=imap` now opens the configured TCP listener and serves the
  IMAP server shell with greeting, `CAPABILITY`, `NOOP`, `LOGIN`, `SELECT`,
  `FETCH`/`UID FETCH`, `STORE`/`UID STORE`, `SEARCH`, `IDLE`, `STARTTLS`, and
  `LOGOUT`, while destructive mailbox mutation semantics remain deferred.
- `gogomail --mode=imap` now runs its own Redis consumer group for committed
  `mail.stored` events and publishes UID-bearing `EXISTS` updates into the
  process-local mailbox event broker for live IDLE sessions.
- IMAP listener creation now uses a TLS listener whenever IMAP TLS config is
  present, keeping the runtime listener path aligned with the authentication
  policy guardrails.
- The shared event worker now ensures IMAP UID rows for committed `mail.stored`
  receive events, moving received messages toward UID-visible state without
  coupling SMTP receive to future IMAP listener work; IMAP UID assignment event
  decoding rejects CR/LF-bearing or oversized message/user/folder IDs before
  UID work or mailbox event fan-out, and stale moved/deleted-message events are
  no-ops instead of permanent retries.
- Redis-backed event/search/API-metering/push/delivery workers reclaim idle
  pending stream messages with configurable claim-idle windows so crashed
  consumers do not strand at-least-once work indefinitely.
- Redis stream consumers move repeatedly handler-failing messages into a
  durable Redis dead-letter stream before acknowledging the original event,
  preventing one poison event from pinning a worker forever while still
  allowing normal transient retries first. Event, search-index, API-metering,
  push-notification, and delivery workers expose per-worker max-delivery and
  dead-letter-stream settings.
- Redis worker stream, group, and consumer-name settings for event,
  search-index, API-metering, push-notification, and delivery workers are
  required, CR/LF-rejected, and size-bounded during startup config validation,
  surfacing worker identity mistakes before consumer construction.
- `eventstream.NewRedisConsumer` applies the same trim, required,
  CR/LF-rejection, and size-bound guardrails to stream, group, and consumer
  identifiers, keeping direct adapter callers aligned with runtime config
  validation.
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
  address-list and `References` counts are also capped with truncation flags,
  and oversized structured subject/address/message-id-list headers are
  pre-bounded before decoding or list parsing so they cannot expand downstream
  storage and search metadata without bound.
- `internal/message` exposes a bounded streaming MIME-structure parser that
  walks multipart trees, preserves raw transfer-encoding metadata, counts body
  octets/lines, and avoids retaining attachment payloads for future IMAP
  `BODYSTRUCTURE` serialization.
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
- `GOGOMAIL_ENV` accepts only `development`, `test`, or `production`, so
  environment typos cannot silently bypass production-only safety gates.
- Redis-backed deduplication, recipient rate limiting, and SMTP backpressure
  backend selectors accept only `none` or `redis`, preventing typos from
  silently disabling operational controls.
- Redis-backed RCPT rate-limit keys normalize remote addresses to the remote
  host/IP bucket instead of the full `ip:port`, preventing source-port churn
  from bypassing recipient abuse controls.
- RCPT rate-limit and outbox relay batch, poll, and max-attempt settings are
  validated as positive values during startup config validation, surfacing
  relay/limit misconfiguration before workers start.
- HTTP, SMTP, inbound SMTP, Submission, and optional SMTPS listener addresses
  are validated as TCP `host:port` values at startup, surfacing bind
  configuration mistakes before runtime listener setup.
- Delivery retry delay schedules and maximum delay caps are validated as
  positive durations, preventing retry jobs from being exhausted accidentally
  or scheduled in the past by malformed runtime configuration.
- `GOGOMAIL_DELIVERY_SMTP_HELLO` is validated as a non-empty whitespace-free
  hostname during startup config validation, surfacing outbound SMTP EHLO
  configuration mistakes before delivery worker startup.
- Admin API can persist a domain operational policy model in `domains.settings.policy`, and Mail API send/draft-send enforces outbound recipient-count and composed-size guardrails when `outbound_mode=enforce`.
- DKIM key creation derives the public DNS TXT record from the private key when omitted, reducing operator DNS setup errors while preserving private-key omission from admin list responses.
- Admin API exposes domain DNS verification for MX, SPF, DMARC, and active DKIM TXT records, and each check is persisted with an audit log entry for domain onboarding traceability before frontend implementation.
- Delivery workers can opt into PostgreSQL-backed delivery routes through `GOGOMAIL_DELIVERY_ROUTE_BACKEND=postgres`, reusing the existing delivery router boundary and falling back to direct MX delivery when no active route matches.
- Static smart-host configuration rejects password-only auth plus CR/LF-bearing
  or oversized auth username, password, and identity values during startup
  config validation, matching the Admin delivery-route guardrails before
  delivery worker startup.
- Admin delivery-route creation rejects oversized farm, SMTP hello, pool,
  description, and relay auth identity/username/password metadata before route
  storage or audit work.
- Admin domain/user create validation rejects malformed domains, unsafe usernames, invalid ACE names, and mismatched primary address ownership.
- SMTP receive/submission paths now include TCP-level protocol integration coverage for inbound delivery, AUTH PLAIN submission, policy rejection, and SMTPS.
- Authenticated Submission applies enforcing per-domain recipient caps during
  `RCPT TO`, not only after `DATA`, so oversized envelopes receive earlier SMTP
  feedback before message streaming/spooling.
- Authentication-Results trace header formatting strips control characters and
  bounds verifier metadata before formatting SPF/DKIM/DMARC results, preventing
  DNS/library diagnostics from injecting or bloating stored trace headers.
- SMTP wire coverage now exercises enabled/disabled extension advertisement, DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` propagation including `NOTIFY=NEVER`, unsupported extension rejection, STARTTLS-gated AUTH, implicit TLS, trusted relay CIDR rejection, and repeated transactions on a single connection.
- Outbound SMTP wire coverage now verifies DSN parameters are emitted only when the remote MTA advertises DSN support, preventing accidental RFC 3461 option leakage to non-DSN peers. Outbound EAI addresses fail closed with a permanent SMTPUTF8 error when the remote MTA does not advertise SMTPUTF8.
- Outbound SMTP controlled-sink coverage now verifies accepted DATA can coexist with per-recipient permanent and temporary RCPT failures, preserving retry/bounce classification for delivery handlers.
- DSN/bounce generation validates inbound event metadata before composing and queueing null reverse-path DSNs, and retry-exhausted delivery events generate sender-facing RFC 3464 failure DSNs with deterministic dedupe keys while still honoring `NOTIFY=NEVER` and null reverse-path suppression.
- DSN/bounce generation now honors RFC 3461 `RET=HDRS` by attaching bounded,
  sanitized original message headers as a `text/rfc822-headers` report part
  when delivery events carry a safe original `.eml` storage path.
- DSN/bounce generation also honors RFC 3461 `RET=FULL` by attaching the
  bounded original `.eml` as a `message/rfc822` report part after validating
  the stored object key and message parseability.
- DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext identity metadata before it can reach outbound SMTP command generation or RFC 3464 report composition.
- Delivery partial-failure handling preserves recipient-level retry/bounce decisions even when every RCPT is rejected.
- Attachment upload storage paths reject absolute, parent-traversal, backslash,
  newline, oversized total-key, and oversized segment forms, and generated
  attachment object paths sanitize path segments before writing to storage.
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
- `docs/openapi.yaml` provides the first backend-only OpenAPI 3.1 draft and is guarded against YAML syntax errors, backend contract version drift, registered-route drift, stale documented routes, dangling component references, request-body omissions, response envelope reference drift, message flag enum drift, list limit contract drift, and thread-list parameter leakage.
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
- Remote Ed25519 manifest signer status-error diagnostics collapse signer
  response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing
  external signer errors from leaking into export/billing diagnostics.
- Push-notification and attachment-scan webhooks now reject CR/LF-bearing
  configured tokens/endpoints and collapse non-2xx HTTP response bodies into
  bounded one-line UTF-8 previews before surfacing delivery failures.
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
- OpenAPI documents the development-only `user_id` fallback parameter on every
  user-scoped Mail operation that can use it when JWT auth is disabled, keeping
  generated local/all-in-one clients aligned with runtime behavior.
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
  traversal, newline, backslash-bearing, oversized, or empty stored keys where a
  body is required.
- Local storage enforces the same strict object-key contract at the adapter
  boundary, rejecting absolute, traversal, newline, backslash-bearing,
  duplicate-separator, dot-segment, and otherwise non-canonical keys before
  filesystem access.
- S3-compatible storage reuses the same strict object-key contract at the
  adapter boundary and signs streaming `PUT`, `GET`, and `DELETE` requests with
  AWS SigV4, keeping AWS S3 and MinIO-style deployments behind the existing
  storage interface.
- S3-compatible storage preserves single escaping for virtual-hosted and
  path-style object URLs, including keys with spaces or other URL-sensitive
  characters, before SigV4 canonical request signing.
- S3-compatible storage status-error diagnostics collapse backend response
  bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing object
  store errors from leaking into readiness or storage operation diagnostics.
- S3-compatible bucket names are validated with shared adapter/config guardrails
  before runtime wiring, surfacing uppercase, undersized, slash-bearing, or
  punctuation-adjacent deployment mistakes before storage calls.
- S3-compatible regions are validated with shared adapter/config guardrails
  before SigV4 signing, rejecting blank, whitespace-bearing, slash-bearing, or
  uppercase region values before object-storage requests are created.
- S3-compatible object prefixes are validated as canonical relative object-key
  prefixes during config validation, surfacing duplicate separators, dot
  segments, traversal, or backslash mistakes before adapter construction.
- Optional S3-compatible integration coverage can exercise real
  `PUT`/`GET`/`DELETE` round trips against MinIO or AWS S3 when
  `GOGOMAIL_TEST_S3_ENDPOINT`, bucket, and credential environment variables are
  configured.
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
- Mail API read/search/list, draft-search, attachment capability/session/download,
  and push-device list routes reject unknown query parameter names before
  dispatch, making generated-client typos visible as HTTP 400 responses.
- Mail API mutation routes reject unknown query parameter names before dispatch,
  and JSON-backed compose/draft/attachment/send mutations accept the documented
  development-only `user_id` query fallback when JWT auth is disabled.
- Admin company/domain/DNS-check/user list routes reject unknown query
  parameter names before dispatch, keeping core operator filters aligned with
  the documented contract.
- Admin API usage aggregate, ledger, retention, export-batch, artifact,
  manifest-digest, and manifest-signature routes reject unknown query parameter
  names before dispatch, including unexpected query strings on detail,
  download, verification, and mutation routes with no query controls.
- Admin queue, outbox, audit, backpressure, quota, attachment-session,
  delivery-attempt, push-notification, suppression-list, trusted-relay,
  delivery-route, and DKIM read routes reject unknown query parameter names
  before dispatch.
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
