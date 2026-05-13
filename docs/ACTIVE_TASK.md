# ACTIVE_TASK

## TASK-260: CardDAV sync-token retention audit

### 배경

CardDAV sync change pruning은 오래된 marker를 정리하되, 현재 addressbook 행이
가리키는 최신 `sync_token`은 절대 삭제하면 안 된다. 단순히 더 최신 change가
있다는 조건만으로 후보를 고르면 레거시/부분 데이터에서 현재 token까지 prune될 수
있으므로 현재 컬렉션 token을 명시적으로 보존한다.

### 구현 대상

- `internal/carddavgw/repository.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV sync change prune 후보에서 현재 addressbook `sync_token`을 제외한다.
- [x] dry-run과 실제 delete prune 경로가 동일한 보존 조건을 사용한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-261: CalDAV scheduling component persistence audit
