# ADR 0016: CalDAV Scheduling Attendee Resolution

Date: 2026-05-08

## Status

Accepted

## Context

The CalDAV scheduling handler (RFC 6047 iMIP) currently extracts ATTENDEE and ORGANIZER
mailto: addresses from iCalendar payloads without resolving them against the platform's
Directory or user's CardDAV contacts. This means:

1. Internal users referenced by email address cannot be identified as platform principals
2. External attendees who exist in a user's CardDAV address book cannot be enriched with contact metadata
3. Scheduling cannot distinguish internal vs external delivery paths
4. Future features (internal meeting routing, resource booking, delegated scheduling) are blocked

ADR 0010 established the CalDAV gateway boundary and deferred scheduling semantics.
ADR 0015 added timezone support (RFC 7809). This ADR adds attendee resolution as the
bridge between iMIP scheduling and the Directory/Contacts platform layer.

## Decision

### AttendeeResolution Result Type

Add an `AttendeeResolution` type that captures the result of resolving one ATTENDEE address:

```go
type AttendeeResolution struct {
    Address     string          // original address (mailto: stripped)
    Kind        AttendeeKind    // internal-user | directory-alias | carddav-contact | external
    UserID      string          // set if Kind is internal-user
    Principal   directory.Principal  // set if Kind is internal-user or directory-alias
    Contact     *ContactObject  // set if Kind is carddav-contact
}
```

### AttendeeResolver Interface

```go
type AttendeeResolver interface {
    ResolveAttendees(ctx context.Context, userID string, addresses []string) ([]AttendeeResolution, error)
}
```

### Resolution Chain

When `ResolveAttendees` is called for a list of addresses:

1. **Directory user lookup**: For each address, first try exact match in `user_addresses` table.
   If found and active, return `internal-user` with the user principal.

2. **Directory alias resolution**: If not a user, try `directory_aliases` lookup via `ResolveAlias`.
   If found and active, return `directory-alias` with the alias target principal.

3. **CardDAV contact search**: If not a directory entry, search the user's active address books
   for vCards containing the email address in an EMAIL property. If found, return `carddav-contact`
   with the contact object (raw vCard available for enrichment).

4. **External**: If none of the above, return `external` with no principal/contact.

### Directory: Exact User Email Lookup

Add `ResolveUserByEmail` to `directory.Repository`:

```go
func (r *Repository) ResolveUserByEmail(ctx context.Context, address string, activeOnly bool) (Principal, error)
```

Query: exact match on `lower(user_addresses.address) = lower($1)` joined to `users`,
`domains`, `companies` with status checks. Returns the user `Principal` or
`ErrPrincipalNotFound`.

This is an exact, indexed lookup — not a fuzzy search like `SearchPrincipals`.

### CardDAV: Email Search Across Address Books

Add `SearchContactsByEmail` to `carddavgw.Repository`:

```go
type SearchContactsByEmailRequest struct {
    UserID  string
    Email   string
    Limit   int
}

func (r *Repository) SearchContactsByEmail(ctx context.Context, req SearchContactsByEmailRequest) ([]ContactObject, error)
```

Implementation: scan active address books owned by `userID`, return contact objects whose
stored vCard contains the email in an EMAIL property. The vCard EMAIL property is
matched case-insensitively. Returns up to `Limit` results (default 10, max 50).

The search is bounded: only the user's own address books are queried, respecting
CardDAV ownership semantics.

### Scheduling Handler Integration

Update `scheduling.Handler.HandleEvent` to accept an optional `AttendeeResolver`:

```go
type Handler struct {
    logger         *slog.Logger
    queue          Queue
    store          ObjectStore
    attendeeResolver AttendeeResolver  // nil means skip resolution
}
```

When `attendeeResolver` is set:
- Before sending iTIP to each attendee, resolve the address
- Log the resolution kind (internal-user/directory-alias/carddav-contact/external)
- Keep existing SMTP delivery flow unchanged; resolution is informational for now

This keeps external attendees on the existing `mail.outbound.general` path while
enabling future internal routing decisions without changing the handler's contract.

## Consequences

- Scheduling handler gains Directory + CardDAV visibility for attendee addresses
- Internal users are identified before iTIP delivery, enabling future internal routing
- External attendees that exist in CardDAV contacts can be enriched with contact metadata
- Existing scheduling delivery flow is unchanged; resolution is a non-breaking additive layer
- The `AttendeeResolver` interface is pluggable, allowing future implementations
  (e.g., LDAP, external directory) without changing the scheduling handler

## References

- RFC 6047: iMIP (Internet Media Implementation)
- RFC 4791: CalDAV Calendar Extensions
- RFC 6352: CardDAV Internationalized Feature
- ADR 0010: CalDAV Gateway Boundary
- ADR 0015: CalDAV Timezone Support