# gogomail storage backends

gogomail stores raw `.eml` objects, attachments, exports, and readiness probes
through the shared storage interface. Deployments can switch backends by
configuration without changing stored object keys.

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
Deletes are idempotent for missing objects, matching S3-style cleanup behavior
so lifecycle workers behave consistently across storage backends.

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
of surfacing later as opaque S3 authentication failures.

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
AWS S3, MinIO, and strict compatible providers.
For file-backed or otherwise seekable upload bodies, gogomail sets a precise
`Content-Length` without buffering the object in memory, improving PUT
compatibility while preserving streaming-first storage paths.
Deletes are idempotent for missing objects, including `404 Not Found` responses
from compatible providers, so lifecycle cleanup behaves consistently across
AWS S3, MinIO-style endpoints, and local/NFS storage.
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
