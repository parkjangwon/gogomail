# ACTIVE_TASK

## ✅ TASK-179: CalDAV calendar-query time-range limit 정확성 보강

### 배경

CalDAV `calendar-query`는 time-range 필터가 있는 경우에도 먼저 `limit+1`개 객체만 읽고,
그 뒤에 iCalendar time-range 매칭을 수행했다.
이 순서는 최근 non-match 객체가 먼저 잡히면 오래된 matching 객체를 놓치거나,
필터 후에는 결과가 제한 안에 들어오는데도 truncation 오류를 반환할 수 있다.

RFC 4791 관점에서 `limit`은 최종 매칭 결과에 적용되어야 하므로,
time-range 쿼리는 후보 객체를 읽은 뒤 컴포넌트/time-range 필터를 먼저 적용하고
응답 추가 직전에 제한 초과 여부를 판단하도록 정리했다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] time-range `calendar-query`가 non-matching 선행 객체 때문에 matching 객체를 놓치지 않는다.
- [x] time-range 필터 후 최종 결과가 limit 안에 있으면 truncation 오류를 반환하지 않는다.
- [x] 최종 matching 결과가 limit을 초과할 때만 기존 truncation 처리를 유지한다.
- [x] 회귀 테스트가 선행 non-match + 후행 match + `nresults=1` 케이스를 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-180: CalDAV calendar-query persisted time-window index 설계 및 성능 고도화
