# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-METADATA-CHARS — Notification metadata character hardening**

## Background

Notification titles, bodies, ids, action URLs, tags, and WebPush display text now reject or normalize malformed characters. Runtime notification metadata is bounded to flat primitive values, but metadata keys and string values can still retain ASCII control characters or backslashes before entering state and localStorage.

This task continues the notification hardening track in `docs/backend-roadmap.md` by making runtime metadata safe for persisted notification state.

## Scope

- Add failing E2E coverage for runtime notification metadata keys and string values containing control characters or backslashes.
- Drop unsafe metadata keys instead of persisting malformed key names.
- Normalize control-character runs in metadata string values to spaces, reject backslash-containing string values, and preserve existing string length bounds.
- Preserve safe string, number, and boolean metadata behavior.
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
