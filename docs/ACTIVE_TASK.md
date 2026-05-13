# ACTIVE_TASK

## TASK-262: CalDAV sync-token retention audit

### 배경

CalDAV sync change pruning도 오래된 marker를 정리하되, 현재 calendar 행이
가리키는 최신 `sync_token`은 절대 삭제하면 안 된다. CardDAV와 동일하게 현재
컬렉션 token을 prune 후보에서 명시적으로 제외해 증분 동기화 기준점을 보존한다.

### 구현 대상

- `internal/caldavgw/repository.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV sync change prune 후보에서 현재 calendar `sync_token`을 제외한다.
- [x] dry-run과 실제 delete prune 경로가 동일한 보존 조건을 사용한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-263: CalDAV calendar-query candidate optimization audit
