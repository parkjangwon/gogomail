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

## Consequences

- Future webmail calendar APIs can share calendar storage while CalDAV handles
  standards-shaped client synchronization.
- CalDAV path parsing, XML parsing, iCalendar parsing, ETag handling, and sync
  token behavior can be tested independently from Mail API and Admin API code.
- Production CalDAV authentication and TLS policy must be reviewed before the
  listener is advertised as ready.
- The frontend calendar can be built later without bypassing the standards
  boundary needed by native clients.
