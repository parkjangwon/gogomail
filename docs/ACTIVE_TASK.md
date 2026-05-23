# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-BROWSER-TAG-LENGTH — Browser notification tag length hardening**

## Background

Runtime notification ids are capped before state, storage, dedupe, and browser mirroring. Browser notification mirroring builds the native `NotificationOptions.tag` from `category-id`, which can exceed the id cap once the category prefix is added.

This task continues the notification hardening track in `docs/backend-roadmap.md` by aligning browser mirror replacement-key length with the service-worker WebPush tag boundary.

## Scope

- Add failing E2E coverage for native browser notification tags generated from max-length runtime ids.
- Cap browser mirror `NotificationOptions.tag` at 128 characters.
- Preserve existing browser notification mirroring and dedupe behavior.
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
