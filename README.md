# gogomail

<img width="1456" height="720" alt="1777874812592" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go-first backend implementation for the gogomail webmail/mail-server platform:
a standards-first, backend-contract-first mail platform designed for RFC
correctness, high-throughput operation, and generated-client friendly APIs.

gogomail is intentionally organized for agent-assisted development. The repo
carries current status, roadmap, ADRs, OpenAPI contracts, release gates, and
change checklists so future agents can resume work without rediscovering the
same product direction. Treat those files as part of the product surface: code,
tests, contracts, and docs should move together.

## Current scope

This repository is currently in the backend platform hardening phase. The goal
is a production mail platform, not a demo server: protocol capabilities should
be advertised only when runtime behavior, tests, and client compatibility are
ready to back them.

The SMTP engine is materially advanced, and current work is broadening into
tenant/domain operations, Admin API, Mail API contracts, delivery routing,
DNS/DKIM onboarding, quota/policy enforcement, OpenAPI drift prevention, IMAP
client-compatibility hardening, DAV interoperability, and storage portability
across local, NFS, MinIO, and AWS/S3-compatible deployments.

## Backend capability tiers

Use these tiers when deciding whether to expose a surface to operators,
generated clients, or native protocol clients:

| Surface | Current tier | Notes |
| --- | --- | --- |
| SMTP receive/delivery | Advanced backend | Standards-first core with pluggable policy, storage, queue, and notification boundaries. Keep new extensions gated until runtime semantics and RFC-shaped tests exist. |
| Mail API | Release-readiness workbench | User-scoped webmail backend contracts are broad and tested, but frontend client release should still follow OpenAPI/runtime drift checks. |
| Admin API | Release-readiness workbench | Operator console contracts exist under `/admin/v1` with no-store authenticated JSON responses and generated-client base-path pins. |
| IMAP | Service-backed, gated | The gateway is real and increasingly strict, including bounded ENVELOPE/BODY metadata rendering, but public client readiness remains gated by RFC syntax/state/literal/MODSEQ/UID compatibility coverage. |
| CalDAV/CardDAV | Backend-only experimental | Native DAV interoperability is progressing with real runtime modes, repository boundaries, sync tokens, validation, and conditional guards, but discovery/client compatibility remains intentionally gated. |
| Drive | Backend groundwork | Storage/quota-backed metadata, object I/O, range download, cleanup, and public share-link APIs are present without starting frontend implementation. |
| Frontend apps | Planned | Next.js TypeScript, shadcn/ui, `DESIGN.md`, and a Notion Mail-like product feel are planned after the frontend start gate is explicitly opened. |

## Engineering posture

gogomail is being built as a standards-first platform with small replaceable
boundaries rather than one large product service. Protocol engines should stay
RFC-correct, streaming-aware, and conservative about advertised capabilities;
product features such as spam filtering, push delivery, indexing, retention,
and audit should attach through explicit service, worker, event, or adapter
boundaries.

High-level design rules:

- keep SMTP, IMAP, CalDAV, CardDAV, storage, search, notification, and HTTP API
  contracts independently testable
- prefer streaming readers and bounded scans on mail, attachment, Drive, and
  DAV hot paths
- expose capabilities only when the runtime, OpenAPI/docs, and regression tests
  agree
- preserve tenant/user/domain isolation before repository, storage, queue, and
  event fan-out work begins
- keep frontend implementation gated until backend contracts are stable and the
  explicit frontend start signal is given
- make release claims only after the implementation, regression tests,
  generated-client contracts, and operator docs describe the same behavior

Runtime concurrency is Go-native. Network listeners, protocol sessions,
workers, event consumers, notification fan-out, cleanup loops, and mailbox
brokers are designed around goroutines, channels, contexts, bounded batch
sizes, timeouts, and explicit cancellation. The goal is not merely "parallel by
default"; hot paths should stay observable, backpressure-aware, and safe to
split across dedicated modes as deployments grow.

Recent release-readiness work also includes:

- Mail API readiness for production webmail chrome, including mailbox
  overview, folder/message/thread lists, previews, bounded bulk actions,
  restore flows, attachment access, draft/send flows, and capability discovery
  that exposes only runtime-backed filters and limits
- Admin API readiness for future operator consoles, including capability
  bootstrap under `/admin/v1`, tenant/domain/user operations, queue/outbox and
  delivery triage, audit/backpressure/quota/DKIM/API-usage surfaces, strict
  request-boundary validation, and OpenAPI/runtime drift tests
- storage portability across local filesystem, explicit NFS mounts, local
  MinIO, and AWS/S3-compatible object storage through the shared storage
  interface, validated `configs/storage.*.yaml` profiles, and extensible
  compatibility labels that keep support claims conservative, with local/NFS
  symlink parent rejection preserving object-root boundaries on mounted
  filesystems. The S3-compatible adapter now treats provider metadata as a
  strict contract: list pages recheck returned keys against the requested
  logical prefix, object sizes and full/range content lengths use exact unsigned
  decimal grammar without whitespace padding, full-object `GET` rejects
  contradictory known zero-length `Content-Length` metadata, offset-zero
  `200 OK` range compatibility also rejects contradictory known zero-length
  `Content-Length` metadata, duplicate `Content-Length` and range
  `Content-Range` headers fail closed,
  `HEAD`/`Stat` rejects malformed or contradictory
  `Content-Length` metadata, blank or malformed present `Last-Modified`,
  unpadded `ETag`, and unpadded RFC-shaped ASCII `Content-Type` metadata
  including duplicate `Last-Modified`, `ETag`, and `Content-Type` headers, truncated
  pages require canonical continuation state, malformed list entries fail
  closed, malformed quoted, double-quoted, whitespace-padded, non-ASCII, or
  otherwise non-printable S3 ETags fail closed across headers and XML success
  metadata, and
  `ListObjectsV2`/`CopyObject` success XML accepts only namespace-free or AWS
  S3 namespaces plus required bounded copy ETags, timestamp metadata, and
  known core/standard list metadata elements, rejecting foreign-namespace
  standard list metadata, duplicate simple root metadata, inconsistent
  `KeyCount`, present-but-blank `KeyCount`/`MaxKeys`, malformed or
  under-counting `MaxKeys`, mismatched returned
  nonblank exact `Prefix` and bucket `Name` echoes when present, any returned
  `EncodingType`, missing-request or mismatched returned `ContinuationToken`
  echoes when present, and any returned `StartAfter`, requester-pays response
  headers across S3 adapter success paths, including blank or whitespace-only
  values, any delimiter/grouping controls, and duplicate single-value object
  metadata such as `StorageClass`, `ChecksumType`,
  `Owner`, and `RestoreStatus`, direct text inside structured `Owner` or
  `RestoreStatus` metadata, plus nested XML inside simple core/standard
  provider metadata fields, while standard S3 error XML,
  including duplicate, nested, foreign-namespace,
  or otherwise ambiguous safe error fields, and
  `200 OK` `ListObjectsV2`, successful `PutObject`/`DeleteObject`, and nested
  `CopyObjectResult` embedded error bodies, is streamed into bounded one-line
  diagnostics with request-id and host-id context when supplied, even when the
  provider body is truncated. Successful `PutObject`/`DeleteObject` bodies must
  otherwise be empty apart from whitespace. Optional `PutObject` success `ETag`
  headers must also be singular, nonblank, unpadded, printable, and bounded
  when supplied, so provider write identity metadata cannot depend on header
  collapse or malformed values.
- service-backed IMAP hardening, including UIDPLUS `COPYUID`/`APPENDUID`
  behavior, `UIDNOTSTICKY` handling, sparse `UID EXPUNGE`, RFC 5258
  `LIST-EXTENDED`/RFC 5819 `LIST-STATUS` capability alignment, LIST/LSUB
  namespace compatibility, RFC 5258 `SUBSCRIBED` selection, pattern-list, and
  quoted/printable-literal pattern-list support, return-option support,
  SEARCHRES `$` reuse across SEARCH/SORT/THREAD workflows, exact `$` atom handling,
  selected-mailbox lifecycle cleanup for saved SEARCHRES state,
  strict STATUS item-list framing with LIST-STATUS compatibility coverage,
  strict LIST selection option-list framing,
  strict LIST RETURN option-list framing,
  strict SEARCH/SORT/THREAD RETURN option-list framing,
  strict CONDSTORE select-param framing, CONDSTORE/MODSEQ-shaped behavior,
  syntax-before-state validation, strict 7-bit atom/quoted-string parsing and
  response quoting, atom-only command tag handling,
  exact SASL continuation cancellation, exact SASL PLAIN response token
  handling, atom-only AUTHENTICATE mechanism/SASL-IR controls,
  SASL-IR syntax-before-policy diagnostics, empty quoted `LOGIN`
  passwords routed to backend authentication failures instead of protocol
  `BAD`, strict 7-bit atom-only command and UID subcommand dispatch, strict
  SEARCH/SORT/THREAD charset and raw thread-algorithm atoms,
  strict raw SORT criterion-list framing, strict FETCH data-item framing,
  atom-only FETCH data-item controls, atom-only ENABLE capability controls,
  SEARCH/SORT/THREAD diagnostics, RFC-shaped sequence-set and
  STORE control-atom checks, raw parenthesized flag/status/select-param list
  controls, raw LIST selection/return option-list controls,
  raw SEARCH/SORT/THREAD return-option controls,
  numeric grammar checks including rejection of quoted/literal-framed set
  values in selected-state FETCH/STORE/COPY/MOVE/UID mutation commands and
  SEARCH/SORT/THREAD set operands,
  partial windows, bounded SEARCH size criteria,
  atom-only search numeric operands, atom-only search charset/date/keyword
  controls,
  RFC-shaped `HEADER.FIELDS` field-list grammar, CONDSTORE zero-boundary
  semantics, BODY/BODYSTRUCTURE MIME token sanitization, and RFC 2971 `ID`
  including bare no-argument probes, `NIL`, and
  bounded field/value parameter lists, rejection of whitespace-padded numeric
  and date search criteria,
  rejection of whitespace-padded CONDSTORE modifier and MODSEQ entry-type
  values, exact `IDLE` `DONE` continuation handling, exact STORE mode and
  `UNCHANGEDSINCE` marker handling, strict APPEND/STORE flag-list framing with
  duplicate flag rejection, canonical duplicate-free `SELECT` permanent flag
  responses, per-message `RECENT`/`NEW`/`OLD` search semantics,
  custom IMAP keyword flags with PostgreSQL `maildb` persistence through the
  IMAP-specific `imap_keywords` JSONB flag array,
  rejection of whitespace-bearing sequence-set range components,
  strict KEYWORD/UNKEYWORD atom validation,
  case-insensitive standard SORT criteria while preserving strict criterion-list
  framing,
  selected-state event draining before sequence-set commands including
  UID-addressed workflows,
  quoted, escaped, and literal-framed destination mailbox names for COPY/MOVE
  backend mutations, explicit rejection of empty required mailbox targets for
  SELECT/EXAMINE, STATUS, mailbox mutation/subscription commands, COPY/MOVE,
  UID COPY/MOVE, and APPEND while preserving empty LIST/LSUB root/pattern
  semantics, exact `INBOX` special-name checks that do not trim quoted
  mailbox names containing real leading or trailing spaces, and subscription
  canonicalization that preserves those spaces through service/repository
  `SUBSCRIBE`/`UNSUBSCRIBE` delegation,
  bounded and UTF-8-safe ENVELOPE/BODY metadata rendering, capped ENVELOPE
  address lists, and dropping malformed empty or incomplete address
  placeholders before they can render as stray address tuples,
  non-blocking
  mailbox event delivery under concurrent subscription cancellation, and
  cumulative command-literal memory caps plus literal framing coverage and
  strict CRLF command, SASL continuation, and IDLE continuation line framing
- backend-only CalDAV foundations for standards-first calendar
  interoperability, with real gateway/runtime mode work, Basic-auth protected
  DAV surfaces, PROPFIND/REPORT/object I/O, sync-token discovery, iCalendar
  validation, sync report discovery gated by runtime change-log support,
  WebDAV conditional mutation guards that recheck observed strong ETags even
  for `If-Match: *`, fail closed on missing-resource
  `If-Unmodified-Since` writes, and reject matching `If-None-Match` validators
  before object or collection deletion, while `COPY`/`MOVE` stay unadvertised
  until calendar relocation and duplication semantics exist, plus
  Directory/Identity, delegation, Notification & Sync, search, policy, and
  audit boundaries treated as platform prerequisites
- backend-only CardDAV foundations for contacts interoperability, with
  address-book principal/object boundaries, vCard validation, sync discovery,
  RFC-shaped query filtering, sync report discovery gated by runtime
  change-log support, WebDAV conditional mutation guards that recheck observed
  strong ETags even for `If-Match: *`, reject missing-resource
  `If-Unmodified-Since` writes, and honor matching `If-None-Match` validators
  before object or collection deletion. `OPTIONS` and unsupported-method
  `Allow` responses advertise only implemented handlers; future WebDAV
  `COPY`/`MOVE` names are constants for later work but remain unadvertised
  until address-book relocation and duplication semantics exist. Native-client
  compatibility gates stay separate from future UI work
- Drive backend groundwork and APIs that reuse the shared storage/quota
  contract for metadata, upload/finalize, rename/move, delete, range download,
  public share-link metadata/download with exact bearer-token path semantics,
  and cleanup readiness without starting frontend implementation
- OpenAPI drift prevention for generated clients, including root-vs-API server
  pins for health/service metadata, `/admin/v1` pins for operator bootstrap
  routes, `/api/v1` pins for registered Mail/Drive routes, readiness checks,
  API usage ledger/aggregate/export surfaces,
  sensitive export artifact and manifest proof routes, core operator
  diagnostics/repair calls, public share-link unauthenticated route contracts,
  and documented admin auth alternatives for generated clients

The Next.js web apps will be added after the backend contracts stabilize and
after the user provides frontend-specific guidance. Planned frontend surfaces
include webmail, Drive UI, calendar UI, contacts UI, admin console, and shared
inbox UI.

When frontend implementation starts, use Next.js with TypeScript, shadcn/ui,
`DESIGN.md`, and a Notion Mail-like product feel. Do not create substantial
frontend apps or screens before that frontend-specific start gate is opened.

## Release-readiness map

Use this repository as a backend-contract workbench before treating any client
surface as public. The short path for new contributors and coding agents is:

- `docs/CURRENT_STATUS.md` for what is actually implemented and hardened now
- `docs/NEXT_STEPS.md` for the next release-oriented backend priorities
- `docs/backend-release-readiness.md` for the first webmail-focused release
  gates
- `docs/backend-api-contracts.md` and `docs/openapi.yaml` for generated-client
  contracts
- `docs/storage-backends.md` for local, NFS, MinIO, AWS S3, and compatible S3
  storage behavior
- `docs/adr/` for architectural boundaries that should not be rediscovered in
  every agent session

Treat a capability as release-ready only when runtime behavior, regression
tests, docs/OpenAPI contracts, and operator guidance all describe the same
thing. Partial protocol work should stay explicitly gated or experimental,
especially for IMAP, CalDAV, CardDAV, and frontend-facing APIs.

## Agent handoff

Future coding agents should read these files before changing code:

- `AGENTS.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/CHANGE_CHECKLIST.md`
- `docs/backend-roadmap.md`
- `docs/backend-api-contracts.md`
- `docs/backend-release-readiness.md`
- `docs/openapi.yaml`

Key architecture decisions:

- `docs/adr/0001-domain-is-tenant.md`
- `docs/adr/0002-smtp-core-is-not-spam-engine.md`
- `docs/adr/0003-company-domain-user-quota-pool.md`
- `docs/adr/0004-api-metering-aggregation-boundary.md`
- `docs/adr/0005-imap-gateway-boundary.md`
- `docs/adr/0006-imap-uid-storage.md`
- `docs/adr/0007-resumable-attachment-upload-boundary.md`
- `docs/adr/0008-imap-auth-session-semantics.md`
- `docs/adr/0009-drive-module-boundary.md`
- `docs/adr/0010-caldav-gateway-boundary.md`
- `docs/adr/0011-directory-principal-boundary.md`
- `docs/adr/0012-carddav-gateway-boundary.md`

Guardrails:

- Implemented SMTP features must follow the relevant email RFCs.
- Do not advertise SMTP extensions before end-to-end semantics are implemented.
- Do not build a spam engine into SMTP core.
- Do not start frontend implementation without user guidance.
- Commit feature-sized changes and push after successful verification.

Agent work protocol:

1. Rebuild context from the handoff documents and recent git history.
2. Make feature-sized, reviewable changes.
3. Update status/roadmap/API/ADR documents when the change affects them.
4. Run the verification commands.
5. Commit and push to `origin/main`.
6. Report what changed, what passed, and whether the push completed.

Meaningful autonomous work should generally land as one reviewable release
unit at a time: implement the behavior, add RFC-shaped or boundary-focused
tests, update the relevant handoff docs, run the release verification script,
commit with a conventional message, and push to `origin/main`.

## Backend modes

```bash
go run ./cmd/gogomail --mode=all-in-one
go run ./cmd/gogomail --mode=edge-mta
go run ./cmd/gogomail --mode=inbound-mta
go run ./cmd/gogomail --mode=outbound-mta
go run ./cmd/gogomail --mode=delivery-worker
go run ./cmd/gogomail --mode=attachment-cleanup-worker
go run ./cmd/gogomail --mode=drive-cleanup-worker
go run ./cmd/gogomail --mode=dav-sync-retention-worker
go run ./cmd/gogomail --mode=search-index-worker
go run ./cmd/gogomail --mode=api-metering-worker
go run ./cmd/gogomail --mode=api-usage-retention-worker
go run ./cmd/gogomail --mode=push-notification-worker
go run ./cmd/gogomail --mode=outbox-relay
go run ./cmd/gogomail --mode=event-worker
go run ./cmd/gogomail --mode=batch-worker
go run ./cmd/gogomail --mode=imap
go run ./cmd/gogomail --mode=caldav
go run ./cmd/gogomail --mode=carddav
go run ./cmd/gogomail --mode=auth-server
go run ./cmd/gogomail --mode=mail-api
go run ./cmd/gogomail --mode=admin-api
```

`imap` starts the service-backed IMAP gateway with the TCP listener, protocol
auth adapter, mailbox event broker, and live `mail.stored` notification
consumer for IDLE/NOOP update delivery. The gateway advertises only the IMAP
extensions currently backed by tests and service semantics; continue treating
RFC correctness and client compatibility as release gates for every advertised
capability. IMAP is intentionally service-backed and advanced, but public
client readiness remains gated by RFC-shaped syntax, state, UID, MODSEQ, and
literal/framing regressions. FETCH rendering also keeps backend metadata
bounded before protocol quoting so malformed MIME, oversized envelope strings,
or placeholder address rows cannot leak into client-visible responses as
ambiguous IMAP syntax. `GOGOMAIL_IMAP_MAX_CONNECTIONS` can set a
process-local concurrent session cap; `0` keeps the listener unlimited for
development or externally limited deployments, while capped deployments reject
excess clients with an initial IMAP `BYE [ALERT]` response.

SMTP receive and submission listeners can also be bounded with
`GOGOMAIL_SMTP_MAX_CONNECTIONS` and `GOGOMAIL_SUBMISSION_MAX_CONNECTIONS`.
Positive values hold one slot per active SMTP session and reject overflow
clients with an RFC-shaped transient `421 4.3.2` banner before closing; `0`
keeps the default unlimited listener for small or externally fronted
deployments.

`all-in-one` serves health, Mail API, and Admin API routes from one HTTP
process for small deployments and local release smoke tests.

`mail-api` serves the user-scoped HTTP backend for future webmail and Drive UI
clients under `/api/v1`. The API contract is documented in
`docs/openapi.yaml`; handlers and the OpenAPI draft should move together so
generated clients do not learn unsupported routes, filters, limits, or response
envelopes.

`admin-api` serves operator and administrator surfaces under `/admin/v1`,
including console capability discovery, tenant/domain/user control,
operational triage, retention/export readiness, storage capability reporting,
and no-store authenticated JSON responses. Admin routes document the accepted
`X-Admin-Token` and bearer-token alternatives where applicable and reject
ambiguous mixed credentials at the shared auth boundary.

`caldav` and `carddav` start the backend-only DAV gateways for calendar and
contacts interoperability. They are real protocol modules, but they should
remain release-gated until native-client behavior, principal resolution,
delegation, sync, policy, audit, and storage semantics are proven together.
Do not advertise DAV methods such as `COPY` or `MOVE` before their full
WebDAV/CardDAV object semantics are implemented and covered by compatibility
tests.

`push-notification-worker` stays disabled until
`GOGOMAIL_PUSH_NOTIFICATION_BACKEND=slog` or
`GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook` is set. The worker consumes
`mail.stored`, resolves active user devices from PostgreSQL, and writes
candidate attempt rows before sink handoff. The `slog` backend logs bounded
notification candidates. The `webhook` backend POSTs raw-token targets and
attempt IDs to an external push gateway. First-party FCM/APNs/Web Push delivery
adapters are intentionally not enabled by default.

`attachment-cleanup-worker` periodically expires stale `uploading` attachment
rows and deletes the corresponding stored objects through the configured local
mailstore. Configure it with:

```bash
GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL=1h
GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE=24h
GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE=100
GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE=false
```

`api-usage-retention-worker` runs the same bounded API usage ledger retention
path exposed by the Admin API. It is dry-run by default and reuses the retention
readiness gate before any destructive delete. Configure it with:

```bash
GOGOMAIL_API_USAGE_RETENTION_INTERVAL=24h
GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE=2160h
GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE=1000
GOGOMAIL_API_USAGE_RETENTION_RUN_ONCE=false
GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=true
GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY=false
GOGOMAIL_API_USAGE_RETENTION_TENANT_ID=
GOGOMAIL_API_USAGE_RETENTION_PRINCIPAL_ID=
```

Set `GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=false` only after export storage and
signing policy are production-ready; validation requires
`GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY=true` and a configured
`remote-ed25519` export manifest signer for destructive runs.

`dav-sync-retention-worker` prunes old CalDAV/CardDAV sync-change rows while
preserving the newest marker per calendar/address book, so current clients keep
their sync-token continuity. It is dry-run by default:

```bash
GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL=24h
GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE=2160h
GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE=1000
GOGOMAIL_DAV_SYNC_RETENTION_RUN_ONCE=false
GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN=true
GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY=false
```

Set `GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN=false` only after choosing the
deployment token-retention policy; validation requires
`GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY=true` for destructive runs.

Webhook push handoff:

```bash
GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL=https://push-gateway.example/send
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT=2s
```

`GOGOMAIL_ENV` must be `development`, `test`, or `production`; unknown values
are rejected so typos cannot bypass production-only safeguards. Production
webhook URLs must use HTTPS. Local development and private test harnesses may
use HTTP.

See `docs/webhook-integrations.md` for the push gateway JSON payload, attempt
state semantics, and authentication contract.

## Verify

```bash
go test ./...
```

Release-oriented local verification:

```bash
scripts/verify-backend-release.sh
```

The release script runs `go test ./...`, `go mod tidy -diff`, optional
database-gated checks when configured, and a clean-worktree check. For narrow
changes, run the closest package test first, then the release script before
committing and pushing.

For storage portability work, also run the backend-neutral storage package
coverage and, when a compatible endpoint is available, the optional S3/MinIO
integration smoke:

```bash
go test ./internal/storage

GOGOMAIL_TEST_S3_ENDPOINT=http://localhost:19000 \
GOGOMAIL_TEST_S3_BUCKET=gogomail \
GOGOMAIL_TEST_S3_ACCESS_KEY_ID=gogomail \
GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY=gogomail123 \
  go test ./internal/storage
```

For AWS S3 or virtual-hosted compatible providers, add
`GOGOMAIL_TEST_S3_REGION` and set
`GOGOMAIL_TEST_S3_FORCE_PATH_STYLE=false`. Private CA or self-signed test
endpoints can use `GOGOMAIL_TEST_S3_CA_CERT_FILE` and
`GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY`; keep those variables test-scoped and
document the chosen TLS posture in deployment runbooks.

Release-oriented PostgreSQL checks are opt-in because they need a disposable
database. They run migrations in a temporary schema and exercise draft send plus
outbox retry behavior against real SQL:

```bash
GOGOMAIL_TEST_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
  go test ./internal/maildb ./internal/outbox
```

See `docs/smtp-release-runbook.md` for the SMTP backend release soak,
STARTTLS/SMTPS, trusted relay, and outbound DSN/bounce smoke checklist.
See `docs/api-usage-export-runbook.md` for the API usage export, handoff, and
retention-readiness operator flow.

## Local infrastructure

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
```

Phase 1 uses PostgreSQL, Redis Streams, and object storage. Kafka, OpenSearch, etcd, Vault, and the Next.js web apps are intentionally deferred.

Run database migrations:

```bash
GOGOMAIL_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
  go run ./cmd/gogomail --migrate --mode=all-in-one
```

You can also load a flat YAML config file and override only the settings that
belong to the current deployment:

```bash
go run ./cmd/gogomail --config=configs/config.example.yaml --mode=all-in-one
```

The config file uses the same runtime validation as environment variables.
Storage can be flipped between `local`, `nfs`, `minio`, and `s3` by changing
the `storage_*` keys, while secrets may still be supplied by environment
variables when they are omitted from the file.

Reviewed storage profile starting points live under `configs/`:

```bash
go run ./cmd/gogomail --config=configs/storage.local.yaml --mode=all-in-one
go run ./cmd/gogomail --config=configs/storage.nfs.yaml --mode=all-in-one
go run ./cmd/gogomail --config=configs/storage.minio.yaml --mode=all-in-one
go run ./cmd/gogomail --config=configs/storage.s3.yaml --mode=all-in-one
```

Use `storage_root` for local/NFS roots in YAML. The older `mailstore_root`
key remains accepted for compatibility, while `GOGOMAIL_STORAGE_ROOT` is the
matching environment variable alias. See `docs/storage-backends.md` for the
object contract, S3-compatible TLS options, backend label compatibility, and
pre-release smoke matrix.

## Receive mail locally

Start the local SMTP receive boundary:

```bash
GOGOMAIL_SMTP_ADDR=127.0.0.1:2525 \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_LOCAL_RECIPIENTS=admin@example.com \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
GOGOMAIL_DEDUP_BACKEND=redis \
GOGOMAIL_RATELIMIT_BACKEND=redis \
GOGOMAIL_BACKPRESSURE_BACKEND=redis \
GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE=60 \
GOGOMAIL_REDIS_ADDR=127.0.0.1:16379 \
  go run ./cmd/gogomail --mode=edge-mta
```

For the post-filter internal receive boundary, run `inbound-mta` on its own address:

```bash
GOGOMAIL_INBOUND_SMTP_ADDR=127.0.0.1:2526 \
GOGOMAIL_INBOUND_TRUSTED_RELAYS=127.0.0.1/32,::1/128 \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_LOCAL_RECIPIENTS=admin@example.com \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
  go run ./cmd/gogomail --mode=inbound-mta
```

`inbound-mta` is intended for the post-filter/internal receive boundary. Set `GOGOMAIL_INBOUND_TRUSTED_RELAYS` to the Edge MTA or spam-filter relay CIDR list; untrusted remotes are rejected with an SMTP policy response before envelope state is accepted.

When `GOGOMAIL_LOCAL_RECIPIENTS` is set, edge-mta uses an in-memory static recipient resolver for local development.

When `GOGOMAIL_LOCAL_RECIPIENTS` is empty, edge-mta uses PostgreSQL:

```txt
user_addresses -> users/domains
folders(system_type=inbox)
messages INSERT
```

When `GOGOMAIL_DEDUP_BACKEND=redis`, edge-mta uses Redis `SET NX` with a 24-hour TTL to skip duplicate `Message-ID + recipient` deliveries.

When `GOGOMAIL_RATELIMIT_BACKEND=redis`, edge-mta applies a Redis-backed fixed-window RCPT rate limit per remote address.

When `GOGOMAIL_BACKPRESSURE_BACKEND=redis`, edge-mta reads `backpressure:smtp:state`.

```txt
normal/warning  -> accept DATA
danger/critical -> reject DATA before spooling
```

Useful SMTP receive guardrails:

```bash
GOGOMAIL_SMTP_MAX_RECIPIENTS=100
GOGOMAIL_SMTP_MAX_MESSAGE_BYTES=26214400
GOGOMAIL_SMTP_MAX_CONNECTIONS=0
GOGOMAIL_SUBMISSION_MAX_CONNECTIONS=0
GOGOMAIL_SMTP_READ_TIMEOUT=30s
GOGOMAIL_SMTP_WRITE_TIMEOUT=30s
GOGOMAIL_SMTP_ADD_RECEIVED_HEADER=true
GOGOMAIL_SMTP_REQUIRE_AUTH=false
GOGOMAIL_SMTP_SUPPORT_SMTPUTF8=false
GOGOMAIL_SMTP_SUPPORT_REQUIRETLS=false
GOGOMAIL_SMTP_SUPPORT_DSN=false
GOGOMAIL_SMTP_SUPPORT_BINARYMIME=false
```

Optional attachment scanner webhook:

```bash
GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL=https://scanner.example/scan
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT=2s
```

The scanner hook runs only for parsed messages with attachments and remains
disabled by default. Production webhook URLs must use HTTPS. The same scanner
configuration is available to `edge-mta`, `inbound-mta`, and `outbound-mta`.
See `docs/webhook-integrations.md` for the scanner request/response contract
and verdict semantics.

Accepted messages are stored as raw `.eml` files under:

```txt
var/mailstore/mailstore/{company_id}/{domain_id}/{user_id}/maildir/{YYYY}/{MM}/{message_id}.eml
```

## Submit outbound mail locally

`outbound-mta` exposes the authenticated SMTP Submission boundary. By default it listens on `:2587` for local development:

```bash
GOGOMAIL_SUBMISSION_ADDR=127.0.0.1:2587 \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=true \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
  go run ./cmd/gogomail --mode=outbound-mta
```

Submission requires `AUTH PLAIN`, verifies that `MAIL FROM` belongs to the authenticated user, stores the raw RFC 5322 `.eml`, then records the message through the existing `mail.outbound.<farm>` outbox flow.

Useful submission guardrails:

```bash
GOGOMAIL_SUBMISSION_MAX_RECIPIENTS=100
GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES=26214400
GOGOMAIL_SMTP_READ_TIMEOUT=30s
GOGOMAIL_SMTP_WRITE_TIMEOUT=30s
GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER=true
GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8=false
GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS=false
GOGOMAIL_SUBMISSION_SUPPORT_DSN=false
GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME=false
```

For production, configure STARTTLS certificates and keep insecure AUTH disabled:

```bash
GOGOMAIL_ENV=production \
GOGOMAIL_SUBMISSION_ADDR=:587 \
GOGOMAIL_SMTP_TLS_CERT_FILE=/etc/gogomail/tls/fullchain.pem \
GOGOMAIL_SMTP_TLS_KEY_FILE=/etc/gogomail/tls/privkey.pem \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false \
  gogomail --mode=outbound-mta
```

If you also need implicit TLS submission for legacy clients, set `GOGOMAIL_SUBMISSION_SMTPS_ADDR` (usually `:465`). SMTPS requires the same `GOGOMAIL_SMTP_TLS_CERT_FILE` and `GOGOMAIL_SMTP_TLS_KEY_FILE` pair and runs alongside the STARTTLS listener:

```bash
GOGOMAIL_ENV=production \
GOGOMAIL_SUBMISSION_ADDR=:587 \
GOGOMAIL_SUBMISSION_SMTPS_ADDR=:465 \
GOGOMAIL_SMTP_TLS_CERT_FILE=/etc/gogomail/tls/fullchain.pem \
GOGOMAIL_SMTP_TLS_KEY_FILE=/etc/gogomail/tls/privkey.pem \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false \
  gogomail --mode=outbound-mta
```

Local users authenticate against `users.password_hash`. Supported formats are:

- `pbkdf2-sha256$<iterations>$<base64-salt>$<base64-key>`
- `sha256:<hex>` for legacy/dev fixtures
- `plain:<password>` only for explicit local development fixtures

Admin user creation and `PATCH /admin/v1/users/{id}/password-hash` can set a
`password_hash` field with one of those pre-hashed values; raw production
passwords should be hashed before they reach the Admin API. User list/detail
responses expose `password_configured` instead of returning stored hashes, and
`GET /admin/v1/users` can filter by `status` and
`password_configured=true|false`.

### Delivery smart host relay

By default the delivery worker performs direct MX delivery. To relay all outbound mail through a gateway/smart host instead, configure:

```bash
GOGOMAIL_DELIVERY_SMARTHOST=smtp.relay.example.net:587
GOGOMAIL_DELIVERY_TLS_MODE=require
GOGOMAIL_DELIVERY_SMARTHOST_USERNAME=relay-user
GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD=relay-secret
```

For implicit TLS relay gateways, typically port 465, enable:

```bash
GOGOMAIL_DELIVERY_SMARTHOST=smtp.relay.example.net:465
GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS=true
GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE=require
```

`GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE` is optional; when omitted, smart-host routes inherit `GOGOMAIL_DELIVERY_TLS_MODE`.

### Bounce / DSN handling

Hard-bounce delivery events generate RFC 3464 `multipart/report` Delivery Status Notifications through the event-worker. DSNs are stored as `.eml` files and queued back to the original envelope sender with a null reverse-path, reducing bounce-loop risk.

Useful DSN settings:

```bash
GOGOMAIL_DSN_POSTMASTER='Mail Delivery Subsystem <postmaster@example.com>'
GOGOMAIL_SMTP_DOMAIN=mx.example.com
```

`NOTIFY=NEVER` is honored, DSN queueing uses deterministic storage paths and outbox dedupe keys, and null reverse-path DSN bounces do not create suppression-list entries.
