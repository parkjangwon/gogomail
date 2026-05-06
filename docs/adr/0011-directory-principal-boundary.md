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
kinds. Directory-owned tables hold groups, resources, aliases, and group
memberships so product modules do not invent their own incompatible principal
stores. Active aliases are resolved by normalized email address and map to a
target Directory principal.

CalDAV discovery must use this shared resolver for active principal lookup
instead of owning a private user/domain/company query. CalDAV remains
responsible for DAV/CalDAV paths, hrefs, XML properties, ETags, sync tokens, and
standards-shaped method semantics.
The current CalDAV discovery gateway may convert only Directory user principals
into CalDAV principals. Organization, group, and resource principals stay in the
Directory/Identity layer until delegated calendars, shared ownership, resource
booking policy, and scheduling semantics are implemented explicitly.

## Consequences

- Active user, organization, group, and resource principal lookup is now shared
  and testable outside CalDAV.
- Alias-to-principal lookup is now shared and can be reused by mail routing,
  attendee resolution, admin consoles, and future shared inbox flows.
- Direct group-membership checks are now shared and auditable before recursive
  membership expansion or policy decisions are introduced.
- Effective membership expansion is bounded by depth and guarded against cycles
  before any product module can use it for access policy.
- Delegated access now has an initial company-scoped relationship table and
  repository check boundary. Delegations are keyed by owner principal, delegate
  principal, product scope, and hierarchical role so CalDAV, CardDAV, Drive,
  and shared inbox features can share one auditable model instead of adding
  module-local principal semantics.
- Effective delegation checks preserve the direct delegation boundary and add a
  bounded nested-group expansion path for group delegates. A group-granted
  delegation can satisfy an effective user, organization, group, or resource
  member only through the shared Directory membership graph, with the same
  active filters, depth cap, cycle guard, and role hierarchy used by the direct
  delegation model.
- Future resource-booking policy and delegation models can grow in
  Directory/Identity without forcing CalDAV, CardDAV, Drive, and webmail to
  invent parallel principal semantics.
- Shared calendars, delegated access, resource booking, attendee resolution,
  and auto-complete remain gated until Directory/Identity and Contacts/CardDAV
  semantics are implemented beyond active user lookup.
