# ACTIVE_TASK

## TASK-399: POP3 QUIT after RSET skips commit audit

### 배경

POP3 `RSET`은 pending delete를 해제하므로, `DELE` 후 `RSET`한 세션의 `QUIT`은 delete
commit 경로를 호출하지 않아야 한다. reset된 삭제 표시가 commit 최적화와 일치하는지
고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `internal/pop3d/pop3d.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 `RSET`한 뒤 `QUIT`이 `+OK`를 반환하는지 검증한다.
- [x] `RSET` 이후 `QUIT`이 `CommitDeletes`를 호출하지 않는지 검증한다.
- [x] `RSET` 이후 삭제 표시가 남지 않는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-400: POP3 QUIT after failed commit retry audit
