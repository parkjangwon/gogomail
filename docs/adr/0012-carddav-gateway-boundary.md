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
