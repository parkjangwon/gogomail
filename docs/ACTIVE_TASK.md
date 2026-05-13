# ACTIVE_TASK

## TASK-398: POP3 QUIT no-delete close audit

### 배경

POP3 `QUIT`에서 pending delete가 없어 commit 경로를 건너뛰더라도 서버는 `+OK`
응답 후 연결을 종료해야 한다. commit skip 최적화가 close 동작을 깨뜨리지 않는지
wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `internal/pop3d/pop3d.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] pending delete가 없는 `QUIT`이 `+OK`를 반환하는지 검증한다.
- [x] pending delete가 없는 `QUIT`이 `CommitDeletes`를 호출하지 않는지 검증한다.
- [x] pending delete가 없는 `QUIT` 이후 TCP 연결이 닫히는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-399: POP3 QUIT after RSET skips commit audit
