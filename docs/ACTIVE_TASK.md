# ACTIVE_TASK

## TASK-369: POP3 USER PASS transaction capability audit

### 배경

POP3 기본 `USER/PASS` 인증 성공 후에도 SASL 인증과 동일하게 transaction 상태로
전환되어야 하며, 인증 전용 capability가 더 이상 노출되지 않아야 한다. 가장 흔한
인증 경로를 명시적으로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `USER/PASS` 정상 credential이 transaction 상태로 전환되는지 검증한다.
- [x] 인증 성공 후 CAPA에서 `USER`와 `SASL PLAIN LOGIN`이 사라지는지 검증한다.
- [x] 인증 성공 후 `STAT`이 성공하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-370: POP3 USER PASS failure capability audit
