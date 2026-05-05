# gogomail current status

Last updated: 2026-05-06 (updated after CardDAV text-match semantics)

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

Actual Next.js frontend implementation has not started. When frontend work
starts, use Next.js with TypeScript, shadcn/ui, and the project `DESIGN.md` as
required guidance, aiming for a Notion Mail-like UI/UX. Before creating or
substantially implementing frontend apps, ask the user for frontend-specific
guidance.

Calendar work is planned as a standards-first CalDAV module, not as a
frontend-only calendar API. The initial `internal/caldavgw` boundary and
`caldav` runtime mode scaffold exist so future webmail calendar features and
native CalDAV/iCalendar clients can share a protocol-correct backend.

CalDAV remains an experimental/backend-only release slice: useful protocol
building blocks now exist, but the module is not yet advertised as public
client-ready. The next compatibility gates are recurrence expansion, scheduling
semantics, retention-aware sync deltas, collection-deletion sync, broader
native-client testing, and the platform boundaries below.

Calendar product features must not grow as isolated CRUD. Before delegated
calendars, shared ownership, attendees, resource booking, reminders, or
organization calendars become public features, the project should establish
clear Directory/Identity, Contacts/CardDAV, Notification & Sync, Search, and
Policy/Audit boundaries. Directory is the platform/org layer for users, teams,
groups, aliases, resources, memberships, delegation, and principal resolution;
Contacts/CardDAV is the user-owned address-book layer for personal/external
people and user-specific metadata.

Contacts/CardDAV work has started as a standards-first backend boundary, not a
generic contacts CRUD API. The initial `internal/carddavgw` package defines
RFC/WebDAV/CardDAV tokens, canonical principal, address-book home,
address-book collection, and `.vcf` contact-object path/href handling, plus
metadata validation for address books, contact object names, UIDs, strong
ETags, size limits, sync tokens, and bounded vCard 4.0 semantic checks.
PostgreSQL storage groundwork now exists for address books, contact objects,
and address-book change logs. A first repository boundary can create/list/get
address-book collections through active user/domain/company scope and records
address-book creation changes. Contact-object repository methods can now
upsert/list/get/delete `.vcf` resources through active address-book scope,
using bounded vCard validation, strong ETags, optional observed-ETag guards,
sync-token updates, and durable change rows. Public CardDAV compatibility now
has bounded WebDAV `PROPFIND` parsing, an internal `OPTIONS`/`PROPFIND`
discovery handler, bounded REPORT request parsing, and internal REPORT
execution for `addressbook-query`, `addressbook-multiget`, and
`sync-collection`. `addressbook-query` now preserves the first
`prop-filter` property name and applies `text-match` to parsed unfolded vCard
property values instead of only scanning the whole object body. It also honors
the RFC 6352 default `i;unicode-casemap` collation plus `equals`, `contains`,
`starts-with`, `ends-with`, and `negate-condition` text-match attributes. It
remains gated on broader vCard/filter-tree/param-filter compatibility and
native-client tests. The handler is deliberately experimental and does not yet
make CardDAV public/client-ready.
`gogomail --mode=carddav` now starts a dedicated CardDAV HTTP listener with
Basic-auth backed by the existing Submission authenticator. WebDAV multistatus
response building is available for CardDAV principal, address-book collection,
contact-object, REPORT, and sync responses.

The first Directory/Identity slice now exists as `internal/directory`: it owns
bounded platform-principal identifiers, principal kinds, active user principal
resolution over user/domain/company state, and organization principal
resolution over organization/domain/company state. Directory schema groundwork
also covers groups, resources, aliases, and group memberships, with resolver
support for group and resource principals plus normalized alias-to-principal
lookup and direct group-membership checks. Active aliases are globally unique
by normalized address. CalDAV discovery uses this shared resolver instead of
embedding its own active-user join, but delegated access, shared calendar
ownership, attendee resolution, and resource booking semantics remain future
release gates.

## Completed or materially advanced

- SMTP receive engine with real TCP integration coverage.
- Authenticated Submission MTA with STARTTLS and SMTPS support.
- Outbound SMTP delivery with direct MX, smart-host, TLS policy, retry, and
  partial recipient failure handling. Admin-created delivery routes reject
  impossible TLS/auth combinations before relay routes are stored.
  Static smart-host configuration now rejects password-only auth plus
  CR/LF-bearing or oversized auth username, password, and identity values during
  startup config validation.
- DSN/bounce handling with RFC 3461/3464-oriented metadata, null reverse-path,
  `NOTIFY=NEVER`, deterministic outbox dedupe, retry-exhaustion failure
  notifications, and loop-risk reduction.
- Shared high-performance-minded EML parsing boundary under `internal/message`.
- PostgreSQL metadata model for companies, domains, users, folders, messages,
  attachments, outbox, audit logs, DKIM keys, trusted relays, delivery routes,
  domain DNS checks, Drive nodes, and policy-bearing domain settings.
- Admin APIs for domains, users, quotas, DKIM keys, trusted relays, delivery
  routes, delivery route resolution, queue stats, delivery attempts,
  outbox event metadata, suppression list, quota usage, domain DNS
  checks/history, backpressure inspection/update, domain policy, per-domain
  stats, DKIM DNS verification, delivery route runtime counters, and exhausted
  delivery attempts with recipient-domain and recent-window filters.
- Admin API exposes `GET /admin/v1/console/capabilities` so a production
  operator console can discover backend contract version, available
  modules, list/cleanup/retention limits, tenancy controls, operational triage
  surfaces, and auth/no-store behavior before rendering console navigation.
- Delivery-attempt list, stats, and exhausted-attempt reads can filter by
  message id, farm, sender, recipient domain, and recent time window for
  targeted retry/bounce incident triage.
- Domain listing can filter by company, lifecycle status, and latest DNS-check
  status for onboarding and tenant triage.
- Domain DNS check history can filter by summary status and RFC3339 `since`
  windows so operators can inspect recent onboarding or deliverability failures
  without re-querying DNS or scanning every persisted check.
- Company listing can filter by lifecycle status for tenant-level suspension
  and disabled-account triage.
- Delivery-route listing can filter by status, farm, and domain pattern for
  targeted route audits.
- Admin delivery-route creation now rejects oversized farm, SMTP hello, pool,
  description, and relay auth identity/username/password metadata before route
  storage or audit work.
- Suppression-list reads can filter by domain, email, and reason for targeted
  bounce triage without direct database access.
- Queue stats include ready, delayed, stale-processing, oldest-ready, and
  next-available metadata so operators can distinguish backlog from scheduled
  retry delay.
- Outbox event metadata can be filtered by topic, partition key, status, and
  recent time window without exposing payload bodies.
- Outbox event list responses bound `last_error` previews at UTF-8 boundaries
  so operational dashboards do not pull oversized diagnostics by default.
- Outbox event detail responses expose full stored `last_error` by id while
  still omitting raw payload bodies.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, cursor-paginated thread lists/thread messages and draft search,
  plus user-scoped sent-message delivery/bounce status.
- Mail API exposes `GET /api/v1/webmail/capabilities` so a production webmail
  frontend can discover backend contract version, active/planned modules, list
  limits, supported message flags, bulk-action bounds, compose/search/thread,
  attachment, and push-device capabilities without hard-coding server limits.
- Mail API exposes `GET /api/v1/mailbox/overview` so production webmail chrome
  can render aggregate total/unread/starred/size counts and system-folder ID
  shortcuts without duplicating folder-list aggregation in every client.
- Mail API message lists now support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters alongside
  folder and cursor controls, enabling production webmail quick views such as
  unread, read, starred, unstarred, and attachment-bearing messages without
  switching to full-text search.
- Mail API thread lists now support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters, where
  `read=false` means conversations with at least one unread message and
  `read=true` means fully-read conversations.
- Mail API thread lists now also support `folder_id`, enabling folder-scoped
  conversation views for inbox, sent, archive, and custom folders without
  falling back to flat message lists.
- Mail API message and thread lists now support `sort=newest|oldest` with
  bounded query validation, giving production webmail clients explicit
  newest-first and oldest-first mailbox/conversation list controls.
- Mail API message and thread summaries now expose a required bounded
  `preview` string sourced from the asynchronous search-document read model,
  letting production webmail lists render body context without opening and
  parsing stored `.eml` objects on the list hot path.
- Mail API now supports bounded thread-level bulk flag updates for
  conversation-list read/starred/answered/forwarded actions, while publishing
  best-effort IMAP flag events for the updated messages.
- Mail API now supports bounded thread-level folder moves, validating
  destination folders, invalidating affected IMAP UID rows transactionally, and
  publishing best-effort IMAP expunge events from the pre-move UID snapshot.
- Mail API now supports bounded thread-level soft deletes, deleting every
  active message in selected conversations while invalidating IMAP UID rows,
  decrementing quota transactionally, and publishing best-effort IMAP expunge
  events from the pre-delete UID snapshot.
- Mail API now supports single-message and bounded bulk message restore for
  soft-deleted messages, clearing `deleted_at` and re-checking/re-incrementing
  the hierarchical quota ledger before messages become active again.
- Mail API now supports bounded thread-level restore for soft-deleted
  conversations, reactivating selected conversation messages only after the
  same hierarchical quota guard used by message restore succeeds.
- Mail API restore flows now best-effort assign IMAP UIDs to restored active
  messages and publish IMAP `EXISTS` events, reducing stale selected-mailbox
  views for clients that are connected while webmail recovery actions run.
- Inbound parsing now extracts RFC `In-Reply-To`/`References`; inbound and
  reply/forward outbound persistence inherit local thread IDs when matching
  source messages exist.
- Reply composition writes RFC `In-Reply-To`/`References` headers into outgoing
  `.eml` messages.
- Outbound text composition rejects CR/LF-bearing subject, display-name, email,
  and explicit Message-ID inputs before writing RFC 5322 headers.
- Outbound RFC 5322 text composition folds long headers, rejects malformed
  explicit Message-ID values, and drops malformed thread IDs before writing
  `In-Reply-To`/`References`.
- Mail API exposes a first Postgres-backed search endpoint for active message
  metadata, with an FTS index for small deployments.
- Received-message body search now has an asynchronous indexing boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml` objects,
  extracts bounded text through the shared parser, and upserts Postgres search
  documents used by the existing search endpoint.
- Search indexing rejects ambiguous `mail.stored` storage paths that would be
  changed by path cleaning, preventing parent-traversal or duplicate-separator
  event payloads from opening a different object key.
- Search indexing caps `mail.stored` event `References` metadata before
  document construction, matching the parser's bounded metadata stance.
- The OpenSearch indexing adapter bounds UTF-8 metadata fields and reference
  arrays before JSON document submission, keeping direct adapter calls aligned
  with worker/parser metadata limits.
- OpenSearch relevance queries bound UTF-8 search/filter text and escape
  wildcard metacharacters in sender/subject filters so client-supplied `*` or
  `?` remain literal substring filters.
- OpenSearch relevance hits now clean bounded message IDs from `_source`/`_id`
  before Postgres hydration, dropping CR/LF-bearing IDs from external search
  responses.
- OpenSearch indexing now rejects blank, CR/LF-bearing, or oversized document
  message IDs before constructing `_doc/{id}` URLs, keeping URL IDs aligned
  with bounded JSON metadata.
- OpenSearch writer/searcher construction now trims usernames while preserving
  password bytes, and rejects CR/LF-bearing or oversized endpoint credentials
  before BasicAuth request headers can be generated.
- OpenSearch username/password configuration is also CR/LF-rejected and
  size-bounded during startup config validation when the OpenSearch backend is
  selected, surfacing credential formatting mistakes before worker/search setup.
- OpenSearch writer construction now rejects CR/LF-bearing direct endpoint
  values before URL parsing, keeping adapter calls aligned with startup config
  endpoint validation.
- OpenSearch relevance response decoding is capped before JSON parsing so
  oversized search backend responses cannot allocate unbounded highlight or hit
  payloads in the Mail API path, and trailing JSON tokens are rejected before
  search hits are accepted.
- OpenSearch index/bootstrap/search status-error diagnostics now collapse
  backend response bodies into bounded one-line UTF-8 previews, preventing
  CR/LF-bearing backend errors from leaking into logs or API diagnostics.
- Shared EML text extraction, retained header metadata, and attachment
  metadata are bounded with UTF-8 boundary preservation; attachment filenames
  are basename-normalized, control-character cleaned, and capped before
  reaching storage/API/search consumers. Subject, address display-name/address,
  message-id, address-list, and `References` metadata are capped before
  downstream storage, search, and threading use them. Oversized structured
  subject, address, and message-id-list headers are pre-bounded before decoding
  or list parsing, with truncation flags for retained metadata/list caps.
- Search responses can now opt into relevance sorting, rank scores, and bounded
  Postgres headline snippets while preserving date-sorted results by default.
- Postgres and OpenSearch relevance search now share a metadata-first tuning
  intent: subject and sender matches rank above indexed body text matches.
- Draft rows remain excluded from `GET /api/v1/search`; drafts now have a
  separate compose-focused `GET /api/v1/drafts/search` contract over active
  draft subject, sender, recipient JSON, body text, attachment state, and
  newest-updated ordering.
- `gogomail --mode=all-in-one` serves Mail API and Admin API routes from the
  same HTTP process, keeping single-node/local release smoke tests aligned with
  the documented backend mode.
- `/health/ready` can now include runtime database and Redis dependency probes
  for HTTP modes that use those services, returning `degraded` with HTTP 503
  when a required probe fails.
- Database readiness now also compares the applied `goose_db_version` against
  the latest local SQL migration, so stale schemas degrade `/health/ready`
  instead of passing on connectivity alone.
- Mail/Admin HTTP readiness now probes configured storage with a write/read/delete
  cycle, and unsupported HTTP storage backends fail fast instead of silently
  using local storage wiring.
- Local/NFS storage configuration now requires a non-empty bounded
  `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
  `GOGOMAIL_STORAGE_BACKEND=local`, so broken filesystem roots fail during
  config validation instead of surfacing later as storage probe errors.
- Local and S3-compatible storage writes now reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- Local/NFS and S3-compatible storage now expose a shared object `Stat`
  contract, allowing future Drive, lifecycle, and verification paths to inspect
  canonical keys, byte size, and backend metadata without streaming object
  bodies. The S3-compatible adapter implements this with signed `HEAD`
  requests.
- Local/NFS and S3-compatible storage now expose a shared object `Copy`
  contract. Local/NFS copies stream through the same atomic temporary-file
  commit path as normal writes, while S3-compatible copies use signed
  server-side copy requests with escaped `x-amz-copy-source` values.
- Local/NFS and S3-compatible storage now expose a shared bounded object
  `List` contract for validated prefixes, giving future Drive, lifecycle, and
  reconciliation workflows a cursor-paginated way to browse object metadata
  without binding callers to filesystem walks or S3 `ListObjectsV2` directly.
- Local/NFS and S3-compatible storage now expose a shared object `Move`
  contract for Drive-ready rename/relocation workflows. Local/NFS uses
  filesystem rename semantics, while S3-compatible storage performs signed
  server-side copy followed by source delete and documents the non-atomic
  duplicate-cleanup implication.
- Shared storage now provides a bounded `DeletePrefix` helper that composes
  validated prefix `List` pages with idempotent object deletes, giving future
  Drive folder deletion, attachment lifecycle, and reconciliation jobs a
  cursor-driven cleanup path without backend-specific recursive delete logic.
- Drive backend groundwork now has ADR 0009, a `drive_nodes` PostgreSQL
  metadata table, and an internal node-name/type/status validation package.
  Drive object bytes remain behind the shared storage interface while metadata,
  folder trees, lifecycle state, and future quota enforcement stay in the
  database/service boundary.
- Drive now has a first internal repository mutation for active user folder
  creation, deriving company/domain scope from the user row, validating parent
  folders before insertion, using only bound request parameters in SQL, and
  applying Drive node-name/type/status validation before future HTTP routes
  expose the module.
- Drive now has an internal file-finalize repository boundary that validates
  storage backend/object metadata, verifies the object through `storage.Stat`,
  and increments the company/domain/user quota ledger in the same transaction
  as the `drive_nodes` file insert.
- Drive now has an internal node-list repository read model for active,
  trashed, or deleted nodes under a parent folder, with bounded limits and
  folder-first stable ordering for future webmail/Drive clients.
- Drive now has an internal trash repository mutation that marks an active
  node and active descendants as trashed in one transaction, preserving object
  bytes and quota usage for future restore or delayed permanent deletion.
- Drive now has an internal restore repository mutation that marks a trashed
  node and trashed descendants active again in one transaction, clears
  `trashed_at`, and relies on the active sibling uniqueness constraint to keep
  restored folder contents conflict-safe.
- Drive now has an internal permanent-delete repository mutation that marks a
  trashed node and trashed descendants deleted, decrements company/domain/user
  quota for deleted files in the same transaction, and returns storage object
  references for backend-specific byte cleanup.
- Drive now has a backend-object cleanup helper that consumes permanent-delete
  object references, validates storage backend/path input, de-duplicates
  repeated references, honors cancellation, and deletes through the configured
  storage stores with progress-preserving errors.
- Drive now has a small internal service layer that composes repository
  permanent-delete with backend object cleanup, preserving cleanup progress on
  post-transaction storage failures for future retry/reconciliation handling.
- Drive now has canonical object path builders for staged uploads, committed
  node objects, and user prefixes under `drive/users/{user_id}/...`, with
  path-segment-safe ID validation so future cleanup and prefix operations stay
  tenant/user scoped.
- Drive permanent-delete cleanup failures now have a PostgreSQL retry record
  boundary. Structured cleanup errors can be recorded with user/node/object
  context, pending failures are de-duplicated per backend/path, attempts are
  incremented on repeat failures, object paths must stay under the owning
  user's `drive/users/{user_id}/...` prefix, and error text is one-line/UTF-8
  bounded for future operator and worker surfaces.
- Drive cleanup-failure records now have bounded repository list and resolve
  methods with status/user filters, oldest-first pending ordering, limit caps,
  and pending-only resolution, preparing retry workers and admin visibility
  without exposing HTTP contracts yet.
- Drive now has an internal cleanup retry service method that lists pending
  cleanup-failure records, deletes each referenced object through configured
  storage stores, resolves successful records, and re-records failed attempts
  so retry diagnostics remain fresh and bounded.
- Drive cleanup retry can now run as a first-class backend worker mode,
  `drive-cleanup-worker`, with validated interval/batch/run-once config and
  local/MinIO/S3-compatible storage wiring through the shared storage adapter.
- Mail API now exposes the first Drive HTTP routes for production webmail
  integration: bounded node listing, single-node metadata reads, folder
  creation, trash, restore, and permanent delete. The routes use the existing
  user auth/fallback path, shared Drive repository/service boundaries, and
  OpenAPI-documented response envelopes without starting frontend
  implementation.
- Mail API also exposes `POST /api/v1/drive/files/finalize`, letting a
  previously staged object become quota-accounted Drive file metadata through
  the shared storage `Stat` contract and Drive file-finalize repository
  boundary.
- Mail API now exposes `PUT /api/v1/drive/files/staged/{upload_id}/body`,
  streaming a bounded object body to the configured local/NFS, MinIO, or
  S3-compatible backend, deriving the canonical Drive staging key, computing
  size and SHA-256, and returning a frontend-ready staged-object envelope for
  file finalization.
- Drive nodes can now be renamed through `PATCH /api/v1/drive/nodes/{id}/name`,
  keeping active file/folder metadata aligned with normalized-name validation
  and sibling uniqueness before future production Drive UI work begins.
- Drive nodes can now be moved through
  `PATCH /api/v1/drive/nodes/{id}/parent`, validating destination folders,
  root moves, and active-subtree cycle prevention at the repository boundary.
- Drive upload sessions now have a PostgreSQL metadata boundary and
  `internal/drive` validation contract for upload IDs, parent folders,
  declared size, MIME type, storage backend, lifecycle status, and bounded
  expiration before HTTP upload-session routes are exposed.
- `internal/drive.Repository.CreateUploadSession` can create pending Drive
  upload sessions for active users under optional active parent folders,
  preserving the same backend-neutral storage metadata and bounded expiration
  rules that future HTTP clients will use.
- Mail API now exposes `POST /api/v1/drive/upload-sessions`, returning stable
  `drive_upload_session` envelopes for frontend clients that need to declare
  Drive upload metadata before body transfer/finalization.
- Mail API now exposes `GET /api/v1/drive/upload-sessions/{id}`, giving
  frontend clients a stable upload-session status refresh path before body
  retry/finalize routes are added.
- Mail API now exposes `DELETE /api/v1/drive/upload-sessions/{id}`, allowing
  clients to explicitly cancel pending/uploading/failed Drive upload sessions
  instead of waiting for expiry cleanup.
- Drive upload-session body storage now has service/repository boundaries that
  stream each retry to a distinct canonical object path, verify declared size
  and optional SHA-256, update session storage metadata, and best-effort clean
  superseded or failed staged bodies across local/NFS, MinIO, and S3-compatible
  stores.
- Mail API now exposes `PUT /api/v1/drive/upload-sessions/{id}/body`, wiring
  the retry-safe body storage service to frontend clients with an optional
  `X-Content-SHA256` integrity header and explicit `Content-Range` rejection
  until chunked/resumable semantics are specified.
- Drive upload-session finalization now has a repository/service boundary that
  locks a writable session, verifies the stored object size through the shared
  storage `Stat` contract, increments quota, inserts the Drive file node, and
  marks the session finalized in one transaction.
- Mail API now exposes `POST /api/v1/drive/upload-sessions/{id}/finalize`,
  letting frontend clients commit uploaded session bodies into Drive file
  metadata through the same quota and storage verification boundary.
- Webmail capabilities now advertise Drive node operations, upload-session
  create/read/cancel/body/finalize support, checksum preconditions, and Drive
  upload size/TTL limits so production clients can enable Drive flows without
  copying backend constants.
- Drive upload sessions can now be expired in bounded repository batches, and
  the Drive service deletes stored session bodies from the configured backend
  after rows are marked expired.
- `drive-cleanup-worker` now expires stale Drive upload sessions on each run
  before retrying permanent-delete object cleanup failures, keeping abandoned
  upload-session objects out of request paths.
- Mail API now exposes `GET /api/v1/drive/upload-sessions` with status and
  limit filters, and webmail capabilities advertise the list surface for
  production upload manager recovery.
- Admin API now exposes `GET /admin/v1/drive-upload-sessions` with required
  user scope plus status/limit filters, and admin capabilities mark Drive
  upload-session inspection available for operator consoles.
- Drive node listing now supports a bounded `q` name filter on both Mail and
  Admin API list surfaces, with case-insensitive normalization and literal SQL
  wildcard handling inside the selected parent/status scope.
- Admin API now exposes `GET /admin/v1/drive-nodes` with required user scope
  plus parent/status/name/limit filters so operator consoles can inspect a
  user's Drive inventory through bounded backend contracts.
- Admin API now exposes `GET /admin/v1/drive-nodes/{id}` with required user
  scope and lifecycle status filtering so operator consoles can inspect one
  Drive file or folder without entering user-facing auth paths.
- Admin API now exposes `GET /admin/v1/drive-usage` with required user scope
  so operator consoles can render quota, node-count, byte-count, and pending
  upload-session dashboard summaries.
- Mail API now exposes `GET /api/v1/drive/usage`, and webmail capabilities
  advertise the usage summary surface for future Drive storage cards.
- Mail API now exposes `GET /api/v1/drive/nodes/{id}/download`, streaming
  active Drive file bytes from the configured local/NFS, MinIO, or S3-compatible
  backend with bounded identity validation, safe attachment headers,
  `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`; webmail
  capabilities advertise `node_download`.
- Mail API also exposes `HEAD /api/v1/drive/nodes/{id}/download` so production
  clients can verify active file metadata and object existence without opening
  or transferring the object body.
- Drive downloads now support a single satisfiable HTTP byte range through the
  shared local/NFS and S3-compatible `GetRange` storage contract, giving
  production webmail clients resumable download and media-preview building
  blocks without backend-specific object access.
- Drive download, range-download, and download-header responses now expose a
  sanitized `X-Gogomail-Drive-SHA256` header when file metadata carries a
  stored whole-object digest, giving clients an integrity check without
  trusting backend-specific ETags.
- IMAP `ENABLE` now rejects malformed capability atoms before authentication
  or session mutation, keeping RFC 5161 syntax failures distinct from valid
  unauthenticated enable attempts.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/candidates` so
  operators can preview stale Drive upload-session cleanup counts and bounded
  candidate rows before worker cleanup handles them.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/runs` for
  explicit, audited, one-shot stale Drive upload-session expiry outside the
  worker loop.
- Admin API now exposes `GET /admin/v1/drive-cleanup-failures` with user,
  status, and limit filters so operator consoles can inspect Drive backend
  object cleanup drift.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve`,
  allowing audited operator closure after external Drive object cleanup
  verification.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/retry-runs`,
  letting operators trigger audited bounded retries for pending Drive object
  cleanup drift and inspect scanned/deleted/resolved/failed run counts.
- S3-compatible storage requests now reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, and `DELETE` responses now drain a small
  bounded response-body window before close, improving HTTP connection reuse
  for normal S3/MinIO responses without allowing oversized bodies to stall
  cleanup.
- Local/NFS and S3-compatible readiness probes now read the verification object
  through a tight expected-size bound, so malformed or proxy-inflated probe
  responses cannot allocate unbounded memory during `/health/ready` checks.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, CalDAV scaffold, and HTTP runtimes now share storage backend validation and factory
  wiring for local filesystem/NFS-style storage plus S3-compatible object
  storage. `GOGOMAIL_STORAGE_BACKEND=s3` can target AWS S3, while
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same S3-compatible adapter with
  path-style requests for local MinIO-style deployments. Both paths use endpoint,
  region, bucket, prefix, credential, and session-token settings.
- S3-compatible runtime option construction is now isolated and covered by app
  tests, pinning MinIO to path-style requests while preserving virtual-hosted
  S3 defaults unless `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` is set.
- S3-compatible bucket validation now rejects IP-address-shaped names plus
  AWS-reserved bucket prefixes and suffixes during config validation, and
  requires bucket names to start and end with a letter or digit, so S3
  deployment mistakes fail before adapter construction or readiness probes.
- S3-compatible endpoint validation now rejects userinfo, query strings,
  fragments, non-HTTP schemes, CR/LF-bearing targets, and non-canonical base
  paths before adapter construction. Endpoint base paths also reject encoded
  path separators such as `%2F` and `%5C`, keeping SigV4 signing and object
  addressing unambiguous across AWS S3, MinIO, and compatible providers.
- S3-compatible request construction automatically uses path-style addressing
  for dotted bucket names on HTTPS endpoints, avoiding AWS S3 virtual-hosted
  TLS wildcard certificate mismatches while preserving virtual-hosted requests
  for ordinary bucket names by default.
- S3-compatible request construction also automatically uses path-style
  addressing for localhost and IP-address endpoints, avoiding
  `bucket.localhost`/`bucket.127.0.0.1` style drift for local MinIO and other
  local compatible object stores even when the generic `s3` backend is used.
- S3-compatible object key escaping now preserves literal `+` characters as
  `%2B` in segment-escaped paths, keeping object identity and SigV4 canonical
  request paths aligned across AWS S3, MinIO, and strict compatible providers.
- S3-compatible endpoint base paths are now segment-escaped with the same
  literal `+` preservation as object keys, keeping proxy/base-path deployments
  aligned with SigV4 canonical request paths.
- S3-compatible uploads now set a deterministic `Content-Length` for seekable
  PUT bodies without buffering the object in memory, improving compatibility
  for file-backed mail and attachment writes while preserving streaming-first
  storage paths.
- S3-compatible deletes now treat `404 Not Found` as already-cleaned success,
  aligning compatible-provider cleanup behavior with local/NFS idempotent
  deletes.
- S3-compatible secret access keys and session tokens now reject spaces, tabs,
  and line breaks during config validation and adapter construction, surfacing
  copied env/config credential mistakes before readiness probes or runtime
  PUT/GET requests fail with opaque authentication errors.
- S3-compatible access key IDs now reject spaces, tabs, and line breaks during
  config validation and adapter construction, preventing copied credential
  mistakes from being silently trimmed before SigV4 signing.
- S3-compatible access key IDs, secret access keys, and session tokens now also
  reject oversized direct adapter inputs using the same bounds as startup
  config validation, preventing oversized SigV4 header material from reaching
  runtime request construction.
- Local/NFS-style storage writes now stage through unique temporary files in
  the target directory before `rename`, avoiding fixed `.tmp` collisions while
  preserving atomic object replacement semantics.
- Local/NFS-style storage writes now honor context cancellation during body
  copy, removing staged temp objects instead of committing partial data after a
  canceled request.
- Local/NFS-style storage deletes now treat already-missing objects as success,
  aligning cleanup semantics with S3-compatible delete behavior across storage
  backends.
- IMAP `LIST`/`LSUB` CHILDREN attributes now infer immediate parents from
  nested `FullPath` values when backend rows do not carry `ParentID`, so deeper
  hierarchies such as `Projects/2026/Jan` still mark `Projects/2026` with
  `\HasChildren` for clients that depend on hierarchy metadata.
- IMAP `APPEND`, `STORE`, and `UID STORE` flag-list parsing now rejects
  unparenthesized or unbalanced flag lists instead of silently trimming stray
  parentheses, keeping flag mutation syntax closer to RFC-shaped client
  expectations.
- IMAP selected-mailbox `STORE` and `UID STORE` now honor advertised
  `[PERMANENTFLAGS]`, rejecting otherwise valid system flags when the selected
  mailbox did not permit them instead of dispatching unsupported mutations to
  storage. Empty `+FLAGS ()` and `-FLAGS ()` remain successful no-ops, while
  `FLAGS ()` replacement is rejected when no permanent flags are permitted.
- IMAP message sequence sets now explicitly reject sequence numbers above the
  selected mailbox size with tagged `BAD` responses, preserving RFC 3501 bounds
  behavior for `FETCH`, `STORE`, `COPY`, and `MOVE` sequence arguments.
- IMAP quoted-string parsing now rejects adjacent tokens after a closing quote
  and unsupported backslash escapes before authentication or backend work,
  keeping command tokenization aligned with RFC 3501 quoted-special handling.
- IMAP mailbox wire-name formatting now preserves ordinary internal spacing
  while still collapsing control-character runs, preventing `LIST`, `LSUB`, and
  `STATUS` responses from changing distinct user-visible mailbox names.
- IMAP UID `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` commands now resolve
  `*` UID sequence ranges against selected-mailbox UIDs, so common client
  requests such as `UID FETCH 1:*` include the last visible UID without
  expanding through non-existent UID gaps.
- IMAP `SEARCH UID <sequence-set>` and `UID SEARCH UID <sequence-set>` now
  resolve `*` UID ranges against the selected mailbox's visible UIDs, aligning
  search-key filtering with UID command range handling.
- IMAP command tag validation now rejects `+` in tags before command routing,
  matching RFC 3501 tag grammar and avoiding ambiguity with continuation
  protocol markers.
- IMAP `SEARCH`/`UID SEARCH` date criteria now reject malformed date atoms that
  still contain quote characters after command parsing, so broken inputs such
  as `SINCE 05-May-2026"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` date criteria now accept one-digit date-day atoms
  such as `SINCE 5-May-2026` while preserving the malformed quote rejection,
  improving compatibility with clients that do not zero-pad day values.
- IMAP command tokenization now rejects embedded quote characters inside
  unquoted atoms while preserving escaped quotes inside proper quoted strings,
  keeping RFC 3501 atom and quoted-string handling separate.
- IMAP parenthesized `SEARCH`/`UID SEARCH` groups now reject empty `()` groups
  instead of treating them as match-all, while preserving valid `(ALL)` groups.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` numeric thresholds now reject malformed
  values that still contain quote characters after command parsing, so broken
  inputs such as `MODSEQ 20"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types now reject malformed atoms
  that still contain quote characters after command parsing, preventing broken
  `MODSEQ "/flags/\\Seen" all" 17` style inputs from being silently normalized.
- IMAP RFC 2971 `ID` parameter-list parsing now rejects unsupported quoted
  escapes and adjacent quoted tokens without whitespace, while preserving valid
  escaped quoted-special characters inside ID strings.
- IMAP RFC 2971 `ID` parameter-list parsing now also rejects quote and
  backslash atom-special characters inside unquoted ID tokens, keeping raw ID
  argument parsing aligned with the broader RFC 3501 atom/quoted-string split.
- IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the same atom
  validator as command tags and atoms, rejecting literal markers, response
  specials, wildcard specials, quoted specials, and controls consistently.
- IMAP `SEARCH`/`UID SEARCH` `LARGER` and `SMALLER` size criteria now require
  digit-only RFC 3501 number atoms, rejecting signed values such as `+20`
  instead of silently treating them as valid sizes.
- IMAP mod-sequence numeric inputs now require digit-only atoms across
  `SEARCH MODSEQ`, `FETCH CHANGEDSINCE`, and conditional `STORE`
  `UNCHANGEDSINCE`, rejecting signed values such as `+17`.
- IMAP UID and message sequence-set numbers now require digit-only atoms,
  rejecting signed values such as `UID FETCH +7` and `FETCH +1` before command
  execution.
- IMAP UID and message sequence-set expansion now accepts common client-scale
  ranges such as `1:1000` and `1:*` while still enforcing an explicit expansion
  cap, reducing false `BAD` responses during mailbox synchronization.
- IMAP UID set resolution now intersects authenticated selected-mailbox UID
  ranges and comma-separated UID sets with visible message UIDs, so sparse
  requests such as `UID FETCH 1:999` and `UID FETCH 1,7,999` skip missing UIDs
  instead of failing the whole command.
- IMAP MIME body-part paths and partial body fetch windows now require
  digit-only number atoms, rejecting signed forms such as `BODY[+1]` and
  `BODY[]<+12.34>`, and partial fetch counts must be non-zero as required by
  RFC 3501 `nz-number` grammar. Partial fetch tokens also reject trailing
  characters after the closing `>`.
- IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments now reject malformed
  atoms that still contain quote characters after command parsing, preventing
  broken values such as `UTF-8"` from being silently normalized.
- IMAP `THREAD` algorithm arguments now reject malformed atoms that still
  contain quote characters after command parsing, preventing broken values such
  as `ORDEREDSUBJECT"` from being silently normalized.
- IMAP `SEARCH`/`UID SEARCH` text, body, and header string arguments now reject
  malformed atoms that still contain quote characters after command parsing,
  preventing broken values such as `SUBJECT IMAP"` from being normalized.
- IMAP `SEARCH` text arguments now preserve valid RFC quoted-special escaped
  quotes from proper quoted strings, so standards-shaped searches such as
  `SUBJECT "Project \"Q2\""` remain compatible while malformed atom quotes are
  rejected by command parsing.
- IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria now reject
  malformed keyword atoms that still contain quote characters after command
  parsing, preventing broken values such as `KEYWORD custom"` from being
  silently normalized.
- IMAP command tokenization now rejects dangling quote characters at the end of
  unquoted atoms, preventing broken commands such as `SUBJECT IMAP"` and
  `LIST "" INBOX"` from reaching command-specific normalization while
  preserving valid escaped quotes inside proper quoted strings.
- IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT` lists now
  validate RFC-shaped header field names instead of trimming stray brackets,
  rejecting malformed requests such as `HEADER.FIELDS ([Subject])`.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` now requires the RFC-shaped
  parenthesized modifier form and rejects bare or over-closed variants such as
  `FETCH 7 FLAGS CHANGEDSINCE 17`.
- IMAP `FETCH`/`UID FETCH` macros now remain valid only as standalone macro
  arguments, rejecting malformed list usage such as `FETCH 1 (FAST)` or
  `UID FETCH 7 (FLAGS FAST)`.
- IMAP `STORE`/`UID STORE` `UNCHANGEDSINCE` now requires the RFC-shaped
  parenthesized modifier form and rejects malformed over-closed values such as
  `(UNCHANGEDSINCE 27))`.
- IMAP `FETCH`/`UID FETCH` data items now reject over-parenthesized tokens
  before item normalization, preventing malformed requests such as
  `FETCH 1 ((FLAGS))` and `UID FETCH 7 BODY.PEEK[]))` from being repaired.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, including the `GOGOMAIL_STORAGE_ROOT` compatibility alias for
  `GOGOMAIL_MAILSTORE_ROOT`, and the development compose stack includes
  `minio-init` to create the default local `gogomail` bucket.
- HTTP server runtime guardrails are configurable and validated: read, write,
  idle, read-header timeout, and maximum header bytes are wired into the shared
  Mail/Admin/API-metered HTTP server.
- Admin backpressure overrides now persist bounded hash-chain audit rows after
  Redis state changes, recording previous/current SMTP pressure levels without
  silently accepting unaudited operational receive throttles.
- Admin suppression-list deletions now persist hash-chain audit rows in the
  same transaction as the delete, preserving suppression entry, domain, email,
  reason, and source-message evidence for deliverability forensics.
- Admin outbox retry now persists a hash-chain audit row in the same transaction
  as the retry reset, preserving previous topic, partition key, status,
  attempts, and bounded error evidence for replay forensics.
- Admin push-notification outcome updates now persist hash-chain audit rows in
  the same transaction as provider-status updates and invalid-token device
  deletion, without including raw push tokens or token suffixes in audit detail.
- Admin attachment cleanup runs now persist bounded hash-chain audit rows after
  stale upload and upload-session expiry sweeps, recording cutoff, normalized
  limit, expired counts, and bounded ID samples without storage paths.
- Admin IMAP UID backfill now persists a hash-chain audit row in the same
  transaction as UID assignment, recording mailbox/user scope, normalized
  limit, assigned count, and a bounded message/UID sample.
- Admin API-usage export batch creation now persists a hash-chain audit row in
  the same transaction as the batch, recording tenant/principal scope, export
  window, event/request counts, bytes, latency totals, and export format.
- Admin API-usage export artifact creation/upsert now persists a hash-chain
  audit row in the same transaction as the artifact row, recording object key,
  storage backend, content type, byte/event counts, and SHA-256 digest without
  copying artifact metadata into the audit detail.
- Admin API-usage export manifest digest and signature creation now persist
  hash-chain audit rows in the same transaction as the evidence rows, recording
  bounded digest/signature evidence without copying raw manifests, metadata, or
  full signature material into audit detail.
- Admin API-usage ledger retention runs now persist hash-chain audit rows in the
  same transaction as run records and destructive deletes, recording dry-run,
  blocked, no-op, and completed outcomes with bounded readiness evidence.
- Admin user creation and password-hash rotation can persist a validated
  `password_hash`, giving operators a path to create and maintain SMTP
  Submission-capable local users without storing raw production passwords
  through the API. User read models expose `password_configured` without
  returning stored password hashes, and Admin user listing can filter by status
  and that readiness flag.
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
- Admin quota usage pressure reads can filter by scope, domain, over-limit
  status, and over-allocation status for targeted capacity triage.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with message and attachment source rows.
- Admin API can run operator-controlled quota reconciliation corrections with
  transaction/advisory locking and bounded audit-log detail for dry-run and
  applied correction attempts.
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
- API usage daily/monthly aggregate reads can filter by tenant, company, domain,
  user, API key, principal, auth source, method, route, status, and time window
  for scoped billing and operational triage.
- API metering events now use `2026-05-04.api-usage.v2` payloads with
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  worker stores those dimensions in the idempotency ledger and keys daily/monthly
  aggregates by identity so usage from different tenants or principals does not
  merge.
- API metering auth-source dimensions are normalized to the known set
  `anonymous|bearer|admin_token|query_user_id|unknown`; unexpected values fold
  to `unknown` before ledger/aggregate storage.
- API metering request identity extraction trims tenant/company/domain/user/API
  key/principal dimensions, drops CR/LF-bearing or oversized default request
  dimensions, and no longer classifies blank or unsafe `Authorization: Bearer`
  headers as bearer traffic.
- API metering durable event metrics clamp negative byte/latency values to zero
  and default nonpositive request counts to one before ledger/aggregate storage.
- API metering outbox payloads clamp negative byte/latency values to zero before
  deterministic event IDs are generated.
- API metering durable events require nonblank method/route keys and HTTP-like
  status codes before ledger/aggregate storage.
- API metering middleware route-key extraction drops CR/LF-bearing or oversized
  ServeMux patterns and fallback paths before sink dispatch.
- API metering durable event decoding rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before ledger/aggregate storage.
- Admin API usage ledger, NDJSON export, stats, export-batch creation, and
  retention-readiness tenant/principal filters now reject CR/LF-bearing or
  oversized values before service dispatch.
- Admin user listing, IMAP UID backfill, DKIM key listing, and delivery-route
  resolution query filters now share the same CR/LF and size boundary checks;
  DKIM key listing can also filter by `active|inactive` status.
- API usage export batch, artifact, manifest-digest, and signature path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Admin company, domain, and user detail/mutation path identifiers now use the
  same blank, CR/LF, and size validation before service dispatch.
- Admin IMAP UID backfill mailbox IDs, outbox event/retry IDs, DKIM key IDs,
  suppression IDs, trusted-relay IDs, and delivery-route IDs now use the same
  path boundary validation before service dispatch.
- Mail API development `user_id` query fallback values now reject CR/LF-bearing
  or oversized identifiers before service dispatch.
- OpenAPI now wires the Mail API development `user_id` fallback parameter into
  every user-scoped Mail operation, keeping local/all-in-one generated clients
  aligned with JWT-disabled runtime behavior.
- Mail API folder, thread, message, draft, attachment, and push-device path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Push-device create/update validation now rejects invalid-UTF-8,
  CR/LF-bearing, or oversized user and token metadata before repository upsert,
  keeping raw provider tokens bounded at the storage boundary.
- Mail API message-list `folder_id` and search text/filter query parameters now
  reject CR/LF-bearing or oversized values before service dispatch.
- Mail API bearer JWT `user_id` and `sub` identities now reject CR/LF-bearing
  or oversized claims during signing and verification before request scoping.
- Mail API bearer JWT verification now rejects oversized token, header,
  payload, and signature segments before base64 decoding claim data.
- Mail and Admin API authentication headers now reject oversized `Authorization`
  and `X-Admin-Token` values before bearer/JWT parsing or token comparison.
- Password hash verification now rejects oversized stored hashes, excessive
  PBKDF2 iteration counts, and oversized PBKDF2 salt/key metadata before
  expensive derivation or decoded allocation.
- Mail API search control query values and direct multipart attachment
  `draft_id` fields now reject CR/LF-bearing or oversized values at the HTTP
  boundary before service dispatch.
- VERP return-path parsing now rejects oversized addresses, local parts, tokens,
  and encoded recipients before base64 decoding DSN recipient metadata.
- API usage export Ed25519 signer/verifier key configuration now rejects
  oversized base64 public/private keys before decoding.
- API usage export manifest signer configuration now rejects CR/LF-bearing or
  oversized key IDs and remote signer tokens, and local HMAC signing rejects
  oversized secrets before MAC generation.
- API usage export HMAC and Ed25519 signature verification now rejects
  incorrectly sized signature hex before decoding.
- Remote Ed25519 manifest signer responses now reject oversized bodies and
  trailing JSON tokens before signature evidence is accepted.
- Remote Ed25519 manifest signer status-error diagnostics now collapse signer
  response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing
  external signer errors from leaking into export/billing diagnostics. Remote
  signer HTTP responses now use the shared bounded drain-and-close helper so
  keep-alive connections can be reused without unbounded cleanup reads.
- Attachment scan and push-notification webhooks now reject CR/LF-bearing
  configured tokens or endpoints and collapse non-2xx HTTP response bodies into
  bounded one-line UTF-8 previews before surfacing delivery failures. Shared
  webhook HTTP response cleanup now drains a small bounded body window before
  close so keep-alive connections can be reused without unbounded cleanup reads.
- API metering middleware falls back to `METHOD /path` when no `http.ServeMux`
  route pattern is available, keeping durable event route keys nonblank.
- API metering now records immutable `api_usage_ledger` rows before aggregate
  upserts. Admin API exposes bounded ledger list, NDJSON export, and stats
  endpoints for billing/export preparation while keeping HTTP request handling
  fail-open.
- Admin API exposes API usage ledger retention readiness so operators can check
  whether non-future cutoff-bound ledger rows are covered by a completed export
  batch with artifact, manifest digest, and signature evidence before retention
  is allowed.
- Admin API exposes bounded API usage ledger retention runs. Destructive runs
  require `confirm_ready=true`, reuse the readiness gate, and delete only a
  normalized batch of ready ledger rows, while dry-runs return the same envelope
  without mutation.
- Optional PostgreSQL integration coverage verifies retention runs do not delete
  blocked candidates, dry-runs do not mutate ready candidates, and destructive
  ready runs persist retention-run audit rows, delete only the requested bounded
  batch, and preserve newer ledger rows.
- Admin API exposes list/detail reads for persisted API usage ledger retention
  runs so operators can inspect blocked, dry-run, and destructive retention
  attempts after the fact.
- `api-usage-retention-worker` can run bounded API usage ledger retention on an
  interval or once-and-exit, dry-run by default, reusing the same readiness gate
  and persisted retention-run audit rows as the Admin API.
- Destructive API usage retention worker runs require both explicit
  `confirm_ready` configuration and a production-oriented `remote-ed25519`
  export manifest signer backend.
- API usage export capabilities now advertise retention-run support, retention
  worker support, and the remote-key requirement for destructive worker runs.
- API usage ledger retention now rejects future cutoffs at the repository
  boundary as well as the HTTP boundary, keeping worker/direct-call behavior
  aligned with the Admin API guardrail.
- Admin API exposes bounded audit-log list/detail reads with category, action,
  result, target-type, company/domain/user, and recent-window filters so stored
  operational audit records can be inspected through the release API surface.
- Domain DNS check and quota reconciliation correction audit rows now reuse the
  shared audit writer hash-chain logic instead of inserting empty hash fields.
- Trusted relay create/delete mutations now write hash-chain audit rows in the
  same database transaction as the policy change, keeping inbound relay-policy
  administration inspectable through the Admin audit API.
- Delivery route create/status/delete mutations now write hash-chain audit rows
  in the same database transaction as the gateway policy change, excluding
  relay auth secrets from audit detail.
- DKIM key create/upsert, deactivate, and DNS-verification mutations now write
  hash-chain audit rows in the same database transaction as the persisted key
  lifecycle change, without including private key material in audit detail.
- Domain and user lifecycle status updates now write hash-chain audit rows in
  the same database transaction as the status change, scoped by company/domain
  identifiers for tenant forensics.
- Company, domain, and user quota mutations now write hash-chain audit rows in
  the same database transaction as the quota change, including domain default
  user quota propagation counts for quota forensics.
- Domain policy mutations now write hash-chain audit rows in the same database
  transaction as the policy change, preserving inbound/outbound mode and size
  guardrail evidence for SMTP/Mail API enforcement forensics.
- Domain/user provisioning and user password-hash rotation now write hash-chain
  audit rows in the same database transaction as the persisted change, without
  including password hash material in audit detail.
- Shared audit-log normalization now bounds scalar metadata and JSON detail size
  before hash computation or database insertion.
- Admin API exposes a bounded audit-log integrity check that recomputes recent
  row hashes and reports hash or in-window prev-hash breaks without mutating the
  audit trail.
- API usage exports now have persisted batch manifests/checkpoints. Admin API
  can create/list/get manifest rows and replay a saved manifest window as NDJSON
  by batch ID. Batch creation now requires explicit RFC3339 `from`/`to`
  bounds, preventing accidental all-ledger checkpoints.
- API usage export batch listing can filter by tenant, principal, status, and
  export window so operators can find covering manifests without scanning every
  saved batch.
- API usage ledger/export/retention tenant and principal query filters are
  trimmed at the Admin API boundary before billing/export service dispatch.
- API usage export batches can now carry external artifact metadata rows with
  object key, content type, byte count, SHA-256, event count, and JSON metadata.
  Artifacts are deduplicated per batch by object key and SHA-256.
- API usage export artifact writes reject ambiguous object keys that would be
  changed by path cleaning or contain backslash/path-traversal segments before
  writing billing handoff objects.
- Admin API can now write API usage export batch artifacts to the configured
  object store, register the resulting byte count/SHA-256 metadata, and download
  or verify stored NDJSON artifacts for handoff verification.
- API usage export batches now have canonical manifest digest rows and
  verification endpoints. Operators can generate SHA-256 digests over the saved
  batch plus registered artifacts, list/fetch digest records, and re-check the
  stored manifest against its canonical digest before billing handoff.
- API usage export manifest digesting rejects unsupported explicit manifest
  schema versions before canonical digest evidence is generated.
- API usage export manifest digests can now be signed through disabled-by-
  default local HMAC, local Ed25519, or remote Ed25519 signers. The remote
  signer is intended for an external KMS-backed signing service and is verified
  locally with a configured public key before persistence. Admin API exposes
  signature create/list/detail and verification endpoints while keeping the
  signer backend pluggable.
- API usage export manifest signing validates key IDs for local and remote
  signers, rejecting blank, CR/LF-bearing, or oversized key metadata before
  signature evidence is returned.
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
  current wired verifiers are local-HMAC and Ed25519, supporting both local and
  remote Ed25519 signer backends.
- Admin API exposes API usage export capabilities so operators can see the
  configured signer backend, signer key ID, verifier availability, and whether
  production/verified billing readiness is supported before creating handoff
  batches.
- Push notification enqueue now has an async worker boundary:
  `push-notification-worker` consumes `mail.stored` events, resolves active
  user devices from PostgreSQL, and can emit disabled-by-default `slog`
  notification candidates or POST raw-token targets to a configured HTTP
  webhook push gateway with an optional bounded bearer token and Postgres
  candidate-attempt audit rows without touching SMTP hot paths or committing to
  FCM/APNs SDKs. `docs/webhook-integrations.md` documents the push gateway
  payload, authentication, HTTPS requirement, and queued/failed attempt
  semantics. Malformed resolved targets with blank or CR/LF-bearing device IDs
  or tokens, oversized device IDs or tokens, or unsupported platforms, are
  dropped before candidate recording and sink handoff; optional target labels
  and token suffixes are UTF-8 safely bounded. The webhook sink also bounds and
  normalizes direct-call payload metadata before JSON serialization.
- Admin API exposes `GET /admin/v1/push-notification-attempts` for inspecting
  push notification candidate fan-out by message, status, user, platform,
  device, provider status, provider message id, or recent time window.
- Admin API exposes `GET /admin/v1/push-notification-attempts/{id}` for
  single-attempt troubleshooting.
- Admin API exposes
  `PATCH /admin/v1/push-notification-attempts/{id}/outcome` for authenticated
  operator/provider handoff updates to queued, delivered, failed, or
  invalid-token outcomes with bounded provider diagnostics.
- Admin API exposes `GET /admin/v1/push-notification-stats` for a compact
  active-device and attempt-status summary, with optional `message_id`,
  `user_id`, `platform`, `device_id`, and `since` scoping for message-level,
  user-level, provider-platform, device-level, and recent-window
  troubleshooting.
- Push notification sinks receive the persisted candidate attempt id with each
  target, preparing clean vendor outcome updates later.
- Push notification candidate and provider-outcome diagnostics are capped at
  UTF-8 boundaries before Postgres storage, preserving internationalized
  subjects and vendor messages in Admin API views.
- Push notification candidate recording rejects invalid-UTF-8, CR/LF-bearing,
  or oversized message/user/device/company/domain IDs before SQL insert
  dispatch, and rejects unsupported platforms at the recorder boundary.
- The push worker marks attempts `queued` after a successful sink handoff while
  marking failed sink handoffs as `failed` with the sink error before returning
  the handler error for Redis stream retry.
- Existing push attempts can be updated to queued, delivered, failed, or
  invalid-token outcomes through the internal recorder or Admin API.
- The push worker's internal outcome recorder now delegates to the same
  `maildb` outcome update path used by the Admin API, keeping provider status
  validation, diagnostic bounds, timestamp updates, and invalid-token device
  deletion in one storage boundary.
- Push notification outcome recording rejects invalid-UTF-8, CR/LF-bearing, or
  oversized attempt IDs before SQL update dispatch.
- Invalid-token outcomes automatically soft-delete the affected push device in
  the same Postgres transaction.
- `mail.stored` events now carry an explicit
  `2026-05-04.mail-stored.v1` schema version for downstream audit, search, and
  push workers.
- Audit, search indexing, and push notification consumers reject unsupported
  explicit `mail.stored` schema versions while accepting versionless legacy
  events.
- The audit `mail.stored` consumer trims event, tenant, recipient, subject,
  storage, and timestamp fields and rejects CR/LF-bearing message identifiers
  before audit-log persistence.
- Delivery-status audit consumers trim event, tenant, sender, recipient, farm,
  status, error, and timestamp fields and reject CR/LF-bearing message
  identifiers before audit-log persistence.
- Delivery outcome and exhausted outbox event payloads trim message, tenant,
  farm, sender, recipient, error, and DSN metadata before event persistence.
- Mail API now has user-scoped push device registration/list/delete contracts
  for `apns`, `fcm`, and `webpush`; raw device tokens are accepted only on
  write and are not returned in API JSON responses.
- Push-device list and delete service methods trim user and device identifiers
  before repository work, and delete rejects blank, CR/LF-bearing, or oversized
  device identifiers before repository dispatch.
- DKIM key DNS verification workflow with `dns_verified_at` persistence.
- Delivery route runtime counters (`RouteCounters`) with Admin API exposure.
- Retry exhaustion hook: `mail.delivery_exhausted` outbox event emitted and
  `delivery_attempts` row with status `exhausted` written when all retries fail.
- The delivery worker wires retry exhaustion recording at runtime, so terminal
  retry exhaustion diagnostics and `mail.delivery_exhausted` events are emitted
  by the actual worker path.
- Retry-exhausted delivery events now carry recipient-level DSN metadata and
  safe original storage paths into the event worker, generating sender-facing
  RFC 3464 failure DSNs with deterministic dedupe keys while preserving
  `NOTIFY=NEVER` and null reverse-path suppression.
- Admin delivery attempt lists can be scoped by status, recipient domain, and
  recent time window for bounded retry/bounce triage.
- Admin delivery attempt stats summarize total attempts, unique messages,
  unique recipients, and delivered/failed/bounced/exhausted buckets with the
  same status, recipient-domain, and recent-window filters.
- Admin delivery-route status/delete handlers trim route IDs at the HTTP
  boundary before operator mutations are passed to the service layer.
- User-scoped sent-message delivery status treats failed attempts with RFC 3463
  `4.x.x` enhanced status codes as retrying rather than terminal failed.
- User-scoped sent-message delivery status treats terminal `exhausted`
  attempts as failed so retry budgets do not remain visible as pending forever.
- DMARC reject policy enforcement at SMTP receive (`DMARCEnforce` flag).
- Authentication-Results trace header formatting now strips control characters
  and bounds verifier metadata before formatting SPF/DKIM/DMARC results,
  preventing DNS/library diagnostics from injecting or bloating stored headers.
- SMTPUTF8 declared correctly on outbound MAIL FROM for all internationalized
  addresses, and outbound delivery now fails closed with a permanent SMTPUTF8
  error when the remote MTA does not advertise SMTPUTF8.
- DSN composition supports optional `text/rfc822-headers` and `message/rfc822`
  returned-content parts for RFC 3464 reports, keeping header-only returns
  sanitized while allowing bounded full-message returns.
- Bounce DSN generation now honors `RET=HDRS` when the delivery event carries a
  safe original message storage path, reading bounded original EML headers and
  attaching them as sanitized `text/rfc822-headers` content.
- Bounce DSN generation now also honors `RET=FULL` by attaching the bounded
  original `.eml` as `message/rfc822` after validating the stored object key and
  message parseability.
- Migration guardrails now require every SQL migration to declare explicit
  goose Up/Down sections, and legacy API-usage, push, IMAP, and audit-index
  migrations have been normalized to that structure without changing their
  applied SQL.
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests. Path parameters, Mail search/Admin query filters,
  request schemas, response envelopes, and status enums are contract-tested for
  generated-client readiness. The draft is parsed as YAML and checked for stale
  documented routes that are not registered by the Go HTTP mux. Thread list
  parameters are guarded against accidental Admin/API-usage filter leakage.
  Non-JSON download/export responses are guarded so NDJSON streams and binary
  attachments are not modeled as JSON envelopes. All schemas are kept in sync
  with Go types.
- Admin token authorization and API metering admin-token classification compare
  fixed-length SHA-256 digests of trimmed token values for both bearer tokens
  and `X-Admin-Token`.
- Mail API JWT verification rejects unsupported JWT `alg` values and non-JWT
  `typ` headers before accepting signed bearer claims. JWT `user_id`/`sub`
  identities are whitespace-normalized and blank identities are rejected during
  both signing and verification. Tokens with `iat` values more than one minute
  in the future are rejected before Mail API claims are trusted.
- Redis event consumers acknowledge malformed stream entries after logging
  decode failures and move repeatedly handler-failing messages into a durable
  Redis dead-letter stream before acknowledging the original event, preventing
  poison messages from pinning worker progress indefinitely. Event,
  search-index, API-metering, push-notification, and delivery workers expose
  per-worker max-delivery and dead-letter-stream settings for production tuning.
- Redis event/search/API-metering/push/delivery workers reclaim idle pending
  Redis Stream messages via configurable claim-idle settings, improving crash
  recovery for at-least-once event processing. Startup validation now also
  rejects nonpositive event and delivery consumer count/block settings before
  workers run with unusable Redis Stream options.
- Redis worker stream, group, and consumer-name settings for event,
  search-index, API-metering, push-notification, and delivery workers are now
  required, CR/LF-rejected, and size-bounded during startup config validation,
  surfacing worker identity mistakes before consumer construction.
- `eventstream.NewRedisConsumer` now applies the same trim, required,
  CR/LF-rejection, and size-bound guardrails to stream, group, and consumer
  identifiers, keeping direct adapter callers aligned with runtime config
  validation.
- Event routing trims registered and payload event names and rejects
  CR/LF-bearing or oversized event names before worker dispatch.
- Redis stream event decoding rejects CR/LF-bearing or oversized outbox IDs,
  partition keys, and payloads before worker fan-out.
- Redis stream event decoding trims outbox id, partition key, and payload
  fields and rejects blank metadata before handler dispatch.
- Redis outbox publishing trims event id, topic, partition key, and payload
  metadata and rejects invalid topics or non-JSON payloads before stream writes.
- Admin API exposes a bounded IMAP mailbox UID backfill endpoint for future
  IMAP bootstrap/operator runs without enabling an IMAP protocol listener.
- Push notification workers no longer redeliver a Redis event solely because
  queued-outcome recording failed after the sink accepted the notification,
  reducing duplicate push risk while keeping the candidate attempt visible.
- Backend release verification script and SMTP release runbook.
- API usage export runbook covering capability checks, artifact/digest/signature
  handoff evidence, deep readiness, and retention-readiness gates.
- Public GitHub repository:
  <https://github.com/parkjangwon/gogomail>

## Explicitly not started

- Next.js shell/webmail/admin frontend implementation.
- Built-in spam scoring or pattern filtering.
- POP3 protocol server work. The future POP3 server should follow the same
  strict RFC, performance, and client-compatibility standard as IMAP.
- OpenSearch as the default/mandatory search backend.
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
- `attachment-cleanup-worker` can now run the stale upload cleanup loop
  periodically with configurable interval, stale age, and batch size, turning
  the repository/service cleanup path into an operational mode. It can also run
  once and exit for CronJob or timer-style deployments.
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
- OpenSearch writer/searcher HTTP responses now use the shared bounded
  drain-and-close helper, improving connection reuse for indexing, bootstrap,
  and relevance queries without allowing oversized responses to stall cleanup.
- OpenSearch endpoint configuration is now validated as an HTTP(S) URL with a
  host during startup config validation, so malformed search backend endpoints
  fail before worker/search adapter construction.
- OpenSearch index names are now validated during startup config validation
  using the same unsafe-character and reserved-prefix guardrails as the
  adapter, so invalid index configuration fails before worker/search setup.
- Search contract expansion: clients can request `sort=relevance`,
  `include_rank=true`, and `include_highlights=true` without changing the
  default message list shape.
- Quota operations read models: capacity fields and reconciliation reporting
  show ledger pressure and drift without mutating counters.
- Quota correction actions: operators can explicitly apply reconciliation
  results to company/domain/user ledgers after reviewing drift, with dry-run and
  applied correction attempts recorded in audit logs.
- IMAP gateway planning: native backend interfaces, RFC-shaped flag/mailbox
  helpers, and durable UID/MODSEQ storage exist without starting a TCP protocol
  server.
- The first IMAP adapter path can list/get mailboxes, list mailbox messages,
  resolve messages by UID, stream raw stored message bodies, and mutate
  persisted IMAP-visible flags with MODSEQ advancement as `internal/imapgw`
  DTOs while ensuring UID state.
- Existing active mailbox contents can be backfilled with stable mailbox-local
  IMAP UIDs in bounded batches before any live IMAP listener is enabled.
- The shared `event-worker` now consumes committed `mail.stored` events through
  an IMAP UID handler that ensures newly received active messages get
  mailbox-local UIDs asynchronously after SMTP storage commits. Stale
  `mail.stored` events for messages that were moved or deleted before UID
  assignment are treated as no-ops instead of retrying forever.
- IMAP UID assignment event decoding rejects CR/LF-bearing or oversized
  message, user, and folder IDs before UID work or mailbox event fan-out.
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
- Authenticated Submission now applies enforcing per-domain recipient caps during
  `RCPT TO`, not only after `DATA`, so oversized envelopes receive earlier
  SMTP feedback before message streaming/spooling.
- Attachment scanner hook rejection/tempfail reasons are CR/LF-stripped and
  UTF-8 safely bounded before they are surfaced as SMTP hook errors.
- `GOGOMAIL_ENV` now accepts only `development`, `test`, or `production`, so
  environment typos cannot silently bypass production-only safety gates.
- Redis-backed deduplication, recipient rate limiting, and SMTP backpressure
  backend selectors now accept only `none` or `redis`, preventing typos from
  silently disabling operational controls.
- Redis-backed RCPT rate-limit keys now normalize remote addresses to the
  remote host/IP bucket instead of the full `ip:port`, preventing source-port
  churn from bypassing recipient abuse controls.
- RCPT rate-limit and outbox relay batch, poll, and max-attempt settings are
  now validated as positive values during startup config validation, surfacing
  relay/limit misconfiguration before workers start.
- HTTP, SMTP, inbound SMTP, Submission, and optional SMTPS listener addresses
  are now validated as TCP `host:port` values at startup, surfacing bind
  configuration mistakes before runtime listener setup.
- Delivery retry delay schedules and maximum delay caps are now validated as
  positive durations, preventing retry jobs from being exhausted accidentally
  or scheduled in the past by malformed runtime configuration.
- `GOGOMAIL_DELIVERY_SMTP_HELLO` is now validated as a non-empty
  whitespace-free hostname during startup config validation, surfacing outbound
  SMTP EHLO configuration mistakes before delivery worker startup.
- Attachment scanning can be enabled with a configured HTTP webhook backend;
  the hook remains disabled by default, supports an optional bounded bearer
  token, requires HTTPS in production, and is wired only at SMTP
  receive/submission app boundaries. `docs/webhook-integrations.md` documents
  the scanner request, bounded response, and accept/reject/tempfail verdict
  contract. Scanner webhook requests bound and normalize message, address,
  subject, recipient, and attachment metadata before JSON serialization.
- Redis duplicate-message detection uses fixed-length hashed dedup keys so raw
  message IDs or recipient addresses cannot create oversized Redis keys.
- Mail API move/delete operations invalidate stale IMAP UID rows in the same
  transaction, and IMAP UID idempotency checks require the same active
  user/mailbox before reusing an existing UID, keeping mailbox-local UID state
  from leaking across folders.
- Optional PostgreSQL integration coverage now exercises IMAP UID backfill and
  move invalidation when `GOGOMAIL_TEST_DATABASE_URL` is available.
- `internal/imapgw` has a small in-memory mailbox event broker for live IDLE
  and NOOP fan-out through the protocol listener; broker delivery is scoped by
  both user and mailbox to preserve tenant isolation.
- `mailservice.StoreIMAPFlags` can publish IMAP mailbox `flags` events through
  an optional event publisher after repository flag mutations succeed.
- Mail API single and bulk flag mutations can look up existing IMAP UID rows and
  publish mailbox `flags` events for UID-visible messages after the database
  update succeeds.
- Mail API detail reads that auto-mark unread messages as read now also publish
  mailbox `flags` events for UID-visible messages after the read-flag write
  succeeds.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages after the database move succeeds.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages after soft-delete succeeds.
- `mailservice` exposes IMAP mailbox/message listing and mailbox-event
  subscription methods, keeping the protocol listener pointed at the service
  boundary instead of `maildb` internals.
- `mailservice` exposes bounded IMAP UID backfill through the same service
  boundary for future operator/bootstrap modes.
- IMAP mailbox event publication from service mutations is best-effort, so a
  fan-out failure does not turn an already-committed mail mutation into a client
  error.
- Mail API move/delete expunge notifications carry mailbox sequence numbers
  from IMAP UID lookup, allowing selected `NOOP`/`IDLE` clients to receive
  renderable untagged `EXPUNGE` updates.
- `mailservice` has an `IMAPStoreAdapter` that satisfies `imapgw.Store`, so a
  protocol listener can depend on the gateway interface while still routing
  through service methods.
- `IMAPStoreAdapter` now also satisfies `imapgw.MailboxSessionStore` for
  mailbox selection, service-backed COPY/MOVE/EXPUNGE, and event subscription.
- IMAP `UID FETCH` and `UID STORE` untagged `FETCH` responses use message
  sequence numbers per RFC 3501 while keeping the requested UID in response
  attributes, and `RFC822.SIZE` metadata requests do not trigger body streaming.
- IMAP `UID FETCH` accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]` for clients that batch reads without read-flag side effects.
- IMAP non-UID `FETCH` accepts bounded sequence sets, including `*`, and maps
  them through the selected mailbox list before returning fetch responses.
- IMAP `EXAMINE` supports read-only mailbox selection and blocks `UID STORE`
  mutations in that state.
- IMAP `EXAMINE` passes read-only selection intent through the backend
  `SelectMailboxRequest`, so service adapters can distinguish read-only
  sessions from writable `SELECT`.
- IMAP `SELECT`/`EXAMINE` now establish mailbox event subscriptions before
  emitting selected-mailbox response data, avoiding ambiguous partial selection
  state when subscription setup fails.
- IMAP `CHECK` and `CLOSE` support selected-mailbox lifecycle handling; `CLOSE`
  silently expunges `\Deleted` messages for writable selections before clearing
  selected state, while read-only selections only clear state.
- IMAP `STATUS` validates requested status data items and returns only the
  requested mailbox metadata fields.
- IMAP mailbox lookup resolves wire names such as `INBOX` and `Archive/2026`
  to the stored mailbox ID before selected-mailbox state is used by follow-up
  commands.
- IMAP `LIST` filters mailbox responses with exact, `*`, and `%` patterns over
  decoded mailbox names, then emits non-ASCII names and ampersands as RFC 3501
  modified UTF-7 instead of raw UTF-8 while `UTF8=ACCEPT` is not advertised.
- IMAP `CAPABILITY` advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`; `LIST`
  includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy attributes
  plus RFC 6154 special-use attributes for system folders such as Drafts, Sent,
  Trash, Junk, Archive, All, and Flagged when those folder roles are present in
  storage metadata, and extended
  `LIST (SPECIAL-USE)` / `RETURN (SPECIAL-USE)` forms are accepted.
- IMAP `CAPABILITY` advertises RFC 5819 `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits requested `STATUS` metadata after each
  matching selectable mailbox, and rejects malformed `RETURN (STATUS MESSAGES)`
  style status item lists before mailbox listing work.
- IMAP `CAPABILITY` advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
  `LIST-STATUS` can return per-mailbox total active message octets without
  fetching every message's `RFC822.SIZE`.
- IMAP `CAPABILITY` advertises RFC 5256 `SORT`; `SORT` and `UID SORT` evaluate
  the existing selected-mailbox search criteria, require `US-ASCII` or `UTF-8`
  charset arguments, and return sequence-number or UID `SORT` responses over
  RFC 5256 sort keys including base-subject, sent-date, arrival-date, address,
  and size ordering.
- IMAP `CAPABILITY` advertises RFC 5256 `THREAD=ORDEREDSUBJECT`; `THREAD
  ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` reuse the selected-mailbox
  search evaluator, enforce mandatory `US-ASCII`/`UTF-8` charset handling, and
  return RFC-shaped ordered-subject thread trees while leaving the more complex
  `REFERENCES` algorithm unadvertised until its Message-ID normalization and
  ancestry rules are implemented.
- IMAP RFC 5256 base-subject handling decodes RFC 2047 encoded-word subjects
  before removing reply/forward artifacts, keeping internationalized
  `SORT SUBJECT` and `THREAD ORDEREDSUBJECT` behavior aligned with compatible
  clients.
- IMAP `LIST "" ""` and `LSUB "" ""` return the hierarchy root with
  `\Noselect` and `/` delimiter metadata for clients that probe namespace
  delimiters through LIST-compatible commands.
- IMAP `SELECT`/`EXAMINE` emit `[PERMANENTFLAGS]` response codes for writable
  versus read-only selected-mailbox state.
- IMAP `SELECT`/`EXAMINE` emit RFC-shaped untagged `RECENT` counts alongside
  `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints, `UIDVALIDITY`,
  `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from durable mailbox
  UID state.
- IMAP `SELECT`/`EXAMINE` now emit `[UIDNOTSTICKY]` when the backend marks a
  mailbox's UIDs as non-sticky, keeping UIDPLUS-adjacent client state aligned
  with the selected mailbox's persistence guarantees.
- IMAP `UID STORE` supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses when requested.
- IMAP `FETCH`/`UID FETCH` can include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes from message summaries for mailbox list rendering.
- Service-backed IMAP message summaries now hydrate stored `To`, `Cc`, and
  `Bcc` address JSON into RFC-shaped ENVELOPE address lists, keeping real
  repository-backed `FETCH ENVELOPE`, address search, and address sort behavior
  aligned with the advertised protocol surface.
- IMAP shared fetch failure paths now tag failures with the command actually
  issued by the client, so regular `FETCH` failures no longer surface as
  `UID FETCH failed` responses while UID fetches retain UID-specific wording.
- IMAP `FETCH`/`UID FETCH` now apply RFC 3501 `\Seen` side effects for
  successful `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads while
  preserving `BODY.PEEK[...]` and `RFC822.HEADER` as non-mutating preview
  requests.
- IMAP `FETCH`/`UID FETCH` now preserves RFC 3501 `RFC822`,
  `RFC822.HEADER`, and `RFC822.TEXT` response data item names instead of
  returning their `BODY[...]` equivalents on the wire.
- IMAP `CAPABILITY` advertises `CONDSTORE` and `ENABLE`; RFC 5161-shaped
  `ENABLE CONDSTORE` marks sessions CONDSTORE-aware before mailbox selection.
- IMAP `FETCH`/`UID FETCH` can include RFC 4551-shaped `MODSEQ (n)` attributes
  when requested, using durable per-message mod-sequences.
- IMAP `SEARCH`/`UID SEARCH` can match RFC 4551-shaped `MODSEQ` criteria,
  including optional metadata entry/type arguments, and append the highest
  matched mod-sequence to non-empty SEARCH responses.
- IMAP `CAPABILITY` advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)` and
  `UID SEARCH RETURN (...)` can return `MIN`, `MAX`, compact `ALL`, `COUNT`,
  UID indicators, and CONDSTORE `MODSEQ` data in single untagged `ESEARCH`
  responses.
- IMAP `CAPABILITY` advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN (SAVE)`
  stores the last search result in the selected session so `$` can be reused in
  subsequent `FETCH`, `UID FETCH`, `SEARCH`, `UID SEARCH`, `STORE`, `COPY`,
  `MOVE`, and `UID EXPUNGE` set positions.
- IMAP `SEARCH RETURN (SAVE)` now clears the selected-session `$` result when a
  save-requested search fails with tagged `NO`, matching RFC 5182 failure
  semantics while leaving tagged `BAD` searches non-mutating.
- IMAP `FETCH`/`UID FETCH` supports RFC 4551-shaped `CHANGEDSINCE` modifiers,
  returning only messages with greater per-message mod-sequences and
  implicitly including `MODSEQ` response attributes.
- IMAP sessions become CONDSTORE-aware after `FETCH MODSEQ`,
  `FETCH CHANGEDSINCE`, `SEARCH MODSEQ`, or `STATUS HIGHESTMODSEQ`, and
  subsequent flag `FETCH` event/STORE echo responses include `MODSEQ`.
- IMAP `STORE`/`UID STORE` supports RFC 4551-shaped `(UNCHANGEDSINCE n)`
  modifiers with transactional per-message mod-sequence checks, applying
  passing updates and returning `[MODIFIED uid-set]` / `[MODIFIED sequence-set]`
  for stale messages. Conditional store response/event paths filter modified
  stale UIDs out of successful `FETCH` echoes and mailbox flag notifications.
- IMAP `SELECT` and `EXAMINE` accept the RFC 4551-shaped `(CONDSTORE)`
  parameter and mark the session CONDSTORE-aware.
- IMAP `FETCH`/`UID FETCH` can return a conservative single-part
  `BODYSTRUCTURE` response; full MIME tree serialization remains future work.
- IMAP single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
  parameters, content-transfer-encoding, ID, and description from bounded raw
  message headers instead of always reporting text/plain defaults.
- IMAP `BODYSTRUCTURE` now uses the streaming MIME-structure parser for
  metadata-only fetches, returning multipart child order, subtype, parameters,
  transfer encodings, dispositions, body octets, and text line counts without
  retaining attachment payloads.
- IMAP `BODYSTRUCTURE` now emits RFC 3501-shaped `message/rfc822` bodies with
  encapsulated message header-derived envelope metadata, parsed nested body
  structure, and line counts instead of treating attached messages as generic
  basic parts.
- The shared MIME-structure parser now descends into `message/rfc822` parts
  while counting the encapsulated message bytes/lines and capturing bounded
  envelope metadata, so forwarded-message attachments expose nested body
  metadata without retaining payloads.
- IMAP `FETCH`/`UID FETCH` can now return RFC 3501-shaped
  `BODY[n.HEADER]` and `BODY[n.TEXT]` literals for `message/rfc822` parts,
  including forwarded-message attachments inside multipart messages.
- IMAP `FETCH`/`UID FETCH` can now return `BODY[n.HEADER.FIELDS (...)]` and
  `BODY[n.HEADER.FIELDS.NOT (...)]` subsets for `message/rfc822` parts, so
  clients can preview forwarded-message headers without fetching whole nested
  headers.
- IMAP `FETCH`/`UID FETCH` can now follow multipart body-part numbering inside
  top-level `message/rfc822` parts, including nested part MIME headers such as
  `BODY[1.2]` and `BODY[1.2.MIME]`.
- IMAP literal-fetch regression coverage now includes multipart messages that
  attach a `message/rfc822` whose encapsulated body is itself multipart,
  guarding forwarded-message paths such as `BODY[2.2]` and `BODY[2.2.MIME]`.
- IMAP `BODYSTRUCTURE` regression coverage now includes the same forwarded
  multipart shape, guarding nested `MESSAGE/RFC822` serialization when the
  encapsulated message body is multipart.
- Malformed encapsulated `message/rfc822` literals now degrade gracefully for
  nested section fetches, returning an empty header section and raw text bytes
  instead of failing the whole IMAP `FETCH`.
- IMAP combined `BODYSTRUCTURE` plus literal body/header fetches can reopen the
  raw message for MIME metadata while preserving the original reader for
  literal streaming, so common preview/header fetch batches keep rich structure
  responses.
- IMAP `FETCH`/`UID FETCH` supports standard `FAST`, `ALL`, and `FULL` macros,
  including the non-extensible `BODY` attribute for `FULL`.
- IMAP `FETCH`/`UID FETCH` can stream bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- IMAP non-UID `FETCH` uses the same bounded header literal path as `UID FETCH`
  for `BODY[HEADER]` and `RFC822.HEADER`.
- IMAP `FETCH`/`UID FETCH` can stream bounded text-only literals for
  `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and `RFC822.TEXT`, with regression coverage
  rejecting oversized section bodies before unbounded allocation.
- IMAP `FETCH`/`UID FETCH` can stream conservative single-part text literals
  for `BODY[1]` and `BODY.PEEK[1]`.
- IMAP `FETCH`/`UID FETCH` can stream bounded top-level multipart body-section
  literals such as `BODY[1]` and `BODY[2]`, allowing clients to read individual
  MIME parts without fetching the full message.
- IMAP `FETCH`/`UID FETCH` can stream bounded nested multipart body-section
  literals such as `BODY[1.2]` with a capped MIME part path depth.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial windows over multipart
  body-section literals such as `BODY.PEEK[2]<4.4>`.
- IMAP `FETCH`/`UID FETCH` can answer conservative single-part MIME header
  requests for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
- IMAP `FETCH`/`UID FETCH` can stream actual multipart child MIME headers for
  `BODY[n.MIME]`/`BODY.PEEK[n.MIME]` requests when the selected part exists.
- IMAP `UID STORE` accepts bounded UID sets/ranges for batched flag mutation.
- IMAP non-UID `STORE` accepts bounded sequence sets/ranges and maps them to
  the same service-backed flag mutation boundary as `UID STORE`.
- IMAP non-UID `STORE` supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- IMAP `NOOP` drains queued selected-mailbox events into untagged `EXISTS`,
  `EXPUNGE`, and flag `FETCH` updates, suppressing stale or duplicate
  exact-count `EXISTS` events relative to the selected mailbox state.
- IMAP advertises and accepts `IDLE`, entering continuation mode and streaming
  selected-mailbox `EXISTS`, `EXPUNGE`, and flag `FETCH` updates while waiting
  for `DONE`.
- IMAP `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` work over the
  selected mailbox message list.
- IMAP `SEARCH`/`UID SEARCH` accepts sequence-set criteria such as `2:*`,
  letting clients intersect standard search predicates with selected mailbox
  sequence ranges.
- IMAP `SEARCH`/`UID SEARCH` can combine supported criteria with RFC default
  AND semantics, including `ALL` plus flag, date, size, address, and UID
  filters.
- IMAP `SEARCH`/`UID SEARCH` supports RFC `NOT` and binary `OR` criteria
  composition over the supported search predicate set.
- IMAP `SEARCH`/`UID SEARCH` accepts parenthesized search-key groups, combining
  grouped predicates with RFC default AND semantics and allowing grouped
  operands inside `OR`.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial full-body literals for
  `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial section literals for
  common `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]` requests.
- IMAP `SEARCH`/`UID SEARCH` supports common flag criteria for unread, starred,
  answered, and draft client views.
- IMAP `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag
  separately from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose
  that flag through `FLAGS`, `DELETED`, and `UNDELETED`.
- IMAP `SEARCH`/`UID SEARCH` supports `RECENT`, `OLD`, and `NEW`, returning no
  recent/new matches while durable recent-state semantics remain deferred and
  treating active messages as old.
- IMAP `SEARCH`/`UID SEARCH` supports `KEYWORD` and `UNKEYWORD` criteria with
  validated keyword atoms, and the webmail `forwarded` state is exposed as an
  IMAP `$Forwarded` keyword across `FETCH FLAGS`, `SEARCH KEYWORD`, and
  permitted `STORE` mutations.
- IMAP `FETCH`/`UID FETCH` supports bounded `BODY[HEADER.FIELDS (...)]` and
  `BODY.PEEK[HEADER.FIELDS (...)]` literals.
- IMAP `FETCH`/`UID FETCH` supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  literals.
- IMAP `FETCH`/`UID FETCH` supports bounded `BODY[HEADER.FIELDS.NOT (...)]` and
  `BODY.PEEK[HEADER.FIELDS.NOT (...)]` literals.
- IMAP `SEARCH`/`UID SEARCH` supports `SINCE`, `BEFORE`, and `ON` over message
  `INTERNALDATE`, plus `SENTSINCE`, `SENTBEFORE`, and `SENTON` over envelope
  dates.
- IMAP `SEARCH`/`UID SEARCH` supports basic `FROM`, `TO`, `CC`, `BCC`, and
  `SUBJECT` substring criteria over selected-mailbox summaries.
- IMAP `SEARCH`/`UID SEARCH` supports bounded `BODY` and `TEXT` raw-message
  criteria scans, with `BODY` excluding the RFC 5322 header block.
- IMAP `SEARCH`/`UID SEARCH` supports bounded RFC `HEADER <field> <value>`
  criteria scans over the raw message header block.
- IMAP `SEARCH`/`UID SEARCH` supports RFC 3501 `LARGER` and `SMALLER`
  criteria over message `RFC822.SIZE` metadata.
- IMAP `SEARCH`/`UID SEARCH` accepts `CHARSET US-ASCII` and `CHARSET UTF-8`
  prefixes and returns an RFC-shaped `[BADCHARSET]` response for unsupported
  search charsets.
- IMAP supports authenticated `NAMESPACE` for personal namespace and hierarchy
  delimiter discovery.
- IMAP `CAPABILITY` now advertises `NAMESPACE` alongside the implemented
  namespace command so client discovery matches the supported command surface.
- IMAP persists authenticated `SUBSCRIBE`/`UNSUBSCRIBE` mailbox subscriptions
  through the service/repository boundary, and `LSUB` now returns the saved
  subscription set instead of every visible mailbox.
- IMAP subscription canonicalization preserves hierarchy delimiters, quoting,
  and internal spacing while keeping case-insensitive matching, preventing
  distinct subscribed mailbox names from silently collapsing into one `LSUB`
  row.
- IMAP `SUBSCRIBE` can retain a mailbox name even when that mailbox does not
  currently exist, allowing `LSUB` to expose it with `\Noselect` for
  standards-friendly client migration and deleted-mailbox recovery flows.
- IMAP `LSUB` preserves subscribed mailbox names even when the mailbox no
  longer exists, returning missing names with `\Noselect`, and handles the RFC
  3501 `%` hierarchy case by returning subscribed parent levels.
- IMAP mailbox-taking commands now decode RFC 3501 modified UTF-7 at the
  protocol boundary for `SELECT`, `EXAMINE`, `STATUS`, `APPEND`, `COPY`,
  `MOVE`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, `UNSUBSCRIBE`, `LIST`,
  and `LSUB`, reject raw 8-bit or malformed modified UTF-7 forms, and keep
  internal service/storage mailbox names as UTF-8.
- IMAP quoted-string response formatting now escapes quotes/backslashes and
  cleans controls without collapsing ordinary spacing, preserving mailbox names,
  MIME parameters, and other wire values whose internal spaces are significant.
- IMAP advertises and supports RFC 2971 `ID`, validating `NIL` or bounded
  field/value parameter lists before returning gogomail server identity.
- IMAP advertises and supports `UNSELECT`, clearing selected-mailbox state
  without invoking `CLOSE`/EXPUNGE semantics.
- IMAP `EXPUNGE` and `UID EXPUNGE` delete only messages marked with the
  IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
  `EXPUNGE` responses, remove stale mailbox UID rows, and publish best-effort
  expunge events through the service boundary.
- IMAP `COPY` and `UID COPY` resolve sequence/UID sets through the selected
  mailbox, validate the destination mailbox, duplicate active message metadata
  and attachment rows transactionally, assign fresh destination mailbox UIDs,
  return UIDPLUS `[COPYUID ...]` response codes when destination UIDs are
  available, return `[TRYCREATE]` when the destination mailbox is missing, and
  publish best-effort destination `EXISTS` events through the service boundary.
- IMAP `MOVE` and `UID MOVE` resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, move active messages
  transactionally, assign fresh destination UIDs, and allow moves back into the
  selected mailbox by creating a fresh same-mailbox message before expunging
  the source UID. MOVE responses return UIDPLUS `[COPYUID ...]` mappings in
  the final tagged OK when destination UIDs are available, advance and return
  source mailbox `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients,
  emit RFC-shaped source `EXPUNGE` responses, and return `[TRYCREATE]` when
  the destination mailbox is missing.
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
  APPEND internaldate parsing accepts RFC 3501 space-padded one-digit date-days
  such as `" 5-May-2026 ..."`. The service boundary now rejects CR/LF-bearing
  or oversized APPEND user and mailbox identifiers before repository lookup,
  spooling, parsing, storage, or quota work.
- IMAP empty flag-lists are accepted where RFC-shaped clients can send them:
  `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears supported
  flags, and empty `+FLAGS ()`/`-FLAGS ()` are treated as successful no-ops.
- IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  mutation dispatch or mailbox event publication.
- IMAP service-backed read/list/subscription/backfill operations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  reads, storage opens, event subscriptions, or UID backfill work.
- IMAP service-backed `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
  reject zero UIDs before repository or storage work, keeping direct service
  callers aligned with RFC 3501's positive UID model.
- IMAP service-backed `STORE`, `COPY`, and `MOVE` calls reject empty UID sets
  before repository work, while `EXPUNGE` preserves nil UID sets for `CLOSE`
  style "all deleted messages" semantics.
- IMAP selected-mailbox `APPEND` now prefers the backend-returned appended
  message sequence number for the untagged `EXISTS` count, falling back to a
  local increment only when precise sequence metadata is unavailable.
- IMAP selected-mailbox `COPY` and same-mailbox `MOVE` now also prefer
  backend-returned destination message sequence numbers for untagged `EXISTS`
  counts, falling back to local increments only when precise metadata is
  unavailable.
- IMAP selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE`
  now adjust saved SEARCHRES `$` sequence numbers the same way explicit
  `EXPUNGE` commands do, keeping subsequent `$` sequence-set reuse aligned with
  the client-visible mailbox state.
- IMAP `CREATE`, `DELETE`, and `RENAME` delegate to the service folder
  boundary for authenticated flat user-mailbox management, resolving wire names
  before destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- IMAP `CREATE INBOX` and `DELETE INBOX` return explicit RFC 3501-shaped `NO`
  failures, and `RENAME INBOX` is rejected instead of incorrectly routing it
  through generic mailbox rename before its required special "move messages and
  leave INBOX empty" semantics are implemented.
- IMAP `EXAMINE` setup failures now return `NO EXAMINE failed` instead of
  `NO SELECT failed`, keeping tagged failure responses aligned with the command
  clients actually issued.
- IMAP malformed `UID` subcommands now route to their specific handlers when
  the subcommand is recognized, so incomplete `UID SEARCH`, `UID FETCH`,
  `UID STORE`, `UID EXPUNGE`, and `UID COPY` requests receive precise tagged
  `BAD` responses instead of a generic `UID command not implemented` failure.
- IMAP missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`, `DELETE`,
  and `RENAME` now return tagged `[NONEXISTENT]` response codes instead of
  generic command failures, making absent folder state machine-readable for
  standards-aware clients.
- IMAP selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`, and
  `EXPUNGE` now reject extra arguments with tagged `BAD` responses instead of
  ignoring malformed input; this prevents accidental destructive expunge work
  from malformed `EXPUNGE` commands.
- IMAP any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT` now
  reject extra arguments with tagged `BAD` responses instead of silently
  ignoring them or ending the session for malformed logout attempts.
- IMAP `STATUS` now requires a parenthesized status item list, rejecting
  malformed `STATUS mailbox MESSAGES`-style requests before mailbox metadata
  lookup.
- IMAP command dispatch now rejects malformed tags containing atom-special
  characters with untagged `BAD` responses before command handling, avoiding
  ambiguous tagged replies for invalid client command tags.
- IMAP command parsing now rejects control characters inside unquoted atoms,
  aligning atom parsing with the existing quoted-string control-character
  guardrail before command dispatch.
- IMAP supports `STARTTLS` on plaintext listeners with configured TLS and stops
  advertising it after upgrade.
- IMAP `STARTTLS` completion includes an updated `[CAPABILITY ...]` response
  code for the post-TLS command surface.
- IMAP advertises `LOGINDISABLED` and rejects plaintext `LOGIN`/`AUTHENTICATE`
  with `[PRIVACYREQUIRED]` when insecure auth is disabled before STARTTLS.
- IMAP `CAPABILITY` drops `AUTH=PLAIN` after authentication, and unsupported
  command literal tokens are rejected instead of being treated as ordinary
  atoms. Bounded synchronizing command literals are consumed with a
  continuation response.
- IMAP now advertises `LITERAL+` and accepts bounded non-synchronizing command
  literals such as `APPEND ... {n+}` without an extra continuation round trip,
  while preserving the existing synchronizing literal path for conservative
  clients.
- IMAP command reading now supports bounded literals in non-final command
  positions and multiple literals in one command, so RFC-shaped literalized
  credentials or string arguments parse consistently with the advertised
  `LITERAL+` capability.
- IMAP `AUTHENTICATE PLAIN` supports the standard continuation response,
  RFC-shaped tagged `BAD` cancellation, and SASL PLAIN credential decoding over
  the existing protocol auth adapter. Non-empty SASL PLAIN authorization
  identities are accepted only when they match the authentication identity,
  preventing delegated auth requests from being silently ignored until the
  backend contract explicitly supports them. Failed `LOGIN` and
  `AUTHENTICATE` attempts include RFC 5530 `[AUTHENTICATIONFAILED]` response
  codes for client-readable auth diagnostics.
- IMAP advertises `SASL-IR` before authentication and accepts
  `AUTHENTICATE PLAIN` initial responses to reduce client auth round trips.
- `gogomail --mode=imap` initializes the service-backed IMAP store adapter,
  a process-local mailbox event broker for future IDLE/session fan-out, and the
  configured TCP protocol listener.
- `gogomail --mode=imap` now runs a dedicated Redis consumer group for
  committed `mail.stored` events and publishes UID-bearing `EXISTS` updates
  into the process-local mailbox event broker for live IDLE sessions.
- Runtime config now includes validated `GOGOMAIL_IMAP_ADDR` listener metadata
  for the IMAP protocol listener.
- EML parser guardrails include a truncation-probe test and benchmark for the
  bounded text-body reader on large bodies, plus a `MaxParts` cap that reports
  `PartsTruncated` for pathological MIME part counts, plus address/reference
  metadata caps for oversized headers.
- `internal/message` exposes a bounded streaming MIME-structure parser that
  walks multipart trees, preserves raw transfer-encoding metadata, counts body
  octets/lines, and avoids retaining attachment payloads for future IMAP
  `BODYSTRUCTURE` serialization.
- EML attachment detection records inline parts with filenames and non-text
  inline parts from headers only, improving `has_attachment` accuracy without
  reading attachment bodies.
- Push notification worker boundary: `mail.stored` can be consumed by a
  dedicated notification worker with a replaceable sink and a bounded Postgres
  device-target resolver plus candidate-attempt persistence.
- Push notification attempts are inspectable through the Admin API without
  introducing vendor push delivery as a required runtime dependency.
- Push notification device storage: authenticated users can register, list, and
  delete active device tokens through the Mail API while responses expose only a
  short token suffix. Registration normalizes user, platform, token, and label
  fields before validation/storage.
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
  totals for an explicitly bounded ledger window, preparing idempotent
  downstream export workflows.
- API usage export artifact metadata is now persisted and inspectable through
  Admin API endpoints, preparing object-store handoff without adding a vendor
  dependency to the core service.
- API usage export manifests now have canonical SHA-256 digest generation,
  local-HMAC/local-Ed25519/remote-Ed25519 signing, and verification Admin API
  endpoints, tightening the audit trail before invoice-grade signer deployment.
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
- Domain policy service lookups trim domain and user identifiers before
  repository policy reads for outbound and attachment enforcement.
- Direct multipart attachment uploads now distinguish over-limit HTTP request
  envelopes from malformed multipart bodies, returning 413 for the former and
  preserving 400 for bad multipart syntax.
- Attachment upload reservation and direct-upload service requests normalize
  user, draft, filename, MIME type, and storage-path metadata before quota,
  storage, and repository work, and reject CR/LF-bearing or oversized user,
  draft, and upload-session identifiers before quota reservation, object
  writes, or repository work.
- Stale attachment-upload cleanup validates its time window and limit at the
  service boundary before repository cleanup/object deletion work, and app
  configuration validates the worker interval, stale age, and batch size before
  runtime. Stored-object delete failures are now surfaced to the caller while
  missing objects are treated as already-cleaned idempotent deletes. Cleanup
  batch sizes use an attachment-specific 1000-row cap instead of the smaller
  message-list pagination cap.
- Admin API exposes `POST /admin/v1/attachment-cleanup/runs` for authenticated
  on-demand stale upload cleanup with an explicit non-future RFC3339 cutoff,
  and supports `dry_run` preview responses with total and batch-limited
  candidate counts before destructive cleanup. Cleanup run responses also
  report upload-session candidate and expired counts so operator dry-runs match
  the background worker's full cleanup scope. Operators can also list bounded
  legacy attachment-upload candidates plus stale upload-session candidates through
  `POST /admin/v1/attachment-cleanup/candidates`.
- Mail API exposes `DELETE /api/v1/attachments/{id}` so users can cancel
  unbound pending uploads immediately, releasing quota and removing any stored
  upload object without waiting for stale cleanup. Draft binding and send
  handoff ignore canceled/deleted uploads, and canceling a draft-bound upload
  clears the draft binding while refreshing the draft attachment-state cache.
- Mail API exposes `GET /api/v1/attachments/capabilities` so clients can
  discover upload limits, supported modes, and resumable-upload readiness
  without hard-coded constants.
- ADR 0007 records the future resumable/chunked upload boundary around explicit
  upload sessions, quota reservation, storage adapters, final attachment rows,
  and bounded cleanup.
- A migration now creates `attachment_upload_sessions` with lifecycle status,
  declared/received byte counts, expiry, checksum, storage adapter metadata, and
  indexes for user lookup and stale-session cleanup.
- `maildb` can create upload session records and reserve declared session bytes
  in the shared quota ledger transactionally.
- `maildb` can cancel resumable upload sessions in `pending`, `uploading`, or
  `failed` state, releasing the declared byte reservation once.
- `maildb` can expire stale resumable upload sessions in bounded batches,
  marking them `expired` and releasing declared quota reservations.
- `maildb` can count stale resumable upload sessions with the same normalized
  cleanup batch cap used by expiry, supporting non-destructive Admin previews.
- `maildb` can list bounded stale resumable upload-session candidates for
  operator cleanup previews without mutating quota reservations.
- Admin API can list bounded attachment upload sessions by user, draft, and
  lifecycle status, giving operators a direct inspection surface before cleanup
  or user-support actions.
- `mailservice` now owns resumable upload session create/cancel/expire methods,
  preserving attachment validation, max-size checks, and domain attachment
  policy enforcement above the repository boundary.
- `attachment-cleanup-worker` now expires stale resumable upload sessions during
  its normal bounded sweep, releasing reserved quota alongside stale direct
  upload cleanup.
- Mail API now exposes resumable upload session create/read/cancel endpoints under
  `/api/v1/attachments/upload-sessions`, reserving declared quota at session
  creation while keeping chunked upload capability disabled until upload/finalize
  routes land. Session creation rejects already-expired `expires_at` values
  before quota reservation.
- Attachment upload capabilities now distinguish upload session availability
  from full resumable chunk support so generated clients can adopt the staged
  lifecycle without assuming chunk receive/finalize routes exist, and expose
  the maximum upload session TTL.
- Mail API can store a complete body for an upload session, persisting it under
  session-scoped storage and recording received bytes plus SHA-256 digest before
  finalize creates the normal attachment row.
- Upload session body replacement writes retries to distinct staged object paths
  before repository metadata updates, preserving the previously recorded body if
  the DB update fails and best-effort deleting the previous staged body after a
  successful replacement.
- Upload-session staged object paths are validated as relative
  `upload-sessions/` keys before repository persistence and again before
  service-side storage reads/deletes, reducing risk from corrupted or manually
  edited rows.
- Upload session body storage can verify an optional client-provided
  `X-Content-SHA256` digest before recording the staged body.
- Upload session body storage now rejects repeated `Content-Range` or
  `X-Content-SHA256` control headers before reading or storing the body.
- Attachment upload capabilities now advertise upload session checksum
  precondition support separately from body storage and finalization support.
- Upload session finalization now converts a ready stored session body into the
  normal pending attachment row without double-reserving quota, and marks the
  session finalized.
- Upload session finalization now verifies the staged object exists and still
  matches the recorded size and SHA-256 before creating the attachment row.
- Upload session cancellation now deletes a staged session body when the
  canceled session has already written one, preventing storage leaks alongside
  quota release.
- Upload session expiry now also deletes staged session bodies after the
  repository marks sessions expired and releases quota.
- Attachment list/download and draft-delete service methods trim user, message,
  attachment, and draft identifiers before repository/storage work; attachment
  reads reject blank, CR/LF-bearing, or oversized message/attachment
  identifiers before repository/storage dispatch.
- Mail API path identifiers and direct-upload `draft_id` fields are trimmed at
  the HTTP boundary before service dispatch, and direct multipart uploads reject
  repeated `draft_id` or `file` parts before storage work begins.
- Mail API search query, folder, sender, and subject filters are trimmed at the
  HTTP and service boundaries before search backend dispatch, and service
  search validation rejects CR/LF-bearing or oversized query/filter fields
  before Postgres or OpenSearch work.
- Mail compose draft/save/send requests normalize user/source/from/address and
  attachment identifier fields at the service boundary before repository,
  storage, suppression, and outbound composition work; draft saves share the
  send-time attachment-count cap so oversized compose payloads cannot drift
  into draft storage, and from/subject plus recipient display names/emails
  reject CR/LF before draft persistence or outbound header composition.
- Draft save/delete/send and reply/forward compose validation reject blank,
  CR/LF-bearing, or oversized draft/source-message identifiers before
  repository dispatch.
- Single-message flag, move, and delete service methods trim user/message/flag
  and folder identifiers before repository mutation and IMAP event fan-out, and
  reject blank, CR/LF-bearing, or oversized message/folder identifiers before
  repository or IMAP UID lookup work.
- Bulk flag, move, and delete service methods also trim user/message/flag and
  folder identifiers before repository mutation, IMAP UID lookup, and mailbox
  event fan-out; bulk message and folder identifiers reject CR/LF and oversized
  values before database query construction.
- Folder, message-list, thread-list, and message-detail service reads trim
  user, folder, thread, message, and folder-name inputs before repository work;
  user folder create/rename rejects blank, path-bearing, CR/LF-bearing, or
  oversized names, and rename/delete reject unsafe folder identifiers before
  repository dispatch. Folder list/create/rename/delete now also reject
  CR/LF-bearing or oversized user identifiers before repository work.
  Folder-scoped message lists and thread-message reads also reject unsafe
  folder/thread identifiers before repository work.
- Message, thread, and push-device list service methods normalize list limits
  to the documented message-list bounds before repository work.
- Message-list cursor decoding rejects oversized opaque cursor strings before
  base64 decode and JSON parsing.
- IMAP service methods trim user/mailbox identifiers and normalize list/backfill
  limits before repository, storage, broker, or mailbox-event work.
- Mail search service queries normalize user, text, folder, sender, subject,
  and sort inputs before Postgres or OpenSearch dispatch.
- Message delivery-status and reply source-thread service lookups trim user,
  message, and source-message identifiers before repository work.
- Admin API domain query identifiers for user listing, DKIM key listing, and
  delivery-route resolution are trimmed at the HTTP boundary before service
  dispatch.
- Admin API DKIM key deactivate and DNS-verify path identifiers are trimmed at
  the HTTP boundary before service dispatch and response envelopes.
- Admin API suppression-list and trusted-relay delete path identifiers are
  trimmed at the HTTP boundary before service dispatch and response envelopes.
- Admin API trusted relay listing now supports bounded CIDR and description
  filters so operators can inspect inbound relay policy without client-side
  full-list scans.
- Admin API company, domain, and user quota/status/policy mutation path
  identifiers are trimmed at the HTTP boundary before service dispatch and
  response envelopes.
- Admin API outbox event topic, partition key, and status filters are trimmed
  at the HTTP boundary before operational queue inspection, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin API delivery-attempt status and recipient-domain filters are trimmed at
  the HTTP boundary before retry/bounce inspection, and CR/LF-bearing or
  oversized filter values are rejected before service dispatch.
- Admin API push-notification attempt and stats filters are trimmed at the HTTP
  boundary before device/provider troubleshooting queries, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin push-notification attempt/stats repository filters also reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- OpenAPI drift tests now pin the push-device list `limit` query parameter so
  generated clients keep pagination controls for device management.
- OpenAPI drift tests now pin attachment reservation/direct-upload HTTP 413
  error responses for size-cap failures.
- Mail and Admin API JSON request handlers now reject trailing JSON tokens and
  unknown object fields before service dispatch, and common JSON request
  decoding is capped at 1 MiB before parsing.
- Mail and Admin API JSON mutation bodies now require `Content-Type:
  application/json`, accepting normal media-type parameters such as
  `charset=utf-8` but rejecting missing, repeated, or non-JSON content types
  before dispatch.
- Mail API read and bodyless mutation routes now reject request bodies and
  `Content-Type` headers before dispatch, preventing ignored JSON or multipart
  metadata on resource reads, deletes, draft-send, upload-session finalization,
  capability discovery, downloads, and push-device list/delete operations.
- Admin GET/DELETE routes and bodyless Admin POST commands now reject request
  bodies and `Content-Type` headers before dispatch, preventing ignored payloads
  on operator reads, deletes, route verification, retry, IMAP UID backfill,
  API-usage export-batch creation, and manifest digest/signature creation.
- Health and service-info GET routes now reject request bodies and
  `Content-Type` headers before writing probe or contract metadata responses,
  keeping bodyless read semantics consistent across HTTP surfaces.
- Health and service-info GET routes now also reject unknown query parameter
  names, making release probe and metadata endpoint typos visible as HTTP 400
  instead of silently ignored inputs.
- Admin bodyless command/delete routes for IMAP UID backfill, DKIM DNS verify,
  outbox retry, DKIM deactivation, suppression deletion, trusted-relay
  deletion, and delivery-route deletion now reject unknown query parameter names
  before dispatch, preventing ignored `dry_run`/`force`-style operator flags.
- Admin JSON mutation routes for tenant quotas, domain/user lifecycle and
  policy, backpressure, attachment cleanup, quota correction, push outcomes,
  trusted relays, delivery routes, and DKIM keys now reject unknown query
  parameter names before dispatch.
- Mail JWT and Admin token authentication now reject repeated credential
  headers, and Admin routes reject mixed `X-Admin-Token` plus bearer credentials
  before dispatch.
- Mail and Admin API scalar query parameters now reject duplicate values before
  dispatch, preventing ambiguous user IDs, list limits, booleans, timestamps,
  and operational filters from being interpreted by first-value wins behavior.
- Mail API read/search/list, draft-search, attachment capability/session/download,
  and push-device list routes now reject unknown query parameter names before
  dispatch, making generated-client typos visible as HTTP 400 responses.
- Mail API mutation routes now reject unknown query parameter names before
  dispatch, and JSON-backed compose/draft/attachment/send mutations honor the
  documented development-only `user_id` query fallback when JWT auth is
  disabled.
- Admin company/domain/DNS-check/user list routes now reject unknown query
  parameter names before dispatch, keeping core operator filters aligned with
  the documented contract.
- Admin API usage aggregate, ledger, retention, export-batch, artifact,
  manifest-digest, and manifest-signature routes now reject unknown query
  parameter names before dispatch, including unexpected query strings on
  detail, download, verification, and mutation routes with no query controls.
- Admin queue, outbox, audit, backpressure, quota, attachment-session,
  delivery-attempt, push-notification, suppression-list, trusted-relay,
  delivery-route, and DKIM read routes now reject unknown query parameter names
  before dispatch.
- API error responses now use `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`, with the reusable OpenAPI error response
  documenting both headers for generated clients.
- Successful Mail/Admin JSON, health, and service-info responses now return
  `X-Content-Type-Options: nosniff`, aligning browser-visible envelopes with
  error, NDJSON, and download response hardening.
- Successful Mail/Admin JSON responses now return `Cache-Control: no-store`
  through the shared writer so sensitive message, audit, usage, and control
  envelopes are not cached.
- Attachment download responses now emit both ASCII fallback and UTF-8
  `filename*` `Content-Disposition` parameters for internationalized filenames,
  with stored filenames bounded before response headers are written.
- Attachment downloads now fall back to `application/octet-stream` for blank,
  unsafe, or media-type-invalid stored MIME types before setting response
  headers.
- OpenAPI now documents attachment download `Content-Disposition` and
  `Cache-Control: no-store` headers with drift coverage.
- API usage artifact downloads now sanitize stored content type and SHA-256
  response headers before streaming export objects, including media-type
  validation before the response `Content-Type` is written.
- API usage outbox production now rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before inserting durable usage events.
- API usage aggregate storage now applies the same route-key, identity, event-id,
  schema-version, and HTTP-like status validation when called directly by
  internal adapters.
- API usage NDJSON exports and stored export artifact downloads now return
  `Cache-Control: no-store`, with OpenAPI drift coverage.
- Attachment downloads, usage NDJSON exports, and stored export artifact
  downloads now return `X-Content-Type-Options: nosniff`, with OpenAPI drift
  coverage.
- Successful JSON responses now return `X-Content-Type-Options: nosniff` across
  Mail, Admin, health, and service-info routes.
- Successful Mail/Admin JSON envelopes now use `Cache-Control: no-store` through
  the shared writer.
- Mailservice now validates DB-returned message and attachment storage object
  paths before body reads or deletes, preventing corrupted rows from reaching
  the storage adapter with absolute, traversal, newline, backslash-bearing, or
  oversized keys.
- Local storage now shares the strict object-path validator used by mailservice,
  rejecting non-canonical, oversized, duplicate-separator, or dot-segment keys
  at the adapter boundary before reads, writes, or deletes.
- IMAP read-only selected-state mutation commands now let malformed
  `STORE`/`MOVE`/`UID STORE`/`UID MOVE`/`UID EXPUNGE` requests return
  command-specific tagged `BAD` responses before valid mutations are rejected
  with `NO mailbox is read-only`, including invalid UID/sequence sets, STORE
  modes/flags, and modified UTF-7 destination mailbox names.
- IMAP mailbox rename handling now rejects attempts to rename any mailbox to
  `INBOX`, keeping the special INBOX namespace out of generic folder mutation
  paths.
- IMAP command dispatch now validates command and UID subcommand atoms before
  routing, so atom-special-bearing command names are rejected as malformed
  syntax instead of falling through as unknown commands.
- IMAP `UID` dispatch now validates missing, malformed, unknown, or
  state-independent malformed subcommands before authentication or
  selected-mailbox state, while valid unauthenticated UID commands still return
  `NO authentication required`.
- Authenticated selected-state IMAP commands now validate obvious malformed
  `FETCH`, `STORE`, `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax
  before returning `NO mailbox must be selected` for valid commands issued
  outside selected state.
- Selected-state action commands now also validate malformed `FETCH`, `STORE`,
  `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox names before
  authentication failures, while valid unauthenticated commands still return
  `NO authentication required`.
- Search-oriented selected-state commands now validate malformed `SEARCH`,
  `SORT`, and `THREAD` argument shape, return options, and sort/thread
  argument lists before authentication failures, while valid unauthenticated
  commands still return `NO authentication required`.
- Selected-state no-argument commands now reject extra arguments on `CHECK`,
  `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before returning authentication
  or selected-mailbox state errors for well-formed commands.
- IMAP `STARTTLS` now rejects extra arguments before TLS availability or
  authentication-state checks, preserving no-argument command syntax diagnostics
  during capability probing.
- IMAP `UID` dispatch validates subcommand arity and destination mailbox-name
  syntax for `FETCH`, `STORE`, `EXPUNGE`, `COPY`, and `MOVE` before
  authentication or selected-mailbox state, while leaving selected-state-
  dependent UID set resolution to the selected command handlers.
- IMAP `LOGIN` and `AUTHENTICATE` now validate malformed argument shape or
  unsupported mechanisms before returning `[PRIVACYREQUIRED]` on plaintext
  TLS-required listeners.
- IMAP mailbox management and subscription commands now validate malformed
  `LIST`, `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and
  `UNSUBSCRIBE` argument shape or modified UTF-7 mailbox names before
  authentication failures, while valid unauthenticated commands still return
  `NO authentication required`.
- IMAP selected-mailbox discovery commands now validate malformed `NAMESPACE`,
  `SELECT`, `EXAMINE`, and `STATUS` argument shape, CONDSTORE options, status
  item lists, or modified UTF-7 mailbox names before authentication failures,
  while valid unauthenticated commands still return `NO authentication
  required`.
- IMAP `APPEND` now validates missing literals, malformed append options, and
  modified UTF-7 mailbox names before authentication failures, while valid
  unauthenticated appends still consume the RFC literal and return
  `NO authentication required` before backend storage.
- IMAP `ENABLE` now validates missing capability arguments before
  authentication failures, while valid unauthenticated enable attempts still
  return `NO authentication required` without mutating session feature state.
- Backend release verification now fails when standard tests leave pending
  repository changes behind, while local OpenChrome session artifacts are
  ignored as developer-machine state.
- Mail API attachment downloads now support `HEAD` metadata probes, validating
  the same message/attachment/storage-object boundary as `GET` and returning
  safe `Content-Disposition`, object-backed `Content-Length`, `no-store`, and
  `nosniff` headers without streaming bytes.
- Drive node copy is now available through `POST /api/v1/drive/nodes/{id}/copy`.
  It copies active files and bounded active folder trees through the configured
  storage adapter, creates quota-accounted metadata with caller-provided
  destination folder/name, exposes `copy_nodes` plus `max_copy_nodes` in
  webmail capabilities, and removes copied objects if DB metadata creation or
  bounded folder-tree copy fails.
- Drive file copy cleanup now records a pending cleanup-failure row if metadata
  creation fails after object copy and the copied object cannot be deleted,
  keeping object-storage drift visible to operator retry/resolve tooling.
- Drive file copies now preallocate the destination node UUID and use that same
  identifier in the copied object's committed storage path and `drive_nodes.id`,
  keeping copy metadata and object keys aligned.
- Drive upload-session finalization, staged-object finalization, and file copy
  now map quota exhaustion to HTTP 507 `insufficient_storage`, giving webmail
  clients a precise storage-pressure response.
- Drive node listing now supports explicit `sort=name|updated|created|size`
  controls on both webmail and admin APIs while preserving folder-first
  ordering for production Drive browser ergonomics.
- Drive node listing now supports `node_type=folder|file` filters on webmail
  and admin APIs, with webmail capabilities advertising supported node types.
- Webmail Drive node listing now accepts `all_parents=true` for whole-user
  Drive search/list views while rejecting ambiguous `parent_id` combinations,
  and webmail capabilities advertise the whole-tree search mode for production
  file pickers and compose-side Drive insertion flows.
- Drive now has a first authenticated share-link metadata boundary:
  `drive_share_links` stores user/file-scoped token hashes, bounded suffixes,
  permissions, status, and expiry; Mail API routes can create, list, and revoke
  links while returning raw share tokens only in the create response.
- CalDAV work has started with ADR 0010, a `caldav` runtime scaffold, and an
  `internal/caldavgw` boundary for RFC/WebDAV standards, DAV tokens, principal
  paths, calendar-home paths, calendar collections, and `.ics` object paths.
- CalDAV storage groundwork now has PostgreSQL `caldav_calendars` and
  `caldav_calendar_objects` tables with user-scoped active uniqueness, ETag,
  sync-token, component, and bounded `.ics` body constraints. `internal/caldavgw`
  also validates calendar metadata, component types, object UIDs, strong ETags,
  and sync-token derivation before WebDAV handlers are exposed.
- CalDAV WebDAV XML parsing groundwork now accepts bounded namespace-aware
  PROPFIND bodies (`allprop`, `propname`, `prop`, and `allprop` `include`),
  parses safe `Depth` header values, and classifies core CalDAV/WebDAV REPORT
  roots (`calendar-query`, `calendar-multiget`, `free-busy-query`, and
  `sync-collection`) with body/property/href/depth limits before handlers are
  advertised.
- CalDAV now has a PostgreSQL repository boundary for calendar create/list/get
  and calendar-object upsert/list/get/soft-delete. Object writes validate `.ics`
  resource names, UID/component metadata, strong ETags, optional observed ETags,
  object-size limits, and bump calendar sync tokens in the same transaction.
- CalDAV object validation now uses `github.com/emersion/go-ical` for RFC 5545
  iCalendar decoding, requiring one `VCALENDAR` with exactly one supported
  top-level calendar component, exactly one bounded UID, and explicit
  component/property count caps before `.ics` bodies reach storage.
- CalDAV now has a WebDAV `multistatus` response builder for future PROPFIND
  and REPORT handlers. It renders per-property `propstat` statuses, principal
  discovery properties, calendar-home hints, calendar collection metadata
  (`supported-calendar-component-set`, `supported-calendar-data`,
  `max-resource-size`, sync token), and calendar-object ETag/content metadata.
- CalDAV now has an internal `OPTIONS`/`PROPFIND` discovery handler boundary
  over a pluggable discovery store. It advertises DAV capabilities, rejects
  unsafe infinite-depth discovery, enforces authenticated user/path scope, and
  can render principal, calendar-home, calendar-collection, and calendar-object
  multistatus responses before the public listener is enabled.
- The PostgreSQL CalDAV repository now satisfies the discovery store boundary,
  including active principal lookup through active user/domain/company scope and
  calendar/object list/get adapters for the internal `PROPFIND` handler.
- CalDAV now has a Basic-auth user resolver that reuses the existing
  authenticated Submission password verifier boundary, requires TLS or an
  HTTPS forwarding signal unless explicitly allowed for development, and
  returns the authenticated user ID for future native CalDAV clients.
- Configuration now includes `GOGOMAIL_CALDAV_ADDR` and
  `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH`, with production validation rejecting
  insecure CalDAV Basic-auth credentials before runtime wiring is enabled.
- `gogomail --mode=caldav` now starts a dedicated HTTP listener using
  `GOGOMAIL_CALDAV_ADDR`, the CalDAV PostgreSQL discovery repository, and the
  Basic-auth resolver. Full CalDAV client-ready compatibility still depends on
  scheduling, recurrence semantics, sync tombstone/change-log support, and
  broader native-client compatibility coverage.
- CalDAV REPORT parsing now validates more protocol shape before handlers run:
  `calendar-query` requires a filter and extracts nested CalDAV time ranges,
  `calendar-multiget` requires bounded hrefs, `free-busy-query` requires a UTC
  single time range, and `sync-collection` requires supported `sync-level=1`
  plus a requested property set and bounded optional `limit`.
- CalDAV now implements a first `REPORT calendar-multiget` handler for
  authenticated calendar collections, returning multistatus object metadata and
  requested `calendar-data` bodies while representing missing hrefs through
  per-resource 404 propstats.
- CalDAV now handles calendar object `GET`, `HEAD`, `PUT`, and `DELETE` over
  authenticated `.ics` object paths. Reads return strong ETags and
  `text/calendar` bodies, writes enforce bounded iCalendar validation and
  `If-Match`/`If-None-Match` preconditions, and deletes honor optional ETag
  preconditions before soft-deleting repository objects.
- CalDAV now handles `REPORT calendar-query` for authenticated calendar
  collections, listing matching `.ics` objects through WebDAV multistatus
  responses and applying RFC 5545-backed VEVENT overlap checks when a CalDAV
  time-range filter is supplied.
- CalDAV now handles a conservative RFC 6578 `REPORT sync-collection` path for
  authenticated calendar collections: empty sync tokens return all active
  objects plus the collection sync token, current tokens return only the
  top-level sync token, stale tokens return a DAV `valid-sync-token`
  precondition error, and truncating limits are rejected until continuation or
  tombstone/change-log semantics exist.
- CalDAV now handles RFC 4791-shaped `REPORT free-busy-query` for authenticated
  calendar collections. `Depth: 1` collects child VEVENT busy periods into a
  `200 OK` `text/calendar` `VFREEBUSY` response, clips periods to the requested
  UTC range, skips `TRANSPARENT` and `CANCELLED` events, maps tentative events
  to `BUSY-TENTATIVE`, ingests stored VFREEBUSY `FREEBUSY` period lists,
  supports UTC start/end and start/duration periods, coalesces same-type
  overlaps, and rejects duplicate free-busy time ranges. Scheduling,
  recurrence expansion, and broader native-client compatibility coverage
  remain incomplete.
- CalDAV now handles `MKCALENDAR` for authenticated calendar collection paths
  whose Request-URI calendar segment is a UUID. The handler parses bounded
  CalDAV/WebDAV creation XML for display name, description, and CalendarServer
  or Apple calendar color, creates the collection at the requested URI, returns
  `201 Created` with `Location`, rejects cross-user paths, existing calendars,
  missing homes, and unsafe non-UUID path ids, and advertises `MKCALENDAR` again
  only because handler semantics now exist. Human-readable slug calendar paths
  remain future path-alias work.
- CalDAV now handles `DELETE` on authenticated calendar collection paths,
  soft-deleting the collection and active child objects in one repository
  transaction while rejecting calendar-home or cross-user deletes. Durable
  tombstone/change-log support is still needed before incremental sync can
  report collection/object deletions to stale-token clients.
- CalDAV now has a durable calendar sync-change table for RFC 6578-style
  `sync-collection` deltas. Calendar create/upsert/delete paths record sync
  markers in the same transaction as object mutations, migrated calendars get a
  baseline marker on first object change, stale-but-known sync tokens can return
  changed object properties or response-level `404 Not Found` tombstones, and
  unknown tokens still fail with DAV `valid-sync-token`. Collection-deletion
  sync for already-deleted collections and long-history retention policy remain
  future work.
- CalDAV now handles RFC 6764-style `/.well-known/caldav` discovery by
  redirecting to `/caldav/`, and `PROPFIND /caldav/` can return
  `current-user-principal`, `principal-collection-set`, and
  `calendar-home-set` for authenticated clients.
- CalDAV now handles WebDAV `PROPPATCH` for authenticated calendar collection
  metadata, using bounded namespace-aware `propertyupdate` parsing and a small
  repository update boundary for `DAV:displayname`,
  `CALDAV:calendar-description`, and CalendarServer/Apple calendar color.
  Updates are transactional, refresh the collection sync token, append a
  `collection-updated` sync marker, and keep calendar objects, scheduling, and
  product-specific policy out of the gateway path.
- CalDAV calendar collection `PROPFIND` now exposes WebDAV
  `supported-report-set` for the REPORT handlers that actually exist today:
  CalDAV `calendar-query`, `calendar-multiget`, `free-busy-query`, and WebDAV
  `sync-collection`. This keeps native-client capability discovery aligned
  with implemented semantics instead of advertising future scheduling or
  recurrence features prematurely.
- CalDAV `REPORT calendar-query` now honors simple top-level component filters
  such as `VEVENT` and `VTODO` by using the repository's stored
  `component_type` metadata before expensive time-range/body work. This keeps
  common client filters from returning unrelated object types while preserving
  the bounded iCalendar parser as the write-time source of truth.
- CalDAV `REPORT calendar-multiget` now scopes href resolution to the request
  resource: collection requests only return objects from that collection, while
  calendar-home requests can fetch the authenticated user's calendar objects
  across collections. Out-of-scope hrefs render WebDAV 404 propstats instead
  of leaking object metadata or `calendar-data`.
- CalDAV `PROPFIND` now returns RFC 4918-shaped `owner`, `creationdate`, and
  `getlastmodified` metadata where the current model can answer them exactly.
  Owners point at the authenticated user's principal URL, creation dates use
  UTC RFC3339 timestamps, and last-modified values use HTTP-date formatting.
- CalDAV calendar object `GET` and `HEAD` now honor `If-None-Match` against
  stored strong ETags, returning `304 Not Modified` with safe cache headers and
  no body when clients already have the current `.ics` representation.
- CalDAV calendar object `PUT` now validates explicit `Content-Type` headers
  before body parsing, accepting `text/calendar` with parameters and rejecting
  incompatible media types with HTTP 415.
- CalDAV calendar object `PUT` now treats `If-Match: *` as an existing-resource
  precondition, returning HTTP 412 when the target `.ics` object does not yet
  exist instead of accidentally creating it.
- CalDAV calendar object `PUT` now evaluates specific `If-Match` and
  `If-None-Match` ETag preconditions before body reads or storage mutation,
  returning HTTP 412 for stale overwrite or no-overwrite requests.
- CalDAV calendar object `GET` and `HEAD` now reject stale `If-Match`
  preconditions before `If-None-Match` revalidation, and `DELETE` accepts
  comma-listed strong ETags through the same comparison helper used by writes.
- CalDAV calendar object `DELETE` now treats `If-Match: *` as an
  existing-resource precondition, returning HTTP 412 for missing `.ics`
  resources instead of surfacing a plain not-found result.
- CalDAV calendar object `GET` and `HEAD` now emit `Last-Modified` from stored
  object update time and honor `If-Modified-Since` revalidation with
  second-precision comparisons, avoiding unnecessary `.ics` body streaming for
  timestamp-valid client caches.
- CalDAV calendar object `PUT` and `DELETE` now honor `If-Unmodified-Since`
  against stored object update timestamps before body reads or repository
  mutation, returning HTTP 412 for stale timestamp-based overwrite/delete
  attempts.
- S3-compatible `GetRange` now caps the returned reader at the validated
  requested byte length even if a provider returns an oversized `206 Partial
  Content` body, matching local/NFS range semantics and keeping partial Drive,
  attachment, and IMAP reads bounded at the storage adapter boundary.
- CalDAV calendar object `GET` and `HEAD` now also honor
  `If-Unmodified-Since` before `If-None-Match` / `If-Modified-Since`
  revalidation, returning HTTP 412 when timestamp preconditions are stale.
- S3-compatible `GetRange` now requires the provider's `Content-Range` header
  to match the requested byte window before exposing the response body, closing
  mismatched partial responses early instead of letting Drive, attachment, or
  IMAP partial-read callers consume the wrong range.
- S3-compatible `GetRange` now reports `io.ErrUnexpectedEOF` when a provider
  returns a matching `Content-Range` header but truncates the response body
  before the requested byte count, making partial-read corruption visible to
  Drive, attachment, and IMAP callers.
- S3-compatible `GetRange` now drains a small bounded remainder on successful
  range-reader close, improving HTTP connection reuse when providers send extra
  partial-response bytes without exposing those bytes to callers.
- S3-compatible `GetRange` now applies the same bounded close-drain behavior
  when callers close a range reader before consuming the full requested window,
  improving connection reuse for preview/cancel paths without unbounded drain
  work.
- IMAP `STATUS` and LIST-STATUS item parsing now rejects duplicate status data
  items before mailbox metadata lookup, avoiding ambiguous duplicate
  client-visible status pairs.
- CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading or
  parsing the XML request body when no active collection already exists at that
  path, keeping the UUID-only creation contract cheap and predictable while
  preserving existing-collection 405 behavior.
- CalDAV calendar collection `DELETE` now honors `If-Unmodified-Since` against
  collection update time and evaluates strong collection ETags derived from
  collection sync state, including comma-listed `If-Match` values and
  `If-Match: *`, before repository mutation.
- CalDAV collection `PROPPATCH` now uses the same collection precondition gate,
  rejecting stale `If-Unmodified-Since` and mismatched collection `If-Match`
  requests before reading XML bodies or updating calendar metadata.
- CalDAV `REPORT` now validates malformed and `Depth: infinity` headers before
  reading XML bodies, applying one shared Depth gate across calendar-query,
  calendar-multiget, sync-collection, and free-busy-query handling.
- CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI `<D:href>` values
  by evaluating only their canonical path component through the same user and
  collection scope checks, while rejecting userinfo-bearing authorities, query,
  fragment, opaque, non-HTTP(S), or unsafe href forms.
- Directory/Identity now has a first protocol-neutral principal resolver under
  `internal/directory`, and CalDAV active principal discovery delegates to it
  instead of owning the user/domain/company active-scope query directly.
- Directory/Identity principal resolution now also supports organization
  principals from the existing organization/domain/company model, preparing
  organization calendars and policy scopes without exposing them publicly yet.
- Directory/Identity storage now has first-class group, resource, alias, and
  group-membership tables plus group/resource principal resolution hooks,
  preparing shared inboxes, resource calendars, delegated access, and admin
  directory workflows without hard-coding those semantics into CalDAV.
- Directory/Identity can resolve normalized alias email addresses to target
  user, organization, group, or resource principals, with active alias
  uniqueness enforced at the normalized address boundary.
- Directory/Identity can check direct active group membership across user,
  organization, group, and resource principals, establishing the first
  auditable membership read boundary before recursive/effective delegation is
  implemented.
- Directory/Identity also has a bounded effective-membership check that expands
  nested groups with an explicit recursion cap and cycle guard, preparing
  delegated access and resource policy evaluation without unbounded graph
  traversal.
- CardDAV groundwork has started with ADR 0012 and `internal/carddavgw`, which
  owns RFC/WebDAV/CardDAV standards names, DAV capability tokens, canonical
  principal/address-book/contact-object paths, `.vcf` resource validation, and
  safe relative or HTTP(S) absolute href parsing.
- CardDAV storage groundwork now has PostgreSQL `carddav_addressbooks`,
  `carddav_contact_objects`, and `carddav_addressbook_changes` tables with
  user-scoped active uniqueness, strong ETag, sync-token, status, size, and
  `.vcf` body constraints. `internal/carddavgw` also validates address-book
  metadata, contact object names/UIDs, strong ETags, object-size limits, and
  sync-token derivation before repository methods are exposed.
- CardDAV address-book repository methods now create/list/get collections
  behind active user/domain/company scope, normalize names, bound list limits,
  and insert durable `addressbook-created` change rows in the create
  transaction.
- CardDAV vCard validation now performs bounded RFC 6350-oriented checks for
  vCard 4.0 contact objects, including BEGIN/END structure, exactly one
  VERSION, required UID/FN, folded content-line handling, line/body caps, and
  nested VCARD rejection.
- CardDAV contact-object repository methods now upsert/list/get/delete active
  `.vcf` objects under active address-book scope, enforce vCard UID alignment,
  compute strong ETags, honor optional observed ETags before overwrite, refresh
  address-book sync tokens, and record `contact-upserted`/`contact-deleted`
  changes transactionally.
- CardDAV REPORT parsing now recognizes bounded `addressbook-query`,
  `addressbook-multiget`, and WebDAV `sync-collection` request bodies,
  collecting requested properties, hrefs, sync token/level, limits, and first
  text-match filter plus its enclosing `prop-filter` name while rejecting
  malformed, oversized, deeply nested, invalid prop-filter, or unsupported
  sync-level shapes before handlers are exposed.
- CardDAV now has a WebDAV `multistatus` response builder for future PROPFIND,
  REPORT, and sync handlers. It renders principal discovery, address-book
  collection metadata, contact-object metadata, requested `address-data`,
  supported reports, supported vCard data types, sync tokens, and per-property
  404 propstats.
- CardDAV now has an internal RFC 6764/WebDAV-style discovery handler for
  `/.well-known/carddav`, `OPTIONS`, and `PROPFIND` over root, principal,
  address-book home, address-book collection, and contact-object resources.
  It rejects cross-user paths, `Depth: infinity`, malformed WebDAV XML, and
  contact-object `PROPFIND` above `Depth: 0`; the PostgreSQL repository
  satisfies the discovery store by delegating active user principal lookup to
  the shared Directory resolver. This remains backend-only until auth/listener
  wiring, REPORT execution, object mutation, and native-client compatibility
  are implemented and tested.
- CardDAV now executes the three parsed REPORT shapes internally:
  `addressbook-multiget` returns requested contact metadata and optional
  `address-data` with per-href 404 propstats, `addressbook-query` scans active
  address-book objects with the current bounded first text-match filter, and
  `sync-collection` returns full snapshots or bounded change rows with root
  sync-token emission and deleted contact 404 responses. Query filtering now
  applies the first `text-match` to the parsed unfolded vCard property named by
  the enclosing `prop-filter`, falling back to whole-body matching only when no
  property is present. The first text-match evaluator honors the RFC 6352
  default `i;unicode-casemap` collation, rejects unsupported collations instead
  of silently changing semantics, and supports `equals`, `contains`,
  `starts-with`, `ends-with`, and `negate-condition`. The repository can
  list address-book changes since a stored sync token and rejects missing or
  unsafe sync tokens before SQL work. This still does not advertise public
  native-client compatibility because broader CardDAV filter trees,
  param-filter semantics, broader vCard compatibility, and client
  compatibility tests are still pending.
- CardDAV now handles contact-object `GET`, `HEAD`, `PUT`, and `DELETE` inside
  the internal handler. Reads emit `text/vcard; charset=utf-8`, strong ETags,
  content length, no-store headers, and `Last-Modified`, while honoring
  `If-Match`, `If-None-Match`, `If-Modified-Since`, and
  `If-Unmodified-Since`. Writes accept `text/vcard`, enforce bounded body
  reads, reuse vCard validation and observed-ETag repository guards, and map
  create/update/delete to standard 201/204/precondition outcomes. This remains
  backend-only until auth/listener wiring and native-client compatibility tests
  are in place.
- CardDAV now has Basic-auth and runtime wiring. `gogomail --mode=carddav`
  opens a dedicated HTTP server on `GOGOMAIL_CARDDAV_ADDR` (default `:8082`),
  reuses the Submission password verifier through `internal/carddavgw`, and
  rejects insecure Basic auth in production through
  `GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH=false`. This enables deployment smoke
  testing of the CardDAV gateway while keeping public/client-ready status gated
  on broader filter trees/param-filters, vCard compatibility, and native-client
  verification.
- Admin Drive node listing now accepts `all_parents=true` for whole-user Drive
  inventory search while rejecting ambiguous `parent_id` combinations.
- Drive file finalize, upload-session cleanup/retry-body replacement,
  permanent-delete cleanup, cleanup-failure retry, download, and copy paths now
  enforce that stored object keys remain under the owning user's
  `drive/users/{user_id}/...` prefix before storage adapter access, tightening
  tenant isolation at the storage boundary.

Next focus areas:

1. Keep draft search separate from `GET /api/v1/search` until an explicit draft
   search contract and indexing path are added.
2. Extend the quota ledger to future Drive writes and large share-link objects.
3. Wire mailbox event publication from append/flag/move/delete paths behind the
   IMAP gateway boundary.
4. Add FCM/APNs/Web Push sink adapters and invalid-token cleanup behind the push
   notification worker.
5. Deploy the remote-Ed25519 signer behind an approved KMS service, or add a
   direct cloud KMS adapter, before using API usage batches for invoices or hard
   limits.
6. Frontend planning and API contract review before webmail implementation.
7. Extend Directory/Identity from stored users, organizations, groups,
   resources, aliases, group memberships, and bounded membership expansion into
   explicit delegated principal relationships before public shared-calendar or
   resource-booking CalDAV features.
8. Extend CardDAV from internal discovery into authenticated client workflows:
   add broader CardDAV filter-tree/param-filter semantics, vCard compatibility, and
   native-client tests before webmail contacts, attendee auto-complete, or
   public native CardDAV compatibility are exposed.
