# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Attachment uploads now support both metadata reservation and direct multipart storage writes.
- Direct multipart uploads write through the configured storage backend and only record metadata after the object write succeeds.
- Attachment upload size is guarded in HTTP and service layers, including multipart request caps and declared-size consistency checks.
- Draft-to-send uses the normal outbound send path, then closes the source draft and links it to the sent message.
- Draft attachment uploads move to the sent message during draft-to-send, keeping sent folder detail and attachment list views consistent.
- Detail reads mark unread messages as read while avoiding redundant writes for already-read messages.
- Compose and draft validation guard user id, intent/source rules, recipient presence, recipient email syntax, recipient count, subject size, text body size, attachment IDs, filename safety, MIME type, and upload size.
- API errors use a stable structured envelope with code, message, HTTP status, and HTTP status text.
- Service info exposes API and backend contract version metadata; readiness exposes a structured checks envelope.
- Admin API supports domain/user list, detail, create, and status updates plus queue, delivery-attempt, suppression, DKIM, retry, and delete operations.
- `docs/backend-api-contracts.md` stages the backend-only OpenAPI contract source.

## Must verify before release cut

- Run `go test ./...`.
- Run `go mod tidy -diff`.
- Exercise draft-to-send against a real PostgreSQL instance with migrations applied.
- Exercise multipart attachment upload against both local storage and the intended object storage adapter.
- Verify frontend contracts for error envelope parsing, upload endpoint naming, and draft send response handling.

## Intentionally out of scope for this release slice

- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam logic.
- IMAP/POP3.
- OpenSearch indexing, push notifications, Kafka, Vault, and etcd.
