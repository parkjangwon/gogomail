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

## Consequences

- Future webmail calendar APIs can share calendar storage while CalDAV handles
  standards-shaped client synchronization.
- CalDAV path parsing, XML parsing, iCalendar parsing, ETag handling, and sync
  token behavior can be tested independently from Mail API and Admin API code.
- Production CalDAV authentication and TLS policy must be reviewed before the
  listener is advertised as ready.
- The frontend calendar can be built later without bypassing the standards
  boundary needed by native clients.
