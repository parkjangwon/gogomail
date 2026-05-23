# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-SW-URL-LENGTH — WebPush notification click URL length hardening**

## Background

The in-app notification store now caps notification `actionUrl` values before runtime state, storage hydration, and browser notification mirroring. The WebPush service worker has a separate `safeNotificationClickUrl` path for closed-tab push payloads and notification-click navigation. That path validates relative URLs and unsafe characters but does not currently bound URL length, leaving an oversized push payload able to persist a very large click target in `NotificationOptions.data` or pass it to `clients.openWindow()`.

This task is derived from the notification hardening track in `docs/backend-roadmap.md` and keeps WebPush click handling aligned with the notification center action URL policy.

## Scope

- Add failing E2E coverage for oversized WebPush push payload URLs and notification-click URLs.
- Cap service-worker notification click URLs at the same 2048-character limit used by the in-app notification store.
- Preserve normal safe relative URL behavior and existing unsafe URL fallbacks.
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
