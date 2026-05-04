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

### 1. Message threading and search

Current state:

- Messages store `thread_id`, `in_reply_to`, `rfc_message_id`.
- Thread aggregation APIs exist for `GET /api/v1/threads` and
  `GET /api/v1/threads/{id}/messages`.
- No full-text search API exists yet.

Next:

- Assign `thread_id` on receive/send using RFC `References` and `In-Reply-To`.
- Evaluate Postgres full-text search vs OpenSearch for subject/body search.
- Add search endpoints and an indexing boundary.

### 2. IMAP gateway planning

Current state:

- No IMAP implementation exists.
- Message, folder, and flag models are IMAP-compatible by design.

Next:

- Design IMAP backend interface over the existing `maildb` read/write models.
- Plan IMAP IDLE support for push-on-connect clients.
- Keep IMAP as a separate binary mode (`--mode=imap`).

### 3. Pipeline extension hooks

Current state:

- SMTP pipeline defines stages/hooks but they are not fully pluggable.
- Spam, FCM, attachment scan hooks are not wired.

Next:

- Add `FCMNotifier` pipeline hook for new-message push notification.
- Add `AttachmentScanner` hook interface at the SMTP receive stage.
- Keep hooks disabled by default and wired only in `app/run.go`.

### 4. Attachment upload API

Current state:

- Attachment table and storage model exist.
- Attachment endpoints exist in the Mail API.

Next:

- Add multipart upload support for large attachments.
- Enforce per-domain attachment size limits.

### 5. OpenAPI/client readiness

Current state:

- Route, request body, response envelope, operationId, and component reference
  drift tests all pass.
- All schemas synchronized with Go types after platform hardening sprint.

Next:

- Keep `docs/openapi.yaml` synchronized with every HTTP route change.
- Consider generating a TypeScript client from the OpenAPI spec for future
  frontend use.

### 6. Frontend planning

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
