# ACTIVE_TASK

## TASK-255: CalDAV/CardDAV password-change policy audit

### 배경

CalDAV/CardDAV Basic Auth는 공통 submission 인증기를 재사용하면서도
`must_change_password` 플래그를 직접 거절하지 않는다. 임시 비밀번호로 생성된
사용자가 웹에서 비밀번호 변경을 완료하기 전 DAV 클라이언트로 로그인하지 못하도록
SMTP/IMAP/POP3와 동일한 정책을 적용한다.

### 구현 대상

- `internal/caldavgw/auth.go`
- `internal/caldavgw/auth_test.go`
- `internal/carddavgw/auth.go`
- `internal/carddavgw/auth_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV Basic Auth가 비밀번호 변경 필요 사용자를 거절한다.
- [x] CardDAV Basic Auth가 비밀번호 변경 필요 사용자를 거절한다.
- [x] 두 DAV 인증 회귀 테스트가 `must_change_password` 사용자를 커버한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-256: CardDAV vCard payload validation audit
