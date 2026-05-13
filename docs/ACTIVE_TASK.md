# ACTIVE_TASK

## TASK-310: POP3 auth user identity freshness audit

### 배경

POP3 gateway는 매 로그인마다 authenticator가 반환한 user ID를 기준으로 mailbox를
조회하고 maildrop lock key를 만들어야 한다. 같은 username이라도 DB identity가
변경되거나 재매핑되면 이전 user ID를 캐시하면 안 된다. 같은 adapter 인스턴스에서
두 번째 로그인의 user ID가 바뀌었을 때 folder/page 조회와 mailbox lock key가 새
user ID를 사용하는지 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 두 번째 POP3 로그인에서 authenticator가 반환한 새 user ID가 maildrop lock key에 반영되는지 검증한다.
- [x] 두 번째 POP3 로그인의 folder 조회가 새 user ID로 수행되는지 검증한다.
- [x] 두 번째 POP3 로그인의 inbox page 조회가 새 user ID로 수행되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-311: POP3 auth user identity trimming audit
