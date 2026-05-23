# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-ICON-CHARS — Notification icon name character hardening**

## Background

Notification icon names are already capped before runtime state and localStorage persistence, but the sanitizer still accepts control characters and backslashes. Even though icon names are currently future-facing metadata, malformed values should not be retained in persisted notification payloads.

This task continues the notification hardening track in `docs/backend-roadmap.md` by aligning `iconName` handling with the existing identifier/tag character policy.

## Scope

- Add failing E2E coverage for runtime notification `iconName` values containing control characters or backslashes.
- Drop unsafe `iconName` values before state and localStorage persistence.
- Preserve valid short icon names and the existing length cap.
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
