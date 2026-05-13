# ACTIVE_TASK

## TASK-233: POP3 multiline RETR/TOP dot-stuffing audit

### 배경

POP3 `RETR`와 `TOP`은 RFC 1939 multi-line response 규칙에 따라 본문 줄이
마침표(`.`)로 시작하면 전송 시 dot-stuffing 해야 한다. 현재 구현은 원문을
그대로 쓰고 마지막에 `.\r\n` terminator를 붙이므로, 메시지 본문에 `.` 단독
또는 `.` 시작 줄이 있으면 클라이언트가 응답을 조기 종료하거나 내용을 잘못
복원할 수 있다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 `RETR` multi-line response가 `.` 시작 줄을 dot-stuffing한다.
- [x] POP3 `TOP` multi-line response가 header/body 모두에서 `.` 시작 줄을 dot-stuffing한다.
- [x] POP3 multi-line 응답은 CRLF canonical form과 `.\r\n` terminator를 유지한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-234: POP3 STLS transaction-state capability audit
