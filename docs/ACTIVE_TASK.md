# ACTIVE_TASK

## Current Task

**DM Export Plan — Task 5: User MCP — dm_export_room tool** (pending 2026-05-26)

## Last Completed Task

**Task 4: Frontend — DMPanel more-menu + export download** — COMPLETE (2026-05-26)

- `apps/webmail/messages/{en,ko,ja,zh-CN}.json`: added `exportRoom`, `exportDownloading`, `exportError` keys inside `dmPanel`
- `apps/webmail/src/lib/api/dm.ts`: added `exportDMRoom(roomId)` using `fetch` → `Blob`
- `apps/webmail/src/components/DMPanel.tsx`: added `EllipsisHorizontalIcon` ⋯ button with dropdown menu + export handler + click-away overlay
- TypeScript: `npx tsc --noEmit` → no errors

## Last Completed Task

**Task 10: Split internal/app/admin_service.go into domain files** — COMPLETE (2026-05-26)

- `internal/app/admin_service.go`: 1,759줄 → 93줄 (struct + interfaces only)
- `admin_service_delivery.go` / `admin_service_storage.go` / `admin_service_directory.go` / `admin_service_user.go` / `admin_service_config.go` 생성
- `go test ./internal/app/... -count=1` → 169 passed

## Recently Completed Plan

10-task codebase improvements plan (`docs/superpowers/plans/2026-05-26-codebase-improvements.md`):
1. docs/CURRENT_STATUS.md 및 NEXT_STEPS.md 정리
2. TypeScript 생성 클라이언트 .gitignore 추가
3. Promtail 컨테이너 보안 강화
4. User MCP tools.ts → 도메인 모듈 분리
5. Manage MCP gogomail.ts → 도메인 모듈 분리
6. webmail/src/lib/api.ts → 도메인별 분리 + 이름 정리
7. ComposeEditorToolbar + ComposeAttachmentPanel 추출
8. DMRoomList + DMMessageList + DMComposer + useDMPanel 추출
9. internal/httpapi/admin.go (8901줄) → 12개 파일로 분리
10. internal/app/admin_service.go (1759줄) → 5개 도메인 파일로 분리
