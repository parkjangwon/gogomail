# ACTIVE_TASK

## TASK-253: POP3 auth password-change policy audit

### 배경

POP3 어댑터도 submission 인증 결과를 재사용하지만, IMAP과 달리
`must_change_password` 사용자를 거절하지 않는다. 비밀번호 변경이 필요한 사용자가
웹에서 회전 절차를 완료하기 전 POP3 세션을 열지 못하도록 정책을 맞춘다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 인증 어댑터가 `must_change_password` 사용자를 거절한다.
- [x] POP3 인증 회귀 테스트가 비밀번호 변경 필요 사용자를 커버한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-254: CardDAV company suspension policy audit
