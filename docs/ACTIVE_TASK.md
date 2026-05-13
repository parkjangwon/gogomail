# ACTIVE_TASK

## TASK-266: CardDAV sync-collection payload projection audit

### 배경

CardDAV `sync-collection` 변경 응답도 동일 연락처의 여러 변경을 최신 상태로
coalesce한 뒤 응답해야 한다. `address-data`가 요청된 경우 coalesce 전에 vCard
payload를 join하면 최종 응답에서 버려질 중간 변경들의 payload까지 읽게 되고,
중복 변경이 limit 판단에도 영향을 줄 수 있다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/repository.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 변경 로그 join은 metadata-only로 수행하고 coalesce 이후 최종 객체만 `address-data`로 배치 조회한다.
- [x] 중복 변경이 있는 `sync-collection`에서 vCard payload를 한 번만 조회하는 회귀 테스트를 추가한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-267: IMAP mailbox status consistency audit
