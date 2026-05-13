# ACTIVE_TASK

## TASK-248: IMAP auth policy enforcement audit

### 배경

IMAP 게이트웨이는 submission 인증 어댑터를 재사용하지만, 인증 결과에 포함되어야
하는 사용자 보안 상태를 확인하지 않는다. 특히 `must_change_password` 사용자는
웹에서 비밀번호를 갱신하기 전까지 장기 프로토콜 세션을 열면 안 된다.

### 구현 대상

- `internal/smtp/submission.go`
- `internal/maildb/submission.go`
- `internal/mailservice/imap_auth.go`
- `internal/mailservice/service_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] SubmissionUser가 `must_change_password` 인증 상태를 표현한다.
- [x] DB submission authenticator가 사용자 `must_change_password` 값을 전달한다.
- [x] IMAP 인증 어댑터가 비밀번호 변경 필요 사용자를 거절한다.
- [x] `go test ./internal/mailservice ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-249: IMAP connection deadline audit
