# ACTIVE_TASK

## TASK-345: POP3 QUIT commit error visibility audit

### 배경

POP3 `QUIT` 중 삭제 커밋이 실패하면 서버는 `-ERR`를 반환하고 삭제 표시를
롤백한다. 내부 상태 검증만으로는 같은 연결에서 메시지가 다시 보이는지 놓칠 수
있으므로 실패 직후 wire-level 조회 명령의 복구를 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 실패하는 `QUIT`이 `-ERR`를 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 같은 연결의 `LIST 1`이 메시지 크기를 다시 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 같은 연결의 `UIDL 1`이 메시지 UIDL을 다시 반환하는지 검증한다.
- [x] 실패한 `QUIT` 이후 같은 연결의 `RETR 1`이 메시지 본문을 다시 반환하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-346: POP3 QUIT success connection close audit
