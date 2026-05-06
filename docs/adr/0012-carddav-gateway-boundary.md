# ADR 0012: CardDAV Gateway Boundary

## Status

Accepted.

## Context

Calendar attendees, organizer resolution, shared address books, webmail
auto-complete, mobile sync, and future CardDAV clients need a standards-shaped
contacts boundary. Directory owns platform principals and organization
relationships; Contacts/CardDAV must own user address books, personal/external
people, and vCard resources.

Building contacts as generic CRUD first would repeat the CalDAV risk: product
APIs could drift away from WebDAV/CardDAV semantics before native clients are
supported.

## Decision

Create `internal/carddavgw` as the CardDAV/WebDAV protocol boundary. The first
slices own RFC names, DAV capability tokens, principal paths, address-book home
paths, address-book collection paths, `.vcf` contact-object resource paths,
safe relative/absolute href parsing, address-book metadata validation, contact
object name/UID/ETag/size validation, sync-token derivation, and bounded
vCard 3.0/4.0 semantic validation. The vCard parser treats the value separator
as the first unquoted colon so quoted parameter values can contain colons
without being rejected.

PostgreSQL storage tables hold address books, contact objects, and
address-book change logs. The first repository methods create/list/get
address-book collections through active user/domain/company scope and record
creation changes transactionally. The first vCard validator checks BEGIN/END
structure, VERSION, UID, FN, folded lines, content-line caps, body caps, and
nested VCARD rejection. Contact-object repository methods can upsert/list/get/
delete `.vcf` resources under active address-book scope, enforce UID alignment,
compute strong ETags, honor optional observed ETags before overwrite, refresh
sync tokens, and record durable changes transactionally. REPORT parsing
recognizes bounded `addressbook-query`, `addressbook-multiget`, and
`sync-collection` request bodies before handlers are exposed. WebDAV
multistatus response building can render CardDAV principal, address-book,
contact-object, REPORT, and sync metadata.

The first internal discovery handler exposes only RFC 6764/WebDAV-style
`/.well-known/carddav`, `OPTIONS`, and bounded `PROPFIND` behavior over the
CardDAV resource model. It delegates active user principal lookup to the shared
Directory resolver, rejects cross-user resources, rejects `Depth: infinity`,
and requires contact-object discovery to use `Depth: 0`.

Internal REPORT execution now covers the parsed `addressbook-multiget`,
`addressbook-query`, and `sync-collection` shapes. Multiget scopes hrefs to the
requested home or collection and requires an explicit `Depth` header before
resolving hrefs, while accepting common Depth 0/1 client shapes. Query
execution evaluates bounded CardDAV filters over parsed unfolded vCard property
values. The text-match evaluator
supports the RFC 6352 default `i;unicode-casemap` collation, `equals`,
`contains`, `starts-with`, `ends-with`, and `negate-condition`, while rejecting
unsupported collations rather than pretending a different matching semantic is
equivalent. Query execution parses vCard content-line parameters for
`param-filter` existence, `is-not-defined`, or text-match checks, and composes
multiple top-level `prop-filter` predicates plus per-property text/parameter
predicates with RFC 6352 `test=anyof|allof` semantics. Unsupported vCard
property or parameter filters fail with the RFC 6352 `CARDDAV:supported-filter`
precondition instead of a misleading empty success response, including
`Depth: 0` requests that otherwise return no child objects. Unsupported CardDAV
filter child elements use the same `CARDDAV:supported-filter` precondition.
Sync execution can return
full snapshots or bounded change rows since a stored sync token. REPORT
`address-data` can project returned vCards to requested property names while
preserving structural BEGIN/VERSION/END lines, and requested address-data
content types/versions are validated against advertised `text/vcard` 4.0/3.0
support with the RFC 6352 `CARDDAV:supported-address-data` precondition before
handler execution. Unsupported text-match collations are likewise surfaced as
the RFC 6352 `CARDDAV:supported-collation` precondition while malformed
collation syntax remains a bad request. Address-book collections advertise RFC
6352 `CARDDAV:supported-collation-set` with `i;ascii-casemap` and
`i;unicode-casemap`, and query evaluation implements both advertised
collations. Capability properties that should not appear in a bare `allprop`
response remain available through explicit `prop`, `include`, and `propname`
discovery. Returned address-data elements carry explicit
`content-type="text/vcard"` plus a `version` attribute matching the stored
vCard body. Addressbook query
execution honors bounded `limit/nresults` values before rendering multistatus
responses, and repository-backed execution can stream contact objects through a
walker boundary so filtering can stop once the response cap is satisfied.
Address-data projection failures are surfaced as explicit handler errors rather
than silently broadening the returned vCard body. `addressbook-query` requires
an explicit RFC 6352 Depth header; `Depth: 1` scans child address-object
resources, `Depth: infinity` is accepted with the same flat address-book scan
semantics, and `Depth: 0` stays collection-scoped and returns no child objects.
Address-book collection `PROPFIND Depth: 1` child discovery uses the same
bounded collection-scan rule: the gateway asks storage for the default limit
plus one contact object and fails explicitly if the multistatus response would
be partial. This keeps native-client address-book caches from mistaking a
truncated discovery response for a complete collection view.
PROPFIND responses expose conservative RFC 3744 current-user privilege
discovery: resources advertise `DAV:read`, and contact objects additionally
advertise `DAV:write-content` because their object write paths are implemented.
Address-book collections advertise `DAV:bind`/`DAV:unbind` because
contact-object `PUT`/`DELETE` can create or remove child `.vcf` members, plus
`DAV:write-properties` now that collection `PROPPATCH` semantics exist.
Address-book homes advertise `DAV:bind` after extended `MKCOL` support and
`DAV:unbind` after collection `DELETE` support. ACL and unimplemented write
privileges are intentionally not advertised until the gateway implements those
exact WebDAV semantics.
Address-book collection discovery also exposes the CalendarServer-compatible
`getctag` extension from the same durable collection sync token used for
WebDAV `sync-token`, keeping legacy client change detection tied to the
gateway's single collection-version model.
It returns RFC 6352 `addressbook-description` from stored address-book metadata
so protocol discovery and repository state do not drift.
WebDAV `PROPPATCH` now updates address-book collection `DAV:displayname` and
RFC 6352 `addressbook-description` through bounded namespace-aware XML parsing
and a small repository mutation boundary. The mutation refreshes the address
book sync token and records an `addressbook-updated` change row; contact-object
I/O stays on separate `PUT`/`DELETE` paths.
Address-book collections derive a strong WebDAV `getetag` value from the same
durable sync token, and collection `PROPPATCH` honors `If-Match` and
`If-Unmodified-Since` before XML request body reads.
RFC 6352-style extended `MKCOL` can create authenticated address-book
collections at UUID request-URI paths after bounded `DAV:resourcetype`,
`DAV:displayname`, and `CARDDAV:addressbook-description` parsing. UUID-only
request IDs keep path identity separate from future human-readable aliases.
Collection `DELETE` soft-deletes an address book and active child contact
objects transactionally, records an `addressbook-deleted` change row, and keeps
per-contact object deletion on the existing object `DELETE` path.
Stale `sync-collection` requests can still advance through a deleted collection
by reading durable change rows and returning the latest deletion sync token,
instead of requiring the collection row to be active.
`sync-collection` enforces RFC 6578 Depth behavior by accepting default or
explicit `Depth: 0` request scope and rejecting `Depth: 1` before sync lookup or
change-log work. Sync parsing distinguishes an empty initial `DAV:sync-token`
element from a missing token element and rejects the latter before snapshot or
change-log work.
Retention pruning is repository-owned as well. A bounded
`PruneAddressBookChanges` boundary can dry-run or delete old address-book
change-log rows while preserving the newest marker per address book, backed by
a prune-order database index. This keeps future Contacts retention policy out
of the HTTP handler and prevents cleanup work from deleting the token needed by
a current client. Public readiness still needs worker wiring,
deployment-specific retention age policy, and native-client compatibility
testing around expired tokens.
Contact-object writes preflight duplicate active vCard UIDs inside the same
address book before the SQL upsert path. The PostgreSQL partial unique index
remains the final concurrency guard, but normal handler/repository failures now
stay predictable and developer-readable. Final unique-index races for active
object names or UIDs are mapped back into stable repository errors instead of
exposing raw database driver diagnostics.
Contact-object deletes carry observed strong ETags into the repository
transaction, so `If-Match` preconditions are rechecked under the address-book
lock before the active object row is removed.

Contact-object HTTP I/O now exists behind the same internal handler:
`GET`/`HEAD` return vCard bodies and metadata with HTTP cache/precondition
support, while `PUT`/`DELETE` reuse bounded body reads, content-type checks,
explicit 3.0/4.0 media-type version matching, strong ETags, and repository
validation/mutation boundaries.

The first runtime wiring starts `gogomail --mode=carddav` on a dedicated
`GOGOMAIL_CARDDAV_ADDR` listener. It uses HTTP Basic authentication over TLS by
default, shares the existing Submission password verifier, permits insecure
Basic auth only when explicitly configured for non-production environments, and
reuses the existing HTTP server timeout/header guardrails. Broader CardDAV
vCard compatibility and native-client verification will be added only when their
semantics are implemented and tested.
HTTP `Allow` headers follow the same implemented-capability rule. The gateway
advertises only methods with actual request handlers through `OPTIONS` and 405
responses, so future WebDAV method work cannot accidentally appear as a
client-ready contact-server capability.

Delegated contacts access consumes the shared Directory/accesspolicy/audit
boundary rather than defining CardDAV-local sharing rows. The handler separates
authenticated actor user ID from address-book owner user ID, requires the
appropriate `contacts` read/write/manage delegation role before cross-user
`GET`, `PUT`, `DELETE`, `MKCOL`, `PROPPATCH`, `REPORT`, or `PROPFIND`
execution, resolves allowed requests against the owner store, and derives
delegated `DAV:current-user-privilege-set` values for discovery and REPORT
responses from the same WebDAV privilege mapping used by CalDAV. Missing
principals fail closed as access denial, while infrastructure and audit-path
failures remain explicit server errors. This is a protocol boundary foundation,
not a public contacts-sharing UX.

## Consequences

- Contacts/CardDAV can evolve as a standards-first module instead of a webmail
  CRUD side table.
- Future CalDAV attendee and resource lookup can depend on Directory plus
  Contacts/CardDAV without inventing private person models.
- Public CardDAV compatibility remains out of scope until authenticated
  native-client testing and broader vCard compatibility are implemented.
