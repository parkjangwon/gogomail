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
vCard 4.0 semantic validation.

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
requested home or collection, and query execution evaluates bounded CardDAV
filters over parsed unfolded vCard property values. The text-match evaluator
supports the RFC 6352 default `i;unicode-casemap` collation, `equals`,
`contains`, `starts-with`, `ends-with`, and `negate-condition`, while rejecting
unsupported collations rather than pretending a different matching semantic is
equivalent. Query execution parses vCard content-line parameters for
`param-filter` existence, `is-not-defined`, or text-match checks, and composes
multiple top-level `prop-filter` predicates plus per-property text/parameter
predicates with RFC 6352 `test=anyof|allof` semantics. Sync execution can return
full snapshots or bounded change rows since a stored sync token. REPORT
`address-data` can project returned vCards to requested property names while
preserving structural BEGIN/VERSION/END lines, and requested address-data
content types/versions are validated against the advertised `text/vcard` 4.0
support before handler execution. Returned address-data elements carry explicit
`content-type="text/vcard"` and `version="4.0"` attributes. Addressbook query
execution honors bounded `limit/nresults` values before rendering multistatus
responses, and repository-backed execution can stream contact objects through a
walker boundary so filtering can stop once the response cap is satisfied.
Address-data projection failures are surfaced as explicit handler errors rather
than silently broadening the returned vCard body.
PROPFIND responses expose conservative RFC 3744 current-user privilege
discovery: resources advertise `DAV:read`, and contact objects additionally
advertise `DAV:write-content` because their object write paths are implemented.
Address-book collections advertise `DAV:write-properties` now that collection
`PROPPATCH` semantics exist. Address-book homes advertise `DAV:bind` after
extended `MKCOL` support and `DAV:unbind` after collection `DELETE` support.
ACL and broader collection write privileges are intentionally not advertised
until the gateway implements those exact WebDAV semantics.
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
Contact-object writes preflight duplicate active vCard UIDs inside the same
address book before the SQL upsert path. The PostgreSQL partial unique index
remains the final concurrency guard, but normal handler/repository failures now
stay predictable and developer-readable. Final unique-index races for active
object names or UIDs are mapped back into stable repository errors instead of
exposing raw database driver diagnostics.

Contact-object HTTP I/O now exists behind the same internal handler:
`GET`/`HEAD` return vCard bodies and metadata with HTTP cache/precondition
support, while `PUT`/`DELETE` reuse bounded body reads, content-type checks,
strong ETags, and repository validation/mutation boundaries.

The first runtime wiring starts `gogomail --mode=carddav` on a dedicated
`GOGOMAIL_CARDDAV_ADDR` listener. It uses HTTP Basic authentication over TLS by
default, shares the existing Submission password verifier, permits insecure
Basic auth only when explicitly configured for non-production environments, and
reuses the existing HTTP server timeout/header guardrails. Broader CardDAV
vCard compatibility and native-client verification will be added only when their
semantics are implemented and tested.

## Consequences

- Contacts/CardDAV can evolve as a standards-first module instead of a webmail
  CRUD side table.
- Future CalDAV attendee and resource lookup can depend on Directory plus
  Contacts/CardDAV without inventing private person models.
- Public CardDAV compatibility remains out of scope until authenticated
  native-client testing and broader vCard compatibility are implemented.
