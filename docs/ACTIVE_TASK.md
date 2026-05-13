# ACTIVE_TASK

## TASK-382: POP3 transaction CAPA stability audit

### 배경

POP3 transaction 상태의 `CAPA`는 반복 호출해도 안정적으로 같은 capability를
반환해야 하며, 인증 전용 capability를 다시 노출하지 않아야 한다. capability 조회가
세션 상태를 오염시키지 않는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] transaction 상태에서 `CAPA`를 반복 호출해 기본 capability가 유지되는지 검증한다.
- [x] 반복 `CAPA`가 `USER`, `SASL PLAIN LOGIN`, `STLS`를 노출하지 않는지 검증한다.
- [x] 반복 `CAPA` 이후 `STAT`이 성공해 transaction 세션이 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-383: POP3 authorization CAPA stability audit
