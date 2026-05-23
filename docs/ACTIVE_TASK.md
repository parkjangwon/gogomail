# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-SW-TEXT-BLANK-FALLBACK — WebPush notification display text blank fallback hardening**

## Background

The WebPush service worker now normalizes control characters in notification title/body display text, but a string made only of control characters can normalize to whitespace and still reach `registration.showNotification()`. Closed-tab notifications should use the same fallback behavior after normalization that they use for initially blank strings.

This task continues the notification hardening track in `docs/backend-roadmap.md` by ensuring malformed display text cannot produce blank native notification titles or whitespace-only bodies.

## Scope

- Add failing E2E coverage for WebPush title/body strings that become blank after control-character normalization.
- Apply title/body fallback checks after display-text normalization and before length caps.
- Preserve normal text, malformed non-string fallbacks, existing length bounds, and tag handling.
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
