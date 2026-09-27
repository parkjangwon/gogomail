# CalDAV Client Interoperability

This document describes how GoGoMail's CalDAV gateway (`internal/caldavgw`)
behaves with real-world calendar clients, the discovery URLs those clients
should use, and the known limitations per client.

GoGoMail implements CalDAV per RFC 4791 (CalDAV), RFC 6638 (scheduling), RFC
7809 (time-zone extensions), RFC 5545 (iCalendar), plus the WebDAV Sync
extension (RFC 6578) and the widely deployed CalendarServer `getctag` /
`Brief` extensions that Apple clients depend on.

The compatibility behavior documented here is exercised by:

- `internal/caldavgw/client_fixtures_test.go` — status-code coverage of each
  client's request sequence.
- `internal/caldavgw/client_matrix_test.go` — body-asserting client-quirk
  matrix (properties, namespaces, ETag quoting, preconditions).

## Discovery URLs

Point the client at the server's base HTTPS URL. Clients that support
`.well-known` bootstrapping (Apple, DAVx5, GNOME) only need the host; older or
stricter clients (some Outlook plugins) may need the explicit principal or
calendar-home URL.

| Purpose | URL | Notes |
|---|---|---|
| Well-known bootstrap | `https://HOST/.well-known/caldav` | `301` redirect to `/caldav/` (query string preserved). |
| Service root | `https://HOST/caldav/` | `PROPFIND` here returns `current-user-principal`. |
| Principal | `https://HOST/caldav/principals/{user}/` | `calendar-home-set`, `calendar-user-address-set`, schedule inbox/outbox URLs. |
| Calendar home | `https://HOST/caldav/calendars/{user}/` | `Depth: 1` `PROPFIND` enumerates the user's calendars. |
| A calendar collection | `https://HOST/caldav/calendars/{user}/{calendar}/` | Calendar object collection; supports REPORT/PUT/DELETE/PROPPATCH. |
| A calendar object | `https://HOST/caldav/calendars/{user}/{calendar}/{name}.ics` | Individual event/todo resource. |
| Time-zone service | `https://HOST/.well-known/caldav-timezones` | `301` to `/caldav/timezones/` (RFC 7809). |

Typical client bootstrap chain:

1. `GET /.well-known/caldav` → `301` → `/caldav/`
2. `PROPFIND /caldav/` (`Depth: 0`) → `current-user-principal`
3. `PROPFIND /caldav/principals/{user}/` (`Depth: 0`) → `calendar-home-set`
4. `PROPFIND /caldav/calendars/{user}/` (`Depth: 1`) → per-calendar
   `resourcetype`, `displayname`, `supported-calendar-component-set`,
   `getctag`, `getetag`, color, etc.
5. Per calendar: `REPORT` `sync-collection` (initial empty token) or
   `calendar-query`/`calendar-multiget`.

## Protocol capabilities

- **Methods:** `OPTIONS`, `PROPFIND`, `PROPPATCH`, `REPORT`, `MKCALENDAR`,
  `GET`, `HEAD`, `PUT`, `DELETE`, `POST` (scheduling).
- **`DAV` header:** `1, 3, calendar-access` (plus `sync-collection` when the
  backing store supports change tracking, and `calendar-schedule` when
  scheduling is enabled).
- **REPORTs:** `calendar-query`, `calendar-multiget`, `free-busy-query`, and
  `sync-collection` (WebDAV Sync).
- **Conditional requests:** `If-Match`, `If-None-Match` (incl. `*`), `If`
  (WebDAV state tokens / entity-tags), `If-Unmodified-Since`,
  `If-Modified-Since`. ETags are always double-quoted per RFC 7232;
  **unquoted** `If-Match`/`If-None-Match` values are rejected with `400`.
- **Max object size:** 10 MiB per calendar object (`max-resource-size`).

### CalendarServer / Apple extensions honored

- **`CS:getctag`** (`http://calendarserver.org/ns/` `getctag`) — served on
  every calendar collection. Its value equals the collection's `getetag` and
  changes whenever any object in the collection changes, so clients can poll it
  cheaply before doing a full sync. Without this, Apple Calendar falls back to
  full re-fetches (and some versions disable the calendar).
- **`Brief: t`** and **`Prefer: return=minimal`** (RFC 8144) — when either is
  present on a `PROPFIND`, the server omits the `404 Not Found` propstat blocks
  for unsupported properties and echoes `Preference-Applied: return=minimal`
  (and `Brief: t`). Apple Calendar/iOS send `Brief: t` on nearly every request.
- **`CS:calendar-color`** — accepted and stored. Apple sends the color as
  **`#RRGGBBAA`** (8 hex digits, trailing alpha channel); other clients send
  **`#RRGGBB`** (6 digits). Both forms are accepted and normalized to
  upper-case; the alpha channel is preserved on round-trip.
- **`I:calendar-slug`** (`http://apple.com/ns/icalendar/`) — GoGoMail's stable
  human-readable calendar alias.

## Per-client support matrix

Legend: ✅ works · ⚠️ works with a caveat · ❌ not supported.

| Capability | Apple (macOS/iOS) | DAVx5 (Android) | Thunderbird | GNOME (Evolution-DS) | Outlook CalDAV plugin |
|---|---|---|---|---|---|
| `.well-known/caldav` discovery | ✅ | ✅ | ✅ | ✅ | ⚠️ some plugins need explicit URL |
| `current-user-principal` bootstrap | ✅ | ✅ | ✅ | ✅ | ✅ |
| `calendar-home-set` discovery | ✅ | ✅ | ✅ | ✅ | ✅ |
| `Depth: 1` calendar enumeration | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CS:getctag` polling | ✅ | ✅ | n/a (uses ctag/etag) | ✅ | ⚠️ plugin-dependent |
| `Brief: t` / `return=minimal` | ✅ honored | ✅ honored | ✅ (full report if not sent) | ✅ | ✅ |
| `supported-calendar-component-set` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `supported-report-set` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `calendar-query` (+ `time-range`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `calendar-multiget` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `sync-collection` (WebDAV Sync) | ✅ | ✅ | ✅ | ✅ | ⚠️ plugin-dependent |
| `free-busy-query` | ✅ | ✅ | ✅ | ✅ | ✅ |
| PUT create (`If-None-Match: *`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| PUT update (`If-Match: <etag>`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| DELETE (`If-Match: <etag>`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| PROPPATCH `calendar-color` | ✅ (`#RRGGBBAA`) | ✅ (`#RRGGBB`) | ✅ (`#RRGGBB`) | ✅ | ✅ |
| PROPPATCH `displayname` | ✅ | ✅ | ✅ | ✅ | ✅ |
| Time-zone service (RFC 7809) | ✅ | ✅ | ⚠️ uses inline VTIMEZONE | ✅ | ⚠️ |
| Scheduling (RFC 6638, POST inbox/outbox) | ⚠️ when enabled | ⚠️ when enabled | ⚠️ when enabled | ⚠️ when enabled | ⚠️ when enabled |

### Client notes

**Apple Calendar (macOS/iOS)**
- Sends `Brief: t` on virtually every `PROPFIND`; 404 propstats are omitted in
  responses and `Preference-Applied: return=minimal` is returned.
- Sends `calendar-color` as `#RRGGBBAA`.
- Polls `CS:getctag` before syncing; relies on it to avoid full re-fetches.
- Creates objects with `If-None-Match: *`; a duplicate create returns `412`.

**DAVx5 (Android)**
- Uses `OPTIONS` to confirm `DAV: 1, 3, calendar-access` and then the standard
  discovery chain.
- Treats `getctag` and the collection `getetag` as interchangeable change
  tokens; GoGoMail keeps them consistent.
- Batches many hrefs in `calendar-multiget`; missing hrefs come back as
  per-href `404` `D:response` entries rather than a whole-request error.
- Updates with `If-Match`; a stale ETag returns `412`.

**Thunderbird (Lightning)**
- Requires `supported-report-set` to advertise `calendar-query` and
  `calendar-multiget` before it will treat a collection as a calendar.
- Stores ETags verbatim and echoes them in `If-Match`; ETags round-trip
  double-quoted.
- Does not send `Brief`/`Prefer`, so it receives the full report including 404
  propstats.

**GNOME Calendar (Evolution Data Server)**
- Standard discovery and `sync-collection` flows.
- Renames calendars via `displayname` PROPPATCH.

**Microsoft Outlook CalDAV plugins (e.g., CalDavSynchronizer)**
- Behavior varies by plugin version. Some need the explicit principal or
  calendar-home URL rather than `.well-known` bootstrapping.
- Some send query strings on the well-known probe; the redirect preserves them.
- A few older/buggy plugins have sent **unquoted** ETags — GoGoMail rejects
  these with `400` (RFC 7232 requires quoting). Configure the plugin to use
  standard quoted ETags.

## Known limitations

- **`Depth: infinity`** on `PROPFIND`/`REPORT` is rejected with `403` (only
  `Depth: 0` and `Depth: 1` are supported), matching common CalDAV server
  practice and preventing unbounded traversal.
- **Scheduling (RFC 6638)** — inbox/outbox `POST` and iTIP handling are only
  active when scheduling is enabled for the deployment. Free/busy queries work
  independently.
- **CardDAV** (contacts) is a separate gateway (`internal/carddavgw`) and is
  not covered here.

## Troubleshooting

- *Client keeps doing full re-syncs:* confirm the client reads `CS:getctag`
  (it should appear in the `Depth: 1` calendar-home PROPFIND) and that the
  backing store advertises `sync-collection` in the `DAV` header.
- *`412 Precondition Failed` on save:* the object changed on the server since
  the client's last read; the client should re-fetch and merge (this is the
  expected optimistic-concurrency behavior).
- *`400 Bad Request` on save/delete with `If-Match`:* the client sent an
  unquoted ETag. Ensure ETags are double-quoted.
- *Apple color not applied:* verify the client sends `CS:calendar-color`; both
  `#RRGGBB` and `#RRGGBBAA` are accepted.
