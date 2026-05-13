# ACTIVE_TASK

## TASK-264: CalDAV free-busy recurrence performance audit

### 배경

CalDAV `free-busy-query`는 VEVENT recurrence와 저장된 VFREEBUSY 기간만 busy
결과에 사용한다. VTODO/VJOURNAL 같은 비 busy 컴포넌트가 많을 때 broad list
한도에 먼저 걸리면 실제 busy 후보가 적어도 truncation 오류가 발생하거나
불필요한 ICS를 읽을 수 있다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `free-busy-query` 대상 조회를 VEVENT/VFREEBUSY component list로 제한한다.
- [x] 비 busy 컴포넌트가 한도 계산에 포함되지 않는 회귀 테스트를 추가한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-265: CalDAV sync-collection payload projection audit
