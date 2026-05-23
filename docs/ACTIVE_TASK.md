# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-DEDUPE-TYPE — Runtime notification dedupe flag type hardening**

## Background

The notification store accepts future server-driven runtime events through `window.__webmailNotifications.push`. The public input type documents `dedupe` as a boolean, but malformed runtime payloads can still carry string/object values. Non-boolean truthy values must not suppress notification insertion or side effects as if the caller had explicitly requested dedupe.

This task continues the notification hardening track in `docs/backend-roadmap.md` by tightening the runtime trust boundary around dedupe semantics.

## Scope

- Add failing E2E coverage for non-boolean runtime `dedupe` values.
- Treat only literal `dedupe: true` as a dedupe request.
- Preserve existing boolean dedupe behavior and unique-id replacement behavior.
- Update `docs/CURRENT_STATUS.md` and `docs/backend-roadmap.md`.

## Completion Checklist

- [x] RED: focused Playwright test fails before implementation.
- [x] GREEN: focused Playwright tests pass after implementation.
- [x] Full notification E2E passes.
- [x] `pnpm -C apps/webmail type-check` passes.
- [x] `go test ./...` passes.
- [x] Docs updated.
- [x] Commit and push to `origin/main`.

## Next Task

Continue SaaS pre-launch notification center usability and hardening audit.
