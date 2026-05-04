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
- Company/domain/user quota read and update APIs exist.
- Mail storage growth/delete paths atomically update company, domain, and user
  quota ledgers in one transaction.
- User quota source is tracked as `default|custom`.
- Domain quota updates can apply a new default user quota to default-following
  users while preserving custom overrides.
- ADR 0003 defines company → domain → user unified storage pool semantics.

Next:

- Extend the same ledger service to attachment upload and future Drive writes.
- Add operator views that show remaining allocatable company/domain capacity.
- Add reconciliation jobs that compare ledger counters against message and
  attachment storage rows.

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

### 8. API metering

Current state:

- Product direction is agreed: collect API usage from the beginning, keep
  billing/rate-limit enforcement policy-driven and off by default.

Next:

- Add a lightweight metering middleware boundary that emits asynchronous usage
  events keyed by company/domain/user/api-key, route, method, status, latency,
  response size, and timestamp.
- Aggregate daily/monthly usage for future SaaS plans, Open API limits, abuse
  detection, and operations dashboards.
- Avoid synchronous writes on hot API paths.

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
