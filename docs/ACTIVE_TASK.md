# ACTIVE_TASK

## TASK-257: CardDAV address-data UID projection audit

### 배경

CardDAV `address-data` 부분 projection은 요청된 속성만 남기면서 `BEGIN`,
`VERSION`, `END`는 유지하지만 저장 모델의 필수 식별자인 `UID`를 생략할 수 있다.
부분 응답도 클라이언트가 동일 연락처를 안정적으로 식별할 수 있도록 `UID`를 항상
보존한다.

### 구현 대상

- `internal/carddavgw/response.go`
- `internal/carddavgw/response_test.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `address-data` property projection이 `UID`를 항상 유지한다.
- [x] response/handler 회귀 테스트가 부분 projection의 UID 보존을 커버한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-258: CardDAV addressbook-query filter semantics audit
