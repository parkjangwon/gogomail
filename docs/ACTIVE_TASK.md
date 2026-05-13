# ACTIVE_TASK

## TASK-349: POP3 STLS failure connection close audit

### 배경

POP3 `STLS`는 `+OK` 이후 TLS handshake를 시작한다. 이때 클라이언트가 TLS가 아닌
평문을 보내 handshake가 실패하면 서버가 연결을 닫아야 하므로 실패 경로가 열린
세션으로 남지 않는지 wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `STLS` `+OK` 이후 TLS가 아닌 평문 payload를 보내 handshake 실패를 유발한다.
- [x] handshake 실패 시 서버가 `-ERR` 또는 즉시 close로 응답할 수 있음을 허용한다.
- [x] 실패 응답이 온 경우에도 이후 TCP 연결이 닫히는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-350: POP3 STLS transaction-state denial audit
