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
sync tokens, and record durable changes transactionally. REPORT handling, sync
handlers, auth, broader vCard compatibility, and HTTP listener wiring will be
added only when their semantics are implemented and tested.

## Consequences

- Contacts/CardDAV can evolve as a standards-first module instead of a webmail
  CRUD side table.
- Future CalDAV attendee and resource lookup can depend on Directory plus
  Contacts/CardDAV without inventing private person models.
- Public CardDAV compatibility remains out of scope until vCard validation,
  address-book storage, REPORT behavior, sync, auth, and native-client tests
  are implemented.
