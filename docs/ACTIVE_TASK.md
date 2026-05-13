# ACTIVE_TASK

## TASK-241: POP3 AUTH cancellation audit

### 배경

SASL continuation을 사용하는 POP3 `AUTH PLAIN` 및 `AUTH LOGIN` 흐름에서
클라이언트는 `*` 한 줄로 인증 시도를 취소할 수 있어야 한다. 현재 서버는 이를
base64 오류처럼 처리해 의미가 불명확하고, LOGIN 중간 단계에서 취소해도 다음
프롬프트를 계속 보낼 위험이 있다. 취소는 즉시 `-ERR authentication cancelled`로
응답하고 AUTHORIZATION 상태를 유지해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH PLAIN` continuation에서 `*` 취소를 명시적으로 처리한다.
- [x] `AUTH LOGIN` username/password continuation에서 `*` 취소를 명시적으로 처리한다.
- [x] AUTH 취소 후 같은 연결에서 정상 USER/PASS 인증이 가능하다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-242: POP3 STLS USER/PASS reset audit
