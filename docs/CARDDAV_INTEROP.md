# CardDAV client interoperability

GoGoMail's CardDAV gateway (`internal/carddavgw`) implements
[RFC 6352](https://www.rfc-editor.org/rfc/rfc6352) (CardDAV) over
[RFC 4918](https://www.rfc-editor.org/rfc/rfc4918) (WebDAV), and stores contacts
as [RFC 6350](https://www.rfc-editor.org/rfc/rfc6350) (vCard 4.0) or
[RFC 2426](https://www.rfc-editor.org/rfc/rfc2426) (vCard 3.0) objects.
Autodiscovery follows [RFC 6764](https://www.rfc-editor.org/rfc/rfc6764).

This document records how the gateway behaves with the major native address-book
clients, the vCard versions each speaks, discovery URLs, and known limitations.

## Discovery

| Purpose | URL |
|---|---|
| Well-known autodiscovery | `https://<host>/.well-known/carddav` → `301` to `/carddav/` |
| Service root | `/carddav/` |
| Principal | `/carddav/principals/{userID}/` |
| Address-book home set | `/carddav/addressbooks/{userID}/` |
| Address-book collection | `/carddav/addressbooks/{userID}/{addressBookID}/` |
| Contact object | `/carddav/addressbooks/{userID}/{addressBookID}/{name}.vcf` |

Clients should be pointed at the bare host (or `/.well-known/carddav`); they
follow `current-user-principal` and `addressbook-home-set` from PROPFIND to
locate the address books. RFC 6764 DNS `SRV`/`TXT` records may also be published
so users only enter an email address.

`OPTIONS` advertises DAV classes `1, 3`, `addressbook`, `extended-mkcol`, and
(when the sync store is available) `sync-collection`. Supported reports are
`addressbook-query`, `addressbook-multiget`, and `sync-collection`.

## Supported vCard versions

Both vCard 3.0 and 4.0 are accepted on `PUT` and returned on `GET` /
`addressbook-multiget` / `addressbook-query`. The stored body is preserved
verbatim except for the PHOTO and CATEGORIES/GROUP properties, which are
extracted into dedicated columns and re-merged on read (see
[Photo handling](#photo-handling)). `VERSION` must be exactly `3.0` or `4.0`;
vCard 2.1 is rejected. Every card must carry `UID` and `FN` (both mandatory per
RFC 6350 / RFC 2426).

The `Content-Type` on `PUT` may include a `version` parameter
(`text/vcard; version=4.0`). When present, it must agree with the body's
`VERSION` line or the request is rejected with `400`.

## Per-client support matrix

| Client | Platform | vCard version(s) | Inline photo | Notes |
|---|---|---|---|---|
| Apple Contacts | macOS, iOS | 3.0 (default), 4.0 | `PHOTO;ENCODING=b;TYPE=JPEG` | Uses `item1.`-grouped properties with `X-ABLabel`; may send quoted `TYPE="HOME,VOICE"` and repeated `TYPE=` params. |
| DAVx5 | Android | 4.0 (default), 3.0 | `PHOTO:data:image/…;base64,…` and `PHOTO;ENCODING=b` | Relies on property/parameter order being preserved; issues `addressbook-query` param-filters on `TYPE`. |
| Mozilla Thunderbird / TbSync | Desktop | 3.0 (primary), 4.0 (basic) | `PHOTO;ENCODING=b` | Emits `QUOTED-PRINTABLE`/`CHARSET` params for non-ASCII in 3.0; limited 4.0 support. |
| GNOME Contacts / Evolution | Linux | 3.0, 4.0 | `PHOTO;ENCODING=b` | Folds long lines aggressively; emits `CATEGORIES`. |

All four clients round-trip through the gateway without field loss, including
unknown `X-` properties, item grouping, folded lines, and inline photos. These
behaviors are locked in by regression tests named after each client in
`internal/carddavgw/client_matrix_test.go` and
`internal/carddavgw/report_filter_matrix_test.go`.

## Property parameter handling

- **`TYPE` with quoted comma** — `TEL;TYPE="HOME,VOICE":…` keeps the comma as a
  literal single value; `TEL;TYPE=HOME,VOICE:…` splits into two values. Both
  forms and the repeated-parameter form (`TYPE=HOME;TYPE=VOICE`) parse
  correctly.
- **Item grouping** — Apple's `item1.EMAIL` / `item1.X-ABLabel` grouping is
  preserved verbatim; the group prefix is stripped only for property-name
  matching, never from the stored body.
- **Line folding** — continuation lines beginning with a space or tab are
  unfolded per RFC 6350 §3.2 during parsing and filtering; re-merged inline
  photos are folded back at 75 octets so no physical line exceeds the limit.
- **Charset / encoding** — `CHARSET` and `ENCODING=QUOTED-PRINTABLE` parameters
  (Thunderbird 3.0) are accepted and preserved unchanged.

## Photo handling

Inline photos are extracted into a dedicated store column so the webmail avatar
UI can use them, then re-merged into the vCard on read. The following inline
forms are recognized and captured losslessly:

| Form | Example | Emitted by |
|---|---|---|
| vCard 3.0 parameterized base64 | `PHOTO;ENCODING=b;TYPE=JPEG:<base64>` (also `ENCODING=B`, `ENCODING=BASE64`) | Apple, DAVx5, Thunderbird, GNOME |
| vCard 4.0 data URI | `PHOTO:data:image/jpeg;base64,<base64>` | DAVx5, Apple (4.0) |
| vCard 4.0 `MEDIATYPE` + base64 | `PHOTO;MEDIATYPE=image/jpeg;ENCODING=b:<base64>` | DAVx5 |

On read the photo is re-emitted as the widely interoperable vCard 3.0 form
`PHOTO;ENCODING=b;TYPE=<SUBTYPE>:` with the base64 payload folded at 75 octets.
The stored media type is normalized to `image/<subtype>`.

Photos that are **not** inline base64 are left untouched in the vCard body:

- External URI references (`PHOTO:https://…`, `PHOTO;MEDIATYPE=image/jpeg:https://…`)
  round-trip in place.
- Inline payloads larger than the 5 MiB photo cap are left in the body rather
  than being dropped.

## REPORT filtering

`addressbook-query` supports `prop-filter` and `param-filter` with `text-match`
(`equals`, `contains`, `starts-with`, `ends-with`, optional `negate-condition`)
and `is-not-defined`, combined with `test="anyof"` (default) or `test="allof"`.
Collations `i;unicode-casemap` (default) and `i;ascii-casemap` are supported.

- `param-filter` matches against a property's parameter values, e.g.
  `EMAIL` with `TYPE` `equals` `work`.
- `param-filter`/`prop-filter` `is-not-defined` selects records lacking the
  parameter/property.
- Unsupported filter properties, parameters, collations, or `address-data`
  content types return the appropriate CardDAV precondition
  (`supported-filter`, `supported-collation`, `supported-address-data`) with
  HTTP `403`, rather than a silent empty result.

`addressbook-multiget` returns the requested contact objects by href, with
`404` propstat entries for hrefs that are missing or out of scope.

### address-data projection

When a client requests a subset of properties via
`<C:address-data><C:prop name="…"/></C:address-data>`, the projected card always
retains `BEGIN`, `VERSION`, `UID`, `FN`, and `END` in addition to the requested
properties. `FN` is mandatory in both vCard versions; retaining it keeps the
projected card valid so clients such as Apple Contacts do not reject it.

## ETag, If-Match, and UID stability

- Each contact object carries a strong `ETag` derived from a SHA-256 of the
  vCard body. Identical bodies yield identical ETags; any change yields a new
  one.
- `PUT` honors `If-Match` (optimistic concurrency; stale tag → `412`),
  `If-None-Match: *` (create-only; existing object → `412`), `If-None-Match`
  with a tag, `If`, `If-Unmodified-Since`, and the `Content-Type` `version`
  parameter.
- `GET` honors `If-None-Match` / `If-Modified-Since` and returns `304 Not
  Modified` with the current validator headers.
- `DELETE` honors `If-Match` / `If-None-Match` / `If` / `If-Unmodified-Since`.
- A `3.0 → 4.0 → 3.0` version conversion driven by a client preserves the `UID`
  and all fields, and never duplicates the `UID`, because the gateway stores the
  body the client sends and only re-merges the extracted PHOTO/CATEGORIES/GROUP.

## Known limitations

- The gateway does not transcode between vCard 3.0 and 4.0; it stores and
  returns the version the client submits. Cross-version conversion (if any) is
  the client's responsibility.
- `QUOTED-PRINTABLE` values are preserved but not decoded server-side; search
  and autocomplete match against the raw stored text.
- `sync-collection` is only advertised when the sync store is configured;
  otherwise clients fall back to `addressbook-query` / `getctag` polling.
- `Depth: infinity` is not supported for REPORT (`403`), per common CardDAV
  server practice.
- The photo store cap is 5 MiB; larger inline photos remain embedded in the
  vCard body and are not surfaced to the avatar UI.

## Test coverage

- `internal/carddavgw/client_matrix_test.go` — vCard 3.0/4.0 matrix, Apple /
  DAVx5 / Thunderbird / GNOME quirks, X- property preservation, inline/URI/
  oversize photo handling, photo fold width.
- `internal/carddavgw/report_filter_matrix_test.go` — `addressbook-query`
  `prop-filter`/`param-filter` (`TYPE`, `is-not-defined`, `allof`/`anyof`),
  unsupported-filter preconditions, `addressbook-multiget`, address-data
  projection retaining `FN`, ETag/If-Match/If-None-Match preconditions, and
  UID stability across a `3.0 → 4.0 → 3.0` round trip.
- `internal/carddavgw/metadata_test.go`, `handler_test.go`, `xml_test.go`,
  `response_test.go` — parsing, handler behavior, and REPORT/PROPFIND coverage.
