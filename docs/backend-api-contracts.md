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
- `{"domains":[...]}`
- `{"users":[...]}`
- `{"queues":[...]}`
- `{"delivery_attempts":[...]}`
- `{"suppression_list":[...]}`
- `{"dkim_keys":[...]}`

Successful resource responses keep a stable singular key:

- `{"message":{...}}`
- `{"delivery_status":{...}}`
- `{"draft":{...}}`
- `{"attachment":{...}}`
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

Bulk endpoints reject missing, blank, duplicate, or over-limit message IDs instead of silently ignoring ambiguous client intent.

## Attachment lifecycle

Attachment uploads start as `uploading`, become draft-bound or message-bound records when saved/sent, and stale `uploading` records can be expired by backend cleanup code. Cleanup marks rows `deleted` first and then asks the configured storage backend to remove the object, keeping database ownership checks separate from object-store lifecycle mechanics.

## Admin operations

Admin domain/user CRUD includes list, detail, create, status update, and quota update contracts:

- `PATCH /admin/v1/domains/{id}/quota`
- `PATCH /admin/v1/domains/{id}/policy`
- `PATCH /admin/v1/users/{id}/quota`

`quota_limit: 0` clears the limit and negative values are rejected.
Quota semantics follow ADR 0003: company owns the contracted storage pool,
domains receive allocations within that pool, and users receive unified personal
storage usable across mailbox, attachments, future Drive, and other user-owned
features. Domain default user quota changes should apply to users that still
follow the default while preserving explicit custom user quota overrides.
Domain policy updates store a backend-only operational model under
`domains.settings.policy` with `inherit|monitor|enforce` inbound/outbound modes
and optional max-recipient/max-message-byte guardrail hints. SMTP core should
continue to treat these as policy-boundary data until explicit runtime wiring is
added. Mail API send/draft-send now reads the outbound domain policy after
resolving the authenticated sender. In `outbound_mode=enforce`, it rejects
messages whose unique recipient count exceeds `max_recipients_per_message` or
whose composed RFC 5322 message size exceeds `max_message_bytes`. `monitor` and
`inherit` remain non-blocking.

Admin operational read models also keep explicit envelope keys:

- `GET /admin/v1/queue` returns `{"queues":[...]}`
- `GET /admin/v1/backpressure` returns `{"backpressure":{...}}`
- `GET /admin/v1/quota-usage` returns `{"quota_usage":[...]}`
- `GET /admin/v1/delivery-attempts` returns `{"delivery_attempts":[...]}`
- `GET /admin/v1/suppression-list` returns `{"suppression_list":[...]}`
- `GET /admin/v1/dkim-keys` returns `{"dkim_keys":[...]}`
- `GET /admin/v1/trusted-relays` returns `{"trusted_relays":[...]}`
- `GET /admin/v1/delivery-routes` returns `{"delivery_routes":[...]}`
- `GET /admin/v1/domains/{id}/dns-checks` returns `{"dns_checks":[...]}`

Admin deletion/retry/status/quota mutations return `{"status":"ok","id":"..."}`
so consoles can reconcile optimistic updates against the affected backend id.

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

Outbound gateway and smart-host administration includes delivery route
management:

- `GET /admin/v1/delivery-routes`
- `POST /admin/v1/delivery-routes`
- `GET /admin/v1/delivery-routes/resolve?domain=mail.example.net`
- `PATCH /admin/v1/delivery-routes/{id}/status`
- `DELETE /admin/v1/delivery-routes/{id}`

Delivery routes accept an exact domain, wildcard suffix such as
`*.example.net`, or `*` as the domain pattern. Hosts are stored without ports;
the route-level port, TLS mode, implicit TLS flag, pool name, and optional SMTP
AUTH identity/username/password keep gateway policy out of SMTP protocol core.
The resolve endpoint is a dry-run observability surface; it returns
`{"delivery_route_resolution":{"domain":"...","matched":true|false,"route":...}}`
without sending mail.

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

Message search starts with a small-deployment Postgres implementation:

- `GET /api/v1/search`

The current backend searches message metadata (`subject`, `from_addr`,
`from_name`) plus `draft_text_body` using a simple Postgres FTS expression and
bounded list limits. Full received-body indexing remains intentionally deferred
to the future indexing boundary/OpenSearch worker so SMTP receive and message
read hot paths stay streaming and allocation-aware.

## Deferred from this contract

- Next.js/frontend screens and shells.
- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam modules.
- IMAP, push notifications, Kafka, OpenSearch, etcd, and Vault.
