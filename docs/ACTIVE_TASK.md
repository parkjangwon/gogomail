# ACTIVE_TASK

## ✅ TASK-181: CardDAV write/delete 락 경합 축소 및 재시도 보강

### 배경

CardDAV contact object 쓰기/삭제와 address-book 속성 변경은 동시 요청에서 주소록 row `FOR UPDATE`,
contact UID 선조회, sync marker 조회+삽입 분리로 불필요한 경합 구간을 만들 수 있었다.
CalDAV에서 검증한 패턴처럼 고유 제약과 조건부 ETag 검증을 유지하면서 잠금 범위를 줄이고,
serialization/deadlock/lock contention 오류는 bounded retry/backoff로 복구한다.

### 구현 대상

- `internal/carddavgw/repository.go`
- `internal/carddavgw/repository_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV contact upsert/delete에서 주소록 `FOR UPDATE` 선잠금을 제거한다.
- [x] contact UID 중복 선조회 경로를 제거하고 active UID unique index 오류 매핑으로 수렴한다.
- [x] contact/object 및 address-book 조건부 ETag 검증에서 불필요한 `FOR UPDATE`를 제거한다.
- [x] sync marker 보장 경로를 단일 CTE 쿼리로 정리한다.
- [x] contact upsert/delete, address-book delete/proppatch 트랜잭션에 retry/backoff를 적용한다.
- [x] retryable PostgreSQL 오류 분류 및 bounded retry 동작을 단위 테스트로 고정한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-182: CardDAV addressbook-query metadata/search index 고도화
