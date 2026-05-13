# ACTIVE_TASK

## TASK-237: POP3 per-user exclusive maildrop lock audit

### 배경

RFC 1939의 POP3 maildrop 모델은 한 사용자의 메일함을 한 번에 하나의
TRANSACTION 세션에서만 열어야 삭제 표시와 UPDATE 단계가 충돌하지 않는다. 현재
서버는 같은 사용자로 여러 POP3 세션이 동시에 인증되어 같은 메일함을 조작할 수
있으므로, 인증 성공 직후 사용자 단위 독점 잠금을 획득하고 세션 종료 시 반드시
해제해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `internal/mailservice/pop3_adapter.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 같은 사용자 maildrop에 대해 두 번째 POP3 인증 세션이 `-ERR`로 거절된다.
- [x] 기존 세션이 QUIT 또는 연결 종료로 끝나면 maildrop 잠금이 해제된다.
- [x] mailservice POP3 mailbox가 정규 userID를 잠금 키로 제공한다.
- [x] `go test ./internal/pop3d ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-238: POP3 configured connection limit audit
