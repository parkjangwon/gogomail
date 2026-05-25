# ACTIVE_TASK

## Current Task

**TASK-DM-COMPLETE-SPEC — DM instant messaging spec implementation**

## Background

`docs/superpowers/specs/2026-05-25-dm-design.md` defines a lightweight
domain-scoped DM product with encrypted per-room message storage, direct and
group rooms, group ownership/invites, attachments, reactions, read state,
search/media read models, polling APIs, and a webmail global panel.

## Scope

- Implement the DM PostgreSQL schema and backend service/API under the existing
  mail API authentication boundary.
- Enforce same-domain access, participant-only message decryption, direct-room
  dedupe, per-room AES-256-GCM keys, key destruction on room deletion, and no
  admin DM read surface.
- Implement group membership, owner transfer, invite links, system messages,
  text/file/Drive messages, reactions, read marks, search, media/link views,
  and attachment handling.
- Implement the webmail DM panel, sidebar entry, unread badge, polling cadence,
  message bubbles, file/Drive actions, and reaction/edit/delete controls.
- Update docs and API contracts, then verify with Go tests, webmail type-check,
  and browser/E2E coverage.

## Completion Checklist

- [x] DM schema migration added.
- [x] DM crypto/service/HTTP groundwork compiles.
- [x] Text message encryption, URL extraction, reactions/read/search/media core covered by Go tests.
- [x] Attachment upload path encryption covered by Go tests.
- [x] Group membership/owner/invite flows insert encrypted system messages.
- [x] Room key destruction and hard-delete lifecycle covered.
- [x] `docs/openapi.yaml` and `docs/backend-api-contracts.md` updated.
- [x] Webmail DM panel implemented.
- [x] `pnpm -C apps/webmail type-check` passes.
- [x] DM browser/E2E smoke passes.
- [x] `go test ./...` passes after current backend implementation.
- [x] Docs updated.
- [x] Commit and push to `origin/main`.
- [x] Fix: Search function now uses consistent 1000 limit in ListSearchCandidates call (was 10000).
- [x] Refactor: Extracted hardcoded Korean system message strings into injectable `SystemMessages` struct with `DefaultSystemMessages()` and `WithSystemMessages()` for i18n-readiness.

## Task 3: Complete

ListMedia 타입 정규화 수정:
- [x] Switch statement updated to normalize API types to store tokens
- [x] MCP `"drive_link"` → store `"drive"`
- [x] MCP `"link"` → store `"links"`
- [x] Unknown types default to `"file"`
- [x] `go test ./internal/dm/...` passes (14 tests)

## Task 4: Complete

metrics interface{} → 타입 안전 로컬 인터페이스:
- [x] `caldavgw`: `gatewayMetrics` 인터페이스 정의 (RecordCommand, RecordError), `metrics interface{}` 필드 → `metrics gatewayMetrics`, `SetMetrics` 시그니처 변경, type assertion 제거
- [x] `carddavgw`: 동일 패턴 적용
- [x] `imapgw`: `gatewayMetrics` 인터페이스 (RecordConnect, RecordDisconnect, RecordCommand, RecordError), mutex 로컬 복사 패턴 유지
- [x] `go test ./internal/caldavgw/... ./internal/carddavgw/... ./internal/imapgw/...` 통과 (1530 tests)

## Next Task

Task 5: Grafana 기본 비밀번호 제거 (docker-compose 3곳)
