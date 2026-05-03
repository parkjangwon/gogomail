# gogomail backend release readiness

This checklist tracks the backend surfaces needed for the first webmail-focused release.

## Ready or materially advanced

- Mail API exposes folder list/create/rename/delete, message list/detail, move/delete, flag updates, attachment list/download, draft save/update/delete, direct send, and draft send.
- Mail API exposes bounded bulk flag, move, and soft-delete actions for efficient webmail list operations.
- Attachment uploads now support both metadata reservation and direct multipart storage writes.
- Stale attachment uploads have a repository/service cleanup path and a partial index for efficient lifecycle sweeps.
- Direct multipart uploads write through the configured storage backend and only record metadata after the object write succeeds.
- Attachment upload size is guarded in HTTP and service layers, including multipart request caps and declared-size consistency checks.
- Draft-to-send uses the normal outbound send path, then closes the source draft and links it to the sent message.
- Draft attachment uploads move to the sent message during draft-to-send, keeping sent folder detail and attachment list views consistent.
- Detail reads mark unread messages as read while avoiding redundant writes for already-read messages.
- Compose and draft validation guard user id, intent/source rules, recipient presence, recipient email syntax, recipient count, subject size, text body size, attachment IDs, filename safety, MIME type, and upload size.
- API errors use a stable structured envelope with code, message, HTTP status, and HTTP status text.
- Service info exposes API and backend contract version metadata; readiness exposes a structured checks envelope.
- Readiness checks now include contract/storage/outbox boundary metadata for deployment automation.
- Admin API supports domain/user list, detail, create, and status updates plus queue, delivery-attempt, suppression, DKIM, retry, and delete operations.
- Admin API now exposes trusted relay CIDR list/create/delete operations backed by PostgreSQL, preparing inbound SMTP relay policy for auditable runtime administration.
- Admin domain/user create validation rejects malformed domains, unsafe usernames, invalid ACE names, and mismatched primary address ownership.
- SMTP receive/submission paths now include TCP-level protocol integration coverage for inbound delivery, AUTH PLAIN submission, policy rejection, and SMTPS.
- SMTP wire coverage now exercises enabled/disabled extension advertisement, DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` propagation including `NOTIFY=NEVER`, unsupported extension rejection, STARTTLS-gated AUTH, implicit TLS, trusted relay CIDR rejection, and repeated transactions on a single connection.
- Outbound SMTP wire coverage now verifies DSN parameters are emitted only when the remote MTA advertises DSN support, preventing accidental RFC 3461 option leakage to non-DSN peers.
- Outbound SMTP controlled-sink coverage now verifies accepted DATA can coexist with per-recipient permanent and temporary RCPT failures, preserving retry/bounce classification for delivery handlers.
- DSN/bounce generation validates inbound event metadata before composing and queueing null reverse-path DSNs.
- DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext identity metadata before it can reach outbound SMTP command generation or RFC 3464 report composition.
- Delivery partial-failure handling preserves recipient-level retry/bounce decisions even when every RCPT is rejected.
- Attachment upload storage paths reject absolute, parent-traversal, backslash, and newline forms, and generated attachment object paths sanitize path segments before writing to storage.
- `docs/backend-api-contracts.md` stages the backend-only OpenAPI contract source.
- `docs/openapi.yaml` provides the first backend-only OpenAPI 3.1 draft and is guarded against backend contract version drift, registered-route drift, dangling component references, request-body omissions, response envelope reference drift, message flag enum drift, and list limit contract drift.
- OpenAPI response components now document the Mail/Admin JSON envelope keys used by generated clients, including admin queue, delivery attempt, suppression, DKIM, domain, and user read models.
- OpenAPI operations now carry stable lower-camel `operationId` values and default reusable Error responses for protected/mutable operations, reducing generated-client naming and error-decoding drift.
- HTTP list endpoints now enforce the documented `1 <= limit <= 200` boundary before reaching repository pagination, so generated clients can rely on the OpenAPI limit bounds.
- `docs/smtp-release-runbook.md` now records operator-facing SMTP soak, STARTTLS, SMTPS, trusted relay, and outbound DSN/bounce smoke procedures.
- `scripts/verify-backend-release.sh` runs the standard backend release checks (`go test ./...`, `go mod tidy -diff`, optional PostgreSQL integration tests when `GOGOMAIL_TEST_DATABASE_URL` is set, and `git status --short`).
- PostgreSQL-backed integration tests can be enabled with `GOGOMAIL_TEST_DATABASE_URL` to run migrations in a temporary schema and exercise draft-to-send/outbox/retry behavior against real SQL.

## Must verify before release cut

- Run `go test ./...`.
- Run `go mod tidy -diff`.
- Or run `./scripts/verify-backend-release.sh` to execute the standard backend release verification bundle.
- Verify `docs/openapi.yaml` still matches Go routes through the `internal/httpapi` contract tests before generating frontend clients.
- Verify generated clients preserve the documented top-level envelope keys rather than flattening Mail/Admin response bodies.
- Run `GOGOMAIL_TEST_DATABASE_URL=... go test ./internal/maildb ./internal/outbox` against a disposable PostgreSQL database/schema.
- Run focused SMTP soak checks for repeated same-connection transactions and STARTTLS/SMTPS startup in the intended deployment environment.
- Exercise multipart attachment upload against the intended object storage adapter. Local-storage path safety, declared-size mismatch, oversize body cleanup, and metadata-after-object-write behavior are now covered in automated tests.
- Exercise outbound DSN/bounce generation against a deployment-level controlled SMTP sink. Unit and wire tests now cover `NOTIFY=NEVER`, null reverse-path queueing/suppression, DSN option suppression to non-DSN peers, and retry/bounce recipient classification for temporary/permanent recipient failures.
- Verify frontend contracts for error envelope parsing, upload endpoint naming, and draft send response handling.

## Intentionally out of scope for this release slice

- Built-in spam scoring, pattern filtering, quarantine, or vendor-specific spam logic.
- IMAP/POP3.
- OpenSearch indexing, push notifications, Kafka, Vault, and etcd.
