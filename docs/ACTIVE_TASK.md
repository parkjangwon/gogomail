# ACTIVE_TASK

## ✅ TASK-185: CalDAV calendar-query time-range 후보 인덱스 고도화

### 배경

CalDAV `calendar-query` time-range 경로는 RFC 정합성을 위해 limit을 최종 time-range 필터 뒤에 적용하지만,
그 과정에서 component 후보 인덱스를 사용하지 못하고 전체 캘린더 객체 본문을 스캔했다.
기존 `component_type` 인덱스가 있는 만큼 `VEVENT`/`VTODO` 등 요청 component 후보만 스트리밍하고,
최종 time-range 판정과 truncation 판단은 기존 RFC 4791 iCalendar matcher에 맡긴다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/repository_discovery.go`
- `internal/caldavgw/repository_discovery_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] time-range `calendar-query`가 component 후보 walker를 우선 사용한다.
- [x] repository candidate walker가 `user_id/calendar_id/status/component_type` scope를 적용해 기존 component 인덱스를 활용한다.
- [x] 후보 결과는 기존 `CalendarObjectMatchesTimeRange`로 재검증해 recurrence/timezone/VTODO 정합성을 보존한다.
- [x] `nresults` limit은 time-range 최종 매칭 뒤에 적용해 최근 정합성 수정을 유지한다.
- [x] handler 회귀 테스트가 candidate path 사용과 non-requested component 제외를 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-186: CalDAV sync-collection delta duplicate href coalescing
