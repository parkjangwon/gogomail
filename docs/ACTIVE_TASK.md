# ACTIVE_TASK

## Current Task

**TASK-WEBMAIL-NOTIFICATION-BROWSER-TAG-COLLISION — Browser notification tag collision hardening**

## Background

Browser notification mirroring now caps native `NotificationOptions.tag` values at 128 characters, but simple truncation can collapse two distinct max-length runtime ids with the same prefix into the same native replacement key. That can cause the browser to replace or group unrelated high-severity notifications.

This task continues the notification hardening track in `docs/backend-roadmap.md` by preserving bounded browser tags without sacrificing uniqueness for long event ids.

## Scope

- Add failing E2E coverage for distinct max-length runtime ids that share a long prefix.
- Keep browser mirror tags capped at 128 characters while adding a stable hash suffix for overlong derived tags.
- Preserve existing browser notification mirror dedupe behavior for exact repeated ids.
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
