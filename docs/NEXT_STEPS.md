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

### 1. Quota enforcement

Current state:

- Admin quota read model exists.
- Domain/user quota updates exist.

Next:

- Enforce quota during SMTP receive.
- Enforce quota during Mail API send/save/draft/attachment flows.
- Make quota exceeded errors map to the correct SMTP/API status semantics.
- Keep company/domain/user hierarchy in mind.

### 2. Domain policy runtime application

Current state:

- Domain policy admin contract exists through domain settings.

Next:

- Define policy read helpers.
- Apply policy at SMTP, Submission, Mail API, and delivery boundaries without
  hard-coding optional product features into protocol core.
- Keep policy values typed and testable.

### 3. User-facing delivery status

Current state:

- Admin delivery attempt read model exists.
- Mail API send lifecycle status is documented.
- Mail API exposes user-scoped `GET /api/v1/messages/{id}/delivery-status`.

Next:

- Add aggregate recipient delivery timelines if webmail needs richer per-recipient
  visualization.
- Keep delivery attempts scoped by message ownership before exposing them to
  non-admin users.

### 4. DNS and DKIM onboarding

Current state:

- Domain DNS checker exists for MX/SPF/DMARC/DKIM.
- Admin DNS check endpoint exists.
- DNS check persistence exists.
- DKIM public DNS record can be derived from private key.
- DNS check history/list endpoint exists and domain views expose latest DNS
  check status/timestamp.

Next:

- Add DKIM record verification workflow around active keys.

### 5. Backpressure and operational observability

Current state:

- SMTP backpressure primitives exist.
- Delivery route resolution dry-run API exists.

Next:

- Add Admin API for backpressure state inspection/update.
- Add delivery route runtime counters if available.
- Expose queue pressure in a stable envelope.

### 6. OpenAPI/client readiness

Current state:

- Route and response-envelope drift tests exist.

Next:

- Keep `docs/openapi.yaml` synchronized with every HTTP route change.
- Expand response schemas when new read models are added.
- Preserve stable operation IDs.

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
