# gogomail storage backends

gogomail stores raw `.eml` objects, attachments, exports, and readiness probes
through the shared storage interface. Deployments can switch backends by
configuration without changing stored object keys.
Object paths, prefixes, and list cursors are validated as bounded valid UTF-8
text before adapter use, keeping local/NFS paths, S3 URLs, SigV4 canonical
paths, logs, and cleanup cursors unambiguous across backends. Shared object
paths and prefixes also reject percent-encoded path separators such as `%2F`
and `%5C`, including double-encoded forms such as `%252F` and `%255C`, so
deployments do not create keys whose meaning depends on provider- or
proxy-specific double-decoding behavior.

The shared interface supports `Put`, `Get`, `GetRange`, `Stat`, `Copy`,
`Move`, `List`, and `Delete`.
`Stat` returns object size and optional backend metadata without streaming the
object body, using filesystem metadata on local/NFS storage and signed `HEAD`
requests on S3-compatible storage. `GetRange` opens a validated bounded byte
range without requiring callers to stream and discard a prefix, using
filesystem seek/limited reads locally and signed S3 `Range` requests remotely.
S3-compatible range reads require `206 Partial Content` with a matching
`Content-Range` unless a provider returns a compatibility `200 OK` response
that can be proven to represent the requested full window: matching
`Content-Range`, or offset-zero `Content-Length` exactly equal to the requested
length. Ambiguous `200 OK` range responses are rejected and drained before any
caller sees bytes. Duplicate `Content-Range` headers also fail closed on both
`206 Partial Content` and `200 OK` range-compatibility responses.
`Copy` preserves object keys and adapter semantics while avoiding caller-side
read/write loops when the backend can copy server-side. `Move` gives callers
one backend-neutral object relocation contract: local/NFS uses filesystem
rename semantics and falls back to copy-delete only for cross-device `EXDEV`
rename failures, while S3-compatible storage performs server-side copy followed
by source delete. `List` returns bounded, cursor-paginated object
metadata under a validated prefix, using a local/NFS directory walk or signed
S3 `ListObjectsV2` requests; truncated S3 pages must include a continuation
token before callers see the page, and provider-returned keys are not trimmed
before prefix/object-path validation. S3-compatible listings are also rechecked
against the requested logical prefix after bucket-prefix stripping, so overly
broad provider pages cannot expose sibling prefixes to callers. Provider
responses must also respect the requested page size for matching objects, so a
malformed S3-compatible service cannot bypass the shared bounded-list contract.
Local/NFS listings now also return a single-object page when the requested
prefix exactly names an object, matching S3 `Prefix` behavior and keeping
config-only backend flips from turning exact-object reconciliation probes into
"not a directory" failures.
Future Drive and lifecycle modules should prefer `Stat` for existence/size
checks, `GetRange` for resumable downloads and media preview windows, `Copy`
for object duplication workflows, `Move` for file rename/relocation workflows,
and `List` for prefix-scoped browsing, reconciliation, and cleanup scans.
S3-compatible listings validate provider continuation tokens only when a page
is actually truncated and always clear final-page cursors, so ignored
`NextContinuationToken` values on non-truncated provider responses do not leak
into callers.
`storage.DeletePrefix` composes `List` and idempotent `Delete` into a bounded
page-level cleanup helper for future Drive folder deletion, attachment
lifecycle, and reconciliation jobs without requiring callers to know whether
the backend is local/NFS or S3-compatible storage.
If a listing page is truncated, it must include a continuation cursor before
`DeletePrefix` deletes any listed object; otherwise cleanup fails closed rather
than risking an unresumable partial prefix delete.
Every listed object is rechecked against the requested canonical cleanup prefix
before deletion, so a backend that returns a safe but out-of-scope sibling key
cannot cause shared cleanup to delete outside the caller's prefix.
If a backend listing returns an unsafe object path, `DeletePrefix` preserves
completed delete progress and returns a structured unsafe-listed-object error,
letting cleanup workers distinguish corrupt listing data from provider delete
failures.

## Backend migration smoke matrix

Before changing a deployment from local/NFS storage to MinIO/AWS S3, or between
S3-compatible providers, operators should run the same backend-neutral object
contract against the target backend:

- `Put` stores a canonical slash-separated key with mail-like and Drive-like
  segments, including literal `+`, `@`, spaces, and `=` characters.
- `Get` and `GetRange` return the exact original bytes without caller-side
  buffering or prefix streaming.
- `Stat` reports the canonical object key and byte size without reading the
  object body.
- `List` returns only keys under the requested canonical prefix, hides any
  deployment-specific S3 bucket prefix from callers, and validates object
  metadata only after a backend key maps back to a canonical gogomail object
  path. Exact-object prefixes return that single object when it exists.
- `Copy` preserves bytes at a new key; `Move` relocates through the backend's
  documented semantics while removing the old key.
- `Delete` is idempotent for already-missing objects.
- `storage.DeletePrefix` removes a bounded page of remaining objects under the
  prefix without touching sibling prefixes and reports unsafe listed object
  paths separately from delete failures.

Runtime readiness also writes, reads, stats, range-reads, and deletes a short
probe object, so full-object bytes, metadata paths, and partial-read paths are
checked before an instance reports ready. `TestLocalStorePortabilityContract`
always runs this contract for local/NFS semantics. `TestS3StoreIntegrationRoundTrip`
reuses the same contract when `GOGOMAIL_TEST_S3_*` variables are set, giving
MinIO and AWS S3 deployments a single smoke test before a storage backend flip.
The Admin Console capabilities endpoint also exposes a redacted runtime storage
profile under `admin_console_capabilities.storage`: configured backend, active
compatibility labels, backend class, supported object primitives, S3 path-style
status, sanitized endpoint/bucket/prefix/region fields when applicable, and
`secrets_redacted=true`. It intentionally does not expose access keys, session
tokens, or local filesystem root paths.

Validated config overlays live under `configs/` for common storage profiles:

- `configs/storage.local.yaml`
- `configs/storage.nfs.yaml`
- `configs/storage.minio.yaml`
- `configs/storage.s3.yaml`

Each profile is parsed, validated, and passed through the CLI `--config`
handoff by the test suite. The NFS profile smoke path also asserts that its
`storage_root` and explicit `local` compatibility label reach runtime config,
so local/NFS flips are covered beyond the backend name alone. Operators can use
these files as reviewed `--config=<path>` starting points, replacing secrets,
bucket names, prefixes, and roots while keeping backend-specific knobs
explicit. YAML config files accept `storage_root` as the file-level alias for
the same local/NFS root controlled by `GOGOMAIL_STORAGE_ROOT`; legacy
`mailstore_root` remains supported for backward compatibility.

## Local filesystem or NFS

Local storage is the default and can point at a normal disk path or an
NFS-style mounted directory.

```sh
GOGOMAIL_STORAGE_BACKEND=local
GOGOMAIL_STORAGE_ROOT=./data/storage
```

Operators who want configuration to state that the local filesystem root is an
NFS-backed mount can use the explicit alias:

```sh
GOGOMAIL_STORAGE_BACKEND=nfs
GOGOMAIL_STORAGE_ROOT=/mnt/gogomail-storage
```

`nfs` uses the same local filesystem adapter and object-key contract as
`local`. Runtime storage wiring registers `local` and `nfs` as bidirectional
compatibility labels when either backend is active, so rows recorded under one
label can still be served after a config-only local/NFS alias flip.

Use local storage for development, single-node installs, or deployments where
the mount itself provides durability and availability.
`GOGOMAIL_STORAGE_ROOT` is the storage-focused alias for
`GOGOMAIL_MAILSTORE_ROOT`; if both are set, `GOGOMAIL_MAILSTORE_ROOT` wins for
backward compatibility. YAML config overlays can use `storage_root` for the
same value, while `mailstore_root` remains accepted for older configs. The
effective root must be non-empty, bounded, and free of line breaks when the
local backend is active.
Writes are staged through unique temporary files in the destination directory
and committed with `rename`, avoiding fixed `.tmp` collisions on local or
NFS-style mounts while preserving atomic object replacement semantics.
Canceled write contexts stop body copy, remove the staged temp object, and do
not commit partial data.
Deletes are idempotent for missing objects, matching S3-style cleanup behavior
so lifecycle workers behave consistently across storage backends.
`Stat` reports the canonical object key, byte size, and filesystem
last-modified time without opening the file body.
Symbolic links under the local/NFS storage root are not treated as objects:
reads, range reads, metadata probes, deletes, copies, moves, writes, and prefix
listings reject symlinked intermediate directories, and list operations hide
final-object symlinks. This keeps mounted filesystems from escaping the
object-store contract through host-specific link behavior.
`GetRange` seeks to the requested byte offset and returns a closeable limited
reader for the requested positive byte count, keeping partial reads efficient
for local disk and NFS-style mounts. If the requested local/NFS byte window
extends past the stored object, callers see `io.ErrUnexpectedEOF`, matching the
S3-compatible range-reader contract instead of receiving a silent short read.
`Copy` streams from the source object into the destination through the same
atomic temporary-file write path used by normal local/NFS writes.
`Move` creates missing destination directories and uses `rename`, keeping local
and NFS-style file relocation efficient and avoiding caller-side copy/delete
loops on the normal path. If `rename` reports cross-device `EXDEV`, `Move`
falls back to the same validated copy-delete sequence used by the shared
storage contract so bind-mount or NFS-style boundary layouts can still relocate
objects safely.
`List` walks under the requested canonical prefix, returns bounded pages, and
uses an opaque cursor so callers do not depend on filesystem traversal details.
Prefix cleanup uses the same bounded list pages and idempotent object deletes,
so large local/NFS cleanup jobs can advance with explicit cursors instead of
walking and deleting an unbounded tree in one operation. Direct object deletes
reject directories rather than treating filesystem folders as S3-like objects.

## Local MinIO

The development compose stack starts MinIO and creates a `gogomail` bucket via
the `minio-init` service.

```sh
docker compose -f deploy/docker-compose.dev.yml up -d minio minio-init

GOGOMAIL_STORAGE_BACKEND=minio
GOGOMAIL_STORAGE_S3_ENDPOINT=http://localhost:19000
GOGOMAIL_STORAGE_S3_REGION=us-east-1
GOGOMAIL_STORAGE_S3_BUCKET=gogomail
GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID=gogomail
GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY=gogomail123
```

The `minio` backend always uses path-style S3 requests so local endpoints do
not need wildcard DNS.
Drive and upload-session rows persist a `storage_backend` label. At runtime,
gogomail registers the configured S3-compatible store under both `minio` and
`s3` labels when either backend is active, so a deployment can move from local
MinIO to AWS S3-style configuration, or back, without breaking existing rows
solely because their stored backend label differs. Operators still need to
migrate or replicate the actual bucket contents and preserve object keys.

## Staged backend label migrations

Rows for Drive files and upload sessions intentionally remember the
`storage_backend` label that was active when the object was written. By
default, gogomail serves only the currently configured backend label, plus the
built-in `s3`/`minio` aliases for S3-compatible stores. Other historical labels
remain fail-closed so a configuration typo does not silently route reads or
cleanup to the wrong storage system.

During a planned migration, copy or replicate object bytes first, preserve the
same object keys, then opt into legacy label compatibility explicitly:

```sh
GOGOMAIL_STORAGE_BACKEND=s3
GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS=local
```

This example lets Drive operations resolve historical `local`/NFS-labelled
rows through the configured S3-compatible store after operators have migrated
the underlying bytes. The same setting can be used in rollback windows, for
example to serve `s3`-labelled rows from a local/NFS store after object keys
have been copied back. Keep the compatibility window short, verify reads,
downloads, deletes, and cleanup retries, then backfill row labels or remove the
compatibility setting once the cutover is complete.

## AWS S3 or compatible object storage

Use the `s3` backend for AWS S3 or S3-compatible services that accept SigV4.
Bucket names follow AWS general purpose bucket naming rules: gogomail rejects
IP-address-shaped names plus reserved AWS prefixes and suffixes during config
validation, before adapter construction.

```sh
GOGOMAIL_STORAGE_BACKEND=s3
GOGOMAIL_STORAGE_S3_ENDPOINT=https://s3.us-east-1.amazonaws.com
GOGOMAIL_STORAGE_S3_REGION=us-east-1
GOGOMAIL_STORAGE_S3_BUCKET=gogomail-prod
GOGOMAIL_STORAGE_S3_PREFIX=mail
GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID=...
GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY=...
```

Credential values are validated before adapter construction. Access key IDs,
secret access keys, and session tokens must not contain spaces, tabs, or line
breaks, so copied environment values with hidden whitespace fail fast instead
of surfacing later as opaque S3 authentication failures. Direct adapter inputs
also enforce the same size bounds as startup config validation: access key IDs
and secret access keys are capped at 4096 bytes, and session tokens are capped
at 8192 bytes.

Production `s3` deployments must set `GOGOMAIL_STORAGE_S3_ENDPOINT`
explicitly, even when using AWS S3, so release configs clearly show which
object-storage endpoint will receive mail, Drive, attachment, and export
objects. Development and test configs may omit it; the adapter derives the
standard AWS regional endpoint from `GOGOMAIL_STORAGE_S3_REGION`. Production
`s3` endpoints must use HTTPS. This keeps streaming SigV4
`UNSIGNED-PAYLOAD` requests behind transport integrity for AWS/S3-compatible
object stores; local HTTP MinIO remains available through the explicit
`minio` backend and development profile.

Set `GOGOMAIL_STORAGE_S3_ENDPOINT` for non-AWS compatible services as well.
Endpoints must be plain HTTP(S) origins with an optional canonical base path;
userinfo, query strings, fragments, duplicate separators, dot segments, and
encoded path separators such as `%2F` or `%5C` are rejected so SigV4 signing
and object addressing stay unambiguous. Set
`GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` when the provider or local network
does not support virtual-hosted bucket names. HTTPS endpoints automatically use
path-style requests for bucket names that contain periods, matching AWS's
certificate compatibility guidance for dotted bucket names. Localhost and
IP-address endpoints also use path-style requests automatically, so local
MinIO or other local compatible stores do not accidentally receive
`bucket.localhost` or `bucket.127.0.0.1` style hosts when the generic `s3`
backend is used.

Private MinIO and S3-compatible deployments that use an internal certificate
authority can add that trust root without changing host trust globally:

```sh
GOGOMAIL_STORAGE_S3_CA_CERT_FILE=/etc/gogomail/s3-ca.pem
```

The file must be a readable PEM bundle with at least one certificate.
gogomail appends it to the system certificate pool and uses a dedicated S3
HTTP client with TLS 1.2 or newer. For local development against temporary
self-signed endpoints, `GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY=true` can
disable certificate verification, but startup validation rejects that setting
when `GOGOMAIL_ENV=production`.

Object keys are path-escaped segment by segment so literal `+` characters stay
encoded as `%2B`, preserving object identity and SigV4 canonical paths across
AWS S3, MinIO, and strict compatible providers. Endpoint base paths use the
same segment escaping, so reverse-proxy paths such as `/base+proxy` keep their
literal plus signs in canonical request paths.
Configured storage prefixes and object keys must not contain encoded
separators such as `%2F` or `%5C`, including double-encoded forms such as
`%252F` or `%255C`; gogomail fails these values before filesystem path
construction or S3 request signing so the same logical object boundary is used
on local/NFS, AWS S3, MinIO, and stricter S3-compatible gateways.
For file-backed or otherwise seekable upload bodies, gogomail sets a precise
`Content-Length` without buffering the object in memory, improving PUT
compatibility while preserving streaming-first storage paths.
Deletes accept only completed compatible-provider success responses (`200 OK`
or `204 No Content`) plus idempotent `404 Not Found` for already-missing
objects. Accepted/deferred or otherwise ambiguous non-OK 2xx delete responses
are rejected, so lifecycle cleanup behaves consistently across AWS S3,
MinIO-style endpoints, and local/NFS storage.
Missing-object reads also preserve the local/NFS error contract: `Get`,
`GetRange`, and `Stat` wrap `os.ErrNotExist` for compatible-provider
`404 Not Found` responses while retaining sanitized S3 status context.
Standard S3 `<Error>` response bodies are rendered as bounded one-line
`Code: Message` diagnostics with request-id and host-id context when supplied
instead of raw XML, while non-XML provider bodies fall back to the same
sanitized preview path. XML error previewing is streaming and best-effort, so
truncated provider error bodies can still expose parsed S3 fields without
falling back to raw XML snippets. Each parsed XML error field is capped before
formatting, keeping diagnostics bounded even for oversized provider messages.
When a standard S3 error XML body contains no safe preview fields, gogomail
suppresses the preview instead of exposing raw XML fragments. Standard S3 error
XML that repeats or nests safe preview fields such as `Code`, `Message`,
`RequestId`, or `HostId` is treated the same way, avoiding ambiguous
concatenated diagnostics.
The same embedded-error handling applies to `ListObjectsV2` responses that
arrive as `200 OK` with a top-level standard S3 `<Error>` body, so throttling,
auth, or provider-side list failures cannot be misreported as malformed
pagination metadata.
`PutObject` and `DeleteObject` success responses also reject top-level
standard S3 `<Error>` bodies before reporting completed writes or cleanup, so
compatible-provider throttling, auth, or policy failures cannot cross the
shared storage contract as false success. Apart from whitespace, those success
bodies must otherwise be empty; arbitrary provider text or XML is rejected
instead of being treated as a standard empty success. If a provider includes an
`ETag` header on a successful `PutObject`, gogomail requires that header to be
singular, nonblank, and a bounded safe single-line value; providers that omit
the optional header remain compatible.
S3-compatible `Stat` uses a signed `HEAD` request and returns the canonical
object key, byte size, content type, ETag, and last-modified timestamp when the
provider supplies them. Provider-returned content type and ETag metadata are
bounded to safe single-line UTF-8 values before crossing the adapter boundary;
present blank or malformed content type and ETag metadata fail closed instead
of being silently dropped. Duplicate ETag headers are rejected because object
identity metadata is ambiguous. Duplicate Content-Type headers are also
rejected before MIME metadata is exposed. `Content-Length` is treated as exact
unsigned decimal
metadata, so signed or whitespace-padded values fail closed instead of being
normalized; if both the raw header and normalized HTTP response length are
available, they must agree, and duplicate `Content-Length` headers are
rejected as ambiguous provider metadata. A present blank or malformed
`Last-Modified` header is rejected instead of being silently exposed as a zero
timestamp, while duplicate `Last-Modified` headers are rejected before
timestamp parsing. Missing timestamps and
HTTP optional whitespace around otherwise valid timestamp values remain
compatible.
S3-compatible full-object `GET` validates present `Content-Length` metadata
with the same exact unsigned decimal and duplicate-header rules as `Stat`;
when the response length is known, the returned reader is bounded to that size
and reports `io.ErrUnexpectedEOF` if the provider truncates the body.
S3-compatible `GetRange` uses a signed `GET` request with a single
`Range: bytes=start-end` header and requires a `206 Partial Content` response,
so compatible providers cannot silently downgrade partial reads into full
object transfers. The `Content-Range` header must match the requested byte
window before gogomail exposes the body, and the returned reader is capped at
the validated requested length. This matches local/NFS behavior even if a
provider sends an oversized partial-content body. If a provider returns a
matching `Content-Range` but truncates the response body before the requested
byte count, callers see `io.ErrUnexpectedEOF` instead of a silent short read.
When present, range-response `Content-Length` is parsed with the same exact
unsigned decimal and duplicate-header rules and must match the requested
window.
Both full-object and range readers observe context cancellation after the
request has opened, so canceled downloads, previews, and IMAP literal streams
can stop promptly across local/NFS and S3-compatible backends.
S3-compatible `PutObject`, full-object `GET`, `HEAD`/`Stat`, and
`ListObjectsV2` calls require exact `200 OK` responses; unexpected
partial-content, accepted/deferred writes, or other non-OK 2xx statuses are
rejected instead of being treated as durable writes, whole-object metadata, or
body success. Successful S3-compatible full-object `GET` readers also drain a
small bounded remainder on close, improving HTTP connection reuse for
preview/cancel paths without allowing unbounded cleanup reads.
When a range reader is consumed successfully and closed, gogomail drains a
small bounded response remainder so normal oversized partial responses can
still reuse HTTP connections without exposing extra bytes to callers. The same
bounded drain applies when callers close before consuming the requested range,
helping preview/cancel paths reuse connections without unbounded cleanup reads.
S3-compatible `Copy` uses a signed server-side copy request with an escaped
`x-amz-copy-source`, so AWS S3, MinIO, and strict compatible providers can
duplicate objects without pulling object bytes through gogomail. Successful
copy responses must be exact `200 OK` responses with a bounded
`CopyObjectResult` XML body; empty success bodies, unexpected XML, and embedded
`Error` responses inside `200 OK` are rejected so provider-side copy failures
cannot masquerade as successful object duplication; nested standard S3 error
details are surfaced with the same bounded one-line `Code: Message` and
request-id/host-id diagnostics as top-level provider errors. Top-level and
nested copy error bodies use the same capped streaming XML field parser as
status errors. Success metadata is also kept singular for the core S3 fields:
duplicate top-level `ETag` or
`LastModified` elements and nested `Error` elements under `CopyObjectResult`
are rejected before XML unmarshalling can collapse ambiguous provider metadata.
Unknown top-level success children under `CopyObjectResult` are rejected for
the same reason, keeping copy/move success metadata limited to the canonical
S3 fields gogomail understands.
Core `CopyObjectResult` child elements must also be namespace-free or in the
AWS S3 XML namespace, matching the accepted root namespace boundary. `ETag`
is required in successful copy metadata and uses the same bounded safe
single-line validation as `Stat` and `List`, without XML whitespace padding or
ambiguous quote nesting. Non-empty `LastModified` values in successful copy
metadata must parse as exact S3/RFC-compatible timestamps; blank, malformed, or
whitespace-padded values are rejected instead of being treated as a trustworthy
copy success.
Nested child elements inside simple `CopyObjectResult` metadata fields are
rejected before XML unmarshalling can turn structured provider data into
apparently valid string metadata.
S3-compatible `Move` is intentionally documented as a copy-then-delete
operation because S3 has no native atomic object rename. Callers that need
user-visible Drive/file moves should treat failures after copy as recoverable
duplicate-object cleanup work instead of assuming a single atomic transaction.
When copy succeeds but source deletion fails, the adapter returns a structured
cleanup error that includes the source and destination object paths so callers
can retry deletion or schedule reconciliation without losing the successfully
copied destination identity.
S3-compatible `List` uses signed `ListObjectsV2` requests with validated
prefixes, bounded `max-keys`, opaque continuation tokens, and an exact `200 OK`
status requirement. Successful list responses must decode as bounded
`ListBucketResult` XML from either no XML namespace or the AWS S3 XML
namespace, so unexpected success bodies or same-local-name foreign XML cannot
masquerade as empty object pages. Core control and object-metadata elements
inside the result use the same namespace boundary, so foreign-namespace
pagination, `Contents`, key, size, ETag, or timestamp elements are rejected
before XML unmarshalling can collapse them into provider metadata. Standard S3
list metadata such as `Name`, `Prefix`, `KeyCount`, `MaxKeys`, `StorageClass`,
`ChecksumAlgorithm`, `ChecksumType`, and `Owner` is accepted when namespace-free
or AWS-namespaced, but the same fields from foreign namespaces fail closed.
Simple standard metadata such as `Prefix`, `StorageClass`, and `ChecksumType`
cannot contain nested XML, while structured AWS fields such as `Owner` and
`RestoreStatus` remain compatible. Duplicate single-value `StorageClass`,
`ChecksumType`, `Owner`, and `RestoreStatus` object metadata is rejected,
while repeated `ChecksumAlgorithm` metadata remains compatible for providers
that report multiple checksum algorithms. Returned
keys are normalized back to gogomail object paths under the configured storage
prefix, so callers do not
see deployment-specific bucket prefixes. The mapped gogomail path is then
rechecked against the requested logical prefix, preserving local/NFS
sibling-prefix isolation even if a compatible provider returns an overly broad
page. Size and returned ETag metadata are validated only after that canonical
prefix mapping succeeds; object sizes must be exact unsigned decimal digits,
and present ETags must remain nonblank, unpadded, non-double-quoted, and valid
after the same bounded metadata cleanup as `Stat` instead of being silently
dropped. Present
`LastModified` values must be nonblank and parse as exact S3/RFC-compatible
timestamps; malformed or whitespace-padded values are rejected instead of
being silently exposed as zero timestamps. Per-object `Key`, `Size`, `ETag`, and
`LastModified` elements must be singular, so conflicting duplicate metadata is
rejected before XML unmarshalling can overwrite earlier values. Those simple
metadata fields also reject nested child elements before XML unmarshalling can
turn structured provider data into apparently valid object metadata. Optional
root `KeyCount` and `MaxKeys` metadata may be omitted, but present values must
be exact unsigned decimal digits and cannot be blank. Returned root `Prefix`
and `Name` metadata may be omitted, but present values must be nonblank and
match the requested provider prefix or configured bucket name. Provider
responses that include `StartAfter` or `EncodingType`, even as blank elements,
fail closed, and requester-pays response
headers are rejected across success paths, because the adapter does not request
start-after pagination, encoded-key mode, or requester-pays mode. Returned root
`ContinuationToken` echoes must match an explicitly requested cursor and are
rejected when no request cursor was sent. Delimiter grouping is likewise
unsupported: returned `Delimiter` elements, including blank elements, or
`CommonPrefixes` responses are rejected instead of being treated as ordinary
object pages. Responses that return more
matching objects than requested are rejected,
keeping local/NFS and
S3-compatible pagination semantics aligned. Returned keys containing encoded
separators or leading/trailing whitespace fail closed once they map inside the
configured storage prefix, preserving the same portable key-shape rule used for
request paths. `ListObjectsV2` query
parameters are encoded with SigV4 canonical URI rules instead of form-style
query escaping, so prefixes and continuation tokens containing spaces, literal
`+`, `/`, `=`, or `@` characters sign and round-trip consistently across AWS
S3, MinIO, and stricter compatible providers. Top-level pagination controls
must be singular: duplicate `IsTruncated` or `NextContinuationToken` elements
are rejected before XML unmarshalling can collapse ambiguous final/truncated
state or cursor identity. Continuation cursors are treated as opaque identity
tokens: blank cursors are allowed, but nonblank cursors with leading/trailing
whitespace or control characters are rejected instead of trimmed, so provider
pagination tokens are not silently changed between pages or made unsafe for
logs and cleanup cursors.
Prefix cleanup over S3-compatible storage intentionally remains page-based:
callers list a bounded page, delete each canonical object key through signed
`DELETE` requests, and continue from the returned cursor. This keeps cleanup
portable across AWS S3, MinIO, and stricter compatible stores without relying
on provider-specific recursive delete behavior.
S3 `PUT`, failed `GET`, successful `GET` close, and `DELETE` responses drain a
small bounded body window before close so normal S3/MinIO responses can reuse
HTTP connections without letting oversized responses stall cleanup. Local/NFS
and S3 readiness probes read only the expected probe body size plus one byte
before comparing the response.

## Integration verification

Optional S3-compatible integration coverage runs a real `PUT`/`GET`/`DELETE`
round trip when these variables are present:

```sh
GOGOMAIL_TEST_S3_ENDPOINT=http://localhost:19000
GOGOMAIL_TEST_S3_BUCKET=gogomail
GOGOMAIL_TEST_S3_ACCESS_KEY_ID=gogomail
GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY=gogomail123
go test ./internal/storage
```

For AWS S3 virtual-hosted testing, also set the region and explicitly disable
path-style requests:

```sh
GOGOMAIL_TEST_S3_REGION=us-east-1
GOGOMAIL_TEST_S3_FORCE_PATH_STYLE=false
```

For private CA or local self-signed test endpoints, use the test-specific TLS
variables so integration coverage exercises the same custom-client path as
runtime storage:

```sh
GOGOMAIL_TEST_S3_CA_CERT_FILE=/etc/gogomail/s3-ca.pem
GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY=false
```
