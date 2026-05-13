# ACTIVE_TASK

## TASK-408: POP3 QUIT after failed commit TOP audit

### 배경

POP3 `QUIT`에서 delete commit이 실패하면 서버는 pending delete를 롤백하고 같은 세션에
`-ERR`을 반환한다. rollback 이후 `TOP`은 복구된 메시지의 헤더와 요청된 본문 라인만
반환해야 하며, 메시지 조회가 delete mark를 다시 만들거나 no-delete `QUIT` 최적화를 깨면
안 된다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 commit 실패 `QUIT`이 `-ERR`을 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 `TOP 1 1`이 복구된 메시지 헤더와 첫 본문 라인을 반환하는지 검증한다.
- [x] `TOP 1 1`이 요청 범위를 넘는 본문 라인을 반환하지 않는지 검증한다.
- [x] `TOP 1 1` 이후 delete mark가 clear 상태로 남고 no-delete `QUIT`이 `CommitDeletes`를 재호출하지 않는지 검증한다.
- [x] `go test -count=1 ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-409: POP3 QUIT after failed commit UIDL audit
