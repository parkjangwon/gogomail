# ACTIVE_TASK

## TASK-344: POP3 RSET restores wire visibility audit

### 배경

POP3 `RSET`은 transaction 중 `DELE`로 표시한 메시지들을 다시 보이게 해야 한다.
기존 `STAT` 확인만으로는 단일 메시지 조회 명령의 복구를 놓칠 수 있으므로
wire-level에서 `LIST`, `UIDL`, `RETR`까지 함께 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 후 `RSET` 이후 `LIST 1`이 메시지 크기를 다시 반환하는지 검증한다.
- [x] `DELE 1` 후 `RSET` 이후 `UIDL 1`이 메시지 UIDL을 다시 반환하는지 검증한다.
- [x] `DELE 1` 후 `RSET` 이후 `RETR 1`이 메시지 본문을 다시 반환하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-345: POP3 QUIT commit error visibility audit
