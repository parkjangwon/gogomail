# ADR 0011: Directory Principal Boundary

## Status

Accepted.

## Context

CalDAV, CardDAV, webmail, admin APIs, Drive, mobile sync, and future resource
booking all need to answer the same question: who or what is this principal, and
is it currently usable? Keeping that logic inside each product module would make
delegation, resource calendars, group membership, aliases, audit, and policy
hard to implement consistently.

The current CalDAV implementation is still experimental/backend-only. It can
continue to improve protocol semantics, but public shared/delegated calendar
features require a platform Directory/Identity layer first.

## Decision

Create `internal/directory` as the protocol-neutral boundary for platform
principals. The first implementation slices resolve active user principals from
the existing `users`, `domains`, and `companies` tables and organization
principals from the existing `organizations`, `domains`, and `companies`
tables, with bounded principal identifier validation and explicit principal
kinds.

CalDAV discovery must use this shared resolver for active principal lookup
instead of owning a private user/domain/company query. CalDAV remains
responsible for DAV/CalDAV paths, hrefs, XML properties, ETags, sync tokens, and
standards-shaped method semantics.

## Consequences

- Active user and organization principal lookup is now shared and testable
  outside CalDAV.
- Future group, resource, alias, membership, and delegation models can grow in
  Directory/Identity without forcing CalDAV, CardDAV, Drive, and webmail to
  invent parallel principal semantics.
- Shared calendars, delegated access, resource booking, attendee resolution,
  and auto-complete remain gated until Directory/Identity and Contacts/CardDAV
  semantics are implemented beyond active user lookup.
