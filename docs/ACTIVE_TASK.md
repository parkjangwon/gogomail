# ACTIVE_TASK

## TASK-396: POP3 QUIT success commits pending delete audit

### 배경

POP3 `QUIT` 성공은 transaction 중 `DELE`로 표시한 메시지를 커밋해야 한다. 단순히
`+OK` 응답만 확인하면 커밋 경로 호출 누락을 놓칠 수 있으므로 CommitDeletes 호출과
삭제 표시 유지 상태를 함께 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 성공 `QUIT`이 `+OK`를 반환하는지 검증한다.
- [x] 성공 `QUIT`이 `CommitDeletes`를 정확히 1회 호출하는지 검증한다.
- [x] 성공 `QUIT` 이후 삭제 표시가 커밋 상태로 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-397: POP3 QUIT without deletes skips commit audit
