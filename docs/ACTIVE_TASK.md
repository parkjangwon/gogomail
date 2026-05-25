# ACTIVE_TASK

## ID: COMPLETE

DM 대화방 내보내기 기능 구현 완료 (2026-05-26)

- `internal/dm/dm.go`: `Store` 인터페이스에 `GetRoom`, `ListAllMessagesForExport` 추가
- `internal/dm/dm_store.go`: `PostgresStore` 구현 (참여자 전용 접근, `messageSelectSQL` 재사용)
- `internal/dm/dm_export.go`: `RoomExport` 타입 + `FormatExportTXT` 함수 (신규 파일)
- `internal/dm/dm.go`: `Service.GetRoom`, `Service.ExportRoom` 메서드 추가
- `internal/httpapi/dm.go`: `DMService.ExportRoom` + `GET /api/v1/dm/rooms/{roomID}/export` 핸들러
- `docs/openapi.yaml`: `exportDMRoom` 오퍼레이션 추가
- `apps/webmail/src/components/DMPanel.tsx`: ⋯ 더보기 메뉴 + export 핸들러 + 오류 표시
- `apps/webmail/src/lib/api/dm.ts`: `exportDMRoom()` API 함수
- `apps/webmail/messages/{en,ko,ja,zh-CN}.json`: `dmPanel.exportRoom/exportDownloading/exportError` i18n 키
- `apps/gogomail-user-mcp/src/tools/dm.ts`: `gogomail_dm_export_room` 도구 추가 (124번째 도구)
- `apps/gogomail-user-mcp/README.md`: DM (18→19)로 업데이트
- `go test -short ./...`: 6003 passed

## Next Steps

`docs/NEXT_STEPS.md` 백로그에서 다음 태스크를 선택할 것.
