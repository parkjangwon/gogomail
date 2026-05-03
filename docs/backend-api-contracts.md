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
- `{"attachments":[...]}`
- `{"domains":[...]}`
- `{"users":[...]}`
- `{"queues":[...]}`
- `{"delivery_attempts":[...]}`
- `{"suppression_list":[...]}`
- `{"dkim_keys":[...]}`

Successful resource responses keep a stable singular key:

- `{"message":{...}}`
- `{"draft":{...}}`
- `{"attachment":{...}}`
- `{"domain":{...}}`
- `{"user":{...}}`

Successful mutation responses use one of:

- `{"status":"ok"}`
- `{"status":"ok","id":"..."}`
- `{"status":"ok","updated":2}`

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
- `PATCH /admin/v1/users/{id}/quota`

`quota_limit: 0` clears the limit and negative values are rejected.

Admin operational read models also keep explicit envelope keys:

- `GET /admin/v1/queue` returns `{"queues":[...]}`
- `GET /admin/v1/delivery-attempts` returns `{"delivery_attempts":[...]}`
- `GET /admin/v1/suppression-list` returns `{"suppression_list":[...]}`
- `GET /admin/v1/dkim-keys` returns `{"dkim_keys":[...]}`

Admin deletion/retry/status/quota mutations return `{"status":"ok","id":"..."}`
so consoles can reconcile optimistic updates against the affected backend id.

## Deferred from this contract

- Next.js/frontend screens and shells.
- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam modules.
- IMAP, push notifications, Kafka, OpenSearch, etcd, and Vault.
