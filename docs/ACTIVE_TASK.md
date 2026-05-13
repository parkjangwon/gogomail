# ACTIVE_TASK

## TASK-235: POP3 RETR fetch-error response audit

### 배경

POP3 서버는 메시지 원문을 가져오지 못하면 `RETR`/`TOP`에서 성공 응답을 보내면
안 된다. 현재 기본 `Mailbox` 인터페이스는 본문 조회 오류를 표현할 수 없고,
mailservice adapter는 fetch 실패 시 빈 문자열을 반환해 `+OK 0 octets` 같은
성공 응답으로 흐를 수 있다. 본문 조회 오류를 POP3 명령 처리까지 전달해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 `RETR`가 메시지 본문 조회 실패 시 `-ERR`를 반환하고 multiline 성공 응답을 시작하지 않는다.
- [x] POP3 `TOP`이 메시지 본문 조회 실패 시 `-ERR`를 반환하고 multiline 성공 응답을 시작하지 않는다.
- [x] mailservice POP3 adapter가 raw body fetch 오류를 서버까지 전달한다.
- [x] `go test ./internal/pop3d ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-236: POP3 mailbox pagination audit
