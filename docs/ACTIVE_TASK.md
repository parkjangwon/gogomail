# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-SW-TEXT-LENGTH — WebPush notification text length hardening**

## Background

The in-app notification store already caps notification titles and bodies before state, storage, and browser notification mirroring. The WebPush service worker still accepts arbitrary string lengths for closed-tab push `title`, `body`, and `tag` fields before passing them to `registration.showNotification()`. Oversized push payloads can therefore bloat browser notification state even after the in-app notification center is protected.

This task continues the notification hardening track in `docs/backend-roadmap.md` and aligns closed-tab WebPush display payloads with the bounded in-app notification policy.

## Scope

- Add failing E2E coverage for oversized WebPush `title`, `body`, and `tag` fields.
- Cap service-worker notification title/body/tag strings before `showNotification()`.
- Preserve existing fallback behavior for blank and malformed title/body/tag values.
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
