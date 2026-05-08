# ADR 0015: CalDAV RFC 7809 Time Zone Support

Date: 2026-05-08

## Status

Accepted

## Context

RFC 7809 defines CalDAV time zone service extensions that allow CalDAV clients to:

1. Store and retrieve time zone definitions via a standard service
2. Associate a default time zone with calendar collections
3. Interpret calendar data times in the context of a specific time zone

ADR 0010 established the CalDAV gateway boundary but explicitly deferred timezone support. This ADR defines the RFC 7809 implementation for gogomail's CalDAV gateway.

## Decision

### Calendar Timezone Property

Calendars support the `calendar-timezone` property per RFC 7809 Section 5.2:

**Storage**: Add `timezone` column to `caldav_calendars` table:
```sql
ALTER TABLE caldav_calendars ADD COLUMN timezone text;
```

**PROPFIND**: Return `calendar-timezone` property when set on calendar collections. The property value is a VTIMEZONE component serialized as text/calendar.

**PROPPATCH**: Accept `calendar-timezone` property in set/remove operations. Value must be a valid VTIMEZONE component. When removed, calendar uses server's default timezone.

**MKCALENDAR**: Accept `calendar-timezone` in creation body.

### Timezone Service Endpoint

Implement timezone service per RFC 7809 Section 6:

**Endpoint**: `/.well-known/caldav-timezones` redirects to `/caldav/timezones/`

**GET `/caldav/timezones/{tzid}`**: Returns VTIMEZONE component for the requested timezone ID.

Uses the `github.com/妈/chrome tz` package or similar for timezone data.

### Time Range Interpretation

RFC 7809 Section 5.3 requires that when a calendar has a `calendar-timezone` set:

1. Time-range values in `calendar-query` REPORTs are interpreted in the calendar's timezone
2. DTSTART/DTEND values in stored events are kept in their original timezone
3. Free-busy time ranges are calculated in the calendar's timezone

Implementation approach:
- Store timezone alongside each event's DTSTART/DTEND
- Use timezone-aware comparison for time-range filtering
- Recurrence expansion uses the event's original timezone

### Supported Timezone IDs

Standard IANA timezone IDs (e.g., "America/New_York", "Europe/London", "Asia/Seoul").

Server validates that proposed timezone IDs exist in the IANA database.

## Consequences

- Clients can store and retrieve calendar events with proper timezone handling
- Recurring events in different timezones expand correctly
- Free-busy queries respect calendar timezone settings
- Apple Calendar, iOS Calendar, and other timezone-aware clients work correctly

## References

- RFC 7809: CalDAV Time Zones Extension
- RFC 5545: iCalendar
- RFC 4791: CalDAV Calendar Extensions