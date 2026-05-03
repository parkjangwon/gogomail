# gogomail backend API contracts

This document is the staging contract for a future OpenAPI specification. It is intentionally backend-only and does not start frontend implementation.

## Contract metadata

- Public mail API base path: `/api/v1`
- Admin API base path: `/admin/v1`
- Service info: `GET /api/v1/info`
- Current backend contract version: `2026-05-04.backend-release`

## Response envelopes

Successful collection responses keep a stable top-level plural key:

- `{"folders":[...]}`
- `{"messages":[...],"limit":50,"has_more":false,"next_cursor":"..."}`
- `{"attachments":[...]}`
- `{"domains":[...]}`
- `{"users":[...]}`

Successful resource responses keep a stable singular key:

- `{"message":{...}}`
- `{"draft":{...}}`
- `{"attachment":{...}}`
- `{"domain":{...}}`
- `{"user":{...}}`

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

List endpoints that accept `limit` reject non-integer and nonpositive values. Message listing returns opaque `next_cursor`; clients must not parse or manufacture cursors.

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

## Deferred from this contract

- Next.js/frontend screens and shells.
- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam modules.
- IMAP, push notifications, Kafka, OpenSearch, etcd, and Vault.
