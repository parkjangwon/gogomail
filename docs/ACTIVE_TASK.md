# ACTIVE_TASK

## TASK-401: POP3 QUIT after failed commit re-delete retry audit

### 배경

POP3 `QUIT`에서 delete commit이 실패하면 서버는 pending delete를 롤백하고 같은 세션에
`-ERR`을 반환한다. 이후 클라이언트가 같은 메시지를 다시 `DELE`하고 `QUIT`하면 새로운
pending delete로 보고 delete commit을 다시 시도해야 한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 commit 실패 `QUIT`이 `-ERR`을 반환하는지 검증한다.
- [x] 실패한 `QUIT`이 `CommitDeletes`를 정확히 한 번 호출하는지 검증한다.
- [x] commit 실패 롤백 이후 다시 `DELE 1`을 수행할 수 있는지 검증한다.
- [x] 재삭제 후 두 번째 `QUIT`이 `CommitDeletes`를 다시 호출하고 성공 delete mark를 유지하는지 검증한다.
- [x] `go test -count=1 ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-402: POP3 QUIT after failed commit close retry audit
