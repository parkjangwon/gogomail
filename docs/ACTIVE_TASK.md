# ACTIVE_TASK

## TASK-397: POP3 QUIT without deletes skips commit audit

### 배경

POP3 `QUIT` 성공 시 pending delete가 없으면 delete commit 경로를 호출할 필요가 없다.
불필요한 저장소 호출과 실패 주입 영향을 피하도록 삭제 표시가 없을 때 `CommitDeletes`
를 건너뛰는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `internal/pop3d/pop3d.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] pending delete가 없는 `QUIT`이 `+OK`를 반환하는지 검증한다.
- [x] pending delete가 없으면 `CommitDeletes`를 호출하지 않는지 검증한다.
- [x] pending delete가 있는 경우에는 기존 성공 commit 경로가 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-398: POP3 QUIT no-delete close audit
