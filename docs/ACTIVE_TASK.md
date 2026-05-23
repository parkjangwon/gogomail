# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-SW-TAG-CHARS — WebPush notification tag character hardening**

## Background

The WebPush service worker now bounds notification display strings, but it still accepts any non-blank string as a `tag` after truncation. Browser notification tags are used for replacement/deduplication, so control characters and backslashes should not be retained in closed-tab push notification state.

This task continues the notification hardening track in `docs/backend-roadmap.md` and aligns service-worker notification tags with the existing safe identifier/tag policy used elsewhere in the notification center.

## Scope

- Add failing E2E coverage for WebPush payload tags containing control characters and backslashes.
- Replace unsafe service-worker notification tags with the default `gogomail-notification` tag.
- Preserve valid custom tags and existing malformed/blank fallback behavior.
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
