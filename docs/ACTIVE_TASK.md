# ACTIVE_TASK

## TASK-261: CalDAV scheduling component persistence audit

### 배경

CalDAV는 저장되는 calendar object resource와 iTIP scheduling payload를 분리해야
한다. scheduling payload는 `METHOD`를 포함할 수 있지만, 저장 경계의
`UpsertObject`는 `METHOD`가 있는 VCALENDAR를 DB에 persistence하지 않아야 한다.

### 구현 대상

- `internal/caldavgw/repository_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `UpsertObject` 검증 회귀 테스트가 `METHOD` 포함 VCALENDAR 저장을 거절한다.
- [x] scheduling parser 허용 경로와 stored object parser 거절 경로의 계약을 문서화한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-262: CalDAV sync-token retention audit
