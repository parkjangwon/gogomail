# ACTIVE_TASK

## ✅ TASK-183: CalDAV sync-collection delta truncation 응답 정합성

### 배경

CalDAV `sync-collection` 초기 snapshot truncation은 RFC 6578/WebDAV XML precondition 응답을 반환하지만,
증분 변경(delta) 경로는 limit 초과 시 일반 텍스트 `400` 오류로 빠졌다.
동일한 `sync-collection` truncation 상황은 클라이언트가 같은 방식으로 처리할 수 있어야 하므로,
delta over-limit도 기존 snapshot과 같은 typed truncation 오류로 매핑했다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] joined change+object fast path의 over-limit delta가 `TruncatedResultsError`로 반환된다.
- [x] fallback change-list path의 over-limit delta가 `TruncatedResultsError`로 반환된다.
- [x] handler 응답은 기존 snapshot truncation과 같은 `403` XML precondition body를 반환한다.
- [x] 회귀 테스트가 delta truncation 응답의 RFC 6578 `number-of-matches` body를 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-184: CardDAV addressbook-query metadata/search index 고도화
