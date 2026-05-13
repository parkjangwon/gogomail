# ACTIVE_TASK

## TASK-362: POP3 AUTH PLAIN successful challenge authentication audit

### 배경

POP3 `AUTH PLAIN`은 초기 응답 없이 challenge 방식으로도 credential을 받을 수 있다.
성공 경로에서도 continuation 응답을 받은 뒤 transaction 상태로 전환되어야 하며,
인증 전용 capability가 더 이상 노출되지 않아야 한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH PLAIN` challenge continuation의 정상 credential이 `+OK` 인증 성공을 반환하는지 검증한다.
- [x] 인증 성공 후 CAPA에서 `USER`와 `SASL PLAIN LOGIN`이 사라지는지 검증한다.
- [x] 인증 성공 후 `STAT`이 성공해 transaction 상태로 전환되었는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-363: POP3 AUTH PLAIN initial response success capability audit
