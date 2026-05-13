# ACTIVE_TASK

## TASK-324: POP3 folder listing error short-circuit audit

### 배경

POP3 Authenticate는 folder 목록 조회가 실패하면 mailbox 생성과 message page 조회를
시작하면 안 된다. folder 오류를 무시하거나 fallback folder로 진행하면 사용자의 실제
메일함 상태와 POP3 세션 상태가 어긋나므로, folder listing 오류 전파와 page 조회
short-circuit을 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository에 folder listing 오류 주입을 추가한다.
- [x] folder listing 오류가 `list folders` 오류로 전파되는지 검증한다.
- [x] folder listing 오류 시 message page 조회가 수행되지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-325: POP3 message page error propagation audit
