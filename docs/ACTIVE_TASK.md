# ACTIVE_TASK

## TASK-217: CardDAV/CalDAV collection xml:lang If header PostgreSQL integration audit

### 배경

TASK-216에서 handler 수준의 WebDAV `If` 헤더 조건부 `PROPPATCH`
동작을 검증했다. 이제 PostgreSQL-backed repository 경로에서도 collection
ObservedETag가 저장소 트랜잭션 안에서 재검증되고, WebDAV `If` 성공 경로와
동일한 text-only update가 기존 `xml:lang` 값을 보존하는지 확인한다.

### 구현 대상

- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV PostgreSQL update가 observed collection ETag와 text-only mutation을 함께 받으면 기존 language tag를 보존한다.
- [x] CardDAV PostgreSQL update가 observed collection ETag와 text-only mutation을 함께 받으면 기존 language tag를 보존한다.
- [x] CalDAV PostgreSQL update가 stale observed collection ETag에서 text/language mutation을 적용하지 않고 실패한다.
- [x] CardDAV PostgreSQL update가 stale observed collection ETag에서 text/language mutation을 적용하지 않고 실패한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-218: CardDAV/CalDAV collection xml:lang tagged If header audit
