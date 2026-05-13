# ACTIVE_TASK

## TASK-383: POP3 authorization CAPA stability audit

### 배경

POP3 authorization 상태의 `CAPA`는 반복 호출해도 안정적으로 같은 capability를
반환해야 하며, 이후 인증 흐름을 오염시키지 않아야 한다. 인증 전 capability 조회의
상태 보존을 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] authorization 상태에서 `CAPA`를 반복 호출해 기본 capability가 유지되는지 검증한다.
- [x] 반복 `CAPA`가 `USER`와 `SASL PLAIN LOGIN`을 계속 노출하는지 검증한다.
- [x] 반복 `CAPA` 이후 `USER/PASS`와 `STAT`이 성공해 인증 흐름이 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-384: POP3 authorization NOOP stability audit
