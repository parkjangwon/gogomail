# gogomail change checklist

Use this checklist before finishing a coding-agent work unit.

## Context continuity

- [ ] I read `AGENTS.md`.
- [ ] I read `docs/CURRENT_STATUS.md`.
- [ ] I read `docs/NEXT_STEPS.md`.
- [ ] I checked recent `git log --oneline`.
- [ ] I checked `git status --short` before editing.

## Architecture and roadmap

- [ ] If the project phase or completed scope changed, I updated `docs/CURRENT_STATUS.md`.
- [ ] If priorities changed, I updated `docs/NEXT_STEPS.md`.
- [ ] If a meaningful backend capability was completed, I updated `docs/backend-roadmap.md`.
- [ ] If an architectural decision or boundary changed, I added/updated an ADR in `docs/adr/`.

## API and contracts

- [ ] If HTTP API behavior changed, I updated `docs/openapi.yaml`.
- [ ] If API semantics changed, I updated `docs/backend-api-contracts.md`.
- [ ] If release readiness changed, I updated `docs/backend-release-readiness.md`.
- [ ] I kept OpenAPI operation IDs stable and response envelopes explicit.

## Guardrails

- [ ] I did not start frontend implementation without user guidance.
- [ ] I did not add spam scoring/pattern filtering into SMTP core.
- [ ] I did not advertise unsupported SMTP extensions.
- [ ] I preserved RFC correctness for implemented mail behavior.
- [ ] I kept tenant/domain isolation in mind.

## Verification

Run:

```bash
go test ./...
go mod tidy -diff
git status --short
```

Optional when database behavior changed:

```bash
GOGOMAIL_TEST_DATABASE_URL='postgres://...' go test ./internal/maildb ./internal/outbox
GOGOMAIL_TEST_OPENSEARCH_URL='http://localhost:9200' go test ./internal/searchindex
```

## Finish

- [ ] Changes are committed as meaningful, reviewable units.
- [ ] Completed commits are pushed to `origin/main`.
- [ ] The final report mentions tests and push status.
