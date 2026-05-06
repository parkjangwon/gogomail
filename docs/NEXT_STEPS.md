# gogomail next steps

This file is the short task handoff for future coding agents.

## Read first

Before changing code, read:

1. `AGENTS.md`
2. `docs/CURRENT_STATUS.md`
3. `docs/backend-roadmap.md`
4. `docs/backend-api-contracts.md`
5. `docs/backend-release-readiness.md`
6. `DESIGN.md`
7. `docs/openapi.yaml`
8. `docs/storage-backends.md`
9. recent `git log --oneline`

## Immediate backend priorities

### 0. Storage portability

Current state:

- IMAP read-only selected-state mutation handling now validates malformed
  `STORE`, `MOVE`, `UID STORE`, `UID MOVE`, and `UID EXPUNGE` requests before
  returning read-only `NO` failures for syntactically valid mutations,
  including invalid UID/sequence sets, STORE modes/flags, and destination
  mailbox names.
- IMAP mailbox mutation handling rejects `CREATE INBOX`, `DELETE INBOX`,
  `RENAME INBOX ...`, and `RENAME ... INBOX`, keeping the special INBOX
  namespace out of generic folder mutation paths.
- IMAP command dispatch validates command and UID subcommand atoms before
  routing so malformed atom-special-bearing command names do not fall through
  as unknown commands.
- IMAP command parsing returns tagged `BAD` for malformed command lines when
  the command tag is still syntactically recoverable, while malformed or
  missing tags continue to receive untagged `BAD`.
- `UID` dispatch validates missing, malformed, unknown, or state-independent
  malformed subcommands before authentication or selected-mailbox state, while
  valid unauthenticated UID commands still return `NO authentication required`.
  Bare `UID` commands return `BAD UID requires subcommand` instead of looking
  like an unsupported implemented command family.
- Authenticated selected-state commands validate malformed `FETCH`, `STORE`,
  `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax before returning
  selected-mailbox state errors for valid commands.
- IMAP `SELECT` and `EXAMINE` now require optional `CONDSTORE` select
  parameters to use the RFC-shaped parenthesized select-param list, rejecting
  bare `CONDSTORE` and over-parenthesized `((CONDSTORE))` before authentication
  or backend mailbox lookup.
- Selected-state action commands also validate malformed `FETCH`, `STORE`,
  `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox names before
  authentication failures, while well-formed unauthenticated commands still
  return `NO authentication required`.
- Search-oriented selected-state commands validate malformed `SEARCH`, `SORT`,
  and `THREAD` argument shape, return options, and sort/thread argument lists
  before authentication failures, while well-formed unauthenticated commands
  still return `NO authentication required`.
- Selected-state no-argument commands validate extra arguments on `CHECK`,
  `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before returning authentication
  or selected-mailbox state errors.
- `STARTTLS` validates its no-argument syntax before TLS availability and
  authentication-state checks.
- `UID` dispatch validates subcommand arity and destination mailbox-name syntax
  before authentication or selected-mailbox state for `FETCH`, `STORE`,
  `EXPUNGE`, `COPY`, and `MOVE`.
- `LOGIN` and `AUTHENTICATE` validate malformed argument shape before
  plaintext `[PRIVACYREQUIRED]` responses on TLS-required listeners, while
  syntactically valid but unsupported SASL mechanisms return tagged `NO`
  responses so probing clients can fall back cleanly.
- SASL PLAIN decoding rejects oversized encoded and decoded responses before
  credential splitting or backend authentication, keeping `AUTHENTICATE PLAIN`
  continuation and `SASL-IR` literal paths bounded.
- Successful `LOGIN` and `AUTHENTICATE PLAIN` responses now include the
  authenticated `[CAPABILITY ...]` response code, keeping post-auth capability
  discovery explicit for RFC-shaped clients.
- Connection greetings now include a state-aware `[CAPABILITY ...]` response
  code: plaintext TLS-required sessions expose `STARTTLS`/`LOGINDISABLED`,
  while implicit TLS sessions expose immediate `SASL-IR`/`AUTH=PLAIN`.
- Mailbox management and subscription commands validate malformed `LIST`,
  `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and `UNSUBSCRIBE`
  argument shape or modified UTF-7 mailbox names before authentication
  failures, while well-formed unauthenticated commands still return
  `NO authentication required`.
- Selected-mailbox discovery commands validate malformed `NAMESPACE`, `SELECT`,
  `EXAMINE`, and `STATUS` argument shape, CONDSTORE options, status item lists,
  or modified UTF-7 mailbox names before authentication failures, while
  well-formed unauthenticated commands still return `NO authentication
  required`.
- `APPEND` validates missing literals, malformed append options, and modified
  UTF-7 mailbox names before authentication failures, while well-formed
  unauthenticated appends still consume the RFC literal and return
  `NO authentication required` before backend storage.
- `ENABLE` validates missing capability arguments before authentication
  failures, while well-formed unauthenticated enable attempts still return
  `NO authentication required` without mutating session feature state.
- `ENABLE` also validates malformed capability atoms before authentication or
  session mutation, keeping RFC 5161 syntax errors separate from unsupported
  but well-formed capabilities.
- `ENABLE` preserves RFC 5161-compatible unknown capability handling:
  unsupported but syntactically valid capability names are ignored and can
  produce an empty `ENABLED` response when no requested capability is enabled.
- Storage backend portability now has a shared contract test that exercises
  special but canonical object keys and the full object lifecycle across local
  storage and optional S3-compatible integration coverage; use it as the smoke
  matrix before local/NFS, MinIO, or AWS S3 backend flips.
- `LIST`/`LSUB` CHILDREN attributes infer immediate parents from nested
  `FullPath` values when backend rows do not carry `ParentID`, preserving
  `\HasChildren` metadata for deeper hierarchies such as `Projects/2026/Jan`.
- `APPEND`, `STORE`, and `UID STORE` flag-list parsing rejects unparenthesized
  or unbalanced flag lists instead of silently trimming stray parentheses.
- `STORE` and `UID STORE` honor selected-mailbox `[PERMANENTFLAGS]`, rejecting
  otherwise valid system flags when the mailbox did not advertise them as
  permanent before backend mutation dispatch. Empty add/remove flag lists stay
  no-ops, while empty replacement is rejected when no permanent flags are
  permitted.
- IMAP message sequence sets reject numbers above the selected mailbox size
  with tagged `BAD` responses, preserving RFC 3501 bounds behavior.
- IMAP quoted-string parsing rejects adjacent tokens after a closing quote and
  unsupported backslash escapes before authentication or backend work, keeping
  command tokenization aligned with RFC 3501 quoted-special handling.
- IMAP mailbox wire-name formatting preserves ordinary internal spacing while
  still collapsing control-character runs, preventing folder list/status
  responses from changing distinct user-visible mailbox names.
- IMAP UID `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` commands resolve
  `*` UID sequence ranges against selected-mailbox UIDs, so common client
  requests such as `UID FETCH 1:*` include the last visible UID without
  expanding through non-existent UID gaps.
- IMAP `SEARCH UID <sequence-set>` and `UID SEARCH UID <sequence-set>` resolve
  `*` UID ranges against the selected mailbox's visible UIDs, aligning
  search-key filtering with UID command range handling.
- IMAP command tag validation rejects `+` in tags before command routing,
  matching RFC 3501 tag grammar and avoiding ambiguity with continuation
  protocol markers.
- IMAP `SEARCH`/`UID SEARCH` date criteria reject malformed date atoms that
  still contain quote characters after command parsing, so broken inputs such
  as `SINCE 05-May-2026"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` date criteria accept one-digit date-day atoms such
  as `SINCE 5-May-2026` while preserving malformed quote rejection, improving
  client compatibility without weakening syntax guardrails.
- IMAP command tokenization rejects embedded quote characters inside unquoted
  atoms while preserving escaped quotes inside proper quoted strings, keeping
  RFC 3501 atom and quoted-string handling separate.
- IMAP parenthesized `SEARCH`/`UID SEARCH` groups reject empty `()` groups
  instead of treating them as match-all, while preserving valid `(ALL)` groups.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` numeric thresholds reject malformed
  values that still contain quote characters after command parsing, so broken
  inputs such as `MODSEQ 20"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types reject malformed atoms that
  still contain quote characters after command parsing, preventing broken
  `MODSEQ "/flags/\\Seen" all" 17` style inputs from being silently normalized.
- IMAP RFC 2971 `ID` parameter-list parsing rejects unsupported quoted escapes
  and adjacent quoted tokens without whitespace, while preserving valid escaped
  quoted-special characters inside ID strings.
- IMAP RFC 2971 `ID` parameter-list parsing now rejects quote and backslash
  atom-special characters inside unquoted ID tokens, keeping the raw ID parser
  aligned with normal IMAP atom handling.
- IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the common IMAP
  atom validator, so literal markers, response specials, wildcard specials,
  quoted specials, and controls are rejected consistently.
- IMAP RFC 2971 `ID` parameter-list parsing now accepts bounded synchronizing
  and `LITERAL+` string literals inside the parenthesized field/value list,
  while missing or unused literal payloads remain tagged `BAD` syntax errors.
- IMAP `SEARCH`/`UID SEARCH` `LARGER` and `SMALLER` size criteria require
  digit-only RFC 3501 number atoms, rejecting signed values such as `+20`
  instead of silently treating them as valid sizes.
- IMAP mod-sequence numeric inputs require digit-only atoms across
  `SEARCH MODSEQ`, `FETCH CHANGEDSINCE`, and conditional `STORE`
  `UNCHANGEDSINCE`, rejecting signed values such as `+17`.
- IMAP UID and message sequence-set numbers require digit-only atoms, rejecting
  signed values such as `UID FETCH +7` and `FETCH +1` before command execution.
- IMAP UID and message sequence-set expansion accepts common client-scale
  ranges such as `1:1000` and `1:*` while still enforcing an explicit expansion
  cap, reducing false `BAD` responses during mailbox synchronization.
- IMAP UID set resolution intersects authenticated selected-mailbox UID ranges
  and comma-separated UID sets with visible message UIDs, so sparse requests
  such as `UID FETCH 1:999` and `UID FETCH 1,7,999` skip missing UIDs instead
  of failing the whole command.
- IMAP MIME body-part paths and partial body fetch windows require digit-only
  number atoms, rejecting signed forms such as `BODY[+1]` and
  `BODY[]<+12.34>`, and partial fetch counts must be non-zero as required by
  RFC 3501 `nz-number` grammar. Partial fetch tokens also reject trailing
  characters after the closing `>`.
- IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments reject malformed atoms
  that still contain quote characters after command parsing, preventing broken
  values such as `UTF-8"` from being silently normalized.
- IMAP `THREAD` algorithm arguments reject malformed atoms that still contain
  quote characters after command parsing, preventing broken values such as
  `ORDEREDSUBJECT"` from being silently normalized.
- IMAP `SEARCH`/`UID SEARCH` text, body, and header string arguments reject
  malformed atoms that still contain quote characters after command parsing,
  preventing broken values such as `SUBJECT IMAP"` from being normalized.
- IMAP `SEARCH` text arguments preserve valid RFC quoted-special escaped
  quotes from proper quoted strings, so standards-shaped searches such as
  `SUBJECT "Project \"Q2\""` remain compatible while malformed atom quotes are
  rejected by command parsing.
- IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria reject
  malformed keyword atoms that still contain quote characters after command
  parsing, preventing broken values such as `KEYWORD custom"` from being
  silently normalized.
- IMAP command tokenization rejects dangling quote characters at the end of
  unquoted atoms, preventing broken commands such as `SUBJECT IMAP"` and
  `LIST "" INBOX"` from reaching command-specific normalization while
  preserving valid escaped quotes inside proper quoted strings.
- IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT` lists
  validate RFC-shaped header field names instead of trimming stray brackets,
  rejecting malformed requests such as `HEADER.FIELDS ([Subject])`.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` requires the RFC-shaped
  parenthesized modifier form and rejects bare or over-closed variants such as
  `FETCH 7 FLAGS CHANGEDSINCE 17`.
- IMAP `FETCH`/`UID FETCH` macros remain valid only as standalone macro
  arguments, rejecting malformed list usage such as `FETCH 1 (FAST)` or
  `UID FETCH 7 (FLAGS FAST)`.
- IMAP `STORE`/`UID STORE` `UNCHANGEDSINCE` requires the RFC-shaped
  parenthesized modifier form and rejects malformed over-closed values such as
  `(UNCHANGEDSINCE 27))`.
- IMAP `FETCH`/`UID FETCH` data items reject over-parenthesized tokens before
  item normalization, preventing malformed requests such as `FETCH 1
  ((FLAGS))` and `UID FETCH 7 BODY.PEEK[]))` from being repaired.
- Local filesystem storage remains the default and can be backed by local disk
  or NFS-style mounted storage.
- Local/NFS storage configuration requires a non-empty bounded
  `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
  `GOGOMAIL_STORAGE_BACKEND=local`, so broken filesystem roots fail during
  config validation instead of surfacing later as storage probe errors.
- Local/NFS-style storage writes stage data through unique temporary files in
  the destination directory before `rename`, avoiding fixed `.tmp` collisions
  while preserving atomic object replacement semantics.
- Local/NFS-style storage writes honor context cancellation during body copy,
  cleaning staged temp objects and avoiding partial object commits after a
  canceled request.
- Local/NFS and S3-compatible `Get`/`GetRange` readers now observe context
  cancellation after open/request dispatch, so canceled downloads and previews
  stop at the storage adapter boundary instead of continuing to stream bytes.
- Local/NFS `GetRange` now reports `io.ErrUnexpectedEOF` when a requested
  window extends beyond the available object bytes, matching the S3-compatible
  range-reader corruption signal instead of silently returning a short range.
- Local/NFS storage no longer treats filesystem symbolic links as storage
  objects: reads, range reads, metadata probes, deletes, and source moves
  reject them, while list pages hide them so mounted storage cannot escape
  object-key semantics through host-specific link behavior. Local/NFS direct
  deletes also reject directories instead of treating filesystem folders as
  object keys.
- Local and S3-compatible storage writes reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- Local/NFS-style storage deletes treat already-missing objects as success,
  aligning cleanup semantics with S3-compatible object deletion.
- S3-compatible storage requests reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, successful `GET` close, and `DELETE`
  responses drain a small bounded response-body window before close, improving
  HTTP connection reuse for normal S3/MinIO responses without allowing
  oversized bodies to stall cleanup.
- S3-compatible `PutObject`, full-object `GET`, `HEAD`/`Stat`, and
  `ListObjectsV2` now require exact `200 OK` responses, rejecting
  accepted/deferred writes, unexpected partial-content, or other non-OK 2xx
  statuses before callers can treat ambiguous provider responses as durable or
  complete backend-neutral results.
- S3-compatible missing-object reads now wrap `os.ErrNotExist` for `GET`,
  ranged `GET`, and `HEAD`/`Stat` `404 Not Found` responses, keeping
  backend-neutral missing-object checks consistent with local/NFS storage while
  preserving sanitized S3 status diagnostics.
- Local/NFS and S3-compatible readiness probes read the verification object
  through a tight expected-size bound, preventing malformed or proxy-inflated
  probe responses from allocating unbounded memory during health checks.
- Local/NFS and S3-compatible readiness probes now also verify `Stat` metadata
  for the probe object, catching broken filesystem metadata or S3 `HEAD` paths
  before an instance reports ready.
- Local/NFS and S3-compatible readiness probes now also verify a short
  `GetRange` against the probe object, catching broken filesystem seek/range
  handling or S3 `Range` response compatibility before partial-read workflows
  report ready.
- The storage interface is backend-neutral (`Put`, `Get`, `Stat`, `Copy`,
  `Move`, `List`, `Delete`) and object paths share strict canonical key
  validation before adapter use, including valid UTF-8 object paths, prefixes,
  and list cursors.
- S3-compatible `Stat` and `List` now bound and sanitize provider-returned
  `Content-Type`/ETag metadata before exposing `ObjectInfo`, dropping unsafe
  multiline, invalid UTF-8, or oversized metadata while preserving object
  identity and size for compatible providers.
- `GOGOMAIL_STORAGE_BACKEND=s3` can wire AWS S3-compatible object storage, and
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same adapter with path-style
  requests for local MinIO-style deployments. Both use endpoint, region, bucket,
  prefix, credential, and session-token settings.
- Drive runtime wiring now registers the configured S3-compatible store under
  both `s3` and `minio` labels, so rows created under local MinIO can still be
  served after an AWS S3-style backend flip and vice versa when object keys and
  bucket contents have been migrated.
- Drive runtime wiring can also opt into explicit legacy storage labels through
  `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS`, giving operators a controlled
  migration bridge for local/NFS-to-S3-compatible Drive cutovers after object
  bytes are replicated while leaving unmapped legacy labels fail-closed.
- App-level storage option construction now has direct coverage for MinIO
  path-style pinning, ordinary S3 virtual-hosted defaults, and the explicit
  `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` override.
- S3-compatible bucket validation rejects IP-address-shaped names plus
  AWS-reserved bucket prefixes and suffixes before storage adapter construction,
  and requires bucket names to start and end with a letter or digit, keeping AWS
  and MinIO-style deployment failures early and explicit.
- S3-compatible `ListObjectsV2` responses now reject `IsTruncated=true` pages
  that omit a continuation token, preventing Drive/lifecycle cleanup scans from
  accepting a page that cannot be advanced safely.
- S3-compatible `ListObjectsV2` key decoding no longer trims provider-returned
  object keys before prefix/object-path validation, preventing distinct
  whitespace-bearing keys from being silently normalized into canonical
  gogomail object paths.
- S3-compatible `ListObjectsV2` pages reject provider responses that return
  more matching objects than the requested bounded page size, keeping S3,
  MinIO, and local/NFS pagination under the same storage contract.
- S3-compatible endpoint validation rejects userinfo, query strings, fragments,
  non-HTTP schemes, CR/LF-bearing targets, and non-canonical base paths before
  storage adapter construction. Endpoint base paths also reject encoded path
  separators such as `%2F` and `%5C`, keeping SigV4 signing and object
  addressing deterministic.
- S3-compatible request construction automatically switches dotted bucket names
  on HTTPS endpoints to path-style addressing, avoiding AWS S3 virtual-hosted
  TLS wildcard certificate mismatches without changing ordinary bucket defaults.
- S3-compatible request construction also switches localhost and IP-address
  endpoints to path-style addressing, avoiding `bucket.localhost` or
  `bucket.127.0.0.1` drift for local MinIO and other local compatible stores
  even when the generic `s3` backend is used.
- S3-compatible object key escaping preserves literal `+` characters as `%2B`
  in segment-escaped request paths, keeping object identity and SigV4 canonical
  request paths aligned for AWS S3, MinIO, and strict compatible providers.
- S3-compatible endpoint base paths are segment-escaped with the same literal
  `+` preservation, keeping reverse-proxy or base-path deployments aligned with
  SigV4 canonical request paths.
- S3-compatible uploads set a deterministic `Content-Length` for seekable PUT
  bodies without buffering the object in memory, improving compatibility for
  file-backed mail and attachment writes while keeping hot paths streaming-first.
- S3-compatible deletes treat `404 Not Found` as already-cleaned success,
  keeping lifecycle cleanup idempotent across AWS S3, MinIO-style endpoints,
  and local/NFS storage.
- Local/NFS and S3-compatible storage expose a shared object `Move` contract
  for future Drive/file relocation workflows. Local/NFS uses efficient
  filesystem rename semantics; S3-compatible storage uses signed server-side
  copy followed by source delete, so callers should treat post-copy failures as
  duplicate-cleanup work instead of relying on atomic rename semantics.
- S3-compatible `Copy` now requires exact `200 OK` responses with bounded
  `CopyObjectResult` bodies and rejects empty bodies, unexpected XML, and
  embedded `<Error>` XML inside `200 OK` responses, keeping AWS S3/compatible
  copy failures from being accepted as successful Drive or lifecycle object
  duplication.
- Shared storage exposes a bounded `DeletePrefix` helper over the existing
  `List` and idempotent `Delete` contracts, giving future Drive folder
  deletion, attachment lifecycle, and reconciliation jobs a cursor-driven
  cleanup path without relying on provider-specific recursive delete behavior.
- S3-compatible secret access keys and session tokens reject spaces, tabs, and
  line breaks during config validation and adapter construction, making copied
  env/config credential mistakes fail fast before runtime S3 authentication
  errors.
- S3-compatible access key IDs reject spaces, tabs, and line breaks during
  config validation and adapter construction, preventing copied credential
  mistakes from being silently trimmed before SigV4 signing.
- S3-compatible access key IDs, secret access keys, and session tokens also
  reject oversized direct adapter inputs using the same bounds as startup
  config validation, keeping SigV4 request construction bounded.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, including the `GOGOMAIL_STORAGE_ROOT` compatibility alias for
  `GOGOMAIL_MAILSTORE_ROOT`, and the development compose stack includes
  `minio-init` to create the default `gogomail` bucket for local
  S3-compatible runs.

Next:

- Run optional integration coverage against MinIO or another S3-compatible test
  endpoint by setting `GOGOMAIL_TEST_S3_ENDPOINT`,
  `GOGOMAIL_TEST_S3_BUCKET`, `GOGOMAIL_TEST_S3_ACCESS_KEY_ID`, and
  `GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY`.

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
  transaction-scoped advisory locking, affected quota-row locks, and bounded
  audit-log detail for dry-run/applied correction attempts.
- Admin API exposes bounded audit-log list/detail reads so quota correction,
  domain onboarding, mail receive, and delivery-status audit rows are
  inspectable through the operator API.
- Domain DNS check and quota correction audit records now reuse the shared
  hash-chain writer, keeping operator-visible audit rows tamper-evident.
- Admin API can run a bounded audit-log integrity check over recent rows,
  reporting hash and in-window prev-hash breaks for operator triage.
- User quota source is tracked as `default|custom`.
- Domain quota updates can apply a new default user quota to default-following
  users while preserving custom overrides.
- ADR 0003 defines company → domain → user unified storage pool semantics.
- ADR 0009 defines the Drive metadata boundary: Drive nodes are PostgreSQL
  metadata scoped by company/domain/user, object bytes stay behind the shared
  storage interface, and future Drive file writes must consume the same unified
  user quota ledger as mailbox and attachments.
- The `drive_nodes` migration establishes folder/file metadata, active sibling
  uniqueness, storage object references for files, and active/trashed/deleted
  lifecycle state without starting frontend implementation.
- `internal/drive` validates Drive node names, types, and statuses before
  future repository/API code can persist path-bearing, control-character, or
  unsupported lifecycle values.
- `internal/drive.Repository.CreateFolder` can create active folder nodes for
  active users, derive company/domain scope from the user row, validate active
  parent folders, and rely on the `drive_nodes` active sibling uniqueness
  constraint before any Drive HTTP API is exposed. Folder creation SQL uses
  only the bound request parameters, keeping the production folder-create path
  aligned with the HTTP contract.
- `internal/drive.Repository.CreateFileFromObject` validates file metadata,
  verifies the referenced object through the shared storage `Stat` contract,
  and increments the company/domain/user quota ledger in the same transaction
  as the `drive_nodes` file insert.
- `internal/drive.Repository.ListNodes` can read bounded active/trashed/deleted
  folder contents with folder-first stable ordering, preparing Drive list views
  before an HTTP API is exposed.
- `internal/drive.Repository.TrashNode` can mark an active file/folder and its
  active descendants as trashed in one transaction, preserving object bytes and
  quota usage for future restore or delayed permanent deletion.
- `internal/drive.Repository.RestoreNode` can mark a trashed file/folder and
  its trashed descendants active again in one transaction, clearing `trashed_at`
  while keeping active sibling name conflicts protected by the database
  uniqueness constraint.
- `internal/drive.Repository.PermanentDeleteNode` can mark a trashed
  file/folder and its trashed descendants deleted, release deleted file bytes
  from the company/domain/user quota ledger, and return storage object
  references for later backend cleanup.
- `internal/drive.CleanupDeletedObjects` consumes those object references,
  validates backend/path safety, de-duplicates repeats, honors cancellation,
  and deletes objects through the configured storage stores with
  progress-preserving errors.
- `internal/drive.Service.PermanentDeleteNode` now composes repository
  permanent-delete with backend object cleanup and returns cleanup progress
  alongside the committed metadata/quota result.
- Drive object path builders now standardize staged uploads, committed node
  objects, and user cleanup prefixes under `drive/users/{user_id}/...`, with
  path-segment-safe ID checks before storage paths are emitted.
- Drive object cleanup failures now have a PostgreSQL retry record boundary:
  structured cleanup errors can be recorded with user/node/object context,
  pending failures are de-duplicated per backend/path, repeated failures
  increment attempts, object paths must stay under the owning user's
  `drive/users/{user_id}/...` prefix, and error text is one-line/UTF-8 bounded.
- Drive cleanup-failure records now have bounded repository list and resolve
  methods with status/user filters, oldest-first pending ordering, limit caps,
  and pending-only resolution for worker/admin use.
- `internal/drive.Service.RetryObjectCleanupFailures` can process bounded
  pending cleanup records, delete referenced objects through configured stores,
  resolve successful records, and re-record failed attempts with fresh bounded
  diagnostics.
- `drive-cleanup-worker` can now run the Drive cleanup retry service on a
  validated interval or in run-once mode, using the configured local/NFS,
  MinIO, or S3-compatible object store.
- Mail API now exposes first Drive HTTP routes for bounded node listing, folder
  creation, single-node metadata reads, trash, restore, and permanent delete,
  with OpenAPI response envelopes and the existing user auth/fallback path.
- Mail API now exposes `POST /api/v1/drive/files/finalize` for converting a
  staged object into quota-accounted Drive file metadata through the shared
  storage `Stat` contract.
- Mail API now exposes `PUT /api/v1/drive/files/staged/{upload_id}/body` for
  bounded direct staged object uploads, returning canonical storage path, size,
  and SHA-256 for the finalize request.
- Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/name` for validated
  active file/folder renames using the Drive normalized-name rules.
- Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/parent` for moving
  active Drive files/folders into another folder or back to root with cycle
  prevention.
- `drive_upload_sessions` now defines the database and validation boundary for
  resumable Drive uploads, including declared/received sizes, lifecycle status,
  storage metadata, and expiration indexes.
- `internal/drive.Repository.CreateUploadSession` now records pending Drive
  upload sessions for active users and optional active parent folders.
- Mail API now exposes `POST /api/v1/drive/upload-sessions` for creating
  pending Drive upload sessions with declared size, storage backend, and
  optional RFC3339 expiration.
- Mail API now exposes `GET /api/v1/drive/upload-sessions/{id}` for upload
  session status refresh and retry-state hydration.
- Mail API now exposes `DELETE /api/v1/drive/upload-sessions/{id}` for
  explicit cancelation of pending/uploading/failed Drive upload sessions.
- `internal/drive.Service.StoreUploadSessionBody` now streams retry bodies to
  distinct session object paths, verifies declared size and optional checksum,
  records storage metadata, and cleans failed/superseded objects best-effort.
- Mail API now exposes `PUT /api/v1/drive/upload-sessions/{id}/body` for
  retry-safe full-body upload-session storage with optional SHA-256 checking.
- `internal/drive.Repository.FinalizeUploadSession` now commits uploaded
  session bodies into quota-accounted Drive file metadata and marks the session
  finalized in one transaction.
- Mail API now exposes `POST /api/v1/drive/upload-sessions/{id}/finalize`,
  completing the create/read/cancel/body/finalize Drive upload-session API
  flow for full-body uploads.
- Webmail capabilities now advertise Drive node operations, upload-session
  create/read/cancel/body/finalize support, checksum preconditions, and Drive
  upload size/TTL limits for production client bootstrap.
- `internal/drive.Repository.ExpireUploadSessions` now marks stale
  pending/uploading/failed Drive upload sessions expired in bounded batches,
  and the Drive service deletes stored session bodies from the configured
  backend after metadata expiry.
- `drive-cleanup-worker` now expires stale Drive upload sessions on each tick
  before retrying pending permanent-delete object cleanup failures.
- Mail API now exposes `GET /api/v1/drive/upload-sessions` with status and
  limit filters, giving future Drive upload managers a reconnect/recovery
  surface.
- Admin API now exposes `GET /admin/v1/drive-upload-sessions` with required
  user scope plus status/limit filters, and admin capabilities advertise Drive
  upload-session inspection.
- Admin API now exposes `GET /admin/v1/drive-nodes` with required user scope
  plus parent/status/name/limit filters, giving operator consoles a bounded
  Drive inventory view without reusing user-facing auth paths.
- Drive node listing now supports a bounded `q` name filter on both Mail and
  Admin API list surfaces, with case-insensitive normalization and literal
  wildcard handling.
- Admin API now exposes `GET /admin/v1/drive-nodes/{id}` with required user
  scope and lifecycle status filtering for single-node metadata inspection.
- Admin API now exposes `GET /admin/v1/drive-usage` with required user scope
  for quota, node-count, byte-count, and pending upload-session dashboard
  summaries.
- Mail API now exposes `GET /api/v1/drive/usage`, and webmail capabilities
  advertise `usage_summary`, so production Drive panels can show per-user quota
  and storage summaries without admin routes.
- Mail API now exposes `GET /api/v1/drive/nodes/{id}/download`, streaming
  active Drive file bytes through the configured storage backend with safe
  download headers, and webmail capabilities advertise `node_download`.
- Mail API now exposes `HEAD /api/v1/drive/nodes/{id}/download` for metadata
  and object-existence checks without transferring Drive file bytes.
- Mail API Drive downloads now accept one satisfiable `Range: bytes=...`
  request and return `206 Partial Content` through the shared local/NFS and
  S3-compatible `GetRange` storage primitive; webmail capabilities advertise
  `node_range_download`.
- Mail API Drive download, range-download, and download-header responses now
  expose sanitized `X-Gogomail-Drive-SHA256` when a node has a recorded
  whole-object digest.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/candidates` for
  stale Drive upload-session cleanup counts and bounded candidate previews.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/runs` for
  explicit one-shot stale Drive upload-session expiry outside the worker loop.
- Admin API now exposes `GET /admin/v1/drive-cleanup-failures` with user,
  status, and limit filters for operator cleanup-drift inspection.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve`
  for audited operator closure after external cleanup verification.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/retry-runs`
  for audited one-shot retry of pending Drive object cleanup failures, with
  scanned/deleted/resolved/failed counts suitable for an operator console.

Next:

- Extend the same ledger service to large-attachment share-link objects.

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
- Draft rows remain out of `GET /api/v1/search`; draft lookup now has a
  separate compose-focused `GET /api/v1/drafts/search` API over active draft
  subject, sender, recipients, body text, and attachment state, ordered by
  latest draft update with opaque cursor pagination. This keeps active-message
  Postgres/OpenSearch relevance semantics aligned while giving compose UIs a
  bounded search path.

Next:

- Add saved-search style draft filters only if compose UX needs them.

### 3. IMAP gateway planning

Current state:

- A bounded IMAP protocol server exists for the first RFC-shaped handshake,
  authentication, mailbox state, metadata fetch, body fetch, and flag-store
  commands.
- Message, folder, and flag models are IMAP-compatible by design.
- `internal/imapgw` defines native gateway DTOs, backend interfaces, mailbox
  helpers, RFC-shaped flag mapping, and a bounded TCP server shell over the
  service-backed store/session adapter.
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
- `internal/imapgw` includes an in-memory mailbox event broker that live IDLE
  sessions and NOOP polling can subscribe to without blocking write paths.
- `mailservice.StoreIMAPFlags` can publish mailbox `flags` events through the
  broker boundary after repository flag mutations succeed.
- Mail API single and bulk flag mutations can publish mailbox `flags` events
  for messages that already have IMAP UID rows.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages.
- `mailservice` exposes IMAP mailbox/message listing and event subscription
  methods for the protocol listener.
- Admin API exposes bounded IMAP UID backfill for future operator/bootstrap
  modes without enabling an IMAP protocol listener.
- IMAP mailbox event publication is best-effort after successful mutations, so
  IDLE/NOOP fan-out cannot make committed mail writes appear failed.
- Mail API move/delete expunge notifications carry mailbox sequence numbers
  from IMAP UID lookup, allowing selected `NOOP`/`IDLE` clients to receive
  renderable untagged `EXPUNGE` updates.
- `mailservice.IMAPStoreAdapter` satisfies `imapgw.Store` for protocol listener
  wiring through the service boundary.
- `mailservice.IMAPStoreAdapter` also satisfies `imapgw.MailboxSessionStore`
  for SELECT-style mailbox state, service-backed COPY/MOVE/EXPUNGE, and
  mailbox-event subscription.
- `gogomail --mode=imap` is now a separate gateway that opens the
  service-backed IMAP store adapter, wires a process-local mailbox event broker
  for live IDLE sessions, and serves the configured TCP protocol listener.
- `GOGOMAIL_IMAP_ADDR` is loaded and validated as required TCP listener
  metadata for the protocol listener.
- `GOGOMAIL_IMAP_TLS_CERT_FILE`, `GOGOMAIL_IMAP_TLS_KEY_FILE`, and
  `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH` are loaded and validated so production
  IMAP auth cannot be enabled with cleartext credential policy.
- IMAP runtime TLS helper groundwork can load IMAP-specific certificate/key
  files with TLS 1.2 minimum and derive the server name from the IMAP listener
  host before falling back to `GOGOMAIL_SMTP_DOMAIN`.
- ADR 0008 accepts the IMAP authentication/session direction: use a dedicated
  protocol auth adapter over local user password hashes, keep JWT out of IMAP,
  require TLS policy review before production enablement, keep `\Deleted`
  separate from gogomail soft-delete status, and handle MOVE as an IMAP
  source-expunge plus destination folder transition with fresh destination UIDs.
- `mailservice.NewIMAPAuthenticatorAdapter` now maps the existing
  Submission/local-password authentication boundary into `imapgw.Session`
  values, giving the listener a protocol-native authenticator without coupling
  IMAP to JWT middleware.
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
  returns sanitized quoted mailbox names with hierarchy delimiters, encoding
  non-ASCII names and ampersands as RFC 3501 modified UTF-7 while
  `UTF8=ACCEPT` remains unadvertised.
- Authenticated IMAP `STATUS` now maps to service-backed mailbox state and
  returns `MESSAGES`, `UIDNEXT`, `UIDVALIDITY`, and `UNSEEN` metadata.
- IMAP command parsing now supports basic quoted strings with backslash escapes,
  allowing common quoted `LOGIN` credentials and mailbox atoms while rejecting
  malformed quoted controls and unsupported command literal tokens. Bounded
  synchronizing command literals are consumed with a continuation response, and
  bounded non-synchronizing `LITERAL+` command literals are accepted when sent.
- IMAP `CAPABILITY` now advertises `AUTH=PLAIN` only before authentication, so
  post-login clients see capabilities for the selected protocol state.
- IMAP `AUTHENTICATE PLAIN` now accepts the standard continuation response,
  decodes SASL PLAIN credentials, returns tagged `BAD` for RFC cancellation,
  rejects mismatched delegated `authzid` values, and maps successful
  authentication into the same protocol session as `LOGIN`. Failed `LOGIN` and
  `AUTHENTICATE` attempts include RFC 5530 `[AUTHENTICATIONFAILED]` response
  codes for clients that parse machine-readable auth diagnostics.
- Authenticated selected-mailbox `UID FETCH` can now return UID, flags,
  RFC822 size metadata, and `BODY[]` literals streamed from the service-backed
  raw message fetch boundary. Untagged `FETCH` responses now use message
  sequence numbers, and `RFC822.SIZE` metadata requests do not trigger body
  streaming.
- `UID FETCH` accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]` as a body fetch request for read-without-side-effect clients.
- Non-UID `FETCH` accepts bounded message sequence sets, including `*`, and
  resolves them through the selected mailbox list before streaming the same
  metadata/body responses.
- `EXAMINE` now selects a mailbox read-only and blocks `UID STORE`, giving
  clients a standards-shaped read-only mailbox state.
- `EXAMINE` now passes read-only selection intent through the backend
  `SelectMailboxRequest`, so service adapters can distinguish read-only
  sessions from writable `SELECT`.
- `SELECT`/`EXAMINE` now establish mailbox event subscriptions before emitting
  selected-mailbox response data, avoiding ambiguous partial selection state
  when subscription setup fails.
- `CHECK` and `CLOSE` now cover selected-mailbox lifecycle calls; `CLOSE`
  silently expunges `\Deleted` messages for writable selections before clearing
  selected state, while read-only selections only clear state.
- `STATUS` now validates requested status data items and returns only the
  requested mailbox metadata fields.
- IMAP mailbox lookup now resolves wire names such as `INBOX` and
  `Archive/2026` to the stored mailbox ID before selected-mailbox state is used
  by follow-up commands.
- `LIST` now decodes RFC 3501 modified UTF-7 reference/pattern arguments,
  filters mailbox responses with exact, `*`, and `%` patterns over decoded
  names, and emits matching names in modified UTF-7 on the wire.
- `CAPABILITY` now advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`; `LIST`
  includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy attributes
  plus RFC 6154 special-use attributes for system folders such as Drafts, Sent,
  Trash, Junk, Archive, All, and Flagged when those folder roles are present in
  storage metadata, and extended
  `LIST (SPECIAL-USE)`, `RETURN (SPECIAL-USE)`, and no-op
  `RETURN (CHILDREN)` forms are accepted.
- `CAPABILITY` now advertises RFC 5819 `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits the requested `STATUS` data directly
  after each matching selectable mailbox to reduce client folder-list round
  trips, can be combined with `RETURN (CHILDREN)`, and rejects malformed
  `RETURN (STATUS MESSAGES)` style status item lists before mailbox listing
  work.
- `CAPABILITY` now advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
  `LIST-STATUS` can return active message octet totals per mailbox without
  fetching every message's `RFC822.SIZE`.
- `CAPABILITY` now advertises RFC 5256 `SORT`; `SORT` and `UID SORT` reuse the
  selected-mailbox search evaluator, require `US-ASCII` or `UTF-8` charset
  arguments, and return sorted sequence numbers or UIDs for the standard sort
  keys clients use for mailbox list ordering.
- `CAPABILITY` now advertises RFC 5256 `THREAD=ORDEREDSUBJECT`; `THREAD
  ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` return ordered-subject thread
  trees from the selected-mailbox search result while keeping `REFERENCES`
  unadvertised until its Message-ID normalization and ancestry algorithm can be
  implemented without compatibility shortcuts.
- RFC 5256 base-subject extraction now decodes RFC 2047 encoded-word subjects
  before stripping reply/forward artifacts, improving internationalized
  `SORT SUBJECT` and `THREAD ORDEREDSUBJECT` compatibility.
- `LIST "" ""` and `LSUB "" ""` now return the hierarchy root with
  `\Noselect` and `/` delimiter metadata, matching client namespace delimiter
  probes before persistent subscription storage exists.
- `SELECT`/`EXAMINE` now emit `[PERMANENTFLAGS]` response codes so clients can
  distinguish writable and read-only flag state.
- `SELECT`/`EXAMINE` now emit RFC-shaped untagged `RECENT` counts alongside
  `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints, `UIDVALIDITY`,
  `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from durable mailbox
  UID state.
- `SELECT`/`EXAMINE` now emit `[UIDNOTSTICKY]` when the selected mailbox state
  reports non-sticky UIDs, keeping UIDPLUS-adjacent clients aware of mailbox
  UID persistence guarantees.
- `UID STORE` now supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- `FETCH`/`UID FETCH` now include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes when requested, using the service-backed message summary fields.
- Service-backed IMAP summaries now hydrate stored `To`, `Cc`, and `Bcc`
  address JSON into RFC-shaped ENVELOPE address lists, so repository-backed
  `FETCH ENVELOPE`, address search, and address sort paths share the same
  recipient metadata as Mail API storage.
- Shared fetch failure paths now use the issued command name in tagged
  failures, keeping regular `FETCH` failures distinct from `UID FETCH`
  failures in client-visible responses.
- `FETCH`/`UID FETCH` now applies RFC 3501 `\Seen` side effects for successful
  `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads, while
  `BODY.PEEK[...]` and `RFC822.HEADER` remain preview-safe and non-mutating.
- `FETCH`/`UID FETCH` now preserves `RFC822`, `RFC822.HEADER`, and
  `RFC822.TEXT` response data item names on the wire instead of rewriting them
  to their `BODY[...]` equivalent names.
- `CAPABILITY` now advertises `CONDSTORE` and `ENABLE`; RFC 5161-shaped
  `ENABLE CONDSTORE` marks sessions CONDSTORE-aware before mailbox selection.
- `FETCH`/`UID FETCH` now include RFC 4551-shaped `MODSEQ (n)` attributes when
  requested, backed by durable per-message IMAP mod-sequences.
- `SEARCH`/`UID SEARCH` now support RFC 4551-shaped `MODSEQ` criteria,
  including optional metadata entry/type arguments, and return the highest
  matched mod-sequence in non-empty SEARCH responses.
- `FETCH`/`UID FETCH` now support RFC 4551-shaped `CHANGEDSINCE` modifiers,
  returning only messages with greater per-message mod-sequences and
  implicitly adding `MODSEQ` attributes.
- Sessions now become CONDSTORE-aware after `FETCH MODSEQ`,
  `FETCH CHANGEDSINCE`, `SEARCH MODSEQ`, or `STATUS HIGHESTMODSEQ`; subsequent
  flag `FETCH` event/STORE echo responses include `MODSEQ`.
- `STORE`/`UID STORE` now support RFC 4551-shaped `(UNCHANGEDSINCE n)`
  modifiers with transactional per-message mod-sequence checks, partial
  success for passing messages, and UID/sequence `[MODIFIED ...]`
  stale-write responses. Conditional store response/event paths filter modified
  stale UIDs out of successful `FETCH` echoes and mailbox flag notifications.
- `SELECT` and `EXAMINE` now accept the RFC 4551-shaped `(CONDSTORE)`
  parameter and mark the session CONDSTORE-aware.
- `FETCH`/`UID FETCH` now return a conservative single-part `BODYSTRUCTURE`
  response while richer MIME tree serialization remains future work.
- Single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
  parameters, content-transfer-encoding, ID, and description from bounded raw
  message headers instead of always reporting text/plain defaults.
- Metadata-only `BODYSTRUCTURE` fetches now use the streaming MIME-structure
  parser to return multipart child order, subtype, parameters, transfer
  encodings, dispositions, body octets, and text line counts without retaining
  attachment payloads.
- `BODYSTRUCTURE` now emits RFC 3501-shaped `message/rfc822` bodies with
  encapsulated message header-derived envelope metadata, parsed nested body
  structure, and line counts instead of treating attached messages as generic
  basic parts.
- The shared MIME-structure parser now descends into `message/rfc822` parts
  while counting the encapsulated message bytes/lines and capturing bounded
  envelope metadata, so forwarded-message attachments expose nested body
  metadata without retaining payloads.
- `FETCH`/`UID FETCH` can now return RFC 3501-shaped `BODY[n.HEADER]` and
  `BODY[n.TEXT]` literals for `message/rfc822` parts, including
  forwarded-message attachments inside multipart messages.
- `FETCH`/`UID FETCH` can now return `BODY[n.HEADER.FIELDS (...)]` and
  `BODY[n.HEADER.FIELDS.NOT (...)]` subsets for `message/rfc822` parts, so
  clients can preview forwarded-message headers without fetching whole nested
  headers.
- `FETCH`/`UID FETCH` can now follow multipart body-part numbering inside
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
- Combined `BODYSTRUCTURE` plus literal body/header fetches can reopen the raw
  message for MIME metadata while preserving the original reader for literal
  streaming, so common preview/header fetch batches keep rich structure
  responses.
- `FETCH`/`UID FETCH` now supports standard `FAST`, `ALL`, and `FULL` macros,
  including the non-extensible `BODY` attribute for `FULL`.
- `FETCH`/`UID FETCH` now support bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- Non-UID `FETCH` now uses the same bounded header literal path as `UID FETCH`
  for `BODY[HEADER]` and `RFC822.HEADER`.
- `FETCH`/`UID FETCH` now support bounded text-only section literals for
  `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and `RFC822.TEXT`, rejecting oversized
  section bodies before unbounded allocation.
- `FETCH`/`UID FETCH` now support conservative single-part text literals for
  `BODY[1]` and `BODY.PEEK[1]`.
- `FETCH`/`UID FETCH` now supports bounded top-level multipart body-section
  literals such as `BODY[1]` and `BODY[2]`, letting clients read individual
  MIME parts without fetching the full message.
- `FETCH`/`UID FETCH` now supports bounded nested multipart body-section
  literals such as `BODY[1.2]` with a capped MIME part path depth.
- `FETCH`/`UID FETCH` now supports bounded partial windows over multipart
  body-section literals such as `BODY.PEEK[2]<4.4>`.
- `FETCH`/`UID FETCH` now answers conservative single-part MIME header requests
  for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
- `FETCH`/`UID FETCH` now streams actual multipart child MIME headers for
  `BODY[n.MIME]` and `BODY.PEEK[n.MIME]` when the selected part exists.
- `UID STORE` now accepts bounded UID sets/ranges so clients can mutate flags in
  batches instead of issuing one command per message.
- Non-UID `STORE` now accepts bounded sequence sets/ranges and maps them to the
  same service-backed flag mutation boundary as `UID STORE`.
- Non-UID `STORE` now supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- `NOOP` now drains queued selected-mailbox events as untagged `EXISTS`,
  `EXPUNGE`, and flag `FETCH` updates, giving clients a polling path alongside
  live IDLE and suppressing stale or duplicate exact-count `EXISTS` events.
- `IDLE` is now advertised and accepted, streaming selected-mailbox events while
  the client is waiting and completing when the client sends `DONE`.
- `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` now work over the
  selected mailbox message list.
- `SEARCH`/`UID SEARCH` now accepts sequence-set criteria such as `2:*`,
  letting clients intersect standard search predicates with selected mailbox
  sequence ranges.
- `SEARCH`/`UID SEARCH` can combine supported criteria with RFC default AND
  semantics, including `ALL` plus flag, date, size, address, and UID filters.
- `SEARCH`/`UID SEARCH` supports RFC `NOT` and binary `OR` criteria
  composition over the supported search predicate set.
- `SEARCH`/`UID SEARCH` now accepts parenthesized search-key groups, combining
  grouped predicates with RFC default AND semantics and allowing grouped
  operands inside `OR`.
- `CAPABILITY` now advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)` and
  `UID SEARCH RETURN (...)` return single untagged `ESEARCH` responses with
  requested `MIN`, `MAX`, compact `ALL`, `COUNT`, UID indicators, and
  CONDSTORE `MODSEQ` data.
- `CAPABILITY` now advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN (SAVE)`
  stores the selected-session search result so `$` can be reused by later
  sequence-set and UID-set commands without sending the result set back to the
  client.
- `SEARCH RETURN (SAVE)` now clears the selected-session `$` result when the
  save-requested search fails with tagged `NO`, while tagged `BAD` searches
  leave the previous result untouched as required by RFC 5182.
- `FETCH`/`UID FETCH` now supports partial full-body literals for
  `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`.
- `FETCH`/`UID FETCH` now supports bounded partial section literals for common
  `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]` requests.
- `SEARCH`/`UID SEARCH` now support common flag criteria such as `UNSEEN`,
  `FLAGGED`, `ANSWERED`, and `DRAFT` for standard client views.
- `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag separately
  from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose that flag
  through `FLAGS`, `DELETED`, and `UNDELETED`.
- `SEARCH`/`UID SEARCH` now supports `RECENT`, `OLD`, and `NEW`, returning no
  recent/new matches while durable recent-state semantics remain deferred and
  treating active messages as old.
- `SEARCH`/`UID SEARCH` now supports `KEYWORD` and `UNKEYWORD` criteria with
  validated keyword atoms. The existing webmail `forwarded` state now maps to
  an IMAP `$Forwarded` keyword across `FETCH FLAGS`, `SEARCH KEYWORD`, and
  permitted `STORE` mutations while unknown custom keywords still return no
  matches until durable user keyword storage exists.
- `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS (...)]` and
  `BODY.PEEK[HEADER.FIELDS (...)]` for lightweight preview metadata reads.
- `FETCH`/`UID FETCH` now supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  reads.
- `FETCH`/`UID FETCH` now echoes requested `HEADER.FIELDS` and
  `HEADER.FIELDS.NOT` section names in literal response items, including
  partial-window suffixes, instead of normalizing subset literals to
  `BODY[HEADER]`.
- `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS.NOT (...)]` and
  `BODY.PEEK[HEADER.FIELDS.NOT (...)]` for exclude-style header reads.
- `SEARCH`/`UID SEARCH` now supports `SINCE`, `BEFORE`, and `ON` over message
  `INTERNALDATE`, plus `SENTSINCE`, `SENTBEFORE`, and `SENTON` over envelope
  dates.
- `SEARCH`/`UID SEARCH` now supports basic `FROM`, `TO`, `CC`, `BCC`, and
  `SUBJECT` substring criteria over selected-mailbox summaries.
- `SEARCH`/`UID SEARCH` now supports bounded `BODY` and `TEXT` raw-message
  criteria scans, with `BODY` excluding the RFC 5322 header block.
- `SEARCH`/`UID SEARCH` now supports bounded RFC `HEADER <field> <value>`
  criteria scans over the raw message header block.
- `SEARCH`/`UID SEARCH` now preserves RFC 3501 zero-length search string
  semantics for quoted empty strings across envelope, body/text, and header
  substring criteria instead of treating them as guaranteed no-match requests.
- `SEARCH`/`UID SEARCH` now supports RFC 3501 `LARGER` and `SMALLER` criteria
  over message `RFC822.SIZE` metadata.
- `SEARCH`/`UID SEARCH` now accepts `CHARSET US-ASCII` and `CHARSET UTF-8`
  prefixes and returns an RFC-shaped `[BADCHARSET]` response for unsupported
  search charsets.
- Authenticated `NAMESPACE` now advertises the personal namespace and `/`
  hierarchy delimiter for mailbox discovery.
- `CAPABILITY` now advertises `NAMESPACE` alongside the implemented namespace
  command so client discovery matches the supported command surface.
- Authenticated `SUBSCRIBE`/`UNSUBSCRIBE` now persist mailbox subscription
  names, and `LSUB` returns the saved subscription set instead of every visible
  mailbox.
- IMAP subscription canonicalization preserves hierarchy delimiters, quoting,
  and internal spacing while keeping case-insensitive matching, preventing
  distinct subscribed mailbox names from collapsing into the same `LSUB` row.
- `SUBSCRIBE` can now retain missing mailbox names so `LSUB` can expose them
  with `\Noselect`, matching client migration and deleted-mailbox recovery
  behavior that expects subscriptions to outlive selectable mailboxes.
- `LSUB` retains subscribed names after mailbox deletion with `\Noselect` and
  covers the RFC 3501 `%` hierarchy parent response case.
- IMAP mailbox-taking commands decode RFC 3501 modified UTF-7 mailbox
  arguments before crossing into the service boundary, covering selection,
  status, append, copy/move, mutation, and subscription paths while rejecting
  raw 8-bit and malformed alternate forms instead of leaking wire encoding into
  storage.
- IMAP quoted-string response formatting preserves ordinary internal spacing
  while still escaping quotes/backslashes and cleaning controls, so `LIST`,
  `LSUB`, `STATUS`, FETCH metadata, and MIME parameter values keep their wire
  identity.
- IMAP now advertises and supports RFC 2971 `ID`, validating `NIL` or bounded
  field/value parameter lists before returning gogomail server identity.
- IMAP now advertises and supports `UNSELECT`, clearing selected-mailbox state
  without invoking `CLOSE`/EXPUNGE semantics.
- `EXPUNGE` and `UID EXPUNGE` now delete only messages marked with the
  IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
  `EXPUNGE` responses, remove stale mailbox UID rows, and publish best-effort
  expunge events through the service boundary.
- `COPY` and `UID COPY` now resolve source message sequence/UID sets, validate
  the destination mailbox, duplicate active message metadata and attachment
  rows transactionally, assign fresh destination mailbox UIDs, return UIDPLUS
  `[COPYUID ...]` response codes when destination UIDs are available, and
  publish best-effort destination `EXISTS` events through the service boundary.
  Missing destination mailboxes now return `[TRYCREATE]`.
- `MOVE` and `UID MOVE` now resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, move active messages
  transactionally, assign fresh destination UIDs, and allow moves back into the
  selected mailbox by creating a fresh same-mailbox message before expunging
  the source UID. Responses return UIDPLUS `[COPYUID ...]` mappings in the
  final tagged OK when destination UIDs are available, advance and return
  source mailbox `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients,
  emit RFC-shaped source `EXPUNGE` responses, and return `[TRYCREATE]` when
  the destination mailbox is missing.
- `APPEND` now has a protocol-to-backend request boundary for mailbox, optional
  flag-list, optional internal date-time, literal body, and size after bounded
  literal framing. The boundary now carries UIDPLUS-ready append metadata so
  successful storage can emit `[APPENDUID uidvalidity uid]`; the service layer
  spools and size-checks the literal, parses the RFC message, writes raw `.eml`
  through the configured storage backend, and `maildb` records metadata, quota,
  outbox, and mailbox UID state transactionally. Missing destination mailboxes
  now produce an RFC-shaped `[TRYCREATE]` response code, and quota rejection
  produces `[OVERQUOTA]`. APPEND commands without a synchronizing literal are
  now syntax `BAD` responses rather than unsupported-command responses.
  Successful append results include the new message sequence number for precise
  selected-mailbox `EXISTS` event counts. APPEND internaldate parsing accepts
  RFC 3501 space-padded one-digit date-days such as `" 5-May-2026 ..."`.
  Service-level APPEND rejects CR/LF-bearing or oversized user/mailbox
  identifiers before repository lookup, spooling, parsing, storage, or quota
  work.
- Service-level IMAP `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations reject
  CR/LF-bearing or oversized user/mailbox identifiers before repository
  mutation dispatch or mailbox event publication.
- Service-level IMAP read/list/subscription/backfill operations reject
  CR/LF-bearing or oversized user/mailbox identifiers before repository reads,
  storage opens, event subscriptions, or UID backfill work.
- Service-level IMAP `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
  reject zero UIDs before repository or storage work, keeping direct callers
  aligned with RFC 3501's positive UID model.
- Service-level IMAP `STORE`, `COPY`, and `MOVE` calls reject empty UID sets
  before repository work, while `EXPUNGE` preserves nil UID sets for `CLOSE`
  style "all deleted messages" semantics.
- Folder list/create/rename/delete service methods reject CR/LF-bearing or
  oversized user identifiers, and create/rename reject unsafe folder names,
  before repository work.
- Empty IMAP flag-lists are accepted where RFC-shaped clients can send them:
  `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears supported
  flags, and empty `+FLAGS ()`/`-FLAGS ()` are successful no-ops.
- Selected-mailbox `APPEND` now prefers the backend-returned appended message
  sequence number for the untagged `EXISTS` count, falling back to a local
  increment only when precise sequence metadata is unavailable.
- Selected-mailbox `COPY` and same-mailbox `MOVE` now also prefer
  backend-returned destination message sequence numbers for untagged `EXISTS`
  counts, falling back to local increments only when precise metadata is
  unavailable.
- Selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE` now
  adjust saved SEARCHRES `$` sequence numbers the same way explicit `EXPUNGE`
  commands do, keeping subsequent `$` reuse aligned with visible mailbox state.
- `CREATE`, `DELETE`, and `RENAME` now delegate to the service folder boundary
  for authenticated flat user-mailbox management, resolving wire names before
  destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- `CREATE INBOX` and `DELETE INBOX` now return explicit RFC 3501-shaped `NO`
  failures, and `RENAME INBOX` is rejected instead of being treated like a
  generic folder rename until its required special message-moving semantics are
  implemented.
- `EXAMINE` setup failures now return `NO EXAMINE failed` instead of
  `NO SELECT failed`, keeping tagged failure responses aligned with the command
  clients actually issued.
- Malformed recognized `UID` subcommands now reach their command-specific
  validators, so incomplete or structurally invalid `UID SEARCH`, `UID SORT`,
  `UID THREAD`, `UID FETCH`, `UID STORE`, `UID EXPUNGE`, and `UID COPY`
  produce precise tagged `BAD` responses before authentication/selected-state
  checks instead of a generic UID-dispatch failure.
- Missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`, `DELETE`, and
  `RENAME` now return tagged `[NONEXISTENT]` response codes instead of generic
  command failures, so clients can distinguish absent folders from transient
  backend failures.
- Selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`, and
  `EXPUNGE` now reject extra arguments with tagged `BAD` responses instead of
  ignoring malformed input, protecting destructive expunge handling from
  ambiguous client commands.
- Any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT` now reject
  extra arguments with tagged `BAD` responses instead of silently accepting
  malformed commands or ending the session for malformed logout attempts.
- `STATUS` now requires a parenthesized status item list, rejecting malformed
  `STATUS mailbox MESSAGES`-style requests before mailbox metadata lookup.
- Command dispatch now rejects malformed tags containing atom-special
  characters with untagged `BAD` responses before command handling, avoiding
  ambiguous tagged replies for invalid client command tags.
- Command parsing now rejects control characters inside unquoted atoms,
  aligning atom parsing with the existing quoted-string control-character
  guardrail before command dispatch.
- `STARTTLS` is now supported on plaintext IMAP listeners with configured TLS,
  and is advertised only before the connection upgrades.
- `STARTTLS` completion now includes an updated `[CAPABILITY ...]` response
  code for the post-TLS command surface.
- Plaintext IMAP sessions advertise `LOGINDISABLED` and reject
  `LOGIN`/`AUTHENTICATE` with `[PRIVACYREQUIRED]` when insecure auth is
  disabled before STARTTLS.
- IMAP now advertises `LITERAL+` and accepts bounded non-synchronizing command
  literals such as `APPEND ... {n+}` without an extra continuation round trip,
  while preserving the existing synchronizing literal path for conservative
  clients.
- IMAP command reading now supports bounded literals in non-final command
  positions and multiple literals in one command, so literalized credentials,
  mailbox names, and search strings are no longer constrained to terminal
  APPEND-style framing.
- IMAP server coverage now verifies `LOGIN` commands that carry both the user
  name and password as separate synchronizing literals, including the
  credentials delivered to the backend auth boundary.
- IMAP command and IDLE line reads now enforce the command-line byte cap while
  reading from the socket instead of after an unbounded line allocation.
- IMAP oversized command literals now produce an RFC-shaped tagged `BAD`
  response when possible followed by `BYE`, so clients receive a clear protocol
  outcome while the server still closes unrecoverable framing errors instead of
  attempting unsafe stream resynchronization.
- `AUTHENTICATE PLAIN` now supports `SASL-IR` initial responses, reducing
  authentication round trips for compatible IMAP clients.
- `LOGIN` and SASL PLAIN decoded credentials now reject blank, CR/LF-bearing,
  or oversized authentication identities plus empty, oversized, or
  CR/LF-bearing passwords before backend auth work, while preserving
  intentional leading/trailing spaces in RFC string credentials.
- SASL PLAIN encoded and decoded response bytes are now bounded before
  credential splitting, preventing literal initial responses from forcing
  avoidable decode allocation beyond the configured credential caps.
- Authenticated selected-mailbox `UID STORE` now maps `FLAGS`, `+FLAGS`, and
  `-FLAGS` for supported system flags to the service-backed flag mutation
  boundary and returns updated flag metadata.
- `gogomail --mode=imap` now opens the configured TCP listener and serves the
  IMAP server shell with greeting, `CAPABILITY`, `NOOP`, `LOGIN`, `SELECT`,
  `FETCH`/`UID FETCH`, `STORE`/`UID STORE`, `SEARCH`, `SORT`, `IDLE`,
  `STARTTLS`, `CREATE`/`DELETE`/`RENAME`, `APPEND`, `COPY`, `MOVE`, `EXPUNGE`,
  `CLOSE`, `UNSELECT`, and `LOGOUT` over the service-backed mailbox/session
  boundary.
- `gogomail --mode=imap` now starts a dedicated Redis consumer group for
  committed `mail.stored` events and publishes UID-bearing `EXISTS` updates
  into its process-local mailbox event broker so live IDLE sessions can observe
  newly delivered mail.
- `internal/message` now has a bounded streaming MIME-structure parser that
  walks multipart trees, preserves raw transfer-encoding metadata, counts body
  octets/lines, and avoids retaining attachment payloads for future IMAP
  `BODYSTRUCTURE` serialization.

Next:

- Extend MIME literal fetches with captured real-client fixture variants as
  they become available.

Frontend note:

- When frontend implementation starts, use Next.js with TypeScript and shadcn/ui,
  follow `DESIGN.md`, and aim for a Notion Mail-like UI/UX.

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
- Shared webhook/OpenSearch HTTP response cleanup drains a small bounded body
  window before close, improving connection reuse for external scanner, push,
  indexing, bootstrap, and relevance-query calls without unbounded cleanup
  reads.
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
- `maildb` can list stale upload-session candidates in the same bounded order
  used for expiry, giving Admin previews row-level visibility.
- `mailservice` exposes resumable upload session create/cancel/expire methods
  over the repository boundary, reusing attachment metadata validation,
  max-size checks, CR/LF/size-bounded user/session identifiers, and domain
  outbound attachment policy enforcement.
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
- Upload session body replacement preserves the previously recorded staged body
  if repository metadata recording fails, and removes the previous body after a
  successful replacement on a best-effort basis.
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
  candidate and expired counts, and candidate previews include bounded
  upload-session rows, matching the background worker's full cleanup scope.

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
- Mail API now exposes `GET /api/v1/webmail/capabilities` as a production
  webmail bootstrap surface for contract version, module status, list limits,
  supported flags/actions, compose/search limits, attachment upload modes, and
  push-device platforms. Future webmail and Drive module APIs should extend
  this discovery shape instead of forcing frontend hard-coded constants.
- Mail API now exposes `GET /api/v1/mailbox/overview` as a lightweight
  production webmail chrome bootstrap read for aggregate total/unread/starred
  counts, stored-size totals, and system-folder ID shortcuts.
- Mail API message list pagination now accepts optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters for fast
  unread/read/starred/attachment webmail views without forcing clients through
  full-text search.
- Mail API thread list pagination now accepts optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters, with
  `read=false` representing conversations that still contain unread messages.
- Mail API thread list pagination now also accepts `folder_id`, enabling
  folder-scoped conversation views for system and custom folders.
- Mail API message and thread list pagination now accept
  `sort=newest|oldest`, enabling explicit newest-first and oldest-first
  production webmail list controls while retaining opaque cursor pagination.
- Mail API message and thread summaries now expose a required bounded
  `preview` string from the asynchronous search-document read model, so
  production webmail lists can render body context without reading stored EML
  objects during list pagination.
- Mail API now supports bounded `PATCH /api/v1/threads/bulk/flags` for
  conversation-list read/starred/answered/forwarded actions with best-effort
  IMAP flag notifications for the updated messages.
- Mail API now supports bounded `PATCH /api/v1/threads/bulk/folder` for
  conversation-list archive/move workflows with destination-folder validation,
  transactional IMAP UID invalidation, and best-effort expunge notifications.
- Admin API now exposes `GET /admin/v1/console/capabilities` as the operator
  console companion bootstrap surface for module status, common list and
  cleanup/retention limits, tenant/domain/user management, operational triage,
  API usage/export, IMAP UID backfill, and admin auth/no-store behavior.

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
- Optional PostgreSQL integration coverage verifies bounded retention runs
  preserve blocked candidates, keep dry-runs read-only, persist run audit rows,
  and delete only the requested ready batch.
- Admin API can list and fetch persisted API usage ledger retention-run audit
  rows after blocked, dry-run, or destructive attempts.
- `api-usage-retention-worker` can run the same bounded retention path on an
  interval or once-and-exit. It is dry-run by default and requires explicit
  `confirm_ready` plus a `remote-ed25519` export manifest signer before
  destructive runs.
- API usage export capabilities advertise retention-run and retention-worker
  support plus the destructive worker remote-key requirement for generated
  operator clients.
- API usage ledger retention rejects future cutoffs at the repository boundary,
  keeping worker/direct-call behavior aligned with the Admin API.
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
  plug in without coupling gogomail to a specific vendor SDK. Remote signer
  HTTP responses use the shared bounded drain-and-close cleanup path.
- Admin API exposes API usage export capabilities, including signer backend,
  signer key ID, verifier availability, production signature readiness, and
  billing/verified-billing support flags.
- Mail API now supports bounded `POST /api/v1/threads/bulk/delete` for
  conversation-list delete workflows, soft-deleting every active message in
  selected threads while invalidating IMAP UID rows, decrementing quota
  transactionally, and publishing best-effort expunge events from the
  pre-delete UID snapshot.
- Shared storage now supports `Stat` across local/NFS and S3-compatible
  backends, giving future Drive, lifecycle, and verification paths a portable
  way to inspect object size/metadata without streaming object bodies.
- S3-compatible object metadata returned by `Stat` and `List` is bounded to
  safe single-line UTF-8 values before it crosses the storage adapter boundary,
  keeping downstream Drive, lifecycle, logging, and reconciliation consumers
  from inheriting malformed provider metadata.
- Shared storage now supports `Copy` across local/NFS and S3-compatible
  backends, giving future Drive and lifecycle workflows a portable object
  duplication primitive without forcing caller-side read/write loops.
- Shared storage now supports bounded prefix `List` across local/NFS and
  S3-compatible backends, giving future Drive, lifecycle, and reconciliation
  workflows a portable cursor-paginated object metadata scan.
- Mail API now supports single-message and bounded bulk message restore for
  soft-deleted messages, preserving hierarchical quota checks before restored
  messages become active again.
- Mail API now supports bounded thread-level restore so selected soft-deleted
  conversations can be recovered through the same quota-protected restore path.
- Restore actions now best-effort assign IMAP UIDs and publish `EXISTS` events
  for restored active messages, keeping connected IMAP clients closer to
  webmail recovery state without a separate backfill pass.
- Mail API attachment downloads now expose a bodyless `HEAD` metadata probe,
  returning safe download headers and storage-object size before production
  webmail clients decide to stream attachment bytes.
- Drive copy now supports active files and bounded active folder trees via
  `POST /api/v1/drive/nodes/{id}/copy`, using storage `Copy` for local/NFS,
  MinIO, and S3-compatible backends while advertising a `max_copy_nodes` cap.
- Drive copy cleanup failures are now written to the existing cleanup-failure
  queue when a copied object cannot be deleted after metadata creation fails.
- Copied Drive files keep the destination object path ID and `drive_nodes.id`
  synchronized by passing a preallocated node UUID into the metadata insert.
- Drive file write APIs now return HTTP 507 `insufficient_storage` for quota
  exhaustion on finalize/copy paths.
- Drive node listing now supports webmail/admin `sort=name|updated|created|size`
  controls with folder-first ordering, giving future Drive screens predictable
  production browsing controls without frontend-specific assumptions.
- Drive node listing now supports webmail/admin `node_type=folder|file` filters
  and advertises supported node types through webmail capabilities.
- Webmail Drive node listing now accepts `all_parents=true` for whole-user
  Drive search/list views, and webmail capabilities advertise the mode so
  production compose file pickers can search user Drive inventory without
  crawling folders client-side.
- Drive share-link metadata now exists behind `drive_share_links` plus
  authenticated Mail API create/list/revoke routes. Raw share tokens are
  create-response-only, with persisted hashes/suffixes preparing future public
  resolution and compose-side Drive file insertion.
- Drive share-link public resolution/download is now implemented under the Mail
  API: token paths use SHA-256 hash lookup, revoked/expired/inactive owner or
  node state is rejected, metadata responses omit storage internals, and
  `download`-permission links reuse the Drive no-store, checksum, HEAD, and
  single-range download contract.
- Drive public share-link abuse controls now have a configurable Redis
  fixed-window limiter for anonymous metadata/download routes. The limiter
  buckets normalized remote address plus token, returns 429/`Retry-After` when
  the per-minute quota is exhausted, and keeps limiter runtime errors fail-open
  so storage availability does not become a hidden public-download dependency.
- Drive public share-link successful metadata/download accesses, denied
  token/permission checks, and rate-limited requests now write best-effort
  hash-chain audit rows with sanitized link/node/request metadata when
  available plus token suffix, result, status, and remote request metadata.
  This gives Admin audit-log filters immediate visibility into public-link
  access attempts without blocking downloads on audit persistence.
- CalDAV module work has started: ADR 0010 records the standards-first gateway
  boundary, `gogomail --mode=caldav` is a runtime scaffold, and
  `internal/caldavgw` owns RFC/WebDAV method tokens plus principal, calendar
  home, collection, and `.ics` object path parsing.
- CalDAV storage groundwork now includes `caldav_calendars` and
  `caldav_calendar_objects`, with gateway validation for names, colors,
  descriptions, component types, UIDs, strong ETags, sync-token derivation, and
  bounded `.ics` object bodies.
- CalDAV WebDAV XML groundwork now includes bounded namespace-aware PROPFIND
  parsing, safe `Depth` header parsing, `allprop` `include` support, and core
  REPORT root classification for `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection`.
- CalDAV storage tables now have a repository boundary for calendar
  create/list/get and calendar-object upsert/list/get/soft-delete, including
  `.ics` object-name validation, UID/component/ETag checks, optional observed
  ETag guards, and transactional calendar sync-token bumps.
- CalDAV `.ics` validation now wraps `github.com/emersion/go-ical` so object
  writes decode RFC 5545 iCalendar bodies, derive or verify UID/component
  metadata, and reject multiple supported top-level components, missing/duplicate
  UIDs, and excessive component/property counts.
- CalDAV WebDAV response groundwork now has a reusable `multistatus` builder
  with per-property `propstat` statuses and discovery properties for
  principals, calendar homes, calendar collections, and calendar objects.
- CalDAV now has an internal `OPTIONS`/`PROPFIND` discovery handler boundary
  with user/path scope enforcement, safe depth handling, DAV capability headers,
  and multistatus responses over a pluggable discovery store.
- CalDAV PostgreSQL repository methods now satisfy that discovery store
  boundary, including active principal lookup and calendar/object adapters for
  the internal `PROPFIND` handler.
- CalDAV Basic authentication groundwork now reuses the Submission
  authenticator, requires TLS/HTTPS-forwarded requests by default, and returns
  authenticated user IDs for future native client-compatible runtime wiring.
- CalDAV runtime configuration now includes `GOGOMAIL_CALDAV_ADDR` and
  `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH`, with production validation rejecting
  insecure Basic-auth operation.
- `gogomail --mode=caldav` now starts a dedicated HTTP listener backed by the
  CalDAV repository and Basic-auth resolver, with discovery, object I/O, and
  initial REPORT handlers in place.
- CalDAV REPORT parsing now validates `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection` shapes more strictly, including
  nested time-range extraction, required href/filter/range/level fields, and
  bounded sync limits. It also preserves nested RFC 4791 `calendar-data`
  projection requests for `VCALENDAR` and child component property selection.
- CalDAV now handles `REPORT calendar-multiget` for authenticated calendar
  collections, returning requested ETags and `calendar-data` through WebDAV
  multistatus responses. `calendar-multiget`, `calendar-query`, and
  `sync-collection` now project returned iCalendar bodies to requested
  `calendar-data` properties while retaining required RFC 5545 structure
  fields so encoded objects stay valid for clients.
- CalDAV now handles authenticated calendar object `GET`, `HEAD`, `PUT`, and
  `DELETE` with strong ETag headers, bounded iCalendar writes, and
  `If-Match`/`If-None-Match` precondition handling.
- CalDAV now handles `REPORT calendar-query` for authenticated calendar
  collections, returning requested ETags and `calendar-data` while filtering
  VEVENT resources against CalDAV time ranges through the RFC 5545 parser.
  Calendar-query object scans now use bounded `limit/nresults` handling with a
  one-extra-row truncation probe, rejecting partial result sets until
  continuation semantics exist. Unsupported CalDAV query filter elements now
  return a `CALDAV:supported-filter` precondition instead of being silently
  skipped, keeping unimplemented predicates from broadening result sets.
- CalDAV now handles conservative `REPORT sync-collection` requests for
  authenticated calendar collections: initial sync returns active objects plus
  a top-level sync token, current tokens return no resource changes, stale
  tokens return a DAV `valid-sync-token` error, and truncating limits are
  rejected until change-log/continuation semantics are implemented.
- CalDAV now handles `REPORT free-busy-query` on authenticated calendar
  collections, returning RFC-shaped `text/calendar` `VFREEBUSY` responses for
  `Depth: 1` child VEVENTs while bounding child object scans with
  `limit/nresults` and a one-extra-row truncation probe. It clips to the
  requested UTC time range, skips transparent/cancelled events, maps tentative
  events to `BUSY-TENTATIVE`, ingests stored VFREEBUSY `FREEBUSY` source
  periods, and coalesces same-type overlaps.
- CalDAV now handles `MKCALENDAR` on authenticated calendar collection paths
  with UUID Request-URI segments. Creation XML is bounded and namespace-aware
  for display name, description, and CalendarServer/Apple calendar color, and
  successful creates return `201 Created` plus `Location`; slug-style path
  aliases remain future compatibility work.
- CalDAV now handles `DELETE` on authenticated calendar collection paths,
  deleting the collection and active child objects through one repository
  transaction while keeping calendar-home and cross-user deletes forbidden.
- CalDAV now records durable sync-change rows for calendar creation and object
  upsert/delete paths, allowing `REPORT sync-collection` to answer
  stale-but-known tokens with object updates and response-level 404 tombstones
  instead of always forcing a full resync. Collection-deleted tokens can now
  return a final top-level sync token even after the calendar row is gone.
- CalDAV now supports RFC 6764-style service discovery: `/.well-known/caldav`
  redirects to `/caldav/`, and authenticated root `PROPFIND` exposes the
  service root as a read-only collection discovery anchor with
  `current-user-principal` and `principal-collection-set`. Principal-only
  properties such as `calendar-home-set` remain on the principal resource so
  clients do not mistake the service root for an authenticated user principal.
- CalDAV `OPTIONS` and unsupported-method responses now use one implemented
  method list for `Allow`, keeping future-only method names such as `MOVE`
  hidden until their WebDAV behavior is actually implemented.
- CalDAV `PROPFIND /caldav/principals/` now resolves the advertised principal
  collection path, returning collection metadata at `Depth: 0` and the
  authenticated principal as a `Depth: 1` child without exposing other users.
- CalDAV discovery converts shared Directory principals into CalDAV principals
  only when the Directory kind is `user`; organization, group, and resource
  principals stay gated behind future delegation/resource-booking semantics
  instead of being modeled as personal calendar homes.
- CalDAV calendar-home `PROPFIND` now reports WebDAV `current-user-principal`
  and `owner` as the canonical principal URL, preserving a clean boundary for
  future delegated/shared calendar access instead of treating the home
  collection as the principal.
- CalDAV `PROPFIND` now exposes RFC 3744-shaped current-user privilege sets for
  the operations already implemented: read-only principals, calendar-home
  calendar bind/unbind, collection object bind/unbind plus `PROPPATCH`
  metadata writes, and object content writes. ACL and broader delegation
  privileges remain unadvertised until their semantics exist.
- Directory/Identity now includes a bounded `SearchPrincipals` repository
  boundary over users, organizations, groups, and resources. It validates
  tenant/domain/organization scope, permitted principal kinds, query length,
  and result limits, and escapes `LIKE` wildcards before querying. Future
  CalDAV attendee/resource resolution, Contacts/CardDAV autocomplete, shared
  inbox targeting, and admin consoles should use this boundary instead of
  product-local principal lookup.
- CalDAV now handles WebDAV `PROPPATCH` on authenticated calendar collections
  for display name, description, and CalendarServer/Apple calendar color.
  The parser is bounded and namespace-aware, optional properties can be
  removed, `displayname` cannot be removed, and the repository records a
  transactional `collection-updated` sync marker instead of hiding metadata
  changes from WebDAV sync state.
- CalDAV collection discovery now returns WebDAV `supported-report-set` for
  implemented REPORT handlers only: `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection`. Scheduling and other future reports
  remain unadvertised until their semantics exist. Calendar collection
  `PROPFIND Depth: 1` child object discovery now uses the shared bounded
  one-extra-row probe and rejects truncating listings instead of silently
  returning partial metadata.
- CalDAV `calendar-query` now honors simple top-level component filters such as
  `VEVENT` and `VTODO` using stored `component_type` metadata, avoiding
  unrelated object types and avoiding a full iCalendar reparse before component
  filtering.
- CalDAV `calendar-multiget` now respects request-resource scope: collection
  requests cannot fetch sibling collection objects, while calendar-home
  requests remain able to resolve authenticated same-user object hrefs.
- CalDAV `PROPFIND` now exposes WebDAV `owner`, `creationdate`, and
  `getlastmodified` for calendar collections and objects when backed by stored
  metadata, improving native-client discovery without inventing ACL/delegation
  semantics.
- CalDAV object `GET`/`HEAD` now support `If-None-Match` cache revalidation
  against strong object ETags, returning `304 Not Modified` without streaming
  `.ics` bodies when possible.
- CalDAV object `PUT` now rejects explicit non-`text/calendar` media types
  before iCalendar parsing, while still allowing clients that omit
  `Content-Type`.
- CalDAV object `PUT` now enforces `If-Match: *` as an existing-resource
  precondition, preventing accidental object creation through conditional
  overwrite requests.
- CalDAV object `PUT` now rejects stale specific `If-Match` values and matching
  specific `If-None-Match` values before reading/parsing the request body.
- CalDAV object `GET`/`HEAD` now reject stale `If-Match` preconditions before
  cache revalidation, and object `DELETE` now accepts comma-listed strong
  `If-Match` ETags for better WebDAV client compatibility.
- CalDAV object `DELETE` now enforces `If-Match: *` as an existing-resource
  precondition, returning HTTP 412 for missing resources.
- CalDAV object `GET`/`HEAD` now emit `Last-Modified` and honor
  `If-Modified-Since` revalidation from stored object update timestamps.
- CalDAV object `PUT`/`DELETE` now honor `If-Unmodified-Since` before body
  reads or repository mutation, returning HTTP 412 for stale timestamp
  preconditions.
- S3-compatible `GetRange` now bounds returned readers to the validated
  requested length even when a provider sends an oversized `206 Partial
  Content` body, matching local/NFS range-read behavior.
- CalDAV object `GET`/`HEAD` now honor `If-Unmodified-Since` before cache
  revalidation, returning HTTP 412 for stale timestamp read preconditions.
- S3-compatible `GetRange` now validates that `Content-Range` matches the
  requested byte window before returning the bounded response reader.
- S3-compatible `GetRange` now reports `io.ErrUnexpectedEOF` when a matching
  partial response body ends before the requested byte count.
- S3-compatible `GetRange` now drains a small bounded remainder on successful
  range-reader close so oversized partial responses can still reuse HTTP
  connections without exposing extra bytes to callers.
- S3-compatible `GetRange` now also drains a small bounded remainder when
  callers close before consuming the requested range, helping preview/cancel
  paths reuse HTTP connections.
- IMAP `STATUS`/LIST-STATUS parsing now rejects duplicate status data items
  before mailbox metadata lookup.
- CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading
  the XML request body when no active collection already exists at that path.
- CalDAV collection `DELETE` now honors `If-Unmodified-Since` and
  `If-Match: *` preconditions before deleting a calendar collection and its
  children, and strong collection ETags derived from sync state are advertised
  through discovery so specific `If-Match` values can protect stale clients.
- CalDAV collection `PROPPATCH` now shares that precondition gate, rejecting
  stale metadata updates and mismatched collection ETag conditions before XML
  request bodies are read or parsed.
- CalDAV `REPORT` now rejects malformed Depth headers and `Depth: infinity`
  before reading XML request bodies, keeping unsupported WebDAV traversal
  semantics out of calendar-query, calendar-multiget, sync-collection, and
  free-busy-query work.
- CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI hrefs by
  normalizing the URI path through the existing CalDAV path parser and
  preserving same-user / same-collection scope checks; userinfo-bearing
  authorities, query, fragment, opaque, non-HTTP(S), and unsafe hrefs stay
  rejected as per-resource misses.
- CalDAV `REPORT sync-collection` now enforces HTTP `Depth: 0` before sync
  lookup or change-log work, matching the RFC 6578 request-scope model and the
  existing CardDAV behavior while leaving child traversal to the required
  request-body `sync-level`.
- CalDAV `REPORT sync-collection` now requires the request body to carry an
  explicit `DAV:sync-token` element, accepting an empty element for initial
  sync but rejecting omitted sync-token anchors before repository work.
- CalDAV stale-token `sync-collection` delta reads now fetch one extra
  change-log row behind bounded `limit/nresults`, allowing exact-limit deltas
  to succeed while still rejecting responses that would genuinely truncate.
- CalDAV initial `sync-collection` snapshots now also fetch one extra calendar
  object through a sync-specific repository list path, so omitted or exact
  `limit/nresults` requests cannot silently return a partial snapshot with the
  current collection sync token.
- CalDAV `REPORT calendar-query` now honors HTTP `Depth: 0` by returning no
  child calendar-object matches for collection-scoped queries unless clients
  explicitly send `Depth: 1`, keeping WebDAV request scope from silently
  widening during event searches.
- CalDAV `calendar-query` and `free-busy-query` now evaluate bounded VEVENT
  recurrence sets through the RFC 5545 parser, including `RRULE`, `EXDATE`,
  and `RDATE` support from the shared iCalendar library. Dense or unbounded
  rules are capped per object so native-client time-range scans cannot turn
  one stored event into unbounded gateway work.
- CalDAV iCalendar object validation now accepts one VEVENT master plus
  same-UID `RECURRENCE-ID` detached override VEVENTs in a single stored object.
  `calendar-query` and `free-busy-query` scan all VEVENTs in the object and
  suppress the replaced master occurrence when an override is present, matching
  common RFC 5545 recurring-event client output more closely.
- Admin Drive node listing now accepts `all_parents=true` for whole-user Drive
  search/list views while rejecting ambiguous `parent_id` combinations.
- Drive file finalize, upload-session cleanup/retry-body replacement,
  permanent-delete cleanup, cleanup-failure retry, download, and copy paths
  enforce the owning user's
  `drive/users/{user_id}/...` object prefix before storage adapter access.

Next:

- Keep CalDAV in an experimental/backend-only release tier until client-ready
  gates are closed: broader recurrence edge cases, sync retention and
  collection-deletion deltas, slug/path-alias support for friendlier
  MKCALENDAR clients, scheduling semantics, and broader
  Apple/Android/Windows/macOS compatibility tests.
- Before public shared/delegated calendar or resource-booking features,
  extend the initial `internal/directory` user/organization/group/resource
  principal resolver plus alias lookup and group/alias/resource schema into the
  platform boundaries CalDAV depends on: Directory/Identity for delegated
  relationships, resource booking policy, and principal resolution;
  Contacts/CardDAV for personal/external
  people and address books;
  Notification & Sync for reminders, devices, quiet hours, and delta fan-out;
  Search for unified event/person/resource lookup; and Policy/Audit for
  retention, admin controls, and traceable calendar access.
- Directory/Identity now has the first company-scoped delegation table and
  repository check boundary for owner/delegate principals, product scopes, and
  `read`/`write`/`manage` role hierarchy. CalDAV runtime authorization now has
  a first integration through explicit Directory/accesspolicy decisions and
  shared audit insertion, allowing cross-user calendar paths to be resolved
  against the owner store only when the delegated role check allows it.
  Delegated CalDAV `PROPFIND` now derives WebDAV
  `current-user-privilege-set` from that same decision path so discovery stays
  consistent with enforced access. Next CalDAV sharing work should add
  native-client compatibility coverage and write/manage UX semantics before
  public shared calendars are advertised.
- Effective delegation now has a bounded group-expansion read boundary. Next
  product-module integration should still remain deliberate: CalDAV/CardDAV,
  Drive, mailbox sharing, and admin APIs should consume it through explicit
  policy/audit adapters and WebDAV privilege semantics instead of directly
  branching on directory rows in protocol handlers.
- Directory/Identity now also has a bounded delegation listing boundary for
  owner/delegate/scope/role-filtered inspection. This was prioritized before
  deeper CalDAV sharing semantics because admin consoles, shared calendar
  management, Drive shares, shared inboxes, and Contacts/CardDAV delegation all
  need the same observable relationship read model. Next API work should expose
  it through contract-first admin endpoints rather than letting products query
  `directory_delegations` directly.
- That first contract-first admin endpoint now exists as
  `GET /admin/v1/directory/delegations`.
- Audited delegation creation now exists as
  `POST /admin/v1/directory/delegations`, backed by
  `CreateDelegationWithAudit`. It validates active same-company owner/delegate
  principals, scope, role, and self-delegation before inserting the grant and
  `directory_delegation.create` audit row in one transaction.
- Audited delegation deletion now exists as
  `DELETE /admin/v1/directory/delegations/{id}`, backed by
  `DeleteDelegationWithAudit`, so admins can revoke grants with a
  transaction-coupled `directory_delegation.delete` audit row. Next delegation
  work should add update/reassign flows with the same audit shape before
  CalDAV, Drive, or shared inbox modules expose product-facing delegation UX.
- Audited group membership creation now exists as
  `POST /admin/v1/directory/group-memberships`, backed by
  `CreateGroupMembershipWithAudit`. It validates active same-company group and
  member principals, role, self-membership, and nested group cycles before
  inserting the membership and `directory_group_membership.create` audit row in
  one transaction.
- Directory group membership listing now exists as
  `GET /admin/v1/directory/group-memberships`, backed by
  `ListGroupMemberships`, so operators and the future admin console can inspect
  company-scoped group-backed access without querying Directory tables
  directly.
- Audited group membership deletion now exists as
  `DELETE /admin/v1/directory/group-memberships/{id}`, backed by
  `DeleteGroupMembershipWithAudit`, so group-backed delegation can be revoked
  with a transaction-coupled `directory_group_membership.delete` audit row.
- Audited group membership role updates now exist as
  `PATCH /admin/v1/directory/group-memberships/{id}/role`, backed by
  `UpdateGroupMembershipRoleWithAudit`, so operators can promote or demote
  group-backed access without delete/recreate churn.
- Audited group membership reassignment now exists as
  `PATCH /admin/v1/directory/group-memberships/{id}/assignment`, backed by
  `ReassignGroupMembershipWithAudit`, so operators can move active memberships
  between group/member assignments while preserving role and audit continuity.
  Next group-membership work should focus on product-facing policy integration,
  not raw membership table access.
- Directory principal search is also exposed through
  `GET /admin/v1/directory/principals`. Future attendee/resource lookup,
  Contacts/CardDAV autocomplete, Drive sharing, shared inbox targeting, and
  admin console screens should reuse this contract or the underlying
  `SearchPrincipals` boundary rather than adding product-local principal search
  semantics.
- Directory alias resolution is exposed through
  `GET /admin/v1/directory/aliases/resolve`. Future mail routing diagnostics,
  attendee resolution, shared inbox targeting, and admin console screens should
  use this address-to-principal contract instead of re-parsing addresses or
  querying `directory_aliases` directly.
- Directory alias listing now has a bounded repository boundary and admin API
  read path, but product modules should keep using `ListAliases`/`ResolveAlias`
  instead of reaching into `directory_aliases` directly.
- That admin API read path now exists as `GET /admin/v1/directory/aliases`.
- Directory alias creation now has an audited repository mutation boundary and
  `POST /admin/v1/directory/aliases` admin API. It normalizes addresses,
  requires active domain scope, enforces alias-domain alignment, verifies an
  active same-company target principal, returns a predictable duplicate-alias
  error on the active-address unique index, and records
  `directory_alias.create` in the same transaction.
- Directory alias deletion now exists as
  `DELETE /admin/v1/directory/aliases/{id}` and records
  `directory_alias.delete` in the same transaction as the soft delete. Next
  alias work should add update/reassign flows only with the same
  transaction-audited policy shape and without turning this into a product-local
  shared-inbox CRUD model.
- The first `internal/accesspolicy` adapter wraps Directory effective
  delegation into a normalized allow/deny decision. Next integrations should
  add product-specific policy/audit adapters around it before exposing shared
  calendars, delegated address books, Drive shares, or shared inbox actions.
  For WebDAV protocols, use its RFC 4918 privilege mapper instead of inventing
  per-module role-to-privilege tables. Audit integrations should use its
  delegated-access audit detail builder so logs carry normalized principal,
  role, decision, and privilege fields without free-form reason cardinality,
  or its delegated-access audit log builder when they need the full standard
  audit envelope. Admin audit-log queries now support `actor_id` and
  `target_id` filters; future delegated-sharing diagnostics should use those
  filters instead of adding product-local audit tables. When product modules
  start authorizing delegated decisions, prefer `DelegatedAccessAuthorizer` so
  the effective-delegation check and audit insertion remain one fail-closed
  operation; use `DelegationAuditRecorder` only when a product boundary has
  already made and preserved the decision separately.
- CalDAV principal discovery now exposes Directory primary email addresses via
  RFC 4791 `calendar-user-address-set` `mailto:` hrefs when present. Keep the
  next scheduling work on this standards-shaped principal/address boundary:
  attendee resolution should connect through Directory plus Contacts/CardDAV,
  and public scheduling/resource booking should wait for explicit policy and
  audit decisions.
- Continue Contacts/CardDAV as a standards-first module: the current
  `internal/carddavgw` path/href, storage metadata, address-book/contact
  repository, bounded vCard 3.0/4.0 semantic validation, REPORT parsing,
  multistatus rendering, and internal `OPTIONS`/`PROPFIND` discovery handler
  now includes internal `addressbook-query`, `addressbook-multiget`, and
  `sync-collection` execution plus contact-object `GET`, `HEAD`, `PUT`, and
  `DELETE` semantics, and `gogomail --mode=carddav` now exposes an
  experimental Basic-auth runtime listener. The vCard parser now recognizes
  unquoted value separators, preserving quoted parameter values that contain
  colons. It also evaluates
  `/carddav/principals/` as the advertised principal collection, returning the
  authenticated principal at `Depth: 1` without listing unrelated users, and
  `addressbook-query` filters over parsed unfolded vCard property values,
  including RFC 6352 match-type, `negate-condition`, default
  `i;unicode-casemap`, nested `param-filter`, and `test=anyof|allof`
  composition for top-level filters and prop-filters. Unsupported vCard
  property or parameter filters now fail with the RFC 6352
  `CARDDAV:supported-filter` precondition instead of misleading empty success
  responses, including `Depth: 0` requests that otherwise return no child
  objects. Unsupported CardDAV filter child elements now use the same
  `CARDDAV:supported-filter` precondition. REPORT `address-data` can also
  project returned vCards to
  requested property names and rejects unsupported requested address-data
  content types or versions with the RFC 6352
  `CARDDAV:supported-address-data` precondition. Unsupported text-match
  collations now fail with `CARDDAV:supported-collation`; address-book
  collections advertise `CARDDAV:supported-collation-set` with working
  `i;ascii-casemap` and `i;unicode-casemap` matching. Capability properties
  that are not allprop-friendly remain available through explicit `prop`,
  `include`, and `propname`; returned
  address-data also carries explicit `text/vcard` attributes matching the
  stored vCard version. Contact-object `PUT` accepts explicit `text/vcard`
  version parameters only for 3.0/4.0 and requires the media-type version to
  match the body `VERSION` before mutating storage.
  `addressbook-multiget` requires an explicit `Depth` header before resolving
  requested hrefs, while accepting common Depth 0/1 client shapes.
  `addressbook-query` execution honors bounded `limit/nresults` response caps.
  CardDAV `OPTIONS` and unsupported-method responses share one implemented
  method list for `Allow`, keeping native contact clients from seeing methods
  before handler semantics exist.
  Repository-backed query execution can stream contact objects and stop once
  the response cap is satisfied, avoiding whole-address-book materialization on
  that hot path. Address-data projection failures are explicit errors rather
  than silent full-body fallbacks. RFC 6352 `addressbook-query` now requires an
  explicit `Depth` header; `Depth: 1` scans address-object children and
  `Depth: infinity` is accepted with the same flat address-book scan semantics,
  while `Depth: 0` remains collection-scoped without returning child objects.
  PROPFIND responses now expose conservative
  RFC 3744-style current-user privileges, advertising `DAV:read` broadly and
  `DAV:bind`/`DAV:unbind` only on address-book homes where extended `MKCOL`
  can create child collections and collection `DELETE` can remove them, and on
  address-book collections where contact-object `PUT`/`DELETE` can bind or
  unbind child `.vcf` members. Collections also advertise `DAV:write-properties`
  only with implemented `PROPPATCH`, plus `DAV:write-content` only on contact objects with
  implemented write paths.
  Address-book collections also expose CalendarServer-compatible `getctag`
  from the same durable sync token used for WebDAV `sync-token`, keeping
  legacy change detection and RFC 6578 sync anchored to one collection version.
  Address-book collection `PROPFIND Depth: 1` child-object discovery now uses
  the shared bounded one-extra-row probe and rejects truncating listings instead
  of silently returning partial contact metadata.
  RFC 6352 `addressbook-description` is now returned from stored address-book
  metadata. WebDAV `PROPPATCH` now updates authenticated address-book
  collection `DAV:displayname` and `addressbook-description` through a bounded
  parser and repository mutation that refreshes sync state and appends an
  `addressbook-updated` change row. Collection ETags are derived from the
  durable sync token, exposed through PROPFIND `getetag`, and used with
  `If-Match`/`If-Unmodified-Since` to reject stale `PROPPATCH` requests before
  body reads. RFC 6352-style extended `MKCOL` now creates authenticated
  address-book collections at UUID request-URI paths after validating
  `DAV:resourcetype`, `DAV:displayname`, and `addressbook-description`, while
  rejecting existing collections, cross-user paths, missing homes, and unsafe
  path ids before body reads where possible.
  Address-book collection `DELETE` soft-deletes the collection and active child
  contact objects transactionally, honors collection preconditions, and records
  an `addressbook-deleted` change row. `sync-collection` can now return the
  latest deletion sync token for stale-token requests even after the collection
  is no longer active, and enforces RFC 6578 Depth behavior by rejecting
  `Depth: 1` sync requests before sync work. It also distinguishes empty
  initial `DAV:sync-token` elements from missing token elements and rejects the
  latter before sync work. Stale-token delta reads fetch one extra change-log
  row behind bounded `limit/nresults`, matching the CalDAV exact-limit
  behavior while still rejecting genuinely truncating deltas. Initial
  `sync-collection` snapshots use the same one-extra-object probe through a
  sync-specific repository list path, avoiding silent partial address-book
  snapshots when the generic list default would otherwise cap results.
  CalDAV calendar-object `PUT` now rejects duplicate active iCalendar UIDs
  within the same calendar before the SQL upsert path, keeping repository
  errors predictable while the PostgreSQL partial unique index remains the
  final concurrency guard. Final unique-index races are mapped back to stable
  duplicate UID/name repository errors instead of surfacing raw driver messages.
  Contact-object `PUT` now rejects duplicate active vCard UIDs within the same
  address book before the SQL upsert path, keeping
  repository errors predictable while the PostgreSQL partial unique index
  remains the final concurrency guard. Final unique-index races are mapped back
  to stable duplicate UID/name repository errors instead of surfacing raw driver
  messages. Contact-object `DELETE` now passes observed strong ETags into the
  repository transaction so `If-Match` deletes are rechecked under the
  address-book lock before row removal.
  It should be followed by broader vCard compatibility and native-client
  compatibility tests before any public contacts UI or API treats it as
  production-ready.
- Admin audit-log listing now supports bounded `action_prefix` filters, giving
  operators a contract-level way to inspect action families such as
  `share_link.` across successful, denied, and rate-limited public-link
  activity before a dedicated aggregate dashboard exists. Next public-link work
  should add aggregate activity views and configurable tenant policy for whether
  `view` links can preview content beyond metadata before broad public rollout.
- Add a concrete cloud KMS adapter, or deploy the remote-Ed25519 signer service,
  before invoices or hard Open API limits depend on completed export batches.
- Keep scheduled API usage retention dry-run in pre-production until production
  export storage and signer policy are settled.
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
