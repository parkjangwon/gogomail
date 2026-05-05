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
requested home or collection, query execution applies the current bounded first
text-match filter over stored vCard bodies, and sync execution can return full
snapshots or bounded change rows since a stored sync token. Object
`GET`/`PUT`/`DELETE`, richer CardDAV filter semantics, auth, broader vCard
compatibility, native-client verification, and HTTP listener wiring will be
added only when their semantics are implemented and tested.

## Consequences

- Contacts/CardDAV can evolve as a standards-first module instead of a webmail
  CRUD side table.
- Future CalDAV attendee and resource lookup can depend on Directory plus
  Contacts/CardDAV without inventing private person models.
- Public CardDAV compatibility remains out of scope until authenticated
  listener wiring, object mutation/read behavior, richer filters, broader
  vCard compatibility, and native-client tests are implemented.
