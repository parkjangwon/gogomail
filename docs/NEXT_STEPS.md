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
- `UID` dispatch validates missing, malformed, unknown, or state-independent
  malformed subcommands before authentication or selected-mailbox state, while
  valid unauthenticated UID commands still return `NO authentication required`.
- Authenticated selected-state commands validate malformed `FETCH`, `STORE`,
  `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax before returning
  selected-mailbox state errors for valid commands.
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
- `LOGIN` and `AUTHENTICATE` validate malformed argument shape or unsupported
  mechanisms before plaintext `[PRIVACYREQUIRED]` responses on TLS-required
  listeners.
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
- Local and S3-compatible storage writes reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- Local/NFS-style storage deletes treat already-missing objects as success,
  aligning cleanup semantics with S3-compatible object deletion.
- S3-compatible storage requests reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, and `DELETE` responses drain a small
  bounded response-body window before close, improving HTTP connection reuse
  for normal S3/MinIO responses without allowing oversized bodies to stall
  cleanup.
- Local/NFS and S3-compatible readiness probes read the verification object
  through a tight expected-size bound, preventing malformed or proxy-inflated
  probe responses from allocating unbounded memory during health checks.
- The storage interface is backend-neutral (`Put`, `Get`, `Delete`) and object
  paths share strict canonical key validation before adapter use.
- `GOGOMAIL_STORAGE_BACKEND=s3` can wire AWS S3-compatible object storage, and
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same adapter with path-style
  requests for local MinIO-style deployments. Both use endpoint, region, bucket,
  prefix, credential, and session-token settings.
- App-level storage option construction now has direct coverage for MinIO
  path-style pinning, ordinary S3 virtual-hosted defaults, and the explicit
  `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` override.
- S3-compatible bucket validation rejects IP-address-shaped names plus
  AWS-reserved bucket prefixes and suffixes before storage adapter construction,
  and requires bucket names to start and end with a letter or digit, keeping AWS
  and MinIO-style deployment failures early and explicit.
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
  decodes SASL PLAIN credentials, supports cancellation, and maps successful
  authentication into the same protocol session as `LOGIN`.
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
  `LIST (SPECIAL-USE)` / `RETURN (SPECIAL-USE)` forms are accepted.
- `CAPABILITY` now advertises RFC 5819 `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits the requested `STATUS` data directly
  after each matching selectable mailbox to reduce client folder-list round
  trips, and rejects malformed `RETURN (STATUS MESSAGES)` style status item
  lists before mailbox listing work.
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
  validated keyword atoms, returning no custom-keyword matches until durable
  user keyword storage exists and treating active messages as unkeyworded.
- `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS (...)]` and
  `BODY.PEEK[HEADER.FIELDS (...)]` for lightweight preview metadata reads.
- `FETCH`/`UID FETCH` now supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  reads.
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
  validators, so incomplete `UID SEARCH`, `UID FETCH`, `UID STORE`,
  `UID EXPUNGE`, and `UID COPY` produce precise tagged `BAD` responses instead
  of a generic UID-dispatch failure.
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
- `AUTHENTICATE PLAIN` now supports `SASL-IR` initial responses, reducing
  authentication round trips for compatible IMAP clients.
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

Next:

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
