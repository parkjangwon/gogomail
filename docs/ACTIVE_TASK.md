# ACTIVE_TASK

## TASK-400: POP3 QUIT after failed commit retry audit

### 배경

POP3 `QUIT`에서 delete commit이 실패하면 서버는 pending delete를 롤백하고 같은 세션에
`-ERR`을 반환한다. 이후 클라이언트가 다시 `DELE` 없이 `QUIT`하면 stale delete 작업을
재시도하지 않고 no-delete 종료 경로로 처리해야 한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 commit 실패 `QUIT`이 `-ERR`을 반환하는지 검증한다.
- [x] 실패한 `QUIT`이 `CommitDeletes`를 정확히 한 번 호출하는지 검증한다.
- [x] commit 실패 롤백 이후 삭제 표시가 남지 않는지 검증한다.
- [x] 다시 `DELE`하지 않은 두 번째 `QUIT`이 `+OK`를 반환하고 `CommitDeletes`를 재호출하지 않는지 검증한다.
- [x] `go test -count=1 ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-401: POP3 QUIT after failed commit re-delete retry audit
