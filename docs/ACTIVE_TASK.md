# ACTIVE_TASK

## TASK-404: POP3 QUIT after failed commit NOOP audit

### 배경

POP3 `QUIT`에서 delete commit이 실패하면 서버는 pending delete를 롤백하고 같은 세션에
`-ERR`을 반환한다. 이때 연결은 즉시 닫히지 않고 transaction 상태로 유지되므로, 이후
`NOOP` 같은 일반 transaction 명령이 정상 응답해야 한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 commit 실패 `QUIT`이 `-ERR`을 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 같은 연결에서 `NOOP`이 `+OK`를 반환하는지 검증한다.
- [x] `NOOP` 이후 `STAT`이 rollback된 maildrop 상태를 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-405: POP3 QUIT after failed commit RSET audit
