# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-TEXT-SURROGATE-TRUNCATION — Notification text surrogate-boundary truncation**

## Background

Notification display text, metadata strings, browser mirror tags, and WebPush display/tag strings are capped before state, localStorage persistence, or browser notification creation. JavaScript `slice()` can cut UTF-16 surrogate pairs in the middle when an oversized string ends with an emoji or another supplementary-plane character at the cap boundary.

This task continues the notification hardening track in `docs/backend-roadmap.md` by keeping truncated notification strings valid for internationalized launch-day payloads.

## Scope

- Add failing E2E coverage for oversized runtime notification and service-worker WebPush strings that would cut an emoji surrogate pair.
- Truncate notification strings without leaving a dangling high surrogate.
- Preserve existing title/body/metadata/browser-tag/WebPush-tag length caps.
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
