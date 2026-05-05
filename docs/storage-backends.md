# gogomail storage backends

gogomail stores raw `.eml` objects, attachments, exports, and readiness probes
through the shared storage interface. Deployments can switch backends by
configuration without changing stored object keys.

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
S3 `ListObjectsV2` requests. Future Drive and lifecycle modules should prefer
`Stat` for existence/size checks, `GetRange` for resumable downloads and media
preview windows, `Copy` for object duplication workflows, `Move` for file
rename/relocation workflows, and `List` for prefix-scoped browsing,
reconciliation, and cleanup scans.
`storage.DeletePrefix` composes `List` and idempotent `Delete` into a bounded
page-level cleanup helper for future Drive folder deletion, attachment
lifecycle, and reconciliation jobs without requiring callers to know whether
the backend is local/NFS or S3-compatible storage.

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
`GetRange` seeks to the requested byte offset and returns a closeable limited
reader for the requested positive byte count, keeping partial reads efficient
for local disk and NFS-style mounts.
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
S3-compatible `Stat` uses a signed `HEAD` request and returns the canonical
object key, byte size, content type, ETag, and last-modified timestamp when the
provider supplies them.
S3-compatible `GetRange` uses a signed `GET` request with a single
`Range: bytes=start-end` header and requires a `206 Partial Content` response,
so compatible providers cannot silently downgrade partial reads into full
object transfers. The `Content-Range` header must match the requested byte
window before gogomail exposes the body, and the returned reader is capped at
the validated requested length. This matches local/NFS behavior even if a
provider sends an oversized partial-content body. If a provider returns a
matching `Content-Range` but truncates the response body before the requested
byte count, callers see `io.ErrUnexpectedEOF` instead of a silent short read.
When a range reader is consumed successfully and closed, gogomail drains a
small bounded response remainder so normal oversized partial responses can
still reuse HTTP connections without exposing extra bytes to callers.
S3-compatible `Copy` uses a signed server-side copy request with an escaped
`x-amz-copy-source`, so AWS S3, MinIO, and strict compatible providers can
duplicate objects without pulling object bytes through gogomail.
S3-compatible `Move` is intentionally documented as a copy-then-delete
operation because S3 has no native atomic object rename. Callers that need
user-visible Drive/file moves should treat failures after copy as recoverable
duplicate-object cleanup work instead of assuming a single atomic transaction.
S3-compatible `List` uses signed `ListObjectsV2` requests with validated
prefixes, bounded `max-keys`, and opaque continuation tokens. Returned keys are
normalized back to gogomail object paths under the configured storage prefix,
so callers do not see deployment-specific bucket prefixes.
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
