# ADR 0010: CalDAV gateway boundary

Date: 2026-05-06

## Status

Accepted

## Context

gogomail is becoming a production webmail platform, and users will expect
calendar management in both the webmail UI and native clients. Apple Calendar,
iOS, Android calendar apps, Windows clients, macOS clients, and many enterprise
tools integrate through CalDAV/WebDAV and iCalendar rather than gogomail-native
HTTP APIs.

CalDAV compatibility is protocol-sensitive in the same way IMAP compatibility
is protocol-sensitive. A partial or ad-hoc calendar HTTP API would not be enough
for client compatibility because CalDAV clients rely on WebDAV discovery,
principal URLs, calendar-home-set properties, ETags, sync tokens, REPORT bodies,
and standards-shaped iCalendar objects.

## Decision

CalDAV is modeled as a separate gateway boundary over gogomail-native calendar
storage, not as a frontend-only feature and not as generic Mail API routes.

The first package, `internal/caldavgw`, defines standards, resource path
classification, DTOs, and storage interfaces for future protocol handlers. The
initial runtime mode is `gogomail --mode=caldav`, intentionally scaffolded until
the storage tables, authentication semantics, XML parser limits, iCalendar
validation, and WebDAV method handlers are implemented deliberately.

The gateway must be designed around these standards:

- RFC 4918: WebDAV
- RFC 4791: CalDAV
- RFC 5545: iCalendar
- RFC 6638: CalDAV scheduling
- RFC 6578: WebDAV sync collections
- RFC 6764: CalDAV service discovery
- RFC 7809: CalDAV time zones

The CalDAV gateway owns protocol-specific concepts such as principal URLs,
calendar-home-set, calendar collections, `.ics` object paths, ETags, sync
tokens, supported-calendar-component-set, REPORT handling, and WebDAV XML
response envelopes.

The first persisted storage boundary uses dedicated tables:

- `caldav_calendars` for user-owned calendar collections, display metadata,
  lifecycle state, and sync tokens;
- `caldav_calendar_objects` for individual `.ics` resources, iCalendar UID,
  top-level component type, strong ETag, object size, and bounded iCalendar
  payloads.

ETags are strong quoted SHA-256 values over the stored iCalendar bytes. Sync
tokens are stored explicitly on calendars so WebDAV sync behavior can evolve
without deriving client state from timestamps or list pagination.

The first WebDAV XML boundary is intentionally bounded and namespace-aware. It
parses PROPFIND request modes, `allprop` `include` properties, safe `Depth`
header values, and the core CalDAV/WebDAV REPORT roots needed for
`calendar-query`, `calendar-multiget`, `free-busy-query`, and
`sync-collection`. Method handlers will build on this parser rather than
decoding arbitrary XML in request paths.

The first repository boundary keeps calendar and object mutations
user-scoped. Calendar-object upserts and soft deletes lock the owning active
calendar and update its sync token transactionally so future REPORT
`sync-collection` handlers can observe object changes through one collection
state value.

iCalendar parsing is delegated to `github.com/emersion/go-ical` rather than an
ad-hoc line parser. The gateway still applies gogomail-specific storage
constraints around body size, supported top-level components, UID cardinality,
and component/property counts before accepting a calendar object.

WebDAV XML responses are generated through a dedicated `multistatus` builder
that keeps per-property statuses explicit. This avoids scattering raw XML
construction across future `PROPFIND` and `REPORT` handlers and preserves the
mixed-success property semantics WebDAV clients expect.

The first discovery handler boundary supports `OPTIONS` and `PROPFIND` over a
pluggable store, but the public `caldav` runtime remains gated. This keeps
client-visible protocol activation separate from the internal handler work
until authentication, TLS, repository adapter wiring, and compatibility tests
are reviewed together.

The PostgreSQL repository is adapted to the discovery store interface inside
`internal/caldavgw`, so runtime wiring can use the same tested handler boundary
without coupling WebDAV XML generation to raw SQL call sites.

CalDAV authentication is expected to use HTTP Basic authentication over TLS for
native client compatibility. The first resolver reuses the existing
authenticated Submission password verifier so CalDAV, IMAP, SMTP Submission,
and future account APIs can converge on one local-password source instead of
inventing a separate calendar credential path.

CalDAV runtime configuration is separate from the generic Mail/Admin HTTP
listener. `GOGOMAIL_CALDAV_ADDR` gives operators a dedicated protocol endpoint,
and production rejects `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH=true` so Basic
credentials are not accidentally accepted over cleartext transport.

The first runtime wiring starts a discovery-only CalDAV HTTP listener for
`OPTIONS` and `PROPFIND`. It intentionally does not claim full client-ready
compatibility until REPORT handling, object GET/PUT/DELETE, scheduling
semantics, and compatibility tests are completed.

REPORT request parsing is kept in the gateway boundary and validates
handler-critical shape early: required query filters, multiget hrefs,
free-busy UTC time ranges, sync-collection level, and bounded sync limits.
Handlers can therefore focus on storage reads and WebDAV response semantics.

The first REPORT handler is `calendar-multiget`, because common CalDAV clients
use it after discovery to hydrate event resources by href. Missing hrefs remain
inside the multistatus response as 404 propstats instead of failing the whole
REPORT request.

`calendar-data` is part of the protocol response contract, not a generic raw
body toggle. REPORT parsing preserves nested RFC 4791 `calendar-data`
projection requests for `VCALENDAR` and child component properties, and REPORT
responses apply that projection through the RFC 5545 iCalendar parser/encoder.
Projection may retain required structural properties such as `VERSION`,
`PRODID`, `UID`, or `DTSTAMP` when present so returned objects remain valid
iCalendar payloads for native clients.

Calendar object reads and mutations are handled directly by the CalDAV gateway
rather than Mail API routes. `PUT` reuses the iCalendar validation and strong
ETag repository boundary, while `If-Match` and `If-None-Match` checks keep
native client cache/write behavior coherent.

`REPORT calendar-query` is handled in the gateway over the same repository
boundary. The first implementation lists objects from an authenticated calendar
collection, returns requested WebDAV/iCalendar properties, and applies VEVENT
time-range overlap checks through the RFC 5545 parser when a CalDAV
`time-range` filter is supplied. Calendar-query object listing must stay
bounded: handlers request at most the requested/default result limit plus one
extra object so exact-limit responses can complete while genuinely truncating
responses fail closed until continuation semantics exist. Recurrence expansion
and non-VEVENT time-range semantics remain future compatibility work before the
listener is advertised as broadly client-ready.
Unsupported CalDAV filter elements must fail closed with a
`CALDAV:supported-filter` precondition rather than being silently ignored,
because broadening query results under an unimplemented predicate is worse for
client compatibility than an explicit standards-shaped failure.

`REPORT sync-collection` starts with conservative RFC 6578 behavior. Empty
sync-token requests return all active objects and a top-level multistatus
`sync-token`; requests with the current token return only the token;
stale-but-known tokens return bounded deltas/tombstones; unknown or expired
tokens return a DAV `valid-sync-token` precondition error rather than silently
omitting deletes. Truncating limits are rejected until continuation semantics
can be implemented without lying to clients about synchronization completeness.

`REPORT free-busy-query` is a non-multistatus REPORT and returns a
`text/calendar` `VFREEBUSY` response. The gateway owns that shape so scheduling
and native clients do not need product-specific calendar APIs. The first
implementation derives busy periods from child VEVENT resources at `Depth: 1`,
clips periods to the requested UTC range, skips `TRANSPARENT` and `CANCELLED`
events, maps `TENTATIVE` to `BUSY-TENTATIVE`, ingests stored VFREEBUSY source
objects, and coalesces same-type overlaps. Child object reads are bounded with
the same requested/default `limit/nresults` plus one-extra-row truncation probe
used by other CalDAV collection scans. Recurrence expansion remains future
compatibility work.

`MKCALENDAR` creation is handled by the gateway instead of a product calendar
API. To preserve Request-URI semantics with the current UUID-backed storage
schema, the first implementation accepts calendar collection paths whose
calendar segment is a UUID and inserts the collection with that id. The
creation body parser is bounded and namespace-aware for `displayname`,
`calendar-description`, and CalendarServer/Apple `calendar-color`. Friendlier
human-readable slug aliases need a separate storage/path design before they can
be advertised safely to clients.

Calendar collection `DELETE` is also owned by the gateway and maps to a
repository transaction that soft-deletes the collection and active child
objects together. This keeps WebDAV lifecycle behavior out of product APIs and
feeds the durable sync-change-log boundary so stale-token clients can receive
object tombstones and a final collection-deleted sync token. Long-history
retention and continuation semantics remain compatibility gates before CalDAV
is advertised as public/client-ready.

The sync-change-log boundary is durable PostgreSQL state, not an in-memory
gateway cache. Calendar creation and object mutation transactions append sync
markers keyed by collection token; `sync-collection` can then turn
stale-but-known tokens into changed object responses or response-level 404
tombstones, and collection-deleted markers can advance clients to the final
deleted-collection sync token even after the live calendar row is gone. Unknown
tokens still fail with `valid-sync-token`, which is safer than fabricating an
incomplete delta after retention gaps or unsupported history.

Retention pruning is also repository-owned. A bounded
`PruneCalendarSyncChanges` boundary can dry-run or delete old change-log rows
while preserving the newest marker per calendar, backed by a prune-order
database index. The `dav-sync-retention-worker` runs that path with CardDAV
retention on an interval or once-and-exit, dry-run by default and guarded by
explicit confirmation before destructive runs. This keeps retention policy
outside the HTTP handler and prevents cleanup work from deleting the token
needed by a current client. Public readiness still needs deployment-specific
retention age policy and native-client compatibility testing around expired
tokens.

Service discovery starts at the gateway as well: `/.well-known/caldav` redirects
to `/caldav/`, and root `PROPFIND` exposes the authenticated principal and
calendar-home links. This keeps RFC 6764 entry-point compatibility decoupled
from product-specific webmail APIs.

Calendar collection `PROPPATCH` is gateway-owned WebDAV behavior, not a generic
product update API. The first supported properties are `DAV:displayname`,
`CALDAV:calendar-description`, and CalendarServer/Apple `calendar-color`.
Propertyupdate parsing stays bounded and namespace-aware, removal is allowed
only for optional description/color properties, and each accepted update refreshes
the collection sync token plus a durable `collection-updated` marker. Calendar
object property patching, scheduling preference changes, and product policy
decisions remain outside this gateway method until their backend boundaries are
defined.

Calendar collection `supported-report-set` also stays tied to implemented
gateway semantics. The gateway advertises only `calendar-query`,
`calendar-multiget`, `free-busy-query`, and `sync-collection` today. Scheduling,
timezone, delegation, or other future reports must not be exposed through
discovery until their RFC behavior, storage model, and policy boundaries are
implemented.
Calendar collection `PROPFIND Depth: 1` child discovery follows the same
bounded collection-scan rule as REPORT handlers: the gateway asks the storage
adapter for the default limit plus one object and fails explicitly if the result
would be partial. Until continuation or paging semantics are designed, returning
an incomplete WebDAV multistatus would mislead native-client caches.
Likewise, HTTP `Allow` headers must stay bound to implemented gateway methods.
Future WebDAV method constants such as `MOVE` may exist as roadmap markers, but
they must not appear in `OPTIONS` or 405 capability surfaces until the gateway
implements their preconditions, storage moves, ETag behavior, sync effects, and
authorization semantics.

The CalDAV service root is not the authenticated user's principal resource.
`PROPFIND /caldav/` may expose root collection metadata plus the WebDAV
`current-user-principal` and `principal-collection-set` discovery links, but
principal-only CalDAV properties such as `calendar-home-set` must stay on the
principal resource. This keeps RFC 6764-style discovery useful without teaching
clients that the service root owns calendars or delegated identity semantics.

CalDAV must also depend on platform-level principal boundaries instead of
inventing a private calendar identity model. Directory/Identity should own
users, organization hierarchy, teams/groups, aliases, mailing lists, resources,
memberships, delegated relationships, and principal resolution. Contacts/CardDAV
should own personal contacts, external people, personal address books, and
user-specific metadata. Notification & Sync should own domain events, reminder
decisions, device registry, delivery adapters, quiet-hours/per-device policy,
and delta fan-out. Search and Policy/Audit should own unified event/person/
resource lookup, retention, admin controls, and traceable access decisions.
CalDAV therefore emits transactional `dav.event` outbox rows from the same
repository boundary that appends durable calendar sync-change rows, but it does
not decide reminder delivery, mobile push, or indexing behavior inside the
protocol gateway. A generic event-worker instance can consume `dav.event` for
payload validation and audit recording; later Notification & Sync consumers
should attach behind the same stream boundary.
Until these boundaries exist, shared calendars, delegated access, resource
booking, attendee auto-complete, and reminder delivery remain release gates
rather than isolated CalDAV CRUD features.

The first delegated-access integration keeps that boundary explicit. CalDAV
handlers distinguish authenticated actor user IDs from resource owner user IDs
and call a pluggable access authorizer for cross-user calendar paths before
using owner-scoped storage. Runtime `caldav` mode wires the authorizer through
Directory active principal resolution, `accesspolicy.DelegatedAccessAuthorizer`,
and the shared audit repository. This is not a product-specific sharing table
inside CalDAV; it is the protocol gateway consuming the platform delegation and
audit model. Delegated `PROPFIND`, REPORT, and sync privilege discovery also
consume that access policy boundary, deriving
`DAV:current-user-privilege-set` from the mapped read/write/manage decision
instead of advertising owner-level static privileges. Delegated `PROPFIND`
keeps `DAV:current-user-principal` anchored to the authenticated actor while
resource hrefs, `DAV:owner`, and repository access remain owner-scoped. Missing
Directory principals, or resolved non-user owner/actor principals, fail closed
as authorization denial before audit or delegated role checks run, while
infrastructure and audit-path failures remain explicit server errors. Public
shared-calendar behavior still requires write/manage UX semantics,
scheduling/resource policy, and compatibility tests.

## Consequences

- Future webmail calendar APIs can share calendar storage while CalDAV handles
  standards-shaped client synchronization.
- CalDAV path parsing, XML parsing, iCalendar parsing, ETag handling, and sync
  token behavior can be tested independently from Mail API and Admin API code.
- Production CalDAV authentication and TLS policy must be reviewed before the
  listener is advertised as ready.
- The frontend calendar can be built later without bypassing the standards
  boundary needed by native clients.
- Public CalDAV compatibility is experimental until recurrence, scheduling,
  production token-retention policy, native-client compatibility testing, and
  the shared Directory/Contacts/Notification/Search/Policy boundaries are in
  place.
- Admin API now has a read-only DAV sync retention-readiness preview that calls
  the CalDAV and CardDAV retention repositories with dry-run semantics and
  returns bounded candidate counts plus truncation status. This is an operator
  safety gate; destructive Admin retention runs now require explicit
  confirmation, reuse that preview, and fail closed when the probe is
  truncated. Token-retention age policy and native-client expired-token
  behavior still define the release gate.
