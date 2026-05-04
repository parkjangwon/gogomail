# gogomail backend API contracts

This document is the staging contract for a future OpenAPI specification. It is intentionally backend-only and does not start frontend implementation.

The machine-readable draft now lives at `docs/openapi.yaml`. Treat that file as
the source to refine before generating frontend clients or publishing external
API docs.

The OpenAPI draft is intentionally guarded by backend tests. If a Go HTTP route,
backend contract version, request body, supported message flag, list limit, or
component reference changes, update the implementation and `docs/openapi.yaml`
in the same commit.

For generated-client stability, every documented operation has a stable
lower-camel `operationId`, protected/mutable operations reuse the default
`components.responses.Error` response, and non-auth metadata endpoints such as
`GET /api/v1/info` explicitly opt out of global bearer auth.

## Contract metadata

- Public mail API base path: `/api/v1`
- Admin API base path: `/admin/v1`
- Service info: `GET /api/v1/info`
- Current backend contract version: `2026-05-04.backend-release`

## Response envelopes

`docs/openapi.yaml` documents successful JSON responses through reusable
`components.responses.*` entries. Backend handlers and the OpenAPI draft must
keep the top-level envelope key stable so generated clients can model each
operation without path-specific ad-hoc response decoding.

Successful collection responses keep a stable top-level plural key:

- `{"folders":[...]}`
- `{"messages":[...],"limit":50,"has_more":false,"next_cursor":"..."}`
- `{"threads":[...]}`
- `{"attachments":[...]}`
- `{"push_devices":[...]}`
- `{"domains":[...]}`
- `{"users":[...]}`
- `{"queues":[...]}`
- `{"outbox_events":[...]}`
- `{"delivery_attempts":[...]}`
- `{"push_notification_attempts":[...]}`
- `{"suppression_list":[...]}`
- `{"dkim_keys":[...]}`

Successful resource responses keep a stable singular key:

- `{"message":{...}}`
- `{"delivery_status":{...}}`
- `{"draft":{...}}`
- `{"attachment":{...}}`
- `{"push_device":{...}}`
- `{"domain":{...}}`
- `{"user":{...}}`

Successful mutation responses use one of:

- `{"status":"ok"}`
- `{"status":"ok","id":"..."}`
- `{"status":"ok","updated":2}`

Mail API send responses use `{"message":{...}}` with explicit status fields:

- `send_status: "queued"` means the message has been accepted into backend outbound processing.
- `delivery_status: "pending"` means no final delivery result has been recorded yet.
- `bounce_status: "none"` means no bounce has been generated for this queued send response.

Errors use the stable envelope:

```json
{
  "error": {
    "code": "bad_request",
    "message": "limit must be positive",
    "status": 400,
    "status_text": "Bad Request"
  },
  "error_message": "limit must be positive"
}
```

`error_message` remains temporarily for backward compatibility. New clients should read `error`.

## Authentication

- Mail API uses HS256 bearer JWT when `GOGOMAIL_AUTH_JWT_SECRET` is configured.
- Without JWT configuration, development requests must pass `user_id` as a query parameter.
- Admin API uses `Authorization: Bearer <token>` or `X-Admin-Token` when `GOGOMAIL_ADMIN_TOKEN` is configured.

## Pagination

List endpoints that accept `limit` reject non-integer, nonpositive, and over-200 values.
The documented client contract is `1 <= limit <= 200`, defaulting to `50`.
Message listing returns opaque `next_cursor`; clients must not parse or
manufacture cursors.

Search query, folder, sender, and subject filters are whitespace-normalized and
reject CR/LF-bearing or oversized values before either Postgres or OpenSearch
dispatch.

## Folders

User-created folder names reject blank, path-bearing, CR/LF-bearing, or
oversized values. Folder rename/delete identifiers are whitespace-normalized and
reject blank, CR/LF-bearing, or oversized IDs before repository dispatch.

## Mailbox bulk actions

Bulk mailbox mutations are bounded to 500 unique message IDs per request and only affect active messages owned by the authenticated user.

- `PATCH /api/v1/messages/bulk/flags`
  - Body: `{"message_ids":["..."],"flag":"read|starred|answered|forwarded","value":true}`
  - Response: `{"status":"ok","updated":2}`
- `PATCH /api/v1/messages/bulk/folder`
  - Body: `{"message_ids":["..."],"folder_id":"..."}`
  - Response: `{"status":"ok","updated":2}`
- `POST /api/v1/messages/bulk/delete`
  - Body: `{"message_ids":["..."]}`
  - Response: `{"status":"ok","updated":2}`

Bulk endpoints reject missing, blank, duplicate, over-limit, CR/LF-bearing, or
oversized message IDs instead of silently ignoring ambiguous client intent.

## Compose requests

Draft save/update and immediate send requests share the same compose guardrails
for subject size, text-body size, recipient syntax/count, intent/source rules,
attachment IDs, and CR/LF-free header-bearing compose fields. Drafts may omit
recipients while a message is still being composed, but draft attachment ID
lists are capped at the same 100-item limit as send requests so clients cannot
persist oversized compose payloads for later send-time rejection.
Draft IDs and reply/forward source message IDs are also bounded and reject
CR/LF-bearing values before repository dispatch.

## Attachment lifecycle

Attachment uploads start as `uploading`, become draft-bound or message-bound
records when saved/sent, and stale `uploading` records can be expired by
backend cleanup code. Upload metadata creation reserves bytes in the shared
company/domain/user quota ledger; stale upload cleanup marks rows `deleted`,
returns those bytes to the quota ledger, and then asks the configured storage
backend to remove the object. Mail API maps quota exhaustion to HTTP 507
`insufficient_storage` while the SMTP layer continues to use SMTP-appropriate
mailbox-full responses.

Direct multipart attachment uploads are capped at the HTTP request boundary in
addition to service-level declared-size and domain-policy checks. Multipart
requests that exceed the direct upload envelope return HTTP 413
`payload_too_large`; malformed multipart bodies that are within the cap remain
HTTP 400 `bad_request`. Mail API path identifiers and the direct upload
multipart `draft_id` field are trimmed at the HTTP boundary before service
dispatch, keeping user-facing routes tolerant of incidental whitespace without
storing whitespace-padded resource IDs. Attachment reservation and direct-upload
`draft_id` values reject CR/LF-bearing or oversized identifiers before quota
reservation or object writes.

Mail and Admin API JSON request bodies must contain exactly one JSON value.
Handlers reject trailing JSON tokens as HTTP 400 `bad_request` instead of
silently dispatching the first decoded object.

Attachment downloads set private `no-store` responses and include both a safe
ASCII `filename` fallback and a UTF-8 `filename*` parameter in
`Content-Disposition` so internationalized filenames survive browser downloads
without permitting header injection. Unsafe or blank stored attachment MIME
types fall back to `application/octet-stream` at the download boundary. The
OpenAPI contract documents the binary media type plus `Content-Disposition` and
`Cache-Control` response headers for generated clients.

API usage artifact downloads apply the same defensive response-header stance:
unsafe or blank stored content types fall back to `application/x-ndjson`, and
the `X-Gogomail-Artifact-SHA256` header is emitted only when the stored digest
is a valid 64-character lowercase/uppercase hexadecimal SHA-256 value. API
usage ledger NDJSON exports, batch replay exports, and stored artifact
downloads all return `Cache-Control: no-store` because usage exports are
sensitive operational/billing data. Attachment downloads, usage NDJSON exports,
and stored usage artifact downloads also return `X-Content-Type-Options:
nosniff` so browsers do not reinterpret streamed bytes as another content type.

## Push devices

Push notification device tokens are user-scoped Mail API resources:

- `GET /api/v1/push-devices`
- `POST /api/v1/push-devices`
- `DELETE /api/v1/push-devices/{id}`

Supported platforms are `apns`, `fcm`, and `webpush`. Create/update accepts the
raw token, but API responses do not return the raw token; clients receive only
`token_suffix` for diagnostics and display. Delete is a soft delete scoped to
the authenticated user; delete device IDs are whitespace-normalized and reject
blank, CR/LF-bearing, or oversized values before repository dispatch.

When `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=slog`, `push-notification-worker`
resolves active devices for the `mail.stored.user_id` after commit and before
invoking its sink. `GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT` bounds per-message
fan-out. The resolver drops malformed targets with blank or CR/LF-bearing
device IDs/tokens, or unsupported platforms before invoking the sink. Vendor delivery
remains a future sink adapter, not a Mail API or SMTP side effect. The worker
records one `push_notification_attempts` candidate row
per resolved device before invoking the current sink. After a successful sink
handoff, the worker records `queued` for each generated attempt id; if the sink
fails, the worker records `failed` with the sink error while still returning the
handler error so the event can be retried by the stream consumer. The generated
attempt id is attached to each sink target so future vendor
adapters can update that exact row with delivered, failed, or invalid-token
outcomes without coupling notification delivery to the SMTP transaction. Outcome
updates are available inside `internal/pushnotify` and are not exposed as a
public API. Outcomes may also record provider-specific message IDs and status
codes for adapter audit. An `invalid_token` outcome soft-deletes the matching
user device in the same database transaction as the attempt update.

The committed `mail.stored` event includes
`schema_version: "2026-05-04.mail-stored.v1"` plus message, tenant, recipient,
subject, storage, DSN, and authentication fields used by audit, search, and
push workers. Downstream workers should treat the schema version as the
compatibility boundary for future event changes; current audit, search, and
push consumers reject unknown explicit versions while accepting legacy
versionless events.

## Admin operations

Admin domain/user CRUD includes list, detail, create, status update, and quota update contracts:

- `GET /admin/v1/companies`
- `GET /admin/v1/companies/{id}`
- `PATCH /admin/v1/companies/{id}/quota`
- `PATCH /admin/v1/domains/{id}/quota`
- `PATCH /admin/v1/domains/{id}/policy`
- `PATCH /admin/v1/users/{id}/quota`

`quota_limit: 0` clears the limit and negative values are rejected.
Quota semantics follow ADR 0003: company owns the contracted storage pool,
domains receive allocations within that pool, and users receive unified personal
storage usable across mailbox, attachments, future Drive, and other user-owned
features. Domain default user quota changes should apply to users that still
follow the default while preserving explicit custom user quota overrides.
Runtime quota writes now increment/decrement the company, domain, and user
ledgers atomically inside the same PostgreSQL transaction for mail storage
growth/delete flows and attachment upload/cleanup flows. User quota responses
expose `quota_source` as `default|custom`, and domain quota updates may carry
`default_user_quota`.
Domain policy updates store a backend-only operational model under
`domains.settings.policy` with `inherit|monitor|enforce` inbound/outbound modes
and optional max-recipient/max-message-byte/max-attachment-byte guardrail hints. SMTP core should
continue to treat these as policy-boundary data until explicit runtime wiring is
added. Mail API send/draft-send now reads the outbound domain policy after
resolving the authenticated sender. In `outbound_mode=enforce`, it rejects
messages whose unique recipient count exceeds `max_recipients_per_message` or
whose composed RFC 5322 message size exceeds `max_message_bytes`. Attachment
metadata reservation and direct multipart upload also reject files larger than
`max_attachment_bytes` when outbound policy is enforced. `monitor` and `inherit`
remain non-blocking.

Admin operational read models also keep explicit envelope keys:

- `GET /admin/v1/companies` returns `{"companies":[...]}`
- `GET /admin/v1/companies/{id}` returns `{"company":{...}}`
- `GET /admin/v1/queue` returns `{"queues":[...]}` with grouped topic/status
  totals plus ready, delayed, stale-processing, oldest-ready, and
  next-available metadata for operator dashboards.
- `POST /admin/v1/imap/mailboxes/{id}/uid-backfill?user_id=...&limit=...`
  returns `{"imap_uid_backfill":[...]}` with bounded mailbox-local UID
  assignments for future IMAP bootstrap/operator runs.
- `GET /admin/v1/outbox-events` returns `{"outbox_events":[...]}`;
  optional `topic`, `partition_key`, `status`, and RFC3339 `since` filters
  expose outbox event metadata without returning JSON payload bodies. List
  responses include a UTF-8-safe bounded `last_error` preview.
- `GET /admin/v1/outbox-events/{id}` returns `{"outbox_event":{...}}` with full
  event metadata and full stored `last_error`, still without returning the JSON
  payload body.
- `GET /admin/v1/backpressure` returns `{"backpressure":{...}}`
- `GET /admin/v1/quota-usage` returns `{"quota_usage":[...]}`
- `GET /admin/v1/quota-reconciliation` returns `{"quota_reconciliation":[...]}`
- `GET /admin/v1/delivery-attempts` returns `{"delivery_attempts":[...]}`;
  optional `status`, `recipient_domain`, and RFC3339 `since` filters keep
  delivery triage bounded. Attempt rows include sender, enhanced-status, and
  RFC 3461 DSN metadata (`RET`, `ENVID`, `NOTIFY`, and `ORCPT`) when captured
  by the delivery worker.
- `GET /admin/v1/delivery-attempts/stats` returns `{"delivery_attempt_stats":{...}}`;
  optional `status`, `recipient_domain`, and RFC3339 `since` filters mirror the
  attempt list and summarize total, unique-message, unique-recipient, and
  status-bucket counts.
- `GET /admin/v1/delivery-attempts/exhausted` returns `{"exhausted_attempts":[...]}`;
  optional `recipient_domain` and RFC3339 `since` filters keep terminal retry
  triage bounded.
- `GET /admin/v1/push-notification-attempts` returns `{"push_notification_attempts":[...]}`;
  optional `status`, `user_id`, and RFC3339 `since` filters keep fan-out
  inspection bounded.
- `GET /admin/v1/push-notification-stats` returns `{"push_notification_stats":{...}}`;
  optional `user_id` scopes active-device and attempt-status totals to one user,
  while optional RFC3339 `since` scopes attempt-status totals to recent attempts.
- `GET /admin/v1/suppression-list` returns `{"suppression_list":[...]}`
- `GET /admin/v1/dkim-keys` returns `{"dkim_keys":[...]}`
- `GET /admin/v1/trusted-relays` returns `{"trusted_relays":[...]}`
- `GET /admin/v1/delivery-routes` returns `{"delivery_routes":[...]}`
- `GET /admin/v1/domains/{id}/dns-checks` returns `{"dns_checks":[...]}`

Admin deletion/retry/status/quota mutations return `{"status":"ok","id":"..."}`
so consoles can reconcile optimistic updates against the affected backend id.

API call metering is a roadmap item, not a blocking MVP enforcement gate.
Backend routes should be designed so a future metering middleware can aggregate
company/domain/user/api-key, route, method, status, latency, and payload-size
dimensions asynchronously. Billing/rate-limit enforcement should be policy
driven and off by default until product plans require it.

SMTP backpressure administration exposes the shared receive-pressure state used
by Edge/Inbound SMTP receive boundaries when `GOGOMAIL_BACKPRESSURE_BACKEND=redis`:

- `GET /admin/v1/backpressure`
- `PATCH /admin/v1/backpressure`

The state levels are `normal`, `warning`, `danger`, and `critical`. SMTP receive
continues accepting at `normal|warning` and temporarily rejects at
`danger|critical`. The patch endpoint accepts an optional `reason` and `until`
timestamp, keeping human/operator overrides visible without coupling Admin API
to a specific monitoring vendor.

SMTP operational administration includes trusted relay CIDR management:

- `GET /admin/v1/trusted-relays`
- `POST /admin/v1/trusted-relays`
- `DELETE /admin/v1/trusted-relays/{id}`

Trusted relay entries accept IPv4/IPv6 CIDR prefixes or plain IP addresses.
Plain IPs are canonicalized to `/32` or `/128` before persistence.

When creating a DKIM key, `public_key_dns` is optional. If omitted, the backend
derives the `v=DKIM1; k=rsa; p=...` TXT record from `private_key_pem` and stores
that public DNS value for administrator display and DNS setup checks.
`POST /admin/v1/dkim-keys/{id}/verify-dns` returns
`{"dkim_verification":{...}}` and records `dns_verified_at` when the expected
selector TXT record matches.

Outbound gateway and smart-host administration includes delivery route
management:

- `GET /admin/v1/delivery-routes`
- `POST /admin/v1/delivery-routes`
- `GET /admin/v1/delivery-routes/resolve?domain=mail.example.net`
- `GET /admin/v1/delivery-routes/counters`
- `PATCH /admin/v1/delivery-routes/{id}/status`
- `DELETE /admin/v1/delivery-routes/{id}`

Delivery routes accept an exact domain, wildcard suffix such as
`*.example.net`, or `*` as the domain pattern. Hosts are stored without ports;
the route-level port, TLS mode, implicit TLS flag, pool name, and optional SMTP
AUTH identity/username/password keep gateway policy out of SMTP protocol core.
The resolve endpoint is a dry-run observability surface; it returns
`{"delivery_route_resolution":{"domain":"...","matched":true|false,"route":...}}`
without sending mail.
Runtime delivery counters return `{"route_counters":[...]}` with per-pool
delivered, failed, retried, exhausted, and process-start `since` totals so
operators can inspect route behavior without coupling SMTP delivery to an
external metrics vendor.

Domain onboarding and deliverability checks include DNS verification:

- `GET /admin/v1/domains/{id}/dns-check`
- `GET /admin/v1/domains/{id}/dns-checks`

The response is wrapped as `{"dns_check":{...}}` and reports MX, SPF, DMARC,
and active DKIM TXT status values as `ok`, `missing`, `mismatch`, or `error`.
Each run persists the report for operational audit and records an admin audit
log entry with the summarized status.
The history endpoint returns persisted checks newest-first so admin consoles can
show onboarding progress without re-querying DNS on every page load. Domain list
and detail responses also include the latest DNS check status/timestamp when a
check has run.

User-facing delivery status is exposed through:

- `GET /api/v1/messages/{id}/delivery-status`

The read model is scoped by the authenticated/fallback user id before delivery
attempts are read, preventing cross-tenant leakage. It returns
`{"delivery_status":{...}}` with normalized delivery states
`pending|retrying|delivered|partial|failed|bounced`, bounce status
`none|hard`, and up to 200 recent attempts for webmail sent-message detail
panels.

Thread read APIs are exposed through:

- `GET /api/v1/threads`
- `GET /api/v1/threads/{id}/messages`

Thread summaries use `COALESCE(thread_id, id)` so legacy/unthreaded messages
still appear as single-message threads. Thread message lists are scoped by the
authenticated/fallback user id and returned in chronological order for webmail
conversation rendering.
Newly stored inbound mail parses RFC `In-Reply-To` and `References` headers and
attempts to inherit the matching local thread by `rfc_message_id`. Reply/forward
outbound messages inherit the source message thread when `source_message_id` is
present, preserving conversation grouping without exposing cross-user messages.
Reply composition also writes RFC `In-Reply-To` and `References` headers into
the stored/sent `.eml`, allowing external MUAs and remote recipients to retain
conversation threading.

Search indexing now has a backend boundary for received-message body text:

- `gogomail --mode=search-index-worker` can consume `mail.stored` events with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=postgres`.
- The worker reads the already-stored raw `.eml`, extracts bounded plain text
  through the shared parser, and upserts `message_search_documents`.
- `GET /api/v1/search` includes indexed received body text in the existing
  response shape.
- `sort=relevance` orders by Postgres FTS rank before the existing date/id
  tiebreakers. The default `sort=date` preserves newest-first behavior.
- `include_rank=true` adds optional `search_rank` fields to search results.
- `include_highlights=true` adds optional `search_highlights` snippets for
  subject/from/body matches. Unmarked snippets are omitted so clients do not
  render irrelevant preview text as a match.

Quota reconciliation is exposed as a read-only admin report:

- `GET /admin/v1/quota-reconciliation`
- `POST /admin/v1/quota-reconciliation/corrections`
- The report compares company/domain/user ledger counters with current
  source-of-truth message and attachment rows and returns `ledger_used`,
  `actual_used`, `delta`, and `in_sync`.
- Corrections are explicit operator actions. They acquire a transaction-scoped
  advisory lock, lock the affected quota hierarchy rows, and set ledger counters
  from current source rows rather than applying stored deltas.

API call metering can now emit durable usage events:

- Set `GOGOMAIL_API_METERING_BACKEND=outbox` to enqueue `api.usage` events into
  the generic outbox on topic `api.event`.
- Usage event payloads include
  `schema_version: "2026-05-04.api-usage.v2"`, a deterministic `event_id`, and
  tenant/company/domain/user/API-key/principal/auth-source dimensions for
  idempotent accounting and future billing enrichment.
- `auth_source` is normalized to the known set `anonymous`, `bearer`,
  `admin_token`, `query_user_id`, or `unknown`; unexpected values are folded to
  `unknown` before ledger/aggregate storage to avoid billing dimension
  cardinality drift.
- Negative request byte, response byte, and latency values from durable usage
  events are clamped to zero before ledger/aggregate storage; request count
  defaults to one when absent or nonpositive.
- API metering outbox payload creation also clamps negative byte and latency
  values to zero before deterministic event IDs are generated.
- Durable usage events must include nonblank `method` and `route` fields and an
  HTTP-like status code from 100 through 999 before they can enter ledger or
  aggregate storage.
- The metering middleware prefers the `http.ServeMux` route pattern, but falls
  back to `METHOD /path` when no pattern is available so durable events still
  have a stable nonblank route key.
- The aggregate worker claims `event_id` values before daily/monthly upserts, so
  replayed durable events do not double-count operational totals.
- The middleware remains async and fail-open; request handling does not wait on
  downstream aggregation.
- Set `GOGOMAIL_API_METERING_AGGREGATE_BACKEND=postgres` and run
  `gogomail --mode=api-metering-worker` to consume `api.event` and upsert
  daily aggregates into `api_usage_daily`.
- `GET /admin/v1/api-usage/daily` returns `{ "api_usage_daily": [...] }` with
  day/method/route/status plus tenant/company/domain/user/API-key/principal/auth
  dimensions, request/byte counters, and latency totals/maximum/average for
  operations dashboards.
- `GET /admin/v1/api-usage/monthly` returns `{ "api_usage_monthly": [...] }`
  with the same dimensions rolled up by UTC month for plan and billing analysis.
- The worker also records immutable rows in `api_usage_ledger` before updating
  aggregate read models. `GET /admin/v1/api-usage/ledger` returns
  `{ "api_usage_ledger": [...] }` and supports bounded `tenant_id`,
  `principal_id`, `from`, `to`, and `limit` queries for export preparation.
- `GET /admin/v1/api-usage/ledger/export` streams the same bounded ledger query
  as `application/x-ndjson` for downstream billing or warehouse ingestion.
- `GET /admin/v1/api-usage/ledger/stats` returns
  `{ "api_usage_ledger_stats": ... }` with count, byte, latency, and first/last
  event timestamps for export sanity checks.
- `GET /admin/v1/api-usage/ledger/retention-readiness` returns
  `{ "api_usage_ledger_retention_readiness": ... }` for a required exclusive
  `cutoff` plus optional `tenant_id`/`principal_id` filters. Future cutoffs are
  rejected so operators cannot mark an open accounting window ready for
  retention. It reports the candidate ledger counts before the cutoff and only
  marks `ready: true` when there are no candidates or a completed export batch
  with the same filters covers the full candidate time range through the cutoff,
  completed after the latest candidate row was recorded, and has artifact,
  manifest digest, and manifest signature evidence. This is a read-only safety
  gate for future archive/delete jobs; it does not mutate ledger rows.
- `POST /admin/v1/api-usage/export-batches` creates
  `{ "api_usage_export_batch": ... }`, a manifest checkpoint over the bounded
  ledger filter window with fixed event/request/byte/latency totals. The
  request requires explicit RFC3339 `from` and `to` query parameters so
  operators cannot accidentally checkpoint the entire ledger.
- `GET /admin/v1/api-usage/export-batches` returns
  `{ "api_usage_export_batches": [...] }`, and
  `GET /admin/v1/api-usage/export-batches/{id}` returns one saved manifest.
- `GET /admin/v1/api-usage/export-capabilities` returns
  `{ "api_usage_export_capabilities": ... }`, describing the configured export
  format, artifact content type, manifest digest algorithm, signer backend,
  signer key ID, verifier availability, and whether production/billing-ready
  signing is currently supported without exposing signing secrets.
- `GET /admin/v1/api-usage/export-batches/{id}/handoff-readiness` returns
  `{ "api_usage_export_handoff_readiness": ... }`, a read-only operator report
  summarizing batch completion, artifact event coverage, latest manifest
  digest, latest digest signature, operational `ready`, and separate
  `billing_ready`/`readiness_grade` fields. Local-HMAC and local-Ed25519
  signatures can satisfy operational handoff readiness but keep
  `billing_ready: false` with
  `production_manifest_signer_required` until a production signer backend is
  wired. Passing `deep=true` explicitly runs the expensive verification path:
  all registered artifacts are streamed from object storage and checked against
  persisted byte/SHA metadata, the latest manifest digest is recomputed, and the
  latest signature is verified when a verifier is available. Deep mode also
  checks that the latest digest manifest artifact list still matches the
  currently registered artifacts. Deep failures are returned as
  `deep_blocking_reasons` and `deep_verification_errors` without changing the
  metadata-only `ready` or `billing_ready` fields; clients that need object-
  verified billing evidence should read `verified_billing_ready`. Signature
  verification is behind a backend interface; if no verifier is configured for
  the latest signer backend, deep mode reports
  `manifest_signature_verifier_unavailable`.
- `GET /admin/v1/api-usage/export-batches/{id}/export` streams the saved
  manifest window as NDJSON, making export replay idempotent by batch ID.
- `POST /admin/v1/api-usage/export-batches/{id}/artifacts` registers an
  external export artifact with `object_key`, `byte_count`, `sha256_hex`,
  `event_count`, and optional metadata; artifact rows are deduplicated per batch
  by object key and SHA-256.
- `GET /admin/v1/api-usage/export-batches/{id}/artifacts` returns
  `{ "api_usage_export_artifacts": [...] }`, and
  `GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}` returns
  one registered artifact.
- `POST /admin/v1/api-usage/export-batches/{id}/artifacts/write` writes the
  saved batch window as NDJSON to the configured object store, computes byte
  count and SHA-256 while streaming, registers the artifact, and returns
  `{ "api_usage_export_artifact": ... }`.
- `GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/download`
  streams the stored artifact as `application/x-ndjson` and returns the
  persisted SHA-256 in `X-Gogomail-Artifact-SHA256`.
- `GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/verification`
  returns `{ "api_usage_export_artifact_verification": ... }`, recomputing the
  stored object byte count and SHA-256 and comparing them to persisted artifact
  metadata.
- `POST /admin/v1/api-usage/export-batches/{id}/manifest-digests` creates a
  canonical SHA-256 digest over the saved export batch and registered artifact
  metadata, returning `{ "api_usage_export_manifest_digest": ... }`.
- `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests` returns
  `{ "api_usage_export_manifest_digests": [...] }`, and
  `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}`
  returns one digest record with the stored manifest JSON.
- `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification`
  returns `{ "api_usage_export_manifest_digest_verification": ... }`, including
  expected and actual SHA-256 hex values, a `valid` boolean, and the canonical
  manifest JSON used for verification.
- `POST /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures`
  signs the digest with the configured manifest signer and returns
  `{ "api_usage_export_manifest_signature": ... }`. Local signers sign the
  lowercase 64-character manifest digest hex string. `local-hmac` emits
  `hmac-sha256` with a 64-character hex signature; `local-ed25519` emits
  `ed25519` with a 128-character hex signature. `remote-ed25519` POSTs the same
  digest/key payload to a configured HTTPS signer endpoint, requires the remote
  response to match the requested key and digest, and verifies the returned
  Ed25519 signature locally before persisting it. Local signer backends remain
  operational evidence only, not invoice-grade billing evidence.
- `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures`
  returns `{ "api_usage_export_manifest_signatures": [...] }`, and
  `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}`
  returns one persisted signature.
- `GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification`
  returns `{ "api_usage_export_manifest_signature_verification": ... }`,
  verifying the persisted signature through the configured backend verifier and
  confirming that the signed digest still matches the persisted manifest digest.

Message search starts with a small-deployment Postgres implementation:

- `GET /api/v1/search`

The current backend searches active-message metadata (`subject`, `from_addr`,
`from_name`) and indexed received-message body text using a simple Postgres FTS
expression and bounded list limits. Draft rows are intentionally excluded from
`GET /api/v1/search` until an explicit draft search contract and indexing path
are added. Search clients can opt into `sort=relevance`, `include_rank=true`,
and `include_highlights=true` while newest-first ordering remains the default.
`search-index-worker` can also write received-message documents to OpenSearch with
`GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`,
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT`, and
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX`; OpenSearch writer/searcher calls use
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`, defaulting to 10 seconds. API
read-side search uses the current backend contract and falls back to Postgres
when OpenSearch parity is not sufficient. The
OpenSearch writer includes a strict bootstrap mapping for the indexed document
shape so deployments can create the index before enabling the worker, or set
`GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true` to have the worker ensure it
at startup. Mail API can inject the OpenSearch source for relevance-sorted
searches; OpenSearch message IDs are hydrated through Postgres summaries before
responses are returned. Indexed OpenSearch documents include folder, parsed
sender, lower-cased sender/subject, and attachment-presence fields for filter
parity work, and OpenSearch relevance searches can apply folder, from, subject,
and attachment filters before hydration. Relevance tuning is metadata-first:
subject and sender matches are boosted above indexed body text on both Postgres
and OpenSearch paths. OpenSearch highlights map into the existing
`search_highlights` response shape after fragment count and byte-size bounding.
Newest-first search remains on the Postgres path so the default response
ordering stays stable.

## Deferred from this contract

- Next.js/frontend screens and shells.
- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam modules.
- IMAP protocol service, vendor push delivery adapters, Kafka, OpenSearch as the
  default/mandatory search backend, etcd, and Vault.
