# gogomail current status

Last updated: 2026-05-05 (updated after S3-compatible object storage groundwork)

## Current phase

gogomail is in the backend platform hardening phase.

The project has moved beyond SMTP-only development. SMTP remains a critical
RFC-sensitive core, but current work should balance:

- tenant/domain operations
- Admin API
- Mail API contracts
- delivery routing and observability
- DNS/DKIM/domain onboarding
- quota and policy enforcement
- OpenAPI drift prevention

Actual Next.js frontend implementation has not started. When frontend work
starts, use Next.js with TypeScript, shadcn/ui, and the project `DESIGN.md` as
required guidance, aiming for a Notion Mail-like UI/UX. Before creating or
substantially implementing frontend apps, ask the user for frontend-specific
guidance.

## Completed or materially advanced

- SMTP receive engine with real TCP integration coverage.
- Authenticated Submission MTA with STARTTLS and SMTPS support.
- Outbound SMTP delivery with direct MX, smart-host, TLS policy, retry, and
  partial recipient failure handling. Admin-created delivery routes reject
  impossible TLS/auth combinations before relay routes are stored.
  Static smart-host configuration now rejects password-only auth plus
  CR/LF-bearing or oversized auth username, password, and identity values during
  startup config validation.
- DSN/bounce handling with RFC 3461/3464-oriented metadata, null reverse-path,
  `NOTIFY=NEVER`, deterministic outbox dedupe, retry-exhaustion failure
  notifications, and loop-risk reduction.
- Shared high-performance-minded EML parsing boundary under `internal/message`.
- PostgreSQL metadata model for companies, domains, users, folders, messages,
  attachments, outbox, audit logs, DKIM keys, trusted relays, delivery routes,
  domain DNS checks, and policy-bearing domain settings.
- Admin APIs for domains, users, quotas, DKIM keys, trusted relays, delivery
  routes, delivery route resolution, queue stats, delivery attempts,
  outbox event metadata, suppression list, quota usage, domain DNS
  checks/history, backpressure inspection/update, domain policy, per-domain
  stats, DKIM DNS verification, delivery route runtime counters, and exhausted
  delivery attempts with recipient-domain and recent-window filters.
- Delivery-attempt list, stats, and exhausted-attempt reads can filter by
  message id, farm, sender, recipient domain, and recent time window for
  targeted retry/bounce incident triage.
- Domain listing can filter by company, lifecycle status, and latest DNS-check
  status for onboarding and tenant triage.
- Domain DNS check history can filter by summary status and RFC3339 `since`
  windows so operators can inspect recent onboarding or deliverability failures
  without re-querying DNS or scanning every persisted check.
- Company listing can filter by lifecycle status for tenant-level suspension
  and disabled-account triage.
- Delivery-route listing can filter by status, farm, and domain pattern for
  targeted route audits.
- Admin delivery-route creation now rejects oversized farm, SMTP hello, pool,
  description, and relay auth identity/username/password metadata before route
  storage or audit work.
- Suppression-list reads can filter by domain, email, and reason for targeted
  bounce triage without direct database access.
- Queue stats include ready, delayed, stale-processing, oldest-ready, and
  next-available metadata so operators can distinguish backlog from scheduled
  retry delay.
- Outbox event metadata can be filtered by topic, partition key, status, and
  recent time window without exposing payload bodies.
- Outbox event list responses bound `last_error` previews at UTF-8 boundaries
  so operational dashboards do not pull oversized diagnostics by default.
- Outbox event detail responses expose full stored `last_error` by id while
  still omitting raw payload bodies.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, cursor-paginated thread lists/thread messages and draft search,
  plus user-scoped sent-message delivery/bounce status.
- Inbound parsing now extracts RFC `In-Reply-To`/`References`; inbound and
  reply/forward outbound persistence inherit local thread IDs when matching
  source messages exist.
- Reply composition writes RFC `In-Reply-To`/`References` headers into outgoing
  `.eml` messages.
- Outbound text composition rejects CR/LF-bearing subject, display-name, email,
  and explicit Message-ID inputs before writing RFC 5322 headers.
- Outbound RFC 5322 text composition folds long headers, rejects malformed
  explicit Message-ID values, and drops malformed thread IDs before writing
  `In-Reply-To`/`References`.
- Mail API exposes a first Postgres-backed search endpoint for active message
  metadata, with an FTS index for small deployments.
- Received-message body search now has an asynchronous indexing boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml` objects,
  extracts bounded text through the shared parser, and upserts Postgres search
  documents used by the existing search endpoint.
- Search indexing rejects ambiguous `mail.stored` storage paths that would be
  changed by path cleaning, preventing parent-traversal or duplicate-separator
  event payloads from opening a different object key.
- Search indexing caps `mail.stored` event `References` metadata before
  document construction, matching the parser's bounded metadata stance.
- The OpenSearch indexing adapter bounds UTF-8 metadata fields and reference
  arrays before JSON document submission, keeping direct adapter calls aligned
  with worker/parser metadata limits.
- OpenSearch relevance queries bound UTF-8 search/filter text and escape
  wildcard metacharacters in sender/subject filters so client-supplied `*` or
  `?` remain literal substring filters.
- OpenSearch relevance hits now clean bounded message IDs from `_source`/`_id`
  before Postgres hydration, dropping CR/LF-bearing IDs from external search
  responses.
- OpenSearch indexing now rejects blank, CR/LF-bearing, or oversized document
  message IDs before constructing `_doc/{id}` URLs, keeping URL IDs aligned
  with bounded JSON metadata.
- OpenSearch writer/searcher construction now trims usernames while preserving
  password bytes, and rejects CR/LF-bearing or oversized endpoint credentials
  before BasicAuth request headers can be generated.
- OpenSearch username/password configuration is also CR/LF-rejected and
  size-bounded during startup config validation when the OpenSearch backend is
  selected, surfacing credential formatting mistakes before worker/search setup.
- OpenSearch writer construction now rejects CR/LF-bearing direct endpoint
  values before URL parsing, keeping adapter calls aligned with startup config
  endpoint validation.
- OpenSearch relevance response decoding is capped before JSON parsing so
  oversized search backend responses cannot allocate unbounded highlight or hit
  payloads in the Mail API path, and trailing JSON tokens are rejected before
  search hits are accepted.
- OpenSearch index/bootstrap/search status-error diagnostics now collapse
  backend response bodies into bounded one-line UTF-8 previews, preventing
  CR/LF-bearing backend errors from leaking into logs or API diagnostics.
- Shared EML text extraction, retained header metadata, and attachment
  metadata are bounded with UTF-8 boundary preservation; attachment filenames
  are basename-normalized, control-character cleaned, and capped before
  reaching storage/API/search consumers. Subject, address display-name/address,
  message-id, address-list, and `References` metadata are capped before
  downstream storage, search, and threading use them. Oversized structured
  subject, address, and message-id-list headers are pre-bounded before decoding
  or list parsing, with truncation flags for retained metadata/list caps.
- Search responses can now opt into relevance sorting, rank scores, and bounded
  Postgres headline snippets while preserving date-sorted results by default.
- Postgres and OpenSearch relevance search now share a metadata-first tuning
  intent: subject and sender matches rank above indexed body text matches.
- Draft rows remain excluded from `GET /api/v1/search`; drafts now have a
  separate compose-focused `GET /api/v1/drafts/search` contract over active
  draft subject, sender, recipient JSON, body text, attachment state, and
  newest-updated ordering.
- `gogomail --mode=all-in-one` serves Mail API and Admin API routes from the
  same HTTP process, keeping single-node/local release smoke tests aligned with
  the documented backend mode.
- `/health/ready` can now include runtime database and Redis dependency probes
  for HTTP modes that use those services, returning `degraded` with HTTP 503
  when a required probe fails.
- Database readiness now also compares the applied `goose_db_version` against
  the latest local SQL migration, so stale schemas degrade `/health/ready`
  instead of passing on connectivity alone.
- Mail/Admin HTTP readiness now probes configured storage with a write/read/delete
  cycle, and unsupported HTTP storage backends fail fast instead of silently
  using local storage wiring.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, and HTTP runtimes now share storage backend validation and factory
  wiring for local filesystem/NFS-style storage plus S3-compatible object
  storage. `GOGOMAIL_STORAGE_BACKEND=s3` can target AWS S3, while
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same S3-compatible adapter with
  path-style requests for local MinIO-style deployments. Both paths use endpoint,
  region, bucket, prefix, credential, and session-token settings.
- HTTP server runtime guardrails are configurable and validated: read, write,
  idle, read-header timeout, and maximum header bytes are wired into the shared
  Mail/Admin/API-metered HTTP server.
- Admin backpressure overrides now persist bounded hash-chain audit rows after
  Redis state changes, recording previous/current SMTP pressure levels without
  silently accepting unaudited operational receive throttles.
- Admin suppression-list deletions now persist hash-chain audit rows in the
  same transaction as the delete, preserving suppression entry, domain, email,
  reason, and source-message evidence for deliverability forensics.
- Admin outbox retry now persists a hash-chain audit row in the same transaction
  as the retry reset, preserving previous topic, partition key, status,
  attempts, and bounded error evidence for replay forensics.
- Admin push-notification outcome updates now persist hash-chain audit rows in
  the same transaction as provider-status updates and invalid-token device
  deletion, without including raw push tokens or token suffixes in audit detail.
- Admin attachment cleanup runs now persist bounded hash-chain audit rows after
  stale upload and upload-session expiry sweeps, recording cutoff, normalized
  limit, expired counts, and bounded ID samples without storage paths.
- Admin IMAP UID backfill now persists a hash-chain audit row in the same
  transaction as UID assignment, recording mailbox/user scope, normalized
  limit, assigned count, and a bounded message/UID sample.
- Admin API-usage export batch creation now persists a hash-chain audit row in
  the same transaction as the batch, recording tenant/principal scope, export
  window, event/request counts, bytes, latency totals, and export format.
- Admin API-usage export artifact creation/upsert now persists a hash-chain
  audit row in the same transaction as the artifact row, recording object key,
  storage backend, content type, byte/event counts, and SHA-256 digest without
  copying artifact metadata into the audit detail.
- Admin API-usage export manifest digest and signature creation now persist
  hash-chain audit rows in the same transaction as the evidence rows, recording
  bounded digest/signature evidence without copying raw manifests, metadata, or
  full signature material into audit detail.
- Admin API-usage ledger retention runs now persist hash-chain audit rows in the
  same transaction as run records and destructive deletes, recording dry-run,
  blocked, no-op, and completed outcomes with bounded readiness evidence.
- Admin user creation and password-hash rotation can persist a validated
  `password_hash`, giving operators a path to create and maintain SMTP
  Submission-capable local users without storing raw production passwords
  through the API. User read models expose `password_configured` without
  returning stored password hashes, and Admin user listing can filter by status
  and that readiness flag.
- Mail API send/draft-send applies domain outbound policy in enforce mode for
  recipient-count and composed-message-size guardrails.
- Mail API attachment reservation/direct upload applies enforced domain
  `max_attachment_bytes` policy before quota reservation or object storage
  writes.
- Per-domain inbound policy enforced at SMTP receive and Submission MTA (max
  recipients, max message size, inbound mode).
- Hierarchical quota ledger enforced at mail storage write/delete boundaries:
  company, domain, and user usage counters are updated atomically in the same
  PostgreSQL transaction. User quota source is tracked as `default|custom`, and
  domain default user quota updates apply to default-following users while
  preserving custom overrides.
- Attachment upload metadata creation reserves bytes from the same
  company/domain/user quota ledger, stale upload cleanup releases them, and the
  Mail API returns HTTP 507 `insufficient_storage` for quota exhaustion.
- Admin quota views now expose runtime remaining capacity, child-allocation
  usage, allocatable capacity, and over-allocation indicators for
  company/domain/user operations.
- Admin quota usage pressure reads can filter by scope, domain, over-limit
  status, and over-allocation status for targeted capacity triage.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with message and attachment source rows.
- Admin API can run operator-controlled quota reconciliation corrections with
  transaction/advisory locking and bounded audit-log detail for dry-run and
  applied correction attempts.
- Product quota direction is company pool → domain allocation → user unified
  storage allowance. User quota should cover mailbox, attachments, future Drive,
  and other user-owned storage features.
- API metering direction is agreed for future SaaS operations: collect usage
  dimensions early through an async middleware/event boundary, but keep
  billing/rate-limit enforcement policy-driven and disabled by default.
- API metering has a first disabled-by-default middleware boundary with a
  `slog` sink for low-risk operational visibility and an outbox sink for durable
  `api.usage` event emission.
- API metering now has an aggregation worker boundary: `api-metering-worker`
  consumes `api.usage` events from `api.event`, upserts Postgres daily
  and monthly aggregates, and exposes `GET /admin/v1/api-usage/daily` plus
  `GET /admin/v1/api-usage/monthly` for operations.
- API usage daily/monthly aggregate reads can filter by tenant, company, domain,
  user, API key, principal, auth source, method, route, status, and time window
  for scoped billing and operational triage.
- API metering events now use `2026-05-04.api-usage.v2` payloads with
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  worker stores those dimensions in the idempotency ledger and keys daily/monthly
  aggregates by identity so usage from different tenants or principals does not
  merge.
- API metering auth-source dimensions are normalized to the known set
  `anonymous|bearer|admin_token|query_user_id|unknown`; unexpected values fold
  to `unknown` before ledger/aggregate storage.
- API metering request identity extraction trims tenant/company/domain/user/API
  key/principal dimensions, drops CR/LF-bearing or oversized default request
  dimensions, and no longer classifies blank or unsafe `Authorization: Bearer`
  headers as bearer traffic.
- API metering durable event metrics clamp negative byte/latency values to zero
  and default nonpositive request counts to one before ledger/aggregate storage.
- API metering outbox payloads clamp negative byte/latency values to zero before
  deterministic event IDs are generated.
- API metering durable events require nonblank method/route keys and HTTP-like
  status codes before ledger/aggregate storage.
- API metering middleware route-key extraction drops CR/LF-bearing or oversized
  ServeMux patterns and fallback paths before sink dispatch.
- API metering durable event decoding rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before ledger/aggregate storage.
- Admin API usage ledger, NDJSON export, stats, export-batch creation, and
  retention-readiness tenant/principal filters now reject CR/LF-bearing or
  oversized values before service dispatch.
- Admin user listing, IMAP UID backfill, DKIM key listing, and delivery-route
  resolution query filters now share the same CR/LF and size boundary checks;
  DKIM key listing can also filter by `active|inactive` status.
- API usage export batch, artifact, manifest-digest, and signature path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Admin company, domain, and user detail/mutation path identifiers now use the
  same blank, CR/LF, and size validation before service dispatch.
- Admin IMAP UID backfill mailbox IDs, outbox event/retry IDs, DKIM key IDs,
  suppression IDs, trusted-relay IDs, and delivery-route IDs now use the same
  path boundary validation before service dispatch.
- Mail API development `user_id` query fallback values now reject CR/LF-bearing
  or oversized identifiers before service dispatch.
- OpenAPI now wires the Mail API development `user_id` fallback parameter into
  every user-scoped Mail operation, keeping local/all-in-one generated clients
  aligned with JWT-disabled runtime behavior.
- Mail API folder, thread, message, draft, attachment, and push-device path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Push-device create/update validation now rejects invalid-UTF-8,
  CR/LF-bearing, or oversized user and token metadata before repository upsert,
  keeping raw provider tokens bounded at the storage boundary.
- Mail API message-list `folder_id` and search text/filter query parameters now
  reject CR/LF-bearing or oversized values before service dispatch.
- Mail API bearer JWT `user_id` and `sub` identities now reject CR/LF-bearing
  or oversized claims during signing and verification before request scoping.
- Mail API bearer JWT verification now rejects oversized token, header,
  payload, and signature segments before base64 decoding claim data.
- Mail and Admin API authentication headers now reject oversized `Authorization`
  and `X-Admin-Token` values before bearer/JWT parsing or token comparison.
- Password hash verification now rejects oversized stored hashes, excessive
  PBKDF2 iteration counts, and oversized PBKDF2 salt/key metadata before
  expensive derivation or decoded allocation.
- Mail API search control query values and direct multipart attachment
  `draft_id` fields now reject CR/LF-bearing or oversized values at the HTTP
  boundary before service dispatch.
- VERP return-path parsing now rejects oversized addresses, local parts, tokens,
  and encoded recipients before base64 decoding DSN recipient metadata.
- API usage export Ed25519 signer/verifier key configuration now rejects
  oversized base64 public/private keys before decoding.
- API usage export manifest signer configuration now rejects CR/LF-bearing or
  oversized key IDs and remote signer tokens, and local HMAC signing rejects
  oversized secrets before MAC generation.
- API usage export HMAC and Ed25519 signature verification now rejects
  incorrectly sized signature hex before decoding.
- Remote Ed25519 manifest signer responses now reject oversized bodies and
  trailing JSON tokens before signature evidence is accepted.
- Remote Ed25519 manifest signer status-error diagnostics now collapse signer
  response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing
  external signer errors from leaking into export/billing diagnostics.
- Attachment scan and push-notification webhooks now reject CR/LF-bearing
  configured tokens or endpoints and collapse non-2xx HTTP response bodies into
  bounded one-line UTF-8 previews before surfacing delivery failures.
- API metering middleware falls back to `METHOD /path` when no `http.ServeMux`
  route pattern is available, keeping durable event route keys nonblank.
- API metering now records immutable `api_usage_ledger` rows before aggregate
  upserts. Admin API exposes bounded ledger list, NDJSON export, and stats
  endpoints for billing/export preparation while keeping HTTP request handling
  fail-open.
- Admin API exposes API usage ledger retention readiness so operators can check
  whether non-future cutoff-bound ledger rows are covered by a completed export
  batch with artifact, manifest digest, and signature evidence before retention
  is allowed.
- Admin API exposes bounded API usage ledger retention runs. Destructive runs
  require `confirm_ready=true`, reuse the readiness gate, and delete only a
  normalized batch of ready ledger rows, while dry-runs return the same envelope
  without mutation.
- Optional PostgreSQL integration coverage verifies retention runs do not delete
  blocked candidates, dry-runs do not mutate ready candidates, and destructive
  ready runs persist retention-run audit rows, delete only the requested bounded
  batch, and preserve newer ledger rows.
- Admin API exposes list/detail reads for persisted API usage ledger retention
  runs so operators can inspect blocked, dry-run, and destructive retention
  attempts after the fact.
- `api-usage-retention-worker` can run bounded API usage ledger retention on an
  interval or once-and-exit, dry-run by default, reusing the same readiness gate
  and persisted retention-run audit rows as the Admin API.
- Destructive API usage retention worker runs require both explicit
  `confirm_ready` configuration and a production-oriented `remote-ed25519`
  export manifest signer backend.
- API usage export capabilities now advertise retention-run support, retention
  worker support, and the remote-key requirement for destructive worker runs.
- API usage ledger retention now rejects future cutoffs at the repository
  boundary as well as the HTTP boundary, keeping worker/direct-call behavior
  aligned with the Admin API guardrail.
- Admin API exposes bounded audit-log list/detail reads with category, action,
  result, target-type, company/domain/user, and recent-window filters so stored
  operational audit records can be inspected through the release API surface.
- Domain DNS check and quota reconciliation correction audit rows now reuse the
  shared audit writer hash-chain logic instead of inserting empty hash fields.
- Trusted relay create/delete mutations now write hash-chain audit rows in the
  same database transaction as the policy change, keeping inbound relay-policy
  administration inspectable through the Admin audit API.
- Delivery route create/status/delete mutations now write hash-chain audit rows
  in the same database transaction as the gateway policy change, excluding
  relay auth secrets from audit detail.
- DKIM key create/upsert, deactivate, and DNS-verification mutations now write
  hash-chain audit rows in the same database transaction as the persisted key
  lifecycle change, without including private key material in audit detail.
- Domain and user lifecycle status updates now write hash-chain audit rows in
  the same database transaction as the status change, scoped by company/domain
  identifiers for tenant forensics.
- Company, domain, and user quota mutations now write hash-chain audit rows in
  the same database transaction as the quota change, including domain default
  user quota propagation counts for quota forensics.
- Domain policy mutations now write hash-chain audit rows in the same database
  transaction as the policy change, preserving inbound/outbound mode and size
  guardrail evidence for SMTP/Mail API enforcement forensics.
- Domain/user provisioning and user password-hash rotation now write hash-chain
  audit rows in the same database transaction as the persisted change, without
  including password hash material in audit detail.
- Shared audit-log normalization now bounds scalar metadata and JSON detail size
  before hash computation or database insertion.
- Admin API exposes a bounded audit-log integrity check that recomputes recent
  row hashes and reports hash or in-window prev-hash breaks without mutating the
  audit trail.
- API usage exports now have persisted batch manifests/checkpoints. Admin API
  can create/list/get manifest rows and replay a saved manifest window as NDJSON
  by batch ID. Batch creation now requires explicit RFC3339 `from`/`to`
  bounds, preventing accidental all-ledger checkpoints.
- API usage export batch listing can filter by tenant, principal, status, and
  export window so operators can find covering manifests without scanning every
  saved batch.
- API usage ledger/export/retention tenant and principal query filters are
  trimmed at the Admin API boundary before billing/export service dispatch.
- API usage export batches can now carry external artifact metadata rows with
  object key, content type, byte count, SHA-256, event count, and JSON metadata.
  Artifacts are deduplicated per batch by object key and SHA-256.
- API usage export artifact writes reject ambiguous object keys that would be
  changed by path cleaning or contain backslash/path-traversal segments before
  writing billing handoff objects.
- Admin API can now write API usage export batch artifacts to the configured
  object store, register the resulting byte count/SHA-256 metadata, and download
  or verify stored NDJSON artifacts for handoff verification.
- API usage export batches now have canonical manifest digest rows and
  verification endpoints. Operators can generate SHA-256 digests over the saved
  batch plus registered artifacts, list/fetch digest records, and re-check the
  stored manifest against its canonical digest before billing handoff.
- API usage export manifest digesting rejects unsupported explicit manifest
  schema versions before canonical digest evidence is generated.
- API usage export manifest digests can now be signed through disabled-by-
  default local HMAC, local Ed25519, or remote Ed25519 signers. The remote
  signer is intended for an external KMS-backed signing service and is verified
  locally with a configured public key before persistence. Admin API exposes
  signature create/list/detail and verification endpoints while keeping the
  signer backend pluggable.
- API usage export manifest signing validates key IDs for local and remote
  signers, rejecting blank, CR/LF-bearing, or oversized key metadata before
  signature evidence is returned.
- Admin API exposes API usage export handoff readiness by batch. The report
  summarizes artifact coverage, latest digest/signature state, operational
  readiness, and a separate billing readiness grade so local signers are not
  mistaken for invoice-grade exports.
- Handoff readiness can now opt into `deep=true`, which streams registered
  artifacts from object storage for byte/SHA verification and verifies the
  latest manifest digest/signature in one operator report while keeping
  metadata-only readiness fields stable.
- Manifest signature verification now sits behind an
  `apimeter.ExportManifestSignatureVerifier` boundary parallel to signing. The
  current wired verifiers are local-HMAC and Ed25519, supporting both local and
  remote Ed25519 signer backends.
- Admin API exposes API usage export capabilities so operators can see the
  configured signer backend, signer key ID, verifier availability, and whether
  production/verified billing readiness is supported before creating handoff
  batches.
- Push notification enqueue now has an async worker boundary:
  `push-notification-worker` consumes `mail.stored` events, resolves active
  user devices from PostgreSQL, and can emit disabled-by-default `slog`
  notification candidates or POST raw-token targets to a configured HTTP
  webhook push gateway with an optional bounded bearer token and Postgres
  candidate-attempt audit rows without touching SMTP hot paths or committing to
  FCM/APNs SDKs. `docs/webhook-integrations.md` documents the push gateway
  payload, authentication, HTTPS requirement, and queued/failed attempt
  semantics. Malformed resolved targets with blank or CR/LF-bearing device IDs
  or tokens, oversized device IDs or tokens, or unsupported platforms, are
  dropped before candidate recording and sink handoff; optional target labels
  and token suffixes are UTF-8 safely bounded. The webhook sink also bounds and
  normalizes direct-call payload metadata before JSON serialization.
- Admin API exposes `GET /admin/v1/push-notification-attempts` for inspecting
  push notification candidate fan-out by message, status, user, platform,
  device, provider status, provider message id, or recent time window.
- Admin API exposes `GET /admin/v1/push-notification-attempts/{id}` for
  single-attempt troubleshooting.
- Admin API exposes
  `PATCH /admin/v1/push-notification-attempts/{id}/outcome` for authenticated
  operator/provider handoff updates to queued, delivered, failed, or
  invalid-token outcomes with bounded provider diagnostics.
- Admin API exposes `GET /admin/v1/push-notification-stats` for a compact
  active-device and attempt-status summary, with optional `message_id`,
  `user_id`, `platform`, `device_id`, and `since` scoping for message-level,
  user-level, provider-platform, device-level, and recent-window
  troubleshooting.
- Push notification sinks receive the persisted candidate attempt id with each
  target, preparing clean vendor outcome updates later.
- Push notification candidate and provider-outcome diagnostics are capped at
  UTF-8 boundaries before Postgres storage, preserving internationalized
  subjects and vendor messages in Admin API views.
- Push notification candidate recording rejects invalid-UTF-8, CR/LF-bearing,
  or oversized message/user/device/company/domain IDs before SQL insert
  dispatch, and rejects unsupported platforms at the recorder boundary.
- The push worker marks attempts `queued` after a successful sink handoff while
  marking failed sink handoffs as `failed` with the sink error before returning
  the handler error for Redis stream retry.
- Existing push attempts can be updated to queued, delivered, failed, or
  invalid-token outcomes through the internal recorder or Admin API.
- The push worker's internal outcome recorder now delegates to the same
  `maildb` outcome update path used by the Admin API, keeping provider status
  validation, diagnostic bounds, timestamp updates, and invalid-token device
  deletion in one storage boundary.
- Push notification outcome recording rejects invalid-UTF-8, CR/LF-bearing, or
  oversized attempt IDs before SQL update dispatch.
- Invalid-token outcomes automatically soft-delete the affected push device in
  the same Postgres transaction.
- `mail.stored` events now carry an explicit
  `2026-05-04.mail-stored.v1` schema version for downstream audit, search, and
  push workers.
- Audit, search indexing, and push notification consumers reject unsupported
  explicit `mail.stored` schema versions while accepting versionless legacy
  events.
- The audit `mail.stored` consumer trims event, tenant, recipient, subject,
  storage, and timestamp fields and rejects CR/LF-bearing message identifiers
  before audit-log persistence.
- Delivery-status audit consumers trim event, tenant, sender, recipient, farm,
  status, error, and timestamp fields and reject CR/LF-bearing message
  identifiers before audit-log persistence.
- Delivery outcome and exhausted outbox event payloads trim message, tenant,
  farm, sender, recipient, error, and DSN metadata before event persistence.
- Mail API now has user-scoped push device registration/list/delete contracts
  for `apns`, `fcm`, and `webpush`; raw device tokens are accepted only on
  write and are not returned in API JSON responses.
- Push-device list and delete service methods trim user and device identifiers
  before repository work, and delete rejects blank, CR/LF-bearing, or oversized
  device identifiers before repository dispatch.
- DKIM key DNS verification workflow with `dns_verified_at` persistence.
- Delivery route runtime counters (`RouteCounters`) with Admin API exposure.
- Retry exhaustion hook: `mail.delivery_exhausted` outbox event emitted and
  `delivery_attempts` row with status `exhausted` written when all retries fail.
- The delivery worker wires retry exhaustion recording at runtime, so terminal
  retry exhaustion diagnostics and `mail.delivery_exhausted` events are emitted
  by the actual worker path.
- Retry-exhausted delivery events now carry recipient-level DSN metadata and
  safe original storage paths into the event worker, generating sender-facing
  RFC 3464 failure DSNs with deterministic dedupe keys while preserving
  `NOTIFY=NEVER` and null reverse-path suppression.
- Admin delivery attempt lists can be scoped by status, recipient domain, and
  recent time window for bounded retry/bounce triage.
- Admin delivery attempt stats summarize total attempts, unique messages,
  unique recipients, and delivered/failed/bounced/exhausted buckets with the
  same status, recipient-domain, and recent-window filters.
- Admin delivery-route status/delete handlers trim route IDs at the HTTP
  boundary before operator mutations are passed to the service layer.
- User-scoped sent-message delivery status treats failed attempts with RFC 3463
  `4.x.x` enhanced status codes as retrying rather than terminal failed.
- User-scoped sent-message delivery status treats terminal `exhausted`
  attempts as failed so retry budgets do not remain visible as pending forever.
- DMARC reject policy enforcement at SMTP receive (`DMARCEnforce` flag).
- Authentication-Results trace header formatting now strips control characters
  and bounds verifier metadata before formatting SPF/DKIM/DMARC results,
  preventing DNS/library diagnostics from injecting or bloating stored headers.
- SMTPUTF8 declared correctly on outbound MAIL FROM for all internationalized
  addresses, and outbound delivery now fails closed with a permanent SMTPUTF8
  error when the remote MTA does not advertise SMTPUTF8.
- DSN composition supports optional `text/rfc822-headers` and `message/rfc822`
  returned-content parts for RFC 3464 reports, keeping header-only returns
  sanitized while allowing bounded full-message returns.
- Bounce DSN generation now honors `RET=HDRS` when the delivery event carries a
  safe original message storage path, reading bounded original EML headers and
  attaching them as sanitized `text/rfc822-headers` content.
- Bounce DSN generation now also honors `RET=FULL` by attaching the bounded
  original `.eml` as `message/rfc822` after validating the stored object key and
  message parseability.
- Migration guardrails now require every SQL migration to declare explicit
  goose Up/Down sections, and legacy API-usage, push, IMAP, and audit-index
  migrations have been normalized to that structure without changing their
  applied SQL.
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests. Path parameters, Mail search/Admin query filters,
  request schemas, response envelopes, and status enums are contract-tested for
  generated-client readiness. The draft is parsed as YAML and checked for stale
  documented routes that are not registered by the Go HTTP mux. Thread list
  parameters are guarded against accidental Admin/API-usage filter leakage.
  Non-JSON download/export responses are guarded so NDJSON streams and binary
  attachments are not modeled as JSON envelopes. All schemas are kept in sync
  with Go types.
- Admin token authorization and API metering admin-token classification compare
  fixed-length SHA-256 digests of trimmed token values for both bearer tokens
  and `X-Admin-Token`.
- Mail API JWT verification rejects unsupported JWT `alg` values and non-JWT
  `typ` headers before accepting signed bearer claims. JWT `user_id`/`sub`
  identities are whitespace-normalized and blank identities are rejected during
  both signing and verification. Tokens with `iat` values more than one minute
  in the future are rejected before Mail API claims are trusted.
- Redis event consumers acknowledge malformed stream entries after logging
  decode failures and move repeatedly handler-failing messages into a durable
  Redis dead-letter stream before acknowledging the original event, preventing
  poison messages from pinning worker progress indefinitely. Event,
  search-index, API-metering, push-notification, and delivery workers expose
  per-worker max-delivery and dead-letter-stream settings for production tuning.
- Redis event/search/API-metering/push/delivery workers reclaim idle pending
  Redis Stream messages via configurable claim-idle settings, improving crash
  recovery for at-least-once event processing. Startup validation now also
  rejects nonpositive event and delivery consumer count/block settings before
  workers run with unusable Redis Stream options.
- Redis worker stream, group, and consumer-name settings for event,
  search-index, API-metering, push-notification, and delivery workers are now
  required, CR/LF-rejected, and size-bounded during startup config validation,
  surfacing worker identity mistakes before consumer construction.
- `eventstream.NewRedisConsumer` now applies the same trim, required,
  CR/LF-rejection, and size-bound guardrails to stream, group, and consumer
  identifiers, keeping direct adapter callers aligned with runtime config
  validation.
- Event routing trims registered and payload event names and rejects
  CR/LF-bearing or oversized event names before worker dispatch.
- Redis stream event decoding rejects CR/LF-bearing or oversized outbox IDs,
  partition keys, and payloads before worker fan-out.
- Redis stream event decoding trims outbox id, partition key, and payload
  fields and rejects blank metadata before handler dispatch.
- Redis outbox publishing trims event id, topic, partition key, and payload
  metadata and rejects invalid topics or non-JSON payloads before stream writes.
- Admin API exposes a bounded IMAP mailbox UID backfill endpoint for future
  IMAP bootstrap/operator runs without enabling an IMAP protocol listener.
- Push notification workers no longer redeliver a Redis event solely because
  queued-outcome recording failed after the sink accepted the notification,
  reducing duplicate push risk while keeping the candidate attempt visible.
- Backend release verification script and SMTP release runbook.
- API usage export runbook covering capability checks, artifact/digest/signature
  handoff evidence, deep readiness, and retention-readiness gates.
- Public GitHub repository:
  <https://github.com/parkjangwon/gogomail>

## Explicitly not started

- Next.js shell/webmail/admin frontend implementation.
- Built-in spam scoring or pattern filtering.
- POP3 protocol server work. The future POP3 server should follow the same
  strict RFC, performance, and client-compatibility standard as IMAP.
- OpenSearch as the default/mandatory search backend.
- Kafka migration.
- etcd/Vault production control plane.
- Vendor push notification delivery adapters.

## Important guardrails

- Implemented SMTP features must strictly follow the relevant email RFCs.
- Do not advertise SMTP extensions until end-to-end semantics are implemented
  and tested.
- Do not turn SMTP core into a spam engine. Spam relay/filtering belongs behind
  explicit hooks/adapters.
- Keep hot paths streaming and allocation-aware.
- Preserve domain-as-tenant isolation.
- Commit by feature and push after completed work.

## Latest direction

The platform hardening sprint completed the following:

- Mailbox quota enforcement (receive, send, delete)
- Per-domain SMTP inbound policy (max recipients, max message size)
- DKIM DNS verification workflow
- Delivery route runtime counters
- Retry exhaustion events and Admin API exposure
- SMTPUTF8 outbound RFC 6531 fix
- DMARC reject policy enforcement hook
- Domain aggregate stats endpoint
- OpenAPI schema expansion (DKIMKey, DeliveryAttempt, DKIMKeyDNSVerification)
- Hierarchical quota ledger first implementation: company/domain/user Admin
  quota APIs, user quota source, domain default user quota propagation, and
  aggregate quota enforcement for mail writes/deletes.
- Attachment upload quota integration: upload metadata reserves quota, stale
  upload cleanup releases quota, and API quota exhaustion maps to 507.
- `attachment-cleanup-worker` can now run the stale upload cleanup loop
  periodically with configurable interval, stale age, and batch size, turning
  the repository/service cleanup path into an operational mode. It can also run
  once and exit for CronJob or timer-style deployments.
- Search indexing boundary: bounded received body extraction runs in
  `search-index-worker` and stores Postgres search documents outside SMTP hot
  paths.
- OpenSearch indexing has a first `internal/searchindex` writer adapter behind
  the same indexing interface, and `search-index-worker` can select it with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- The OpenSearch writer can bootstrap a strict message index mapping for
  message IDs, tenant/user filters, subject/body text, timestamps, and bounded
  body metadata.
- `search-index-worker` can optionally bootstrap the OpenSearch index on startup
  with `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
- OpenSearch query-side groundwork can search user-scoped documents and return
  ranked gogomail message IDs for later metadata hydration.
- `maildb` can hydrate ordered message ID search hits back into active
  `MessageSummary` rows without changing the Mail API response envelope.
- `mailservice` can compose OpenSearch relevance ID hits with Postgres summary
  hydration when the current API search contract can be preserved; unsupported
  filter/highlight combinations fall back to Postgres search.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, enabling safe relevance-search
  read-side rollout while preserving fallback behavior.
- OpenSearch indexed documents now include parsed sender and attachment
  presence fields needed for Mail API search-filter parity.
- OpenSearch relevance search can apply folder, from, subject, and attachment
  filters before Postgres metadata hydration; sender and subject filtering use
  lower-cased keyword fields to preserve case-insensitive filter behavior.
- OpenSearch relevance search can return subject/from/body highlights and map
  them into the existing Mail API `search_highlights` response field with
  bounded UTF-8-safe fragments.
- Mail API OpenSearch hydration deduplicates repeated external hit IDs before
  loading Postgres summaries while preserving the first rank/highlight result.
- Optional OpenSearch integration coverage can create a disposable index and
  verify indexing plus folder-aware relevance search when
  `GOGOMAIL_TEST_OPENSEARCH_URL` is available.
- Search index worker startup logs include non-secret backend diagnostics,
  including OpenSearch index name and bootstrap state when that backend is
  selected.
- OpenSearch writer/searcher HTTP calls use a configurable timeout through
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`, defaulting to 10 seconds.
- OpenSearch endpoint configuration is now validated as an HTTP(S) URL with a
  host during startup config validation, so malformed search backend endpoints
  fail before worker/search adapter construction.
- OpenSearch index names are now validated during startup config validation
  using the same unsafe-character and reserved-prefix guardrails as the
  adapter, so invalid index configuration fails before worker/search setup.
- Search contract expansion: clients can request `sort=relevance`,
  `include_rank=true`, and `include_highlights=true` without changing the
  default message list shape.
- Quota operations read models: capacity fields and reconciliation reporting
  show ledger pressure and drift without mutating counters.
- Quota correction actions: operators can explicitly apply reconciliation
  results to company/domain/user ledgers after reviewing drift, with dry-run and
  applied correction attempts recorded in audit logs.
- IMAP gateway planning: native backend interfaces, RFC-shaped flag/mailbox
  helpers, and durable UID/MODSEQ storage exist without starting a TCP protocol
  server.
- The first IMAP adapter path can list/get mailboxes, list mailbox messages,
  resolve messages by UID, stream raw stored message bodies, and mutate
  persisted IMAP-visible flags with MODSEQ advancement as `internal/imapgw`
  DTOs while ensuring UID state.
- Existing active mailbox contents can be backfilled with stable mailbox-local
  IMAP UIDs in bounded batches before any live IMAP listener is enabled.
- The shared `event-worker` now consumes committed `mail.stored` events through
  an IMAP UID handler that ensures newly received active messages get
  mailbox-local UIDs asynchronously after SMTP storage commits. Stale
  `mail.stored` events for messages that were moved or deleted before UID
  assignment are treated as no-ops instead of retrying forever.
- IMAP UID assignment event decoding rejects CR/LF-bearing or oversized
  message, user, and folder IDs before UID work or mailbox event fan-out.
- Push notification `mail.stored` event decoding rejects CR/LF-bearing or
  oversized message/user IDs before target resolution or candidate fan-out.
- Search indexing `mail.stored` event decoding rejects oversized message/user
  IDs and storage paths before stored EML objects are opened.
- Mail receive audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery status audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery `mail.queued` decoding rejects oversized message identities and
  storage paths, and rejects ambiguous, absolute, parent-traversal,
  backslash-bearing, or non-`.eml` storage object keys before SMTP transport or
  message storage access.
- Delivery `mail.queued` DSN option decoding rejects oversized
  `original_recipient` values before retry/delivery attempt recording.
- Delivery `mail.queued` decoding rejects oversized recipient and DSN-recipient
  arrays before normalization, routing, or retry bookkeeping.
- Authenticated Submission now applies enforcing per-domain recipient caps during
  `RCPT TO`, not only after `DATA`, so oversized envelopes receive earlier
  SMTP feedback before message streaming/spooling.
- Attachment scanner hook rejection/tempfail reasons are CR/LF-stripped and
  UTF-8 safely bounded before they are surfaced as SMTP hook errors.
- `GOGOMAIL_ENV` now accepts only `development`, `test`, or `production`, so
  environment typos cannot silently bypass production-only safety gates.
- Redis-backed deduplication, recipient rate limiting, and SMTP backpressure
  backend selectors now accept only `none` or `redis`, preventing typos from
  silently disabling operational controls.
- Redis-backed RCPT rate-limit keys now normalize remote addresses to the
  remote host/IP bucket instead of the full `ip:port`, preventing source-port
  churn from bypassing recipient abuse controls.
- RCPT rate-limit and outbox relay batch, poll, and max-attempt settings are
  now validated as positive values during startup config validation, surfacing
  relay/limit misconfiguration before workers start.
- HTTP, SMTP, inbound SMTP, Submission, and optional SMTPS listener addresses
  are now validated as TCP `host:port` values at startup, surfacing bind
  configuration mistakes before runtime listener setup.
- Delivery retry delay schedules and maximum delay caps are now validated as
  positive durations, preventing retry jobs from being exhausted accidentally
  or scheduled in the past by malformed runtime configuration.
- `GOGOMAIL_DELIVERY_SMTP_HELLO` is now validated as a non-empty
  whitespace-free hostname during startup config validation, surfacing outbound
  SMTP EHLO configuration mistakes before delivery worker startup.
- Attachment scanning can be enabled with a configured HTTP webhook backend;
  the hook remains disabled by default, supports an optional bounded bearer
  token, requires HTTPS in production, and is wired only at SMTP
  receive/submission app boundaries. `docs/webhook-integrations.md` documents
  the scanner request, bounded response, and accept/reject/tempfail verdict
  contract. Scanner webhook requests bound and normalize message, address,
  subject, recipient, and attachment metadata before JSON serialization.
- Redis duplicate-message detection uses fixed-length hashed dedup keys so raw
  message IDs or recipient addresses cannot create oversized Redis keys.
- Mail API move/delete operations invalidate stale IMAP UID rows in the same
  transaction, and IMAP UID idempotency checks require the same active
  user/mailbox before reusing an existing UID, keeping mailbox-local UID state
  from leaking across folders.
- Optional PostgreSQL integration coverage now exercises IMAP UID backfill and
  move invalidation when `GOGOMAIL_TEST_DATABASE_URL` is available.
- `internal/imapgw` has a small in-memory mailbox event broker for future IDLE
  fan-out without introducing a protocol listener yet; broker delivery is
  scoped by both user and mailbox to preserve tenant isolation.
- `mailservice.StoreIMAPFlags` can publish IMAP mailbox `flags` events through
  an optional event publisher after repository flag mutations succeed.
- Mail API single and bulk flag mutations can look up existing IMAP UID rows and
  publish mailbox `flags` events for UID-visible messages after the database
  update succeeds.
- Mail API detail reads that auto-mark unread messages as read now also publish
  mailbox `flags` events for UID-visible messages after the read-flag write
  succeeds.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages after the database move succeeds.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages after soft-delete succeeds.
- `mailservice` exposes IMAP mailbox/message listing and mailbox-event
  subscription methods, keeping the protocol listener pointed at the service
  boundary instead of `maildb` internals.
- `mailservice` exposes bounded IMAP UID backfill through the same service
  boundary for future operator/bootstrap modes.
- IMAP mailbox event publication from service mutations is best-effort, so a
  fan-out failure does not turn an already-committed mail mutation into a client
  error.
- `mailservice` has an `IMAPStoreAdapter` that satisfies `imapgw.Store`, so a
  protocol listener can depend on the gateway interface while still routing
  through service methods.
- `IMAPStoreAdapter` now also satisfies `imapgw.MailboxSessionStore` for
  mailbox selection and event subscription, while MOVE and EXPUNGE return an
  explicit unsupported mutation error until IMAP-safe semantics are reviewed.
- IMAP `UID FETCH` and `UID STORE` untagged `FETCH` responses use message
  sequence numbers per RFC 3501 while keeping the requested UID in response
  attributes, and `RFC822.SIZE` metadata requests do not trigger body streaming.
- IMAP `UID FETCH` accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]` for clients that batch reads without read-flag side effects.
- IMAP non-UID `FETCH` accepts bounded sequence sets, including `*`, and maps
  them through the selected mailbox list before returning fetch responses.
- IMAP `EXAMINE` supports read-only mailbox selection and blocks `UID STORE`
  mutations in that state.
- IMAP `CHECK` and `CLOSE` support selected-mailbox lifecycle handling; `CLOSE`
  clears selected state without enabling EXPUNGE/`\Deleted` behavior yet.
- IMAP `STATUS` validates requested status data items and returns only the
  requested mailbox metadata fields.
- IMAP `LIST` filters mailbox responses with exact, `*`, and `%` patterns over
  sanitized wire names.
- IMAP `SELECT`/`EXAMINE` emit `[PERMANENTFLAGS]` response codes for writable
  versus read-only selected-mailbox state.
- IMAP `UID STORE` supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses when requested.
- IMAP `FETCH`/`UID FETCH` can include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes from message summaries for mailbox list rendering.
- IMAP `FETCH`/`UID FETCH` can return a conservative single-part
  `BODYSTRUCTURE` response; full MIME tree serialization remains future work.
- IMAP `FETCH`/`UID FETCH` can stream bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- IMAP `FETCH`/`UID FETCH` can stream text-only literals for `BODY[TEXT]`,
  `BODY.PEEK[TEXT]`, and `RFC822.TEXT`.
- IMAP `UID STORE` accepts bounded UID sets/ranges for batched flag mutation.
- IMAP `CAPABILITY` drops `AUTH=PLAIN` after authentication, and unsupported
  literal tokens are rejected instead of being treated as ordinary atoms.
- IMAP `AUTHENTICATE PLAIN` supports the standard continuation response,
  cancellation, and SASL PLAIN credential decoding over the existing protocol
  auth adapter.
- `gogomail --mode=imap` initializes the service-backed IMAP store adapter,
  a process-local mailbox event broker for future IDLE/session fan-out, and the
  configured TCP protocol listener.
- Runtime config now includes validated `GOGOMAIL_IMAP_ADDR` listener metadata
  for the IMAP protocol listener.
- EML parser guardrails include a truncation-probe test and benchmark for the
  bounded text-body reader on large bodies, plus a `MaxParts` cap that reports
  `PartsTruncated` for pathological MIME part counts, plus address/reference
  metadata caps for oversized headers.
- EML attachment detection records inline parts with filenames and non-text
  inline parts from headers only, improving `has_attachment` accuracy without
  reading attachment bodies.
- Push notification worker boundary: `mail.stored` can be consumed by a
  dedicated notification worker with a replaceable sink and a bounded Postgres
  device-target resolver plus candidate-attempt persistence.
- Push notification attempts are inspectable through the Admin API without
  introducing vendor push delivery as a required runtime dependency.
- Push notification device storage: authenticated users can register, list, and
  delete active device tokens through the Mail API while responses expose only a
  short token suffix. Registration normalizes user, platform, token, and label
  fields before validation/storage.
- API metering boundary: HTTP middleware can emit fail-open usage events to
  logs or the durable outbox, while the disabled-by-default aggregation worker
  can build daily/monthly Postgres read models for operations.
- API metering events now carry an explicit schema version and deterministic
  event ID groundwork for future idempotent billing-grade aggregation.
- API metering aggregation now has an `api_usage_events` idempotency ledger so
  duplicate `event_id` deliveries do not increment daily/monthly counters again.
- API metering Admin API responses now expose tenant/company/domain/user/API-key,
  principal, and auth-source dimensions for daily/monthly aggregates.
- API metering Admin API now exposes immutable ledger list/export/stats endpoints
  so future billing and warehouse jobs can consume event-level usage instead of
  operational aggregates.
- API usage export batch manifests now capture fixed event/request/byte/latency
  totals for an explicitly bounded ledger window, preparing idempotent
  downstream export workflows.
- API usage export artifact metadata is now persisted and inspectable through
  Admin API endpoints, preparing object-store handoff without adding a vendor
  dependency to the core service.
- API usage export manifests now have canonical SHA-256 digest generation,
  local-HMAC/local-Ed25519/remote-Ed25519 signing, and verification Admin API
  endpoints, tightening the audit trail before invoice-grade signer deployment.
- API usage export artifact writing now has a local object-store adapter path
  through Admin API, including full-batch streaming, retry-friendly artifact
  registration, stored artifact download, and object body byte/SHA verification.
- API usage export handoff readiness now has a compact Admin API report that
  shows whether a batch has artifact coverage, a latest manifest digest, and a
  signature for that digest while keeping local signatures billing-blocked until
  production signing is wired.
- API usage export handoff readiness can now run an explicit deep verification
  mode for release/warehouse checks, returning artifact, digest, and signature
  verification evidence plus `verified_billing_ready` without turning the
  default readiness read into an object-store streaming operation.
- Attachment policy hardening: domain outbound policy can cap individual
  attachment upload sizes.
- Domain policy service lookups trim domain and user identifiers before
  repository policy reads for outbound and attachment enforcement.
- Direct multipart attachment uploads now distinguish over-limit HTTP request
  envelopes from malformed multipart bodies, returning 413 for the former and
  preserving 400 for bad multipart syntax.
- Attachment upload reservation and direct-upload service requests normalize
  user, draft, filename, MIME type, and storage-path metadata before quota,
  storage, and repository work, and reject CR/LF-bearing or oversized draft
  identifiers before quota reservation or object writes.
- Stale attachment-upload cleanup validates its time window and limit at the
  service boundary before repository cleanup/object deletion work, and app
  configuration validates the worker interval, stale age, and batch size before
  runtime. Stored-object delete failures are now surfaced to the caller while
  missing objects are treated as already-cleaned idempotent deletes. Cleanup
  batch sizes use an attachment-specific 1000-row cap instead of the smaller
  message-list pagination cap.
- Admin API exposes `POST /admin/v1/attachment-cleanup/runs` for authenticated
  on-demand stale upload cleanup with an explicit non-future RFC3339 cutoff,
  and supports `dry_run` preview responses with total and batch-limited
  candidate counts before destructive cleanup. Cleanup run responses also
  report upload-session candidate and expired counts so operator dry-runs match
  the background worker's full cleanup scope. Operators can also list bounded
  legacy attachment-upload candidates plus stale upload-session candidates through
  `POST /admin/v1/attachment-cleanup/candidates`.
- Mail API exposes `DELETE /api/v1/attachments/{id}` so users can cancel
  unbound pending uploads immediately, releasing quota and removing any stored
  upload object without waiting for stale cleanup. Draft binding and send
  handoff ignore canceled/deleted uploads, and canceling a draft-bound upload
  clears the draft binding while refreshing the draft attachment-state cache.
- Mail API exposes `GET /api/v1/attachments/capabilities` so clients can
  discover upload limits, supported modes, and resumable-upload readiness
  without hard-coded constants.
- ADR 0007 records the future resumable/chunked upload boundary around explicit
  upload sessions, quota reservation, storage adapters, final attachment rows,
  and bounded cleanup.
- A migration now creates `attachment_upload_sessions` with lifecycle status,
  declared/received byte counts, expiry, checksum, storage adapter metadata, and
  indexes for user lookup and stale-session cleanup.
- `maildb` can create upload session records and reserve declared session bytes
  in the shared quota ledger transactionally.
- `maildb` can cancel resumable upload sessions in `pending`, `uploading`, or
  `failed` state, releasing the declared byte reservation once.
- `maildb` can expire stale resumable upload sessions in bounded batches,
  marking them `expired` and releasing declared quota reservations.
- `maildb` can count stale resumable upload sessions with the same normalized
  cleanup batch cap used by expiry, supporting non-destructive Admin previews.
- `maildb` can list bounded stale resumable upload-session candidates for
  operator cleanup previews without mutating quota reservations.
- Admin API can list bounded attachment upload sessions by user, draft, and
  lifecycle status, giving operators a direct inspection surface before cleanup
  or user-support actions.
- `mailservice` now owns resumable upload session create/cancel/expire methods,
  preserving attachment validation, max-size checks, and domain attachment
  policy enforcement above the repository boundary.
- `attachment-cleanup-worker` now expires stale resumable upload sessions during
  its normal bounded sweep, releasing reserved quota alongside stale direct
  upload cleanup.
- Mail API now exposes resumable upload session create/read/cancel endpoints under
  `/api/v1/attachments/upload-sessions`, reserving declared quota at session
  creation while keeping chunked upload capability disabled until upload/finalize
  routes land. Session creation rejects already-expired `expires_at` values
  before quota reservation.
- Attachment upload capabilities now distinguish upload session availability
  from full resumable chunk support so generated clients can adopt the staged
  lifecycle without assuming chunk receive/finalize routes exist, and expose
  the maximum upload session TTL.
- Mail API can store a complete body for an upload session, persisting it under
  session-scoped storage and recording received bytes plus SHA-256 digest before
  finalize creates the normal attachment row.
- Upload session body replacement writes retries to distinct staged object paths
  before repository metadata updates, preserving the previously recorded body if
  the DB update fails and best-effort deleting the previous staged body after a
  successful replacement.
- Upload-session staged object paths are validated as relative
  `upload-sessions/` keys before repository persistence and again before
  service-side storage reads/deletes, reducing risk from corrupted or manually
  edited rows.
- Upload session body storage can verify an optional client-provided
  `X-Content-SHA256` digest before recording the staged body.
- Upload session body storage now rejects repeated `Content-Range` or
  `X-Content-SHA256` control headers before reading or storing the body.
- Attachment upload capabilities now advertise upload session checksum
  precondition support separately from body storage and finalization support.
- Upload session finalization now converts a ready stored session body into the
  normal pending attachment row without double-reserving quota, and marks the
  session finalized.
- Upload session finalization now verifies the staged object exists and still
  matches the recorded size and SHA-256 before creating the attachment row.
- Upload session cancellation now deletes a staged session body when the
  canceled session has already written one, preventing storage leaks alongside
  quota release.
- Upload session expiry now also deletes staged session bodies after the
  repository marks sessions expired and releases quota.
- Attachment list/download and draft-delete service methods trim user, message,
  attachment, and draft identifiers before repository/storage work; attachment
  reads reject blank, CR/LF-bearing, or oversized message/attachment
  identifiers before repository/storage dispatch.
- Mail API path identifiers and direct-upload `draft_id` fields are trimmed at
  the HTTP boundary before service dispatch, and direct multipart uploads reject
  repeated `draft_id` or `file` parts before storage work begins.
- Mail API search query, folder, sender, and subject filters are trimmed at the
  HTTP and service boundaries before search backend dispatch, and service
  search validation rejects CR/LF-bearing or oversized query/filter fields
  before Postgres or OpenSearch work.
- Mail compose draft/save/send requests normalize user/source/from/address and
  attachment identifier fields at the service boundary before repository,
  storage, suppression, and outbound composition work; draft saves share the
  send-time attachment-count cap so oversized compose payloads cannot drift
  into draft storage, and from/subject plus recipient display names/emails
  reject CR/LF before draft persistence or outbound header composition.
- Draft save/delete/send and reply/forward compose validation reject blank,
  CR/LF-bearing, or oversized draft/source-message identifiers before
  repository dispatch.
- Single-message flag, move, and delete service methods trim user/message/flag
  and folder identifiers before repository mutation and IMAP event fan-out, and
  reject blank, CR/LF-bearing, or oversized message/folder identifiers before
  repository or IMAP UID lookup work.
- Bulk flag, move, and delete service methods also trim user/message/flag and
  folder identifiers before repository mutation, IMAP UID lookup, and mailbox
  event fan-out; bulk message and folder identifiers reject CR/LF and oversized
  values before database query construction.
- Folder, message-list, thread-list, and message-detail service reads trim
  user, folder, thread, message, and folder-name inputs before repository work;
  user folder create/rename rejects blank, path-bearing, CR/LF-bearing, or
  oversized names, and rename/delete reject unsafe folder identifiers before
  repository dispatch. Folder-scoped message lists and thread-message reads
  also reject unsafe folder/thread identifiers before repository work.
- Message, thread, and push-device list service methods normalize list limits
  to the documented message-list bounds before repository work.
- Message-list cursor decoding rejects oversized opaque cursor strings before
  base64 decode and JSON parsing.
- IMAP service methods trim user/mailbox identifiers and normalize list/backfill
  limits before repository, storage, broker, or mailbox-event work.
- Mail search service queries normalize user, text, folder, sender, subject,
  and sort inputs before Postgres or OpenSearch dispatch.
- Message delivery-status and reply source-thread service lookups trim user,
  message, and source-message identifiers before repository work.
- Admin API domain query identifiers for user listing, DKIM key listing, and
  delivery-route resolution are trimmed at the HTTP boundary before service
  dispatch.
- Admin API DKIM key deactivate and DNS-verify path identifiers are trimmed at
  the HTTP boundary before service dispatch and response envelopes.
- Admin API suppression-list and trusted-relay delete path identifiers are
  trimmed at the HTTP boundary before service dispatch and response envelopes.
- Admin API trusted relay listing now supports bounded CIDR and description
  filters so operators can inspect inbound relay policy without client-side
  full-list scans.
- Admin API company, domain, and user quota/status/policy mutation path
  identifiers are trimmed at the HTTP boundary before service dispatch and
  response envelopes.
- Admin API outbox event topic, partition key, and status filters are trimmed
  at the HTTP boundary before operational queue inspection, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin API delivery-attempt status and recipient-domain filters are trimmed at
  the HTTP boundary before retry/bounce inspection, and CR/LF-bearing or
  oversized filter values are rejected before service dispatch.
- Admin API push-notification attempt and stats filters are trimmed at the HTTP
  boundary before device/provider troubleshooting queries, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin push-notification attempt/stats repository filters also reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- OpenAPI drift tests now pin the push-device list `limit` query parameter so
  generated clients keep pagination controls for device management.
- OpenAPI drift tests now pin attachment reservation/direct-upload HTTP 413
  error responses for size-cap failures.
- Mail and Admin API JSON request handlers now reject trailing JSON tokens and
  unknown object fields before service dispatch, and common JSON request
  decoding is capped at 1 MiB before parsing.
- Mail and Admin API JSON mutation bodies now require `Content-Type:
  application/json`, accepting normal media-type parameters such as
  `charset=utf-8` but rejecting missing, repeated, or non-JSON content types
  before dispatch.
- Mail API read and bodyless mutation routes now reject request bodies and
  `Content-Type` headers before dispatch, preventing ignored JSON or multipart
  metadata on resource reads, deletes, draft-send, upload-session finalization,
  capability discovery, downloads, and push-device list/delete operations.
- Admin GET/DELETE routes and bodyless Admin POST commands now reject request
  bodies and `Content-Type` headers before dispatch, preventing ignored payloads
  on operator reads, deletes, route verification, retry, IMAP UID backfill,
  API-usage export-batch creation, and manifest digest/signature creation.
- Health and service-info GET routes now reject request bodies and
  `Content-Type` headers before writing probe or contract metadata responses,
  keeping bodyless read semantics consistent across HTTP surfaces.
- Health and service-info GET routes now also reject unknown query parameter
  names, making release probe and metadata endpoint typos visible as HTTP 400
  instead of silently ignored inputs.
- Admin bodyless command/delete routes for IMAP UID backfill, DKIM DNS verify,
  outbox retry, DKIM deactivation, suppression deletion, trusted-relay
  deletion, and delivery-route deletion now reject unknown query parameter names
  before dispatch, preventing ignored `dry_run`/`force`-style operator flags.
- Admin JSON mutation routes for tenant quotas, domain/user lifecycle and
  policy, backpressure, attachment cleanup, quota correction, push outcomes,
  trusted relays, delivery routes, and DKIM keys now reject unknown query
  parameter names before dispatch.
- Mail JWT and Admin token authentication now reject repeated credential
  headers, and Admin routes reject mixed `X-Admin-Token` plus bearer credentials
  before dispatch.
- Mail and Admin API scalar query parameters now reject duplicate values before
  dispatch, preventing ambiguous user IDs, list limits, booleans, timestamps,
  and operational filters from being interpreted by first-value wins behavior.
- Mail API read/search/list, draft-search, attachment capability/session/download,
  and push-device list routes now reject unknown query parameter names before
  dispatch, making generated-client typos visible as HTTP 400 responses.
- Mail API mutation routes now reject unknown query parameter names before
  dispatch, and JSON-backed compose/draft/attachment/send mutations honor the
  documented development-only `user_id` query fallback when JWT auth is
  disabled.
- Admin company/domain/DNS-check/user list routes now reject unknown query
  parameter names before dispatch, keeping core operator filters aligned with
  the documented contract.
- Admin API usage aggregate, ledger, retention, export-batch, artifact,
  manifest-digest, and manifest-signature routes now reject unknown query
  parameter names before dispatch, including unexpected query strings on
  detail, download, verification, and mutation routes with no query controls.
- Admin queue, outbox, audit, backpressure, quota, attachment-session,
  delivery-attempt, push-notification, suppression-list, trusted-relay,
  delivery-route, and DKIM read routes now reject unknown query parameter names
  before dispatch.
- API error responses now use `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`, with the reusable OpenAPI error response
  documenting both headers for generated clients.
- Successful Mail/Admin JSON, health, and service-info responses now return
  `X-Content-Type-Options: nosniff`, aligning browser-visible envelopes with
  error, NDJSON, and download response hardening.
- Successful Mail/Admin JSON responses now return `Cache-Control: no-store`
  through the shared writer so sensitive message, audit, usage, and control
  envelopes are not cached.
- Attachment download responses now emit both ASCII fallback and UTF-8
  `filename*` `Content-Disposition` parameters for internationalized filenames,
  with stored filenames bounded before response headers are written.
- Attachment downloads now fall back to `application/octet-stream` for blank,
  unsafe, or media-type-invalid stored MIME types before setting response
  headers.
- OpenAPI now documents attachment download `Content-Disposition` and
  `Cache-Control: no-store` headers with drift coverage.
- API usage artifact downloads now sanitize stored content type and SHA-256
  response headers before streaming export objects, including media-type
  validation before the response `Content-Type` is written.
- API usage outbox production now rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before inserting durable usage events.
- API usage aggregate storage now applies the same route-key, identity, event-id,
  schema-version, and HTTP-like status validation when called directly by
  internal adapters.
- API usage NDJSON exports and stored export artifact downloads now return
  `Cache-Control: no-store`, with OpenAPI drift coverage.
- Attachment downloads, usage NDJSON exports, and stored export artifact
  downloads now return `X-Content-Type-Options: nosniff`, with OpenAPI drift
  coverage.
- Successful JSON responses now return `X-Content-Type-Options: nosniff` across
  Mail, Admin, health, and service-info routes.
- Successful Mail/Admin JSON envelopes now use `Cache-Control: no-store` through
  the shared writer.
- Mailservice now validates DB-returned message and attachment storage object
  paths before body reads or deletes, preventing corrupted rows from reaching
  the storage adapter with absolute, traversal, newline, backslash-bearing, or
  oversized keys.
- Local storage now shares the strict object-path validator used by mailservice,
  rejecting non-canonical, oversized, duplicate-separator, or dot-segment keys
  at the adapter boundary before reads, writes, or deletes.
- Backend release verification now fails when standard tests leave pending
  repository changes behind, while local OpenChrome session artifacts are
  ignored as developer-machine state.

Next focus areas:

1. Keep draft search separate from `GET /api/v1/search` until an explicit draft
   search contract and indexing path are added.
2. Extend the quota ledger to future Drive writes and large share-link objects.
3. Wire mailbox event publication from append/flag/move/delete paths behind the
   IMAP gateway boundary.
4. Add FCM/APNs/Web Push sink adapters and invalid-token cleanup behind the push
   notification worker.
5. Deploy the remote-Ed25519 signer behind an approved KMS service, or add a
   direct cloud KMS adapter, before using API usage batches for invoices or hard
   limits.
6. Frontend planning and API contract review before webmail implementation.
