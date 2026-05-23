# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-TEXT-CHARS — Notification center display text character hardening**

## Background

Closed-tab WebPush display text is now normalized, but the in-app notification store still accepts control characters in notification titles and bodies from runtime pushes and localStorage hydration. These strings are rendered in the notification center and mirrored into browser notifications, so CR/LF-style payload text should be normalized before it enters notification state.

This task continues the notification hardening track in `docs/backend-roadmap.md` by aligning in-app notification title/body handling with the WebPush display text policy.

## Scope

- Add failing E2E coverage for runtime and stored notification titles/bodies containing control characters.
- Normalize control-character runs in notification title/body display text to single spaces before length caps.
- Fall back to the default notification title when title text becomes blank after normalization.
- Drop optional bodies that become blank after normalization.
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
