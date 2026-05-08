# ADR 0014: CalDAV calendar path slug/alias support

Date: 2026-05-08

## Status

Accepted

## Context

ADR 0010 established the CalDAV gateway boundary with UUID-backed calendar paths. The current implementation requires all calendar collection paths to use the calendar's UUID as the path segment:

```
/caldav/calendars/{user_id}/{calendar_uuid}/
```

This creates usability problems:

1. **Human-unreadable paths** - Users cannot easily identify calendars from URLs
2. **Client discovery issues** - Some CalDAV clients display the raw UUID to users
3. **Migration difficulty** - Changing calendar display names doesn't affect the URL
4. **Apple Calendar quirks** - Apple Calendar creates calendars with display-name-based paths internally that don't match UUID paths

ADR 0010 explicitly deferred slug support:

> "Friendlier human-readable slug aliases need a separate storage/path design before they can be advertised safely to clients."

This ADR defines that storage/path design.

## Decision

### Path Format

CalDAV calendar collection paths support both UUID and slug formats:

```
UUID format:   /caldav/calendars/{user_id}/{calendar_uuid}/
Slug format:   /caldav/calendars/{user_id}/{calendar_slug}/
```

The path segment is disambiguated by format:
- 36-character lowercase hex-with-dashes (UUID) → UUID lookup
- All other valid path segments → slug lookup within user's calendar home

This preserves backward compatibility with existing UUID paths while enabling human-readable slugs.

### Slug Format

A valid calendar slug:
- 1-255 characters
- Lowercase alphanumeric plus hyphens (`a-z0-9-`)
- Cannot start or end with a hyphen
- Cannot contain consecutive hyphens
- Case-insensitive (normalized to lowercase on storage)

Slug normalization ensures `/caldav/calendars/user123/work/` and `/caldav/calendars/user123/Work/` resolve to the same calendar.

### Storage Schema

Add a `slug` column to `caldav_calendars`:

```sql
ALTER TABLE caldav_calendars ADD COLUMN slug text;
CREATE UNIQUE INDEX idx_caldav_calendars_user_active_slug
  ON caldav_calendars (user_id, normalized_slug)
  WHERE status = 'active' AND slug IS NOT NULL;
```

Constraints:
- `slug` is nullable (UUID-backed calendars may have no slug)
- `normalized_slug` is always lowercase, used for uniqueness
- Unique constraint includes `user_id` to scope slugs per user
- Slug uniqueness is enforced only for `active` calendars

### Calendar ID vs Slug

The `id` (UUID) remains the primary key and foreign key reference. The `slug` is an alternative lookup key for path resolution only.

```go
type Calendar struct {
    ID          string    // UUID primary key
    UserID      string
    Name        string    // Display name
    Slug        *string   // Optional path slug (normalized lowercase)
    // ... existing fields
}
```

### Path Resolution Algorithm

When parsing a calendar collection path:

1. Extract the calendar segment from `/caldav/calendars/{user_id}/{calendar_segment}/`
2. If segment is a valid UUID format (36 lowercase hex-with-dashes):
   - Lookup `caldav_calendars` by `id = segment`
   - Reject if not found (404) or deleted (410)
3. If segment is a valid slug format:
   - Lookup `caldav_calendars` by `user_id + slug` where `normalized_slug = lowercase(segment)`
   - Reject if not found (404) or deleted (410)
4. Return the resolved calendar

### MKCALENDAR with Slug Paths

When `MKCALENDAR` is called with a slug path:

```
MKCALENDAR /caldav/calendars/user123/work/
```

1. Validate slug format (lowercase alphanumeric + hyphens, 1-255 chars)
2. Check slug uniqueness within user's active calendars
3. Generate new UUID for `id`
4. Store `slug = "work"` (normalized)
5. Return `201 Created` with `Location: /caldav/calendars/user123/{uuid}/`

**Important**: The `Location` header always returns the UUID path after creation, even if created via slug path. This keeps WebDAV hrefs stable and backward-compatible.

Alternatively, clients may still use UUID paths for `MKCALENDAR`:
```
MKCALENDAR /caldav/calendars/user123/550e8400-e29b-41d4-a716-446655440000/
```
In this case, no slug is created unless explicitly requested via property.

### PROPFIND/PROPPATCH Slug Properties

Support WebDAV properties for slug management:

**DAV:displayname** - Already supported as calendar name
**CALDAV:slug** (Apple Calendar extension) - Read/write slug

```xml
<C:calendar-slug xmlns:C="http://apple.com/ns/icalendar/">work</C:calendar-slug>
```

If client sets `calendar-slug` in MKCALENDAR or PROPPATCH:
- Validate slug format
- Check uniqueness within user's active calendars
- Store the slug

If client does not set `calendar-slug`:
- UUID-only path, no slug stored
- Slug lookup returns null (UUID path only)

### Slug Normalization

Slug normalization (case folding) ensures consistent lookup:

```go
func NormalizeSlug(slug string) (string, error) {
    slug = strings.ToLower(strings.TrimSpace(slug))
    if len(slug) < 1 || len(slug) > 255 {
        return "", errors.New("slug must be 1-255 characters")
    }
    if slug[0] == '-' || slug[len(slug)-1] == '-' {
        return "", errors.New("slug cannot start or end with hyphen")
    }
    if strings.Contains(slug, "--") {
        return "", errors.New("slug cannot contain consecutive hyphens")
    }
    for _, c := range slug {
        if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
            return "", errors.New("slug must be lowercase alphanumeric with hyphens")
        }
    }
    return slug, nil
}
```

### Conflict Resolution

When a slug conflicts with an existing calendar:

1. MKCALENDAR with conflicting slug → `409 Conflict` with `CALDAV:slug-already-exists` precondition
2. PROPPATCH setting slug to existing slug → `409 Conflict`
3. Renaming a calendar to a conflicting slug → `409 Conflict`

When a UUID path conflicts with a slug (edge case where UUID equals normalized slug):

UUID paths take precedence. If user has a slug "abc123" and somehow has a UUID "abc12345...", the UUID path resolves to the UUID-based calendar, not the slug-based one.

## Consequences

### Positive
- Human-readable calendar URLs improve user experience
- Apple Calendar and other clients that use slug-based paths become compatible
- Display name changes don't break URLs (slug persists)
- Backward compatible: existing UUID paths continue to work

### Negative
- Additional database index for slug lookup
- Slug uniqueness constraint adds write overhead
- Path disambiguation by format (UUID vs slug) may cause edge cases

### Migration

Existing calendars without slugs continue to work via UUID paths. No automatic slug creation occurs.

New slugs can be added via PROPPATCH after calendar creation.

## Alternatives Considered

### UUID-only with display-name redirect
Rejected: Breaks WebDAV semantics where hrefs must be stable identifiers.

### Slug as primary identifier
Rejected: Existing integrations depend on UUID primary keys. Breaking change to foreign keys.

### Separate slug table
Rejected: Unnecessary join for common lookup. Slug is an attribute of the calendar, not a separate resource.

### Domain-scoped slugs
Rejected: CalDAV paths are user-scoped (`/caldav/calendars/{user_id}/`). Domain scoping adds unnecessary complexity.

## References

- RFC 4791: CalDAV (calendar-query REPORT)
- Apple Calendar: calendar-slug property extension
- ADR 0010: CalDAV gateway boundary (original UUID-path decision)