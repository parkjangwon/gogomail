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
Directory also owns bounded principal search across users, organizations,
groups, and resources. Search is company-scoped, optionally domain- and
organization-scoped, principal-kind filtered, query-size limited, result-size
limited, and SQL-wildcard escaped before repository execution. CalDAV
attendee/resource lookup, Contacts/CardDAV autocomplete, shared inbox targeting,
and admin consoles should consume that boundary instead of querying
user/group/resource tables directly.

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
- Principal search is now shared and bounded, giving calendar, contacts,
  shared inbox, Drive, and admin surfaces one predictable contract for
  user/group/resource lookup without product-local search semantics. The admin
  backend API may expose this boundary for operator search and autocomplete,
  but product modules should still consume Directory principal search instead
  of inventing incompatible lookup endpoints.
- Alias-to-principal lookup is now shared and can be reused by mail routing,
  attendee resolution, admin consoles, and future shared inbox flows. The admin
  backend API may expose this lookup for diagnostics, but callers should still
  go through Directory alias resolution so address normalization and active
  target-principal checks remain centralized. Alias listing follows the same
  boundary: callers should use the bounded `ListAliases` repository method for
  admin screens or shared-inbox management instead of reading
  `directory_aliases` directly. The admin API may expose that bounded list
  boundary for diagnostics and management screens, but alias mutation policy
  and audit semantics must be explicit before write endpoints are added.
  Directory-owned alias creation may still exist below the API layer as a
  guarded repository mutation boundary: it must normalize addresses, require
  active tenant/domain scope, enforce alias-address/domain alignment, resolve
  active same-company target principals, and return predictable duplicate
  errors instead of leaking database-driver details. Admin-facing alias
  creation must use the audited variant so the alias row and
  `directory_alias.create` audit row commit atomically; product modules still
  should not grow separate alias mutation semantics.
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
  active owner/delegate principal checks, group filters, depth cap, cycle
  guard, and role hierarchy used by the direct delegation model.
- Delegation inspection now has a bounded repository read boundary as well.
  Admin consoles, shared-calendar management, Drive shares, shared inboxes, and
  Contacts/CardDAV delegation should list relationships through
  `ListDelegations`, with company scope, optional owner/delegate filters,
  scope, role, active-only state, and result limits normalized before query
  execution, instead of issuing product-local SQL against
  `directory_delegations`. The admin backend API may expose this read boundary
  for operator diagnostics, but product modules should still avoid owning
  separate delegation list semantics or mutation flows.
- Product modules should consume delegated access through policy adapters, not
  by branching directly on Directory rows. The initial `internal/accesspolicy`
  adapter turns effective delegation into a normalized allow/deny decision so
  protocol-specific WebDAV privilege mapping and audit logging can be attached
  at product boundaries. Its WebDAV privilege mapper is intentionally shared so
  CalDAV and CardDAV do not grow incompatible role-to-privilege tables, and its
  audit detail builder keeps delegated-access log records normalized with fixed
  reason enums instead of caller-supplied strings. Its delegated-access audit
  log builder also fixes the audit category, action, target, actor, and result
  envelope for future product adapters, and the recorder inserts that envelope
  through the shared audit repository interface. Product modules that authorize
  delegated access should prefer the composed authorizer so effective
  delegation checks and audit insertion stay one fail-closed operation rather
  than two caller-coordinated side effects.
- Future resource-booking policy and delegation models can grow in
  Directory/Identity without forcing CalDAV, CardDAV, Drive, and webmail to
  invent parallel principal semantics.
- Shared calendars, delegated access, resource booking, attendee resolution,
  and auto-complete remain gated until Directory/Identity and Contacts/CardDAV
  semantics are implemented beyond active user lookup.
