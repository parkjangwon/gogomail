# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Mail API exposes `GET /api/v1/webmail/capabilities` so production webmail
  clients can discover backend contract version, available/planned modules,
  supported flags/actions, and compose/search/attachment/push limits without
  hard-coded frontend constants.
- Mail API exposes `GET /api/v1/mailbox/overview` so production webmail chrome
  can render total/unread/starred/size badges and system-folder shortcuts from
  one user-scoped bootstrap read.
- Mail API message lists support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters for fast
  unread/read/starred/attachment webmail views while retaining opaque cursor
  pagination and folder scoping.
- Mail API thread lists support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters for
  conversation-level unread/read/starred/attachment quick views while retaining
  opaque cursor pagination.
- Mail API thread lists support `folder_id` so production webmail can render
  folder-scoped conversation views without dropping back to flat message lists.
- Mail API message and thread lists support `sort=newest|oldest` so production
  webmail clients can offer newest-first and oldest-first list views with
  bounded validation and cursor-compatible ordering.
- Mail API message and thread summaries expose a required bounded `preview`
  string from the asynchronous search-document read model, keeping production
  webmail list rendering informative without parsing stored EML bodies on the
  list hot path.
- Mail API exposes bounded thread-level bulk flag updates for efficient
  conversation-list read/starred/answered/forwarded actions with IMAP flag
  event fanout for the updated messages.
- Mail API exposes bounded thread-level folder moves for efficient
  conversation-list archive/move workflows while preserving destination-folder
  validation, IMAP UID invalidation, and expunge event fanout.
- Mail API exposes bounded thread-level soft deletes for efficient
  conversation-list trash/delete workflows while preserving quota decrement,
  IMAP UID invalidation, and expunge event fanout.
- Mail API exposes single-message and bounded bulk message restore for
  soft-deleted messages while preserving hierarchical quota checks before
  restored messages become active again.
- Mail API exposes bounded thread-level restore so production webmail clients
  can recover whole soft-deleted conversations behind the same quota guard.
- Restore actions best-effort assign IMAP UIDs and publish `EXISTS` events for
  restored active messages, so connected IMAP clients can observe webmail
  recovery actions without a separate operator backfill step.
- Shared storage exposes backend-neutral `Stat`, `Copy`, `Move`, and bounded
  `List` primitives across local/NFS and S3-compatible backends, preparing
  attachment lifecycle, future Drive, and object reconciliation work without
  binding product code to filesystem or S3 APIs directly. S3-compatible moves
  are documented as copy-then-delete rather than atomic rename.
- Shared storage also exposes bounded prefix cleanup through `DeletePrefix`,
  allowing future Drive folder deletion and lifecycle workers to delete one
  cursor page at a time instead of depending on provider-specific recursive
  delete behavior.
- Drive backend groundwork is started without frontend implementation: ADR
  0009 defines the metadata/storage/quota boundary, `drive_nodes` persists
  user-scoped file/folder metadata and lifecycle state, and `internal/drive`
  validates node names, types, and statuses before future repository/API code.
- Drive has a first internal folder-create repository mutation that derives
  tenant scope from active user metadata, validates parent folder state, and
  uses only the bound request parameters in SQL
  before insert, preparing the backend module without opening a frontend or
  HTTP API surface yet.
- Drive file finalization can now verify a staged object through the shared
  storage `Stat` contract and commit file metadata plus unified quota usage in
  one database transaction, keeping future Drive uploads aligned with mailbox
  and attachment quota semantics.
- Drive staged object upload now has a bounded Mail API route that writes
  directly through the configured storage adapter, computes size and SHA-256,
  and returns the canonical object reference needed by file finalization.
- Drive active file/folder renames are exposed through a bounded Mail API route
  that reuses repository-side name normalization and active sibling uniqueness,
  giving future webmail Drive views a basic production editing operation.
- Drive active file/folder moves are exposed through a bounded Mail API route
  that validates destination folders, root moves, and active-subtree cycle
  prevention before updating parent metadata.
- Drive active file downloads are exposed through a bounded Mail API route that
  opens file objects through the configured local/NFS, MinIO, or S3-compatible
  storage adapter and returns safe attachment/no-store/nosniff headers.
- Drive active file download headers can also be inspected with `HEAD` without
  opening or transferring file bytes, while still verifying storage-object
  existence.
- Drive active file downloads support single HTTP byte ranges through a shared
  `GetRange` storage primitive implemented for local/NFS and S3-compatible
  stores, preparing resumable downloads, media previews, and large-file
  frontend ergonomics without provider-specific object access.
- Drive download responses expose a sanitized whole-object SHA-256 header when
  metadata carries a digest, giving webmail clients a portable integrity signal
  across local/NFS, MinIO, and S3-compatible storage.
- IMAP `ENABLE` keeps RFC 5161 syntax validation ahead of authentication and
  session mutation, including malformed capability atoms.
- Drive upload-session storage now has a dedicated migration and validation
  contract for resumable uploads, preparing quota-reserving Drive upload APIs
  without binding the HTTP layer to a single storage backend.
- Drive upload-session creation now has a repository/service boundary that
  validates active users, optional active parent folders, storage backend, size,
  and expiration before recording pending upload metadata.
- Drive upload-session creation is now exposed through a Mail API route with
  OpenAPI-documented request and response envelopes, preparing frontend clients
  to start resumable uploads without direct staged-object path construction.
- Drive upload-session reads are now exposed through a Mail API route so
  frontend clients can refresh pending/uploading/finalized/canceled/expired
  state through a stable envelope.
- Drive upload-session cancelation is now exposed through a Mail API route,
  letting clients close abandoned pending/uploading/failed sessions before
  expiry cleanup handles them.
- Drive upload-session body storage now preserves retry safety by writing each
  body to a distinct object path before repository metadata update, deleting
  failed writes and superseded bodies through the shared storage adapter.
- Drive upload-session body storage is now exposed through a Mail API route
  with optional SHA-256 verification and explicit `Content-Range` rejection
  until chunked/resumable upload semantics are finalized.
- Drive upload-session finalization now has an atomic repository/service path
  that verifies stored body size, increments quota, creates file metadata, and
  marks the upload session finalized in the same transaction.
- Drive upload-session finalization is now exposed through a Mail API route,
  completing a production-facing full-body upload path from session creation to
  quota-accounted Drive file metadata.
- Webmail capabilities now expose Drive node/upload-session availability and
  Drive upload size/TTL limits, giving production clients a single bootstrap
  contract for mail, attachment, push, and Drive feature gating.
- Drive upload-session expiry can now run through bounded repository/service
  paths, marking stale writable sessions expired and deleting stored session
  bodies through the configured storage backend.
- `drive-cleanup-worker` now runs upload-session expiry before object cleanup
  failure retries, giving operators a single Drive cleanup mode for abandoned
  session bodies and permanent-delete drift.
- Drive upload sessions can now be listed through Mail API with bounded status
  and limit filters, and webmail capabilities advertise the list contract so
  clients can recover in-progress upload state.
- Drive folder contents can now be read through an internal bounded
  parent/status list model with stable folder-first ordering, preparing the
  backend shape that future Drive UI and API contracts will need.
- Drive single-node metadata can now be read through the Mail API with bounded
  status filtering, giving future detail panels and post-edit refreshes a
  stable response envelope.
- Drive can now move active file/folder metadata into trash recursively without
  deleting object bytes or decrementing quota immediately, giving the future
  product a recoverable delete path before permanent cleanup is exposed.
- Drive can now restore trashed file/folder metadata recursively, clearing the
  trash timestamp while keeping active sibling name conflicts enforced by the
  database before webmail/Drive APIs expose recoverable delete workflows.
- Drive can now permanently mark trashed file/folder metadata deleted while
  releasing file bytes from the unified quota ledger and returning object
  references for backend-specific cleanup workers.
- Drive now has a storage-object cleanup helper for permanent-delete results,
  giving cleanup workers a validated, cancellation-aware, de-duplicated path
  to remove bytes across configured local/NFS, MinIO, or S3-compatible stores.
- Drive now has an internal service workflow for permanent delete that combines
  committed metadata/quota deletion with backend object cleanup and preserves
  cleanup progress for retryable failure reporting.
- Drive object paths are now generated through canonical helpers for staged
  uploads, committed node objects, and user cleanup prefixes, keeping future
  Drive storage layout scoped and portable across storage backends.
- Drive cleanup failures after permanent-delete metadata commits can now be
  recorded as de-duplicated PostgreSQL retry records with bounded diagnostics,
  preparing worker retries and admin visibility for object cleanup drift while
  rejecting object paths outside the owning user's `drive/users/{user_id}/...`
  prefix at record ingestion.
- Drive node list surfaces now expose folder-first `sort=name|updated|created|size`
  ordering for webmail and admin Drive browsers.
- Drive node list surfaces now expose `node_type=folder|file` filters, and
  webmail capabilities advertise the supported node types.
- Webmail Drive node listing can now opt into `all_parents=true` whole-user
  Drive search/list views while rejecting ambiguous `parent_id` combinations,
  giving production compose file pickers a backend-backed search mode.
- Drive share-link metadata now has a PostgreSQL boundary and authenticated
  Mail API create/list/revoke routes, with raw bearer tokens returned only on
  creation and persisted state limited to token hashes, suffixes, permissions,
  expiry, and revoke status.
- Drive share-link public resolution/download routes now resolve only active,
  unexpired token hashes, hide storage internals from public metadata, enforce
  `download` permission before streaming bytes, and reuse Drive no-store,
  checksum, HEAD, and single-range download semantics.
- Drive public share-link metadata/download routes now support an optional
  Redis fixed-window abuse limiter with normalized remote+token bucketing and
  HTTP 429/`Retry-After` responses, giving production deployments a first
  anonymous-traffic guard.
- Drive public share-link successful metadata/download accesses, denied
  token/permission checks, and rate-limited requests now write best-effort
  immutable audit rows with sanitized link/node/request metadata when available
  plus token suffix, result, status, and remote request metadata, letting
  operators inspect public-link access attempts through the existing Admin
  audit-log filters before aggregate activity dashboards are added.
- Admin audit-log listing now supports bounded `action_prefix` filters, so
  operators can inspect action families such as `share_link.` across
  successful, denied, and rate-limited public Drive share activity using the
  existing audit surface.
- CalDAV work now has ADR 0010, `gogomail --mode=caldav` as a runtime scaffold,
  and `internal/caldavgw` tests for standards lists, DAV tokens, and canonical
  principal/calendar/object path parsing before WebDAV handlers are advertised.
- CalDAV storage now has migration-backed calendar and calendar-object tables
  plus gateway validation for calendar metadata, supported top-level iCalendar
  components, UID bounds, strong ETags, sync-token derivation, and maximum `.ics`
  object bytes.
- CalDAV WebDAV XML parsing now has bounded namespace-aware PROPFIND parsing,
  safe `Depth` header validation, `allprop` `include` support, and core REPORT
  root classification for `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection` before protocol handlers are
  advertised.
- CalDAV now has a repository boundary over the calendar storage tables for
  calendar create/list/get and object upsert/list/get/soft-delete, including
  `.ics` object-name validation, strong ETag generation, optional observed-ETag
  guards, and transactional sync-token updates.
- CalDAV object writes now have RFC 5545 iCalendar decode validation through
  `github.com/emersion/go-ical`, deriving/verifying UID and component metadata
  while enforcing `VCALENDAR` `VERSION:2.0`/`PRODID` roots and bounding
  supported component count, property count, UID size, stored body bytes,
  RFC 4791-forbidden stored `METHOD` properties, invalid `VEVENT`/`VTODO`
  duration/end property combinations, and duplicated singleton time/status
  properties on supported calendar components.
- CalDAV object `PUT` rejects non-`text/calendar` media types, non-`2.0`
  `text/calendar` version parameters, and repeated `Content-Type` headers
  before parsing `.ics` bodies, keeping the HTTP media contract aligned with
  advertised supported-calendar-data.
- CalDAV REPORT `calendar-data` parsing rejects unsupported `content-type` and
  non-`2.0` `version` attributes before projection work, so clients cannot ask
  for unadvertised calendar media variants and receive misleading data.
- CalDAV WebDAV response generation now has a reusable `multistatus` builder
  with per-property `propstat` statuses and discovery properties for
  principals, calendar collections, and calendar objects before protocol
  handlers are advertised.
- CalDAV has an internal `OPTIONS`/`PROPFIND` discovery handler boundary with
  DAV capability headers, safe depth rejection for `Depth: infinity`,
  authenticated user/path scoping, and multistatus rendering over a pluggable
  discovery store. The public listener still remains gated until auth/TLS and
  repository wiring are reviewed.
- The PostgreSQL CalDAV repository now implements that discovery store boundary
  for active principal lookup and calendar/object list/get reads, leaving
  runtime listener activation gated on auth/TLS review and compatibility tests.
- CalDAV Basic authentication groundwork now reuses the Submission
  authenticator and rejects non-TLS credential use unless explicitly allowed
  for development, preparing native CalDAV clients without query-parameter
  identity fallback in production.
- CalDAV runtime configuration now exposes a dedicated listener address and
  insecure-auth development toggle, with production validation rejecting
  insecure Basic-auth operation.
- CalDAV mode now starts a dedicated discovery-only HTTP listener backed by the
  CalDAV repository and Basic-auth resolver. It is not yet advertised as
  client-ready because REPORT and object mutation/read handlers are still
  incomplete.
- CalDAV REPORT parsing now enforces core request-shape preconditions for
  calendar query, multiget, free-busy, and sync-collection requests before
  handler logic is added.
- CalDAV now handles `REPORT calendar-multiget` for authenticated calendar
  collections, including requested `calendar-data` bodies and missing-object
  404 propstats. Scheduling handlers remain incomplete.
- CalDAV now handles calendar object `GET`, `HEAD`, `PUT`, and `DELETE` with
  strong ETag headers, bounded iCalendar validation, and conditional request
  preconditions. Broader native-client compatibility tests remain incomplete.
- CalDAV now handles `REPORT calendar-query` for authenticated calendar
  collections, including requested `calendar-data` bodies and VEVENT
  time-range filtering through the RFC 5545 parser. Calendar-query object
  listing is bounded with `limit/nresults` and one-extra-row truncation
  detection so large collections cannot silently return partial result sets.
  VEVENT recurrence sets now expand through the shared RFC 5545 parser for
  `RRULE`, `EXDATE`, and `RDATE`, with a per-object expansion cap so dense or
  unbounded rules cannot make query work unbounded. Stored recurring-event
  objects may now contain one VEVENT master plus same-UID `RECURRENCE-ID`
  detached overrides; query/free-busy evaluation scans those VEVENTs and
  suppresses replaced master occurrences. Scheduling, broader recurrence edge
  cases, and broader device/client compatibility tests remain incomplete.
- CalDAV now handles conservative RFC 6578 `REPORT sync-collection` requests:
  explicit empty-token initial sync returns active objects and the current
  collection sync token, current-token sync returns no resource responses,
  stale tokens produce a DAV `valid-sync-token` precondition error, and
  truncating limits are rejected until continuation or tombstone/change-log
  semantics are added.
- CalDAV now handles RFC 4791-shaped `REPORT free-busy-query` for authenticated
  calendar collections, returning `200 OK` `text/calendar` `VFREEBUSY` bodies
  for `Depth: 1` child VEVENT busy periods while bounding child object scans
  with `limit/nresults` and one-extra-row truncation detection. It clips to the
  requested UTC range, omits transparent/cancelled events, maps tentative
  events to `BUSY-TENTATIVE`, ingests stored VFREEBUSY `FREEBUSY` period lists,
  coalesces same-type overlaps, and rejects duplicate free-busy time ranges.
  Recurrence expansion, scheduling, and broader device/client compatibility
  tests remain incomplete.
- CalDAV now handles `MKCALENDAR` for authenticated calendar collection
  Request-URIs with UUID calendar segments, using bounded namespace-aware XML
  parsing for display name, description, and CalendarServer/Apple calendar
  color, preserving Request-URI creation semantics, and returning `201 Created`
  plus `Location`. Slug-style path aliases remain future compatibility work.
- CalDAV now handles `DELETE` for authenticated calendar collections, using
  one repository transaction to soft-delete the collection and active children
  while forbidding calendar-home and cross-user deletes. Deletion tombstones for
  incremental sync now carry a final collection-deleted sync token for
  stale-token clients.
- CalDAV now persists sync-change rows for calendar creation and object
  upsert/delete mutations, and `REPORT sync-collection` can return
  stale-but-known object updates plus response-level 404 tombstones. Unknown
  tokens still produce DAV `valid-sync-token`; collection-deleted tokens can
  now produce a valid top-level sync token after the collection row is gone,
  while sync retention policy remains incomplete.
- CalDAV now supports RFC 6764-style discovery through `/.well-known/caldav`
  redirect and authenticated root `PROPFIND /caldav/` responses for principal
  and calendar-home discovery.
- CalDAV now resolves the advertised `/caldav/principals/` principal collection
  for `PROPFIND`, including `Depth: 1` discovery of the authenticated principal
  without listing unrelated users.
- CalDAV calendar-home `PROPFIND` now keeps WebDAV `current-user-principal` and
  `owner` hrefs pointed at the canonical principal URL, preserving correct
  discovery semantics for future delegated/shared calendar access.
- CalDAV now returns RFC 3744-shaped `current-user-privilege-set` values for
  implemented behavior only: read-only principals, calendar-home calendar
  bind/unbind, collection object bind/unbind plus metadata property writes, and
  object content writes. ACL and delegation privileges stay unadvertised until
  their semantics are implemented.
- CalDAV principal `PROPFIND` can now return RFC 4791
  `calendar-user-address-set` values sourced from the Directory primary email
  as normalized `mailto:` hrefs. This improves native-client principal
  discovery and prepares organizer/attendee resolution while leaving
  scheduling, resource booking, and delegated/shared calendar access
  experimental.
- CalDAV now handles WebDAV `PROPPATCH` for authenticated calendar collection
  metadata (`displayname`, `calendar-description`, CalendarServer/Apple
  `calendar-color`) with bounded namespace-aware XML parsing, transactional
  sync-token refresh, and durable `collection-updated` sync markers.
- CalDAV collection `PROPFIND` now returns WebDAV `supported-report-set` for
  only the implemented REPORT handlers: `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection`. `Depth: 1` child object metadata
  discovery is bounded with the same one-extra-row truncation probe as REPORT
  collection scans, preventing silent partial listings.
- CalDAV `REPORT calendar-query` now applies simple top-level component
  filters through stored `component_type` metadata before time-range matching,
  keeping common client component queries more accurate and cheaper.
- CalDAV `REPORT calendar-multiget` now scopes hrefs to the request resource,
  preventing collection-level multiget requests from returning sibling
  collection objects while still allowing calendar-home same-user hrefs.
- CalDAV `PROPFIND` now returns exact WebDAV `owner`, `creationdate`, and
  `getlastmodified` metadata for calendar collections and objects, with
  principal owner hrefs and standards-shaped timestamp formatting.
- CalDAV calendar object `GET` and `HEAD` now honor `If-None-Match`, allowing
  native clients to revalidate `.ics` resources through ETags without
  restreaming unchanged bodies.
- CalDAV calendar object `PUT` now rejects explicit unsupported media types
  with HTTP 415 before parsing or storage mutation, while accepting
  `text/calendar` parameters and omitted content types for compatibility.
- CalDAV calendar object `PUT` now treats `If-Match: *` as overwrite-only and
  returns HTTP 412 when no current resource exists.
- CalDAV calendar object `PUT` now preflights specific ETag `If-Match` and
  `If-None-Match` conditions before body parsing or repository mutation.
- CalDAV calendar object `GET` and `HEAD` now fail stale `If-Match`
  preconditions before `If-None-Match` cache revalidation, and object `DELETE`
  now uses shared strong ETag list matching for conditional deletes.
- CalDAV calendar object `DELETE` now treats `If-Match: *` as an
  existing-resource precondition and returns HTTP 412 when the target `.ics`
  resource is missing.
- CalDAV calendar object `GET` and `HEAD` now emit `Last-Modified` from stored
  object update time and honor `If-Modified-Since` timestamp revalidation with
  second-precision comparisons.
- CalDAV calendar object `PUT` and `DELETE` now honor `If-Unmodified-Since`
  against stored object update time before reading request bodies or mutating
  the repository, returning HTTP 412 for stale timestamp guards.
- S3-compatible `GetRange` now returns a reader bounded to the validated
  requested byte length even if a compatible provider sends an oversized
  `206 Partial Content` body, aligning remote range reads with local/NFS
  adapter guarantees.
- CalDAV calendar object `GET` and `HEAD` now honor `If-Unmodified-Since`
  before ETag/date cache revalidation, returning HTTP 412 when timestamp
  preconditions are stale.
- S3-compatible `GetRange` now validates `Content-Range` against the requested
  byte window before exposing the response body, and closes mismatched partial
  responses early.
- S3-compatible `GetRange` now reports `io.ErrUnexpectedEOF` if a provider
  returns a matching partial response header but truncates the body before the
  requested byte count.
- S3-compatible `GetRange` now drains a small bounded remainder on successful
  range-reader close, preserving HTTP connection reuse for oversized partial
  responses without exposing extra bytes to callers.
- S3-compatible `GetRange` now also bounded-drains unread range bytes on early
  close, improving connection reuse for canceled preview/download paths without
  unbounded cleanup reads.
- S3-compatible full-object `GET` readers now also bounded-drain a small
  remainder on close, improving HTTP connection reuse for preview/cancel
  download paths without unbounded cleanup reads.
- Local/NFS and S3-compatible `Get`/`GetRange` readers now observe context
  cancellation after opening the stream, and local/NFS `GetRange` reports
  `io.ErrUnexpectedEOF` for short requested windows so storage backend flips do
  not change partial-read failure semantics.
- Local/NFS storage now rejects filesystem symlinks for object reads, range
  reads, metadata probes, deletes, and source moves, omits them from list
  pages, and rejects direct directory deletes so NFS/local deployments preserve
  object-store semantics even on link-capable filesystems.
- S3-compatible `GET`, ranged `GET`, and `HEAD`/`Stat` now wrap
  `os.ErrNotExist` on `404 Not Found`, so Drive, attachment lifecycle, and mail
  storage callers can use the same missing-object checks across local/NFS,
  MinIO, and AWS S3-style backends.
- IMAP `STATUS` and LIST-STATUS now reject duplicate status data items before
  mailbox metadata lookup, avoiding ambiguous duplicate status pairs in
  client-visible responses.
- CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading the
  XML request body when no active collection exists at that path, preserving
  the UUID-only creation contract without extra parse work.
- CalDAV collection `DELETE` now evaluates `If-Unmodified-Since` and
  strong collection `If-Match` values before repository mutation, preventing
  stale native-client collection deletes while keeping `If-Match: *` as an
  existing-collection guard.
- CalDAV collection `PROPPATCH` now shares the same precondition gate,
  preventing stale metadata edits and failing mismatched collection ETag
  conditions before XML request bodies are read or parsed.
- CalDAV `REPORT` now validates malformed Depth values and rejects
  `Depth: infinity` before XML body reads, making unsupported traversal
  semantics cheap and consistent across implemented REPORT handlers.
- CalDAV `REPORT` and `PROPFIND` also reject repeated HTTP `Depth` headers
  before XML body parsing, avoiding ambiguous traversal scope at the WebDAV
  handler boundary.
- CalDAV object and collection preconditions now combine repeated `If-Match`
  and `If-None-Match` headers into one ETag list before evaluation, matching
  HTTP conditional request semantics for cache validation and write guards.
- CalDAV `REPORT sync-collection` now requires the default/explicit HTTP
  `Depth: 0` request scope before repository lookup or change-log work, keeping
  WebDAV sync traversal governed by the request-body `sync-level` and matching
  the CardDAV sync contract.
- CalDAV `REPORT sync-collection` also requires the request body to include an
  explicit `DAV:sync-token` element, accepting an empty value for initial sync
  while rejecting omitted sync-token anchors before repository work.
- CalDAV stale-token `sync-collection` delta handling now probes one change-log
  row beyond bounded `limit/nresults`, so exact-limit responses are not falsely
  rejected while truly truncating deltas still fail closed.
- CalDAV initial `sync-collection` snapshots now use a sync-specific
  one-extra-object repository list path, preventing omitted-limit snapshots
  from being clipped by generic list defaults while still returning the current
  collection sync token.
- CalDAV `REPORT calendar-query` now keeps child calendar-object scans behind
  explicit `Depth: 1`; default/explicit `Depth: 0` collection queries return no
  child object matches, preserving WebDAV request-scope semantics for native
  clients.
- CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI hrefs from native
  clients by normalizing only the URI path through the existing CalDAV scope
  checks, while rejecting userinfo-bearing authorities, query, fragment,
  opaque, non-HTTP(S), or unsafe href forms.
- CalDAV remains experimental/backend-only for this release slice. Public
  client-ready status is gated on recurrence, scheduling, retention-aware sync,
  collection-deletion deltas, broad native-client compatibility tests, and the
  shared Directory/Identity, Contacts/CardDAV, Notification & Sync, Search, and
  Policy/Audit boundaries needed for shared/delegated/resource calendars.
- The first Directory/Identity boundary is intentionally narrow:
  `internal/directory` resolves active user principals through shared
  user/domain/company state and organization principals through
  organization/domain/company state. Directory storage now also defines groups,
  resources, aliases, and group memberships, and the resolver can load
  group/resource principals, normalized alias targets, and direct group
  memberships. Effective membership expansion is bounded with an explicit
  recursion cap and cycle guard. Directory also has a company-scoped delegation
  table and check boundary for owner/delegate principals, product scopes, and
  hierarchical `read`/`write`/`manage` roles. Effective delegation now expands
  group delegates through bounded nested membership and verifies active
  owner/delegate principals under the requested company scope, so group-granted
  access can satisfy user, organization, group, or resource delegates without
  creating product-local sharing rows, but protocol modules do not use it for
  public sharing yet. CalDAV discovery delegates active user lookup
  to this boundary, keeps calendar-home `current-user-principal` discovery
  anchored to canonical principal URLs, and advertises only local-user WebDAV
  privileges that are implemented today. Directory also exposes a bounded
  `SearchPrincipals` repository boundary for company-scoped user,
  organization, group, and resource search, with validated scope, kind, query,
  and limit inputs plus escaped SQL `LIKE` wildcard handling. This prepares
  CalDAV attendee/resource lookup, Contacts/CardDAV autocomplete, shared inbox
  targeting, and admin consoles without putting principal search semantics in
  product modules. Directory delegation inspection is also now bounded through
  `ListDelegations`, which validates company scope, optional owner/delegate
  principal filters, delegation scope, role, active-only state, and result
  limits before SQL execution. This gives admin consoles, shared-calendar
  management, Drive shares, shared inboxes, and future Contacts/CardDAV
  delegation one observable relationship read model. This does not make shared
  calendars, resource booking, or delegated access public-release ready yet.
  The same boundary is now available to operators through
  `GET /admin/v1/directory/delegations`, with OpenAPI and backend contract
  coverage for bounded company, owner, delegate, scope, role, active-only, and
  limit filters. Audited creation is now available through
  `POST /admin/v1/directory/delegations`, validating active same-company
  principals and committing `directory_delegation.create` with the grant insert.
  Audited deletion is available through
  `DELETE /admin/v1/directory/delegations/{id}`, committing
  `directory_delegation.delete` with the soft delete.
  Audited group membership creation is available through
  `POST /admin/v1/directory/group-memberships`, validating active same-company
  principals and nested group cycles before committing
  `directory_group_membership.create` with the insert.
  Group membership listing is available through
  `GET /admin/v1/directory/group-memberships`, with OpenAPI and backend
  contract coverage for bounded company, group, member, role, active-only, and
  limit filters.
  Audited group membership deletion is available through
  `DELETE /admin/v1/directory/group-memberships/{id}`, committing
  `directory_group_membership.delete` with the soft delete.
  Audited group membership role updates are available through
  `PATCH /admin/v1/directory/group-memberships/{id}/role`, committing
  `directory_group_membership.role_update` with the role change.
  Audited group membership reassignment is available through
  `PATCH /admin/v1/directory/group-memberships/{id}/assignment`, committing
  `directory_group_membership.reassign` with same-company, cycle, and duplicate
  guards.
  Directory principal search is now also available to operators through
  `GET /admin/v1/directory/principals`, with OpenAPI and backend contract
  coverage for bounded company, domain, organization, kind, query, active-only,
  and limit filters. This prepares admin console and future product
  autocomplete flows without making scheduling or sharing public-release ready.
  Directory alias resolution is now available through
  `GET /admin/v1/directory/aliases/resolve`, with OpenAPI and backend contract
  coverage for address normalization and active-only target-principal lookup.
  This prepares mail-routing diagnostics, attendee resolution, and shared inbox
  targeting without duplicating address parsing in product modules.
  Directory alias listing now also has a bounded repository boundary with
  company/domain, target-principal, text query, active-only, and result-limit
  validation, preparing admin alias management without exposing raw
  `directory_aliases` queries. The admin backend API now exposes that boundary
  through `GET /admin/v1/directory/aliases`, with OpenAPI and backend contract
  coverage for target-principal hydrated list responses.
  Directory alias creation now has a guarded repository mutation boundary that
  normalizes addresses, requires active company/domain scope, enforces
  alias-domain alignment, verifies active same-company target principals, and
  maps active-address unique-index races to a stable duplicate-alias error.
  The admin backend API now exposes that audited mutation at
  `POST /admin/v1/directory/aliases`; the alias insert and
  `directory_alias.create` audit row commit in one transaction. Public
  shared-inbox UX and non-admin alias mutation flows remain outside this
  release slice. Audited alias deletion is also available at
  `DELETE /admin/v1/directory/aliases/{id}`, soft-deleting active aliases and
  recording `directory_alias.delete` in the same transaction.
  `internal/accesspolicy` now wraps effective delegation into explicit
  allow/deny decisions so future protocol modules can attach product policy,
  WebDAV privilege mapping, and audit logging without reading Directory rows
  directly. Its initial WebDAV privilege mapper covers delegated
  read/write/manage decisions without making those delegated privileges public
  yet, and its delegated-access audit detail builder emits normalized principal
  and privilege fields with fixed allow/deny reason enums for predictable
  operational logs. The same package now builds the standard delegated-access
  audit envelope so product adapters can insert one consistent `access` /
  `delegation.access_checked` record shape when delegated checks are wired.
  Admin audit-log listing now exposes bounded `actor_id` and `target_id`
  filters with actor/time and target/time read indexes, so delegated-access
  audit records are queryable by acting principal or owner/resource target.
  `accesspolicy` also provides a repository-backed delegated-access audit
  recorder, keeping future protocol integrations on one policy/audit insertion
  boundary. A composed delegated-access authorizer now joins the effective
  delegation check and audit insertion into one fail-closed operation before
  CalDAV/CardDAV/Drive/mailbox sharing surfaces become public.
- CardDAV is pre-public and backend-only. ADR 0012 and `internal/carddavgw`
  currently cover standards constants, DAV tokens, canonical principal,
  address-book home, address-book collection, and `.vcf` object path/href
  handling, plus metadata validation for address-book/contact object names,
  UIDs, strong ETags, size limits, and sync tokens. PostgreSQL storage tables
  now exist for address books, contact objects, and address-book change logs.
  Address-book repository methods can create/list/get active collections while
  recording creation changes transactionally. vCard validation now performs
  bounded vCard 4.0 and common vCard 3.0 checks for BEGIN/END structure,
  VERSION, UID, FN, folded lines, line/body caps, and nested VCARD rejection.
  Content-line parsing preserves quoted parameter values containing colons
  before the unquoted value separator. Contact-object `PUT` rejects repeated
  `Content-Type` headers before vCard media parsing. Contact-object repository
  methods can upsert/list/get/delete `.vcf` resources with active address-book
  scope, UID alignment, strong ETags, optional observed-ETag guards, sync-token
  refreshes, and durable change rows. REPORT parsing recognizes bounded
  `addressbook-query`, `addressbook-multiget`, and `sync-collection` bodies,
  including properties, hrefs, sync token/level, limits, filter/prop-filter
  `test` attributes, text-match predicates, and nested param-filters. WebDAV
  multistatus response building can render CardDAV principal, address-book,
  contact-object, REPORT, and sync metadata. An internal
  RFC 6764/WebDAV-style discovery handler now covers `/.well-known/carddav`,
  `OPTIONS`, and bounded `PROPFIND` over root, the advertised principal
  collection, principal, address-book home, address-book collection, and
  contact-object resources, with cross-user,
  `Depth: infinity`, malformed XML, and contact-object depth guards. The
  PostgreSQL repository satisfies that discovery store through the shared
  Directory principal resolver. Internal REPORT execution now covers
  `addressbook-multiget`, `addressbook-query`, and `sync-collection`, including
  scoped href handling, optional `address-data`, current sync-token emission,
  bounded change reads since a stored sync token, and deleted contact 404
  responses. `addressbook-multiget` requires an explicit `Depth` header before
  resolving requested hrefs, while accepting common Depth 0/1 client shapes.
  Query filtering evaluates multiple `prop-filter` predicates and
  multiple per-property text/parameter conditions with RFC 6352
  `test=anyof|allof` composition. Text-match evaluation honors the RFC default
  `i;unicode-casemap`, `equals`, `contains`, `starts-with`, `ends-with`, and
  `negate-condition`, while rejecting unsupported collations or malformed
  text-match attributes. Query filtering also parses vCard content-line
  parameters for `param-filter` existence, `is-not-defined`, and parameter
  text-match checks. Unsupported vCard property or parameter filters fail with
  the RFC 6352 `CARDDAV:supported-filter` precondition instead of misleading
  empty success responses, including `Depth: 0` requests that otherwise return
  no child objects. Unsupported CardDAV filter child elements now use the same
  `CARDDAV:supported-filter` precondition. REPORT `address-data` can project
  returned vCards to
  requested property names while preserving structural BEGIN/VERSION/END lines.
  Requested address-data content types and versions are validated against the
  advertised `text/vcard` 4.0/3.0 support and fail with the RFC 6352
  `CARDDAV:supported-address-data` precondition before handler execution.
  Unsupported text-match collations now fail with the RFC 6352
  `CARDDAV:supported-collation` precondition, while malformed collation syntax
  remains a bad request. Address-book collections advertise RFC 6352
  `CARDDAV:supported-collation-set` with `i;ascii-casemap` and
  `i;unicode-casemap`, and query evaluation implements both advertised
  collations. Capability properties that should not appear in a bare `allprop`
  response remain available through explicit `prop`, `include`, and `propname`
  discovery.
  Returned `address-data` also carries explicit `content-type="text/vcard"` and
  a `version` attribute matching the stored vCard body.
  `addressbook-query` execution honors bounded `limit/nresults` response caps.
  Repository-backed query execution can stream contact objects and stop once the
  response cap is satisfied instead of materializing the whole address book.
  Address-data projection failures are explicit errors rather than silent
  full-body fallbacks. RFC 6352 `addressbook-query` now requires an explicit
  `Depth` header, uses `Depth: 1` for child address-object scans, accepts
  `Depth: infinity` with the same flat address-book scan semantics, and keeps
  `Depth: 0` collection-scoped without returning child objects. PROPFIND
  responses expose conservative RFC 3744-style
  current-user privileges: readable resources return `DAV:read`, address-book
  homes also return `DAV:bind`/`DAV:unbind` because extended `MKCOL` can create
  child address-book collections and collection `DELETE` can remove them,
  address-book collections also return `DAV:bind`/`DAV:unbind` because
  contact-object `PUT`/`DELETE` can bind or unbind child `.vcf` members,
  collections also return `DAV:write-properties` because collection
  `PROPPATCH` semantics are implemented, and contact objects also return
  `DAV:write-content` because object write semantics are implemented. ACL and
  unimplemented write privileges remain unadvertised.
  Address-book collection PROPFIND also
  exposes CalendarServer-compatible `getctag` from the same durable sync token
  as WebDAV `sync-token`, giving legacy clients change detection without
  adding a second versioning model. `PROPFIND Depth: 1` child-object discovery
  is bounded with the shared one-extra-row truncation probe, preventing silent
  partial contact metadata listings. Address-book collection discovery also
  returns RFC 6352 `addressbook-description` from stored metadata. WebDAV
  `PROPPATCH` can update authenticated address-book collection `displayname`
  and `addressbook-description` through bounded XML parsing and a small
  repository boundary that refreshes sync state and records an
  `addressbook-updated` change. Address-book collections derive a strong ETag
  from the durable sync token, expose it through WebDAV `getetag`, and enforce
  `If-Match`/`If-Unmodified-Since` on collection `PROPPATCH` before reading
  XML request bodies. RFC 6352-style extended `MKCOL` can create authenticated
  address-book collections at UUID request-URI paths with bounded
  `DAV:resourcetype`, `DAV:displayname`, and `addressbook-description`
  parsing, repository-backed sync/change state, and `201 Created` plus
  `Location` responses.
  Address-book collection `DELETE` soft-deletes the collection and active child
  contact objects transactionally, honors collection preconditions, records an
  `addressbook-deleted` change row, and rejects unsafe targets.
  `sync-collection` can answer stale-token requests after collection deletion
  by returning the latest durable deletion sync token without requiring the
  collection to remain active. It also enforces RFC 6578 Depth behavior by
  accepting default/explicit `Depth: 0` and rejecting `Depth: 1` before sync
  lookup or change-log work. `sync-collection` parsing distinguishes empty
  initial `DAV:sync-token` elements from missing token elements and rejects the
  latter before sync lookup or snapshot work. `REPORT` and `PROPFIND` reject
  repeated HTTP `Depth` headers before XML body parsing, keeping address-book
  traversal scope deterministic. Object and collection preconditions combine
  repeated `If-Match` and `If-None-Match` headers into one ETag list before
  evaluation. Stale-token delta reads probe one
  change-log row beyond bounded `limit/nresults`, so exact-limit responses are
  not falsely rejected while truly truncating deltas still fail closed. Initial
  snapshots use the same one-extra-object repository probe, preventing large
  address books from being reported fully synchronized after a generic list cap.
  Contact-object writes preflight duplicate active vCard UIDs within the same
  address book before SQL upsert, keeping failures predictable while the
  PostgreSQL partial unique index remains the final concurrency guard.
  Unique-index races for active contact-object names or UIDs are mapped back to
  stable duplicate repository errors instead of leaking raw driver diagnostics.
  Contact-object `DELETE` carries observed strong ETags into the repository
  transaction so `If-Match` state is rechecked under the address-book lock
  before the active object row is deleted.
  Contact-object `GET`, `HEAD`, `PUT`, and
  `DELETE` now run inside the internal handler with `text/vcard` validation,
  explicit 3.0/4.0 media-type version matching, bounded body reads, ETag and
  Last-Modified headers, cache/precondition handling, and repository-backed
  vCard validation. CardDAV runtime wiring now provides
  `gogomail --mode=carddav`, `GOGOMAIL_CARDDAV_ADDR`, and Basic-auth through
  the existing Submission authenticator, with production insecure-auth
  validation. Client-ready CardDAV remains gated on broader vCard compatibility
  and native-client tests.
- Admin Drive node inspection can now opt into `all_parents=true` whole-user
  inventory search while rejecting ambiguous parent-scoped combinations.
- Drive cleanup-failure records can now be listed and resolved through bounded
  repository methods, giving future cleanup retry workers and admin consoles a
  controlled path to inspect and close pending object cleanup drift.
- Drive cleanup drift can now be retried through an internal service method
  that deletes pending object references, resolves successful records, and
  refreshes failed-attempt diagnostics for future scheduled worker wiring.
- Drive cleanup retry now has a dedicated `drive-cleanup-worker` backend mode
  with validated interval/batch/run-once settings and shared storage adapter
  wiring, so object cleanup drift can be handled outside request paths.
- Drive has first authenticated Mail API routes for node list, folder create,
  trash, restore, and permanent delete, with OpenAPI response envelopes ready
  for future webmail integration.
- Drive file metadata finalization is now exposed through Mail API, verifying
  staged objects through shared storage before committing metadata and unified
  quota usage.
- Admin API exposes `GET /admin/v1/console/capabilities` so production
  operator consoles can discover backend contract version, available
  modules, tenant/domain/user surfaces, operational triage areas, and
  list/cleanup/retention limits before rendering navigation or forms.
- Admin API exposes `GET /admin/v1/drive-upload-sessions` so operator consoles
  can inspect Drive upload session state by required user scope and optional
  lifecycle status before broader Drive admin APIs are built.
- Admin API exposes `GET /admin/v1/drive-nodes` so operator consoles can
  inspect a user's Drive root or folder-scoped inventory with bounded
  lifecycle and name filters before frontend Drive administration starts.
- Admin API exposes `GET /admin/v1/drive-nodes/{id}` so operator consoles can
  inspect one Drive file/folder metadata row with explicit user scope and
  lifecycle filtering.
- Admin API exposes `GET /admin/v1/drive-usage` so operator consoles can show
  user Drive quota, node lifecycle, byte usage, and pending upload-session
  dashboard summaries.
- Mail API exposes `GET /api/v1/drive/usage` so production webmail Drive
  panels can render the authenticated user's quota and storage summary without
  an admin token.
- Admin API exposes `POST /admin/v1/drive-upload-cleanup/candidates` so
  operator consoles can preview stale Drive upload-session cleanup impact
  before relying on the worker loop.
- Admin API exposes `POST /admin/v1/drive-upload-cleanup/runs` so operators can
  trigger explicit audited Drive upload-session expiry with candidate counts.
- Admin API exposes `GET /admin/v1/drive-cleanup-failures` so operators can
  inspect pending/resolved Drive object cleanup drift with bounded filters.
- Admin API exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve` for
  audited manual closure of Drive cleanup drift after external verification.
- Admin API exposes `POST /admin/v1/drive-cleanup-failures/retry-runs` for
  audited bounded retry of pending Drive cleanup drift, returning
  scanned/deleted/resolved/failed counts that an operator console can render
  without tailing worker logs.
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
- IMAP `LIST`/`LSUB` CHILDREN metadata now marks nested immediate parents with
  `\HasChildren` even when backend mailbox rows only provide `FullPath`, keeping
  hierarchy navigation compatible with clients that trust CHILDREN attributes.
- IMAP flag-list parsing for `APPEND`, `STORE`, and `UID STORE` now rejects
  unparenthesized or unbalanced lists before backend mutation, preserving
  stricter protocol syntax for client compatibility.
- IMAP `APPEND` internaldate parsing now enforces RFC 3501 fixed-width
  `date-day-fixed` syntax, accepting zero-padded or space-padded days while
  rejecting bare one-digit dates such as `"5-May-2026 ..."`.
- IMAP selected-mailbox `STORE` and `UID STORE` now honor advertised
  `[PERMANENTFLAGS]`, rejecting otherwise valid system flags when the selected
  mailbox did not permit them instead of dispatching unsupported mutations to
  storage. Empty `+FLAGS ()` and `-FLAGS ()` remain successful no-ops, while
  `FLAGS ()` replacement is rejected when no permanent flags are permitted.
- IMAP message sequence sets reject values above the selected mailbox size with
  tagged `BAD` responses, preserving RFC 3501 bounds behavior for sequence
  arguments.
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
  compatibility with clients that do not zero-pad SEARCH dates.
- IMAP `SEARCH` and `UID SEARCH` reject `CHARSET` prefixes that omit the
  required following search-key before authentication or selected-mailbox
  checks, keeping RFC 3501 grammar failures separate from state failures.
- IMAP `FETCH` and `UID FETCH` reject malformed fetch data-item syntax such as
  nested `((FLAGS))` before authentication or selected-mailbox checks, keeping
  RFC 3501 fetch grammar failures separate from state failures.
- IMAP `STORE` and `UID STORE` reject malformed `UNCHANGEDSINCE`, store mode,
  and flag-list syntax before authentication or selected-mailbox checks,
  keeping RFC 3501/CONDSTORE mutation grammar failures separate from state
  failures.
- IMAP selected-state commands reject malformed message sequence-set and UID
  set syntax, including signed values such as `+1`/`+7`, before authentication
  or selected-mailbox checks while preserving selected-mailbox bounds checks
  for execution time.
- IMAP `SEARCH` and `UID SEARCH` reject malformed search sequence-set and
  `UID` search-key set syntax before authentication or selected-mailbox checks,
  so signed values such as `SEARCH +1` and `UID SEARCH UID +7` fail as grammar
  errors rather than state errors.
- IMAP `SORT`, `UID SORT`, `THREAD`, and `UID THREAD` reuse the same
  syntax-only search-key validation before authentication or selected-mailbox
  checks, keeping malformed embedded search criteria consistent across the
  search/sort/thread command family.
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
- IMAP RFC 2971 `ID` parameter-list parsing now accepts bounded synchronizing
  and non-synchronizing string literals inside the parenthesized field/value
  list, while missing or unused literal payloads remain syntax errors.
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
- IMAP `SELECT`/`EXAMINE` reject bare or over-parenthesized `CONDSTORE`
  select parameters before authentication or backend mailbox lookup, keeping
  optional select-param syntax RFC-shaped while preserving valid
  `SELECT inbox (CONDSTORE)`.
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
- Local/NFS-style storage writes now stage through unique temporary files in
  the destination directory before `rename`, avoiding fixed `.tmp` collisions
  while preserving atomic replacement semantics for local deployments.
- Local/NFS-style storage writes now honor context cancellation during body
  copy, cleaning staged temp objects and avoiding partial object commits after a
  canceled request.
- Local/NFS storage configuration requires a non-empty bounded
  `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
  `GOGOMAIL_STORAGE_BACKEND=local`, so broken filesystem roots fail during
  config validation instead of surfacing later as storage probe errors.
- Local and S3-compatible storage writes reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- Local/NFS and S3-compatible storage expose a shared object `Stat` contract
  for canonical object keys, byte size, and backend metadata without streaming
  object bodies. S3-compatible storage implements this through signed `HEAD`
  requests, giving future Drive and lifecycle workers a portable size/existence
  primitive across local, MinIO, and AWS S3 deployments.
- S3-compatible storage bounds and sanitizes provider-returned `Content-Type`
  and ETag metadata from `Stat`/`List`, dropping unsafe multiline, invalid
  UTF-8, or oversized values without failing the object identity/size result.
- S3-compatible `PutObject`, full-object `GET`, `HEAD`/`Stat`, and
  `ListObjectsV2` require exact `200 OK` responses, rejecting accepted/deferred
  writes, unexpected partial-content, or other non-OK 2xx statuses before
  callers can treat them as durable or complete local/NFS-style object results.
- Shared storage object path, prefix, and list-cursor validation now rejects
  invalid UTF-8 before local/NFS or S3-compatible adapter use, keeping object
  keys, logs, URL escaping, and SigV4 canonical paths text-stable.
- Local/NFS and S3-compatible storage expose a shared object `Copy` contract.
  Local/NFS copies reuse atomic temporary-file commits, and S3-compatible
  copies use signed server-side copy requests with escaped `x-amz-copy-source`
  values so future Drive and lifecycle workflows can duplicate objects without
  forcing caller-side body streaming.
- S3-compatible `Copy` now requires exact `200 OK` responses with bounded
  `CopyObjectResult` bodies and rejects empty bodies, unexpected XML, and
  embedded `<Error>` XML inside `200 OK` responses, so provider-side copy
  failures cannot be mistaken for successful Drive or lifecycle duplication.
- Local/NFS and S3-compatible storage expose a shared bounded prefix `List`
  contract. Local/NFS uses directory walks, and S3-compatible storage uses
  signed `ListObjectsV2`, giving future Drive, lifecycle, and reconciliation
  workflows a portable cursor-paginated object metadata scan.
- S3-compatible `ListObjectsV2` decoding now rejects truncated pages that omit
  a continuation token, so cleanup and reconciliation workers do not accept a
  provider response that cannot be advanced safely.
- S3-compatible `ListObjectsV2` decoding now also requires a
  `ListBucketResult` XML root, preventing unexpected provider success XML from
  being accepted as an empty list page.
- S3-compatible `ListObjectsV2` key decoding preserves provider-returned object
  key identity by rejecting keys that would require leading/trailing whitespace
  trimming before prefix/object-path validation.
- S3-compatible `ListObjectsV2` object-size validation now runs after provider
  keys map back to the requested canonical gogomail prefix, so out-of-scope
  bucket entries are skipped before their metadata can fail a valid listing.
- Shared storage list cursors reject leading/trailing whitespace and control
  characters instead of trimming opaque provider tokens, keeping local/NFS and
  S3-compatible pagination identity exact for Drive, lifecycle, and
  reconciliation scans.
- S3-compatible `ListObjectsV2` pages reject provider responses that return
  more matching objects than the requested bounded page size, keeping S3,
  MinIO, and local/NFS pagination under the same storage contract.
- Local/NFS-style storage deletes are idempotent for missing objects.
  S3-compatible deletes accept completed `200 OK`/`204 No Content` responses
  plus idempotent `404 Not Found`, while rejecting accepted/deferred or other
  ambiguous non-OK 2xx statuses before cleanup workers mark object deletion as
  complete.
- S3-compatible storage requests reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, and `DELETE` responses drain a small
  bounded response-body window before close, improving HTTP connection reuse
  for normal S3/MinIO responses without allowing oversized bodies to stall
  cleanup.
- Local/NFS and S3-compatible readiness probes read the verification object
  through a tight expected-size bound, preventing malformed or proxy-inflated
  probe responses from allocating unbounded memory during `/health/ready`
  checks.
- Local/NFS and S3-compatible readiness probes also verify probe-object `Stat`
  metadata, catching broken filesystem metadata or S3 `HEAD` paths before an
  instance reports ready.
- Local/NFS and S3-compatible readiness probes also verify a short `GetRange`
  against the probe object, catching broken filesystem seek/range handling or
  S3 `Range` response compatibility before partial-read workflows report ready.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, and HTTP runtimes now share storage backend validation and factory
  wiring for local filesystem/NFS-style storage plus S3-compatible object
  storage. `GOGOMAIL_STORAGE_BACKEND=s3` can target AWS S3, while
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same S3-compatible adapter with
  path-style requests for local MinIO-style deployments. Both paths use endpoint,
  region, bucket, prefix, credential, and session-token settings. Runtime option
  construction is covered so MinIO remains path-style by default, ordinary S3
  remains virtual-hosted by default, and
  `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` remains an explicit S3 override.
  Localhost and IP-address endpoints also switch to path-style addressing
  automatically, avoiding `bucket.localhost`/`bucket.127.0.0.1` drift for local
  S3-compatible stores even when `GOGOMAIL_STORAGE_BACKEND=s3` is used.
  Drive runtime wiring registers the active S3-compatible store under both
  `s3` and `minio` labels, keeping existing Drive/upload rows reachable across
  MinIO-to-AWS S3-style backend flips when object keys and bucket contents have
  been migrated.
  `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS` now provides an explicit,
  fail-closed Drive migration bridge for other legacy storage labels, such as
  serving historical `local`/NFS-labelled rows through the configured S3 store
  only after operators have copied object bytes and opted into the label map.
  S3 request paths preserve literal `+` characters as `%2B` so object identity
  and SigV4 canonical paths do not drift for plus-bearing mail object keys or
  plus-bearing endpoint base paths.
  Endpoint base paths reject encoded path separators such as `%2F` and `%5C`;
  bucket names must start and end with a letter or digit, matching AWS S3 naming
  rules before adapter construction. Seekable PUT
  bodies also get deterministic `Content-Length` values without object
  buffering, improving S3-compatible provider behavior for file-backed mail and
  attachment writes. S3-compatible deletes treat `404 Not Found` as
  already-cleaned success, keeping lifecycle cleanup idempotent across
  compatible providers and local/NFS storage. Access key IDs, secret access
  keys, and session tokens reject whitespace and oversized values during
  config validation and direct adapter construction so copied env/config
  mistakes fail before runtime S3 authentication attempts or SigV4 header
  construction.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, including the `GOGOMAIL_STORAGE_ROOT` compatibility alias for
  `GOGOMAIL_MAILSTORE_ROOT`, and the development compose stack includes
  `minio-init` to create the default local `gogomail` bucket.
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
- IMAP IDLE is now advertised and accepted. `internal/imapgw` uses the
  in-memory mailbox event broker for live selected-mailbox session fan-out; the
  broker is scoped by user+mailbox, and service-side flag/move/delete mutations
  publish best-effort `flags`/`expunge` events for UID-visible messages. Mail
  API detail reads that auto-mark unread messages as read also publish `flags`
  events after a successful read-flag write.
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
- IMAP `CAPABILITY` now advertises `AUTH=PLAIN` only before authentication,
  aligning the first command surface with RFC client state expectations.
- IMAP `AUTHENTICATE PLAIN` now supports the standard continuation response and
  SASL PLAIN credential decoding, and RFC-shaped tagged `BAD` cancellation, so
  the advertised `AUTH=PLAIN` mechanism has a real protocol implementation.
  Mismatched SASL PLAIN authorization identities are rejected instead of being
  silently treated as ordinary user/password authentication. Failed `LOGIN` and
  `AUTHENTICATE` attempts include RFC 5530 `[AUTHENTICATIONFAILED]` response
  codes for better client and migration-tool diagnostics.
- IMAP advertises `SASL-IR` before authentication and accepts
  `AUTHENTICATE PLAIN` initial responses to reduce compatible client auth
  round trips. Unsupported but syntactically valid SASL mechanisms return
  tagged `NO`, keeping mechanism probing distinct from malformed auth syntax.
- IMAP SASL PLAIN decoding now bounds encoded and decoded response bytes before
  credential splitting or backend authentication, keeping continuation and
  `SASL-IR` literal initial-response paths allocation-aware.
  tagged `NO`, keeping mechanism probing distinct from malformed auth syntax.
- IMAP successful `LOGIN` and `AUTHENTICATE PLAIN` responses include the
  authenticated `[CAPABILITY ...]` response code, so clients can refresh
  post-auth extension state without an extra probe and without retaining
  pre-auth auth mechanisms.
- IMAP greetings include state-aware `[CAPABILITY ...]` response codes:
  plaintext TLS-required sessions advertise `STARTTLS`/`LOGINDISABLED`, while
  implicit TLS sessions advertise immediate `SASL-IR`/`AUTH=PLAIN` support.
- IMAP `LOGIN` and SASL PLAIN decoded credentials reject blank, CR/LF-bearing,
  or oversized authentication identities plus empty, oversized, or
  CR/LF-bearing passwords at the protocol boundary before backend auth work,
  while preserving intentional leading/trailing spaces in RFC string
  credentials.
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
- IMAP `LIST` now decodes RFC 3501 modified UTF-7 reference/pattern arguments,
  applies exact, `*`, and `%` mailbox pattern matching over decoded names, and
  returns sanitized quoted mailbox names encoded back to modified UTF-7.
- IMAP mailbox-taking commands decode RFC 3501 modified UTF-7 mailbox
  arguments before crossing into service/storage boundaries, covering
  `SELECT`, `EXAMINE`, `STATUS`, `APPEND`, `COPY`, `MOVE`, `CREATE`, `DELETE`,
  `RENAME`, `SUBSCRIBE`, `UNSUBSCRIBE`, `LIST`, and `LSUB` while rejecting raw
  8-bit or malformed modified UTF-7 forms.
- IMAP quoted-string response formatting preserves ordinary internal spacing
  while escaping quotes/backslashes and cleaning controls, preventing mailbox
  names, FETCH metadata, and MIME parameters from being rewritten on output.
- IMAP mailbox management and subscription commands now reject malformed
  `LIST`, `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and
  `UNSUBSCRIBE` syntax before authentication failures, preserving precise
  tagged `BAD` diagnostics while keeping valid unauthenticated commands behind
  `NO authentication required`.
- IMAP selected-mailbox discovery commands now reject malformed `NAMESPACE`,
  `SELECT`, `EXAMINE`, and `STATUS` syntax before authentication failures,
  preserving precise tagged `BAD` diagnostics for invalid CONDSTORE options,
  status item lists, and modified UTF-7 mailbox names.
- IMAP `CAPABILITY` now advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`;
  `LIST` includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy
  attributes plus RFC 6154 special-use attributes for system folders such as
  Drafts, Sent, Trash, Junk, Archive, All, and Flagged when those folder roles
  are present in storage metadata, and extended
  `LIST (SPECIAL-USE)`, `RETURN (SPECIAL-USE)`, and no-op
  `RETURN (CHILDREN)` forms are accepted.
- IMAP `CAPABILITY` now advertises RFC 5819 `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits requested `STATUS` metadata
  immediately after each matching selectable mailbox so compatible clients can
  avoid per-folder `STATUS` round trips, and it can be combined with
  `RETURN (CHILDREN)`.
- IMAP `CAPABILITY` now advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
  `LIST-STATUS` can return per-mailbox total active message octets without
  requiring clients to fetch and sum each message's `RFC822.SIZE`.
- IMAP `CAPABILITY` now advertises RFC 5256 `SORT`; `SORT` and `UID SORT`
  apply selected-mailbox search criteria with mandatory `US-ASCII`/`UTF-8`
  charset handling and return sequence-number or UID sort responses for the
  standard arrival, sent-date, address, subject, and size sort keys.
- Service-backed IMAP message summaries now hydrate stored `To`, `Cc`, and
  `Bcc` address JSON into RFC-shaped ENVELOPE address lists, so real
  repository-backed `FETCH ENVELOPE`, address search, and address sort paths
  use the same recipient metadata that inbound, APPEND, COPY, and MOVE storage
  preserve.
- IMAP `CAPABILITY` now advertises RFC 5256 `THREAD=ORDEREDSUBJECT`; `THREAD
  ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` apply selected-mailbox search
  criteria and return ordered-subject thread trees, while the `REFERENCES`
  algorithm remains unadvertised until its Message-ID normalization and
  ancestry-linking rules are implemented.
- IMAP RFC 5256 base-subject extraction now decodes RFC 2047 encoded-word
  subjects before stripping reply/forward artifacts, so internationalized
  subject sorting and ordered-subject threading match standard-client
  expectations more closely.
- IMAP `SELECT`/`EXAMINE` now emit `[PERMANENTFLAGS]` response codes for
  writable versus read-only selected-mailbox state.
- IMAP `SELECT`/`EXAMINE` now emit RFC-shaped untagged `RECENT` counts
  alongside `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints,
  `UIDVALIDITY`, `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from
  durable mailbox UID state.
- IMAP `SELECT`/`EXAMINE` now emit `[UIDNOTSTICKY]` when selected mailbox
  state reports non-sticky UIDs, preserving UIDPLUS-adjacent client visibility
  into UID persistence guarantees.
- IMAP `UID STORE` now supports `.SILENT` mutation modes while applying the same
  flag changes through the service-backed flag boundary.
- IMAP `FETCH`/`UID FETCH` now include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes when requested, enabling standard mailbox list metadata reads.
- IMAP shared fetch failure paths now preserve the issued command name in
  tagged `NO` responses, so regular `FETCH` failures do not report
  `UID FETCH failed` while UID fetch failures retain UID-specific wording.
- IMAP `FETCH`/`UID FETCH` now follows RFC 3501 `\Seen` side-effect semantics:
  successful `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads mark the
  message seen through the service-backed flag boundary, while `BODY.PEEK[...]`
  and `RFC822.HEADER` do not mutate read state.
- IMAP `FETCH`/`UID FETCH` now preserves RFC 3501 `RFC822`,
  `RFC822.HEADER`, and `RFC822.TEXT` response data item names on the wire
  instead of exposing their internal `BODY[...]` equivalents.
- IMAP `CAPABILITY` now advertises `CONDSTORE` and `ENABLE` after the RFC
  4551-shaped mod-sequence fetch/search/status/select/store paths were wired
  through durable mailbox/message state; RFC 5161-shaped `ENABLE CONDSTORE`
  marks sessions CONDSTORE-aware before mailbox selection.
- IMAP `FETCH`/`UID FETCH` now include RFC 4551-shaped `MODSEQ (n)` attributes
  when requested, surfacing durable per-message mod-sequences.
- IMAP `SEARCH`/`UID SEARCH` now support RFC 4551-shaped `MODSEQ` criteria and
  append the highest matched mod-sequence to non-empty SEARCH responses.
- IMAP `CAPABILITY` now advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)`
  and `UID SEARCH RETURN (...)` return single untagged `ESEARCH` responses with
  requested `MIN`, `MAX`, compact `ALL`, `COUNT`, UID indicators, and
  CONDSTORE `MODSEQ` data.
- IMAP `CAPABILITY` now advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN (SAVE)`
  stores the selected-session search result so `$` can be reused by subsequent
  sequence-set and UID-set commands without a client round trip.
- IMAP `SEARCH RETURN (SAVE)` clears the selected-session `$` result when the
  save-requested search fails with tagged `NO`, while tagged `BAD` searches
  keep the previous result untouched per RFC 5182.
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
- IMAP `FETCH`/`UID FETCH` now supports standard `FAST`, `ALL`, and `FULL`
  macros, including the non-extensible `BODY` attribute for `FULL`.
- IMAP `FETCH`/`UID FETCH` now support bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- IMAP non-UID `FETCH` now uses the same bounded header literal path as
  `UID FETCH` for `BODY[HEADER]` and `RFC822.HEADER`.
- IMAP `FETCH`/`UID FETCH` now support bounded `BODY[TEXT]`,
  `BODY.PEEK[TEXT]`, and `RFC822.TEXT` section literals without returning the
  message headers, rejecting oversized section bodies before unbounded
  allocation.
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
- IMAP `FETCH`/`UID FETCH` now preserves requested `HEADER.FIELDS` and
  `HEADER.FIELDS.NOT` section names in literal response items, including
  partial-window suffixes, instead of collapsing subset reads to
  `BODY[HEADER]`.
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
- IMAP `SEARCH`/`UID SEARCH` now preserves RFC 3501 zero-length search string
  semantics for quoted empty strings across envelope, body/text, and header
  substring criteria instead of treating them as guaranteed no-match requests.
- IMAP `SEARCH`/`UID SEARCH` now supports RFC 3501 `LARGER` and `SMALLER`
  criteria over message `RFC822.SIZE` metadata.
- IMAP advertises `LITERAL+` and accepts bounded non-synchronizing command
  literals such as `APPEND ... {n+}` without an extra continuation round trip,
  while preserving synchronizing literal framing for conservative clients.
- IMAP command framing now supports bounded literals in non-final command
  positions and multiple literals in one command, keeping literalized
  credentials and string arguments compatible with RFC-shaped clients.
- IMAP server coverage verifies literalized `LOGIN` commands with separate
  synchronizing user-name and password literals, including the reconstructed
  credentials delivered to backend authentication.
- IMAP command and IDLE line reads enforce the command-line byte cap while
  reading from the socket, keeping malformed clients from accumulating
  oversized lines in memory before syntax rejection.
- IMAP oversized command literals now return a tagged `BAD` response when the
  command tag can be recovered, then emit `BYE` and close the session cleanly
  instead of leaking an internal server error through the connection boundary.
- IMAP accepts empty flag-lists where RFC-shaped clients can send them:
  `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears supported
  flags, and empty `+FLAGS ()`/`-FLAGS ()` are successful no-ops.
- IMAP selected-mailbox `APPEND` prefers the backend-returned appended message
  sequence number for the untagged `EXISTS` count, preserving precise selected
  mailbox counts when repository metadata is available.
- IMAP selected-mailbox `COPY` and same-mailbox `MOVE` likewise prefer
  backend-returned destination message sequence numbers for untagged `EXISTS`
  counts, preserving precise selected mailbox counts across mutation commands.
- IMAP selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE`
  adjust saved SEARCHRES `$` sequence numbers the same way explicit `EXPUNGE`
  commands do, keeping subsequent `$` reuse aligned with visible mailbox state.
- IMAP `EXAMINE` setup failures return `NO EXAMINE failed` instead of
  `NO SELECT failed`, keeping tagged failure responses aligned with the
  selected-mailbox command clients actually issued.
- IMAP malformed recognized `UID` subcommands are routed to their
  command-specific validators, so incomplete or structurally invalid
  `UID SEARCH`, `UID SORT`, `UID THREAD`, `UID FETCH`, `UID STORE`,
  `UID EXPUNGE`, and `UID COPY` receive precise tagged `BAD` responses before
  authentication/selected-state checks instead of a generic UID-dispatch
  failure.
- IMAP missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`, `DELETE`,
  and `RENAME` now return tagged `[NONEXISTENT]` response codes instead of
  generic command failures, preserving machine-readable absent-folder state for
  clients.
- IMAP selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`, and
  `EXPUNGE` now reject extra arguments with tagged `BAD` responses instead of
  ignoring malformed input, preventing ambiguous destructive expunge handling.
- IMAP any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT` now
  reject extra arguments with tagged `BAD` responses instead of silently
  accepting malformed commands or ending sessions for malformed logout attempts.
- IMAP `STATUS` now requires a parenthesized status item list, rejecting
  malformed `STATUS mailbox MESSAGES`-style requests before mailbox metadata
  lookup.
- IMAP `LIST ... RETURN (STATUS (...))` now also requires a parenthesized
  status item list, rejecting malformed `RETURN (STATUS MESSAGES)` before
  mailbox listing work.
- IMAP command dispatch rejects malformed tags containing atom-special
  characters with untagged `BAD` responses before command handling, avoiding
  ambiguous tagged replies for invalid client command tags.
- IMAP command parsing rejects control characters inside unquoted atoms,
  matching the existing quoted-string control-character guardrail before
  command dispatch. Parser failures now return tagged `BAD` when a
  syntactically valid command tag can still be recovered, keeping client
  command tracking deterministic.
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
- IMAP subscription canonicalization preserves hierarchy delimiters, quoting,
  and internal spacing while keeping case-insensitive matching, preventing
  distinct subscribed mailbox names from silently overwriting each other in
  `LSUB` state.
- IMAP `SUBSCRIBE` can now retain missing mailbox names so `LSUB` can report
  them with `\Noselect`, keeping subscription state useful across mailbox
  migration, deletion, and delayed creation flows.
- IMAP `LIST "" ""` and `LSUB "" ""` now return the hierarchy root with
  `\Noselect` and `/` delimiter metadata for clients that probe namespace
  delimiters through LIST-compatible commands.
- IMAP `LSUB` retains subscribed names after mailbox deletion with `\Noselect`
  and covers the RFC 3501 `%` hierarchy parent response case.
- IMAP now advertises and supports RFC 2971 `ID`, validating `NIL` or bounded
  field/value parameter lists before returning a bounded server identity
  response for compatibility diagnostics.
- IMAP RFC 2971 `ID` parameter-list parsing now rejects quote and backslash
  atom-special characters inside unquoted ID tokens, preserving valid escaped
  quoted-special strings while closing malformed raw-token cases.
- IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the common IMAP
  atom validator, keeping malformed literal-marker, response-special,
  wildcard-special, quoted-special, and control-character cases out of ID
  diagnostics parsing.
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
  the source UID. Responses return UIDPLUS `[COPYUID ...]` mappings in the
  final tagged OK when destination UIDs are available, advance and return
  source mailbox `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients,
  emit RFC-shaped source `EXPUNGE` responses,
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
  APPEND internaldate parsing accepts RFC 3501 space-padded one-digit date-days
  such as `" 5-May-2026 ..."` while rejecting bare one-digit date-days such as
  `"5-May-2026 ..."`. The service boundary rejects CR/LF-bearing or oversized
  APPEND user and mailbox identifiers before repository lookup, spooling,
  parsing, storage, or quota work.
- IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  mutation dispatch or mailbox event publication.
- IMAP service-backed read/list/subscription/backfill operations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  reads, storage opens, event subscriptions, or UID backfill work.
- IMAP service-backed `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
  reject zero UIDs before repository or storage work, keeping direct callers
  aligned with RFC 3501's positive UID model.
- IMAP service-backed `STORE`, `COPY`, and `MOVE` calls reject empty UID sets
  before repository work, while `EXPUNGE` preserves nil UID sets for `CLOSE`
  style "all deleted messages" semantics.
- IMAP `CREATE`, `DELETE`, and `RENAME` now delegate to the service folder
  boundary for authenticated flat user-mailbox management, resolving wire names
  before destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- IMAP `CREATE INBOX` and `DELETE INBOX` now return explicit RFC 3501-shaped
  `NO` failures, and `RENAME INBOX` is rejected instead of being treated like a
  generic folder rename until its required special message-moving semantics are
  implemented.
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
  `FETCH`/`UID FETCH`, `STORE`/`UID STORE`, `SEARCH`, `SORT`, `IDLE`,
  `STARTTLS`, `CREATE`/`DELETE`/`RENAME`, `APPEND`, `COPY`, `MOVE`, `EXPUNGE`,
  `CLOSE`, `UNSELECT`, and `LOGOUT` over the service-backed mailbox/session
  boundary.
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
  storage, and repository work, and reject CR/LF-bearing or oversized user,
  draft, and upload-session identifiers before quota reservation, object
  writes, or repository work.
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
  external signer errors from leaking into export/billing diagnostics. Remote
  signer HTTP responses use the shared bounded drain-and-close helper so
  keep-alive connections can be reused without unbounded cleanup reads.
- Push-notification and attachment-scan webhooks now reject CR/LF-bearing
  configured tokens/endpoints and collapse non-2xx HTTP response bodies into
  bounded one-line UTF-8 previews before surfacing delivery failures. Shared
  webhook HTTP response cleanup now drains a small bounded body window before
  close so keep-alive connections can be reused without unbounded cleanup reads.
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
  repository dispatch. Folder list/create/rename/delete also reject
  CR/LF-bearing or oversized user identifiers before repository work.
  Folder-scoped message lists and thread-message reads also reject unsafe
  folder/thread identifiers before repository work.
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
- S3-compatible `ObjectInfo` metadata from `HEAD` and `ListObjectsV2` also
  stays bounded to safe single-line UTF-8 before it reaches logs, Drive,
  lifecycle, or reconciliation code.
- S3-compatible bucket names are validated with shared adapter/config guardrails
  before runtime wiring, surfacing uppercase, undersized, slash-bearing, or
  punctuation-adjacent deployment mistakes before storage calls.
- S3-compatible bucket validation also rejects IP-address-shaped names plus
  AWS-reserved prefixes and suffixes, aligning config-time failures with current
  AWS general purpose bucket naming restrictions.
- S3-compatible regions are validated with shared adapter/config guardrails
  before SigV4 signing, rejecting blank, whitespace-bearing, slash-bearing, or
  uppercase region values before object-storage requests are created.
- S3-compatible object prefixes are validated as canonical relative object-key
  prefixes during config validation, surfacing duplicate separators, dot
  segments, traversal, or backslash mistakes before adapter construction.
- S3-compatible endpoints are validated as plain HTTP(S) origins without
  userinfo, query strings, fragments, CR/LF-bearing target text, or
  non-canonical base paths, preventing ambiguous SigV4 signing and
  object-addressing configuration from reaching runtime storage probes.
- S3-compatible request construction automatically falls back to path-style
  addressing for dotted bucket names on HTTPS endpoints, avoiding AWS S3
  virtual-hosted TLS wildcard certificate mismatches while keeping
  virtual-hosted requests as the default for ordinary bucket names.
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
- IMAP read-only selected-state mutation handling now preserves
  command-specific tagged `BAD` responses for malformed `STORE`/`MOVE` and UID
  mutation commands before returning read-only `NO` responses for valid
  mutations, without invoking backend mutation paths for syntactically valid
  mutation attempts against `EXAMINE`-selected mailboxes.
- IMAP generic mailbox mutation commands now keep INBOX special semantics out
  of normal folder operations by rejecting create/delete/rename-from/rename-to
  INBOX attempts before backend folder mutation.
- IMAP command atom validation covers the command name and UID subcommand name
  dispatch boundary, preserving precise malformed-command behavior before
  backend or feature dispatch.
- IMAP UID dispatch validates syntax before authentication and selected-mailbox
  state so malformed or unknown UID subcommands produce `BAD` responses instead
  of being hidden behind state errors.
- IMAP bare `UID` commands now return `BAD UID requires subcommand`, keeping
  missing-subcommand diagnostics distinct from unknown but well-formed UID
  subcommands.
- IMAP selected-state command dispatch validates obvious malformed
  `FETCH`/`STORE`/`COPY`/`MOVE`/`SEARCH`/`SORT`/`THREAD` syntax before selected
  mailbox state errors, keeping parser diagnostics precise for client authors.
- IMAP selected-state action commands validate malformed `FETCH`, `STORE`,
  `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox names before
  authentication errors too, preserving precise tagged `BAD` diagnostics during
  client state-machine probing.
- IMAP search-oriented selected-state commands validate malformed `SEARCH`,
  `SORT`, and `THREAD` argument shape, return options, and sort/thread
  argument lists before authentication errors too, preserving precise tagged
  `BAD` diagnostics during capability and state-machine probing.
- IMAP selected-state no-argument commands reject extra arguments before
  authentication and selected-mailbox state errors, keeping destructive
  lifecycle commands from hiding malformed input behind state responses.
- IMAP `STARTTLS` rejects malformed extra-argument commands before availability
  and state checks, keeping TLS capability probing diagnostics precise.
- IMAP `UID` dispatch validates state-independent subcommand syntax before
  authentication and selected-mailbox errors, keeping arity and mailbox-name
  diagnostics visible to clients even before `LOGIN`, `SELECT`, or `EXAMINE`.
- IMAP `APPEND` validates missing literals, malformed append options, and
  modified UTF-7 mailbox names before authentication errors, while valid
  unauthenticated appends still consume the RFC literal and return
  `NO authentication required` before backend storage.
- IMAP literalized `LOGIN` is covered end-to-end with multiple synchronizing
  literals in one command; extend the same fixture shape as more string-taking
  commands graduate to the public compatibility surface.
- IMAP `ENABLE` validates missing capability arguments before authentication
  errors, while valid unauthenticated enable attempts still return
  `NO authentication required` without mutating session feature state.
- IMAP authentication commands validate malformed `LOGIN` arity and unsupported
  `AUTHENTICATE` mechanisms before plaintext privacy-required failures, keeping
  auth handshake diagnostics precise without weakening TLS-required policy.

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
- Verify webmail clients use `HEAD /api/v1/messages/{id}/attachments/{attachment_id}/download` for attachment metadata previews when they need object-backed size/header checks before streaming bytes.
- Verify Drive copy UX respects the advertised `max_copy_nodes` cap when calling `POST /api/v1/drive/nodes/{id}/copy` for files or folder trees.
- Verify Drive clients use the advertised node sort controls instead of assuming only name ordering.
- Verify Drive clients use the advertised node type filters instead of fetching all nodes and filtering locally.
- Verify admin Drive inventory screens use `all_parents=true` for whole-user search instead of crawling every folder client-side.
- Verify Drive cleanup-failure operations include node-less copied-object cleanup rows caused by failed copy metadata creation.
- Verify Drive cleanup-failure recording rejects object paths outside the owning user's `drive/users/{user_id}/...` prefix.
- Verify Drive clients treat HTTP 507 `insufficient_storage` from finalize/copy paths as quota pressure, distinct from validation failures.
- Verify Drive clients only pass storage paths returned by the authenticated user's staged/upload-session endpoints; finalize rejects object keys outside that user's `drive/users/{user_id}/...` prefix.
- Verify native CalDAV client discovery treats `/caldav/` as a service-root
  collection anchor and follows `current-user-principal` before requesting
  principal-only properties such as `calendar-home-set`.

## Intentionally out of scope for this release slice

- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam logic.
- IMAP/POP3.
- OpenSearch as the default/mandatory search backend, vendor push delivery
  adapters, Kafka, Vault, and etcd.
