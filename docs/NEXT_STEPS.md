# gogomail next steps

This file is the short task handoff for future coding agents.

## Read first

Before changing code, read:

1. `AGENTS.md`
2. `docs/CURRENT_STATUS.md`
3. `docs/backend-roadmap.md`
4. `docs/backend-api-contracts.md`
5. `docs/backend-release-readiness.md`
6. `docs/openapi.yaml`
7. recent `git log --oneline`

## Immediate backend priorities

### 1. Hierarchical quota ledger

Current state:

- Mailbox quota is enforced on selected mail write/delete paths.
- Domain/user quota read and update APIs exist.
- ADR 0003 defines company → domain → user unified storage pool semantics.

Next:

- Add company quota read/update models if missing from Admin API.
- Track user quota source as `default|custom`.
- Apply domain default user quota changes to users that still follow the
  default, preserving custom overrides.
- Enforce aggregate company/domain/user quota for storage-increasing mail,
  attachment, and future Drive operations.

### 2. Message threading and search

Current state:

- Messages store `thread_id`, `in_reply_to`, `rfc_message_id`.
- Thread aggregation APIs exist for `GET /api/v1/threads` and
  `GET /api/v1/threads/{id}/messages`.
- New inbound and reply/forward outbound rows inherit thread IDs from local
  `References`/`In-Reply-To`/source messages.
- Reply composition writes RFC thread headers into outgoing `.eml`.
- Mail API exposes `GET /api/v1/search` backed by a small-deployment Postgres
  FTS index over metadata and draft text.

Next:

- Add indexing boundary for received message body text extraction.
- Add OpenSearch adapter behind the same search contract.
- Add highlighting/ranking fields when index-worker exists.

### 3. IMAP gateway planning

Current state:

- No IMAP implementation exists.
- Message, folder, and flag models are IMAP-compatible by design.

Next:

- Design IMAP backend interface over the existing `maildb` read/write models.
- Plan IMAP IDLE support for push-on-connect clients.
- Keep IMAP as a separate binary mode (`--mode=imap`).

### 4. Pipeline extension hooks

Current state:

- SMTP pipeline defines stages/hooks but they are not fully pluggable.
- Spam, FCM, attachment scan hooks are not wired.

Next:

- Add `FCMNotifier` pipeline hook for new-message push notification.
- Add `AttachmentScanner` hook interface at the SMTP receive stage.
- Keep hooks disabled by default and wired only in `app/run.go`.

### 5. Attachment upload API

Current state:

- Attachment table and storage model exist.
- Attachment endpoints exist in the Mail API.

Next:

- Add multipart upload support for large attachments.
- Enforce per-domain attachment size limits.

### 6. OpenAPI/client readiness

Current state:

- Route, request body, response envelope, operationId, and component reference
  drift tests all pass.
- All schemas synchronized with Go types after platform hardening sprint.

Next:

- Keep `docs/openapi.yaml` synchronized with every HTTP route change.
- Consider generating a TypeScript client from the OpenAPI spec for future
  frontend use.

### 7. Frontend planning

Before creating or substantially implementing frontend apps, explicitly ask the
user for frontend-specific guidance.

## Do not do yet

- Do not start frontend implementation without asking the user.
- Do not build a built-in spam engine inside SMTP core.
- Do not add vendor-specific spam/filtering behavior to protocol paths.
- Do not advertise SMTP extensions before full RFC semantics exist.

## Standard finish checklist

```bash
go test ./...
go mod tidy -diff
git status --short
git push
```

Every meaningful feature should be a reviewable commit before pushing.
