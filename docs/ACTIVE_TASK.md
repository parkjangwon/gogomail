# ACTIVE_TASK

## TASK-265: CalDAV sync-collection payload projection audit

### 배경

CalDAV `sync-collection` 변경 응답은 동일 오브젝트의 여러 변경을 최신 상태로
coalesce한 뒤 응답한다. `calendar-data`가 요청된 경우 coalesce 전에 원문 ICS를
join하면 최종 응답에서 버려질 중간 변경들의 payload까지 읽게 되어 대형 캘린더
증분 동기화 비용이 커진다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 변경 로그 join은 metadata-only로 수행하고 coalesce 이후 최종 객체만 `calendar-data`로 배치 조회한다.
- [x] 중복 변경이 있는 `sync-collection`에서 ICS payload를 한 번만 조회하는 회귀 테스트를 추가한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-266: CardDAV sync-collection payload projection audit
