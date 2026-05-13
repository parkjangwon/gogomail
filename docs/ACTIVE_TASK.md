# ACTIVE_TASK

## TASK-250: IMAP capability and session policy audit

### 배경

IMAP 게이트웨이는 TLS가 필요한 평문 연결에서 `LOGINDISABLED`와
`PRIVACYREQUIRED` 정책을 광고하지만, `AUTHENTICATE PLAIN` 초기 응답이 있으면
privacy 정책 확인 전에 SASL payload를 디코딩한다. 평문 인증 데이터는 명령 골격
검사 뒤 즉시 TLS 정책으로 차단해야 한다.

### 구현 대상

- `internal/imapgw/server.go`
- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTHENTICATE PLAIN` 초기 응답이 있어도 TLS 필요 상태에서는 SASL payload를 디코딩하지 않고 `PRIVACYREQUIRED`를 반환한다.
- [x] 명령/메커니즘 토큰 자체가 malformed인 경우는 기존처럼 `BAD`로 유지한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-251: IMAP SELECT snapshot consistency audit
