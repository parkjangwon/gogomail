# ACTIVE_TASK

## TASK-263: CalDAV calendar-query candidate optimization audit

### 배경

CalDAV `calendar-query`는 시간 범위가 있는 경우 컴포넌트별 candidate walker로
직접 들어가야 한다. 핸들러가 broad/component list 조회를 먼저 수행하면 실제
결과는 candidate walker를 쓰더라도 DB 사전 조회가 추가되어 대형 캘린더에서
불필요한 비용이 발생한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 시간 범위 `calendar-query`를 list prefetch 전에 candidate walker 경로로 분기한다.
- [x] component list 및 broad list가 호출되지 않는 회귀 테스트를 추가한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-264: CalDAV free-busy recurrence performance audit
