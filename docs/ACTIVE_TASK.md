# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-SW-TEXT-CHARS — WebPush notification display text character hardening**

## Background

The WebPush service worker now bounds title/body/tag length and rejects unsafe tag values, but notification title and body strings can still contain ASCII control characters. Closed-tab browser notifications should not display CR/LF-style payload text that can visually split or spoof native notification content.

This task continues the notification hardening track in `docs/backend-roadmap.md` by normalizing service-worker display text before `registration.showNotification()`.

## Scope

- Add failing E2E coverage for WebPush payload titles and bodies containing control characters.
- Normalize control characters in service-worker notification display text to spaces before length caps are applied.
- Preserve valid title/body text, blank/malformed fallbacks, and existing length bounds.
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
