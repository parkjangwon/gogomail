# ACTIVE_TASK

## TASK-242: POP3 STLS USER/PASS reset audit

### 배경

RFC 2595 STLS 전환이 성공하면 TLS 협상 전에 얻은 인증 상태와 서버 지식은 버려야
한다. 현재 POP3 서버는 `USER` 이후 `STLS`를 성공해도 기존 `USER` 값을 유지해,
클라이언트가 TLS 이후 `USER`를 다시 보내지 않고 `PASS`만으로 인증을 이어갈 수
있다. STLS 성공 시 pre-TLS 사용자 상태를 초기화해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] STLS 성공 후 pre-TLS `USER` 값이 초기화된다.
- [x] TLS 이후 `PASS`만 보내면 인증 실패하고, `USER`/`PASS`를 다시 보내면 성공한다.
- [x] TLS 핸드셰이크를 포함한 회귀 테스트를 추가한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-243: POP3 implicit TLS listener documentation/runtime audit
