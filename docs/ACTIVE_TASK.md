# ACTIVE_TASK

## TASK-406: POP3 QUIT after failed commit DELE invalid audit

### 배경

POP3 `QUIT`에서 delete commit이 실패하면 서버는 pending delete를 롤백하고 같은 세션에
`-ERR`을 반환한다. rollback 이후 잘못된 메시지 번호의 `DELE`는 `-ERR`을 반환해야 하며,
복구된 메시지의 delete mark를 다시 오염시키면 안 된다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 commit 실패 `QUIT`이 `-ERR`을 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 out-of-range `DELE`가 `-ERR`을 반환하는지 검증한다.
- [x] invalid `DELE` 이후 `LIST 1`이 rollback된 메시지 크기를 반환하는지 검증한다.
- [x] invalid `DELE` 이후 delete mark가 clear 상태로 남는지 검증한다.
- [x] `go test -count=1 ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-407: POP3 QUIT after failed commit RETR audit
