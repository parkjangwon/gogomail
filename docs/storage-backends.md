# gogomail storage backends

gogomail stores raw `.eml` objects, attachments, exports, and readiness probes
through the shared storage interface. Deployments can switch backends by
configuration without changing stored object keys.
Object paths, prefixes, and list cursors are validated as bounded valid UTF-8
text before adapter use, keeping local/NFS paths, S3 URLs, SigV4 canonical
paths, logs, and cleanup cursors unambiguous across backends.

The shared interface supports `Put`, `Get`, `GetRange`, `Stat`, `Copy`,
`Move`, `List`, and `Delete`.
`Stat` returns object size and optional backend metadata without streaming the
object body, using filesystem metadata on local/NFS storage and signed `HEAD`
requests on S3-compatible storage. `GetRange` opens a validated bounded byte
range without requiring callers to stream and discard a prefix, using
filesystem seek/limited reads locally and signed S3 `Range` requests remotely.
`Copy` preserves object keys and adapter semantics while avoiding caller-side
read/write loops when the backend can copy server-side. `Move` gives callers
one backend-neutral object relocation contract: local/NFS uses filesystem
rename semantics, while S3-compatible storage performs server-side copy
followed by source delete. `List` returns bounded, cursor-paginated object
metadata under a validated prefix, using a local/NFS directory walk or signed
S3 `ListObjectsV2` requests; truncated S3 pages must include a continuation
token before callers see the page, and provider-returned keys are not trimmed
before prefix/object-path validation. Provider responses must also respect the
requested page size for matching objects, so a malformed S3-compatible service
cannot bypass the shared bounded-list contract. Future Drive and lifecycle modules should
prefer `Stat` for existence/size checks, `GetRange` for resumable downloads
and media preview windows, `Copy` for object duplication workflows, `Move` for
file rename/relocation workflows, and `List` for prefix-scoped browsing,
reconciliation, and cleanup scans.
`storage.DeletePrefix` composes `List` and idempotent `Delete` into a bounded
page-level cleanup helper for future Drive folder deletion, attachment
lifecycle, and reconciliation jobs without requiring callers to know whether
the backend is local/NFS or S3-compatible storage.

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
- `List` returns only keys under the requested canonical prefix and hides any
  deployment-specific S3 bucket prefix from callers.
- `Copy` preserves bytes at a new key; `Move` relocates through the backend's
  documented semantics while removing the old key.
- `Delete` is idempotent for already-missing objects.
- `storage.DeletePrefix` removes a bounded page of remaining objects under the
  prefix without touching sibling prefixes.

`TestLocalStorePortabilityContract` always runs this contract for local/NFS
semantics. `TestS3StoreIntegrationRoundTrip` reuses the same contract when
`GOGOMAIL_TEST_S3_*` variables are set, giving MinIO and AWS S3 deployments a
single smoke test before a storage backend flip.

## Local filesystem or NFS

Local storage is the default and can point at a normal disk path or an
NFS-style mounted directory.

```sh
GOGOMAIL_STORAGE_BACKEND=local
GOGOMAIL_STORAGE_ROOT=./data/storage
```

Use local storage for development, single-node installs, or deployments where
the mount itself provides durability and availability.
`GOGOMAIL_STORAGE_ROOT` is the storage-focused alias for
`GOGOMAIL_MAILSTORE_ROOT`; if both are set, `GOGOMAIL_MAILSTORE_ROOT` wins for
backward compatibility. The effective root must be non-empty, bounded, and free
of line breaks when the local backend is active.
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
reads, range reads, metadata probes, and source moves reject them, and list
operations hide them. This keeps mounted filesystems from escaping the
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
loops.
`List` walks under the requested canonical prefix, returns bounded pages, and
uses an opaque cursor so callers do not depend on filesystem traversal details.
Prefix cleanup uses the same bounded list pages and idempotent deletes, so
large local/NFS cleanup jobs can advance with explicit cursors instead of
walking and deleting an unbounded tree in one operation.

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

## AWS S3 or compatible object storage

Use the `s3` backend for AWS S3 or S3-compatible services that accept SigV4.
Bucket names follow AWS general purpose bucket naming rules: gogomail rejects
IP-address-shaped names plus reserved AWS prefixes and suffixes during config
validation, before adapter construction.

```sh
GOGOMAIL_STORAGE_BACKEND=s3
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

Set `GOGOMAIL_STORAGE_S3_ENDPOINT` for non-AWS compatible services. Endpoints
must be plain HTTP(S) origins with an optional canonical base path; userinfo,
query strings, fragments, duplicate separators, dot segments, and encoded path
separators such as `%2F` or `%5C` are rejected so SigV4 signing and object
addressing stay unambiguous. Set
`GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` when the provider or local network
does not support virtual-hosted bucket names. HTTPS endpoints automatically use
path-style requests for bucket names that contain periods, matching AWS's
certificate compatibility guidance for dotted bucket names. Localhost and
IP-address endpoints also use path-style requests automatically, so local
MinIO or other local compatible stores do not accidentally receive
`bucket.localhost` or `bucket.127.0.0.1` style hosts when the generic `s3`
backend is used.
Object keys are path-escaped segment by segment so literal `+` characters stay
encoded as `%2B`, preserving object identity and SigV4 canonical paths across
AWS S3, MinIO, and strict compatible providers. Endpoint base paths use the
same segment escaping, so reverse-proxy paths such as `/base+proxy` keep their
literal plus signs in canonical request paths.
For file-backed or otherwise seekable upload bodies, gogomail sets a precise
`Content-Length` without buffering the object in memory, improving PUT
compatibility while preserving streaming-first storage paths.
Deletes are idempotent for missing objects, including `404 Not Found` responses
from compatible providers, so lifecycle cleanup behaves consistently across
AWS S3, MinIO-style endpoints, and local/NFS storage.
Missing-object reads also preserve the local/NFS error contract: `Get`,
`GetRange`, and `Stat` wrap `os.ErrNotExist` for compatible-provider
`404 Not Found` responses while retaining sanitized S3 status context.
S3-compatible `Stat` uses a signed `HEAD` request and returns the canonical
object key, byte size, content type, ETag, and last-modified timestamp when the
provider supplies them. Provider-returned content type and ETag metadata are
bounded to safe single-line UTF-8 values before crossing the adapter boundary;
malformed metadata is dropped while object identity and size remain available.
S3-compatible `GetRange` uses a signed `GET` request with a single
`Range: bytes=start-end` header and requires a `206 Partial Content` response,
so compatible providers cannot silently downgrade partial reads into full
object transfers. The `Content-Range` header must match the requested byte
window before gogomail exposes the body, and the returned reader is capped at
the validated requested length. This matches local/NFS behavior even if a
provider sends an oversized partial-content body. If a provider returns a
matching `Content-Range` but truncates the response body before the requested
byte count, callers see `io.ErrUnexpectedEOF` instead of a silent short read.
Both full-object and range readers observe context cancellation after the
request has opened, so canceled downloads, previews, and IMAP literal streams
can stop promptly across local/NFS and S3-compatible backends.
When a range reader is consumed successfully and closed, gogomail drains a
small bounded response remainder so normal oversized partial responses can
still reuse HTTP connections without exposing extra bytes to callers. The same
bounded drain applies when callers close before consuming the requested range,
helping preview/cancel paths reuse connections without unbounded cleanup reads.
S3-compatible `Copy` uses a signed server-side copy request with an escaped
`x-amz-copy-source`, so AWS S3, MinIO, and strict compatible providers can
duplicate objects without pulling object bytes through gogomail. Successful
copy responses are read through a bounded parser: normal `CopyObjectResult`
responses are accepted, while embedded `Error` responses inside `200 OK` are
rejected so provider-side copy failures cannot masquerade as successful object
duplication.
S3-compatible `Move` is intentionally documented as a copy-then-delete
operation because S3 has no native atomic object rename. Callers that need
user-visible Drive/file moves should treat failures after copy as recoverable
duplicate-object cleanup work instead of assuming a single atomic transaction.
S3-compatible `List` uses signed `ListObjectsV2` requests with validated
prefixes, bounded `max-keys`, and opaque continuation tokens. Returned keys are
normalized back to gogomail object paths under the configured storage prefix,
so callers do not see deployment-specific bucket prefixes. Returned ETags use
the same bounded metadata cleanup as `Stat`. Provider responses that return
more matching objects than requested are rejected, keeping local/NFS and
S3-compatible pagination semantics aligned.
Prefix cleanup over S3-compatible storage intentionally remains page-based:
callers list a bounded page, delete each canonical object key through signed
`DELETE` requests, and continue from the returned cursor. This keeps cleanup
portable across AWS S3, MinIO, and stricter compatible stores without relying
on provider-specific recursive delete behavior.
S3 `PUT`, failed `GET`, and `DELETE` responses drain a small bounded body
window before close so normal S3/MinIO responses can reuse HTTP connections
without letting oversized responses stall cleanup. Local/NFS and S3 readiness
probes read only the expected probe body size plus one byte before comparing
the response.

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
