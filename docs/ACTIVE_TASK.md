# ACTIVE_TASK

## TASK-343: POP3 deleted RETR TOP visibility audit

### 배경

POP3 `DELE` 이후 삭제 표시된 메시지는 본문을 반환하는 RETR/TOP에서도 숨겨져야 한다.
mailbox adapter가 content를 lazy-load할 수 있더라도 POP3 server transaction layer가
`Deleted` 상태를 기준으로 본문 계열 명령을 차단해야 하므로 wire-level 동작을 테스트로
고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 이후 `RETR 1`이 `-ERR`를 반환하는지 검증한다.
- [x] `DELE 1` 이후 `TOP 1 0`이 `-ERR`를 반환하는지 검증한다.
- [x] 삭제 표시된 메시지 본문 명령이 content reader를 열기 전에 차단되는지 wire-level로 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-344: POP3 RSET restores wire visibility audit
