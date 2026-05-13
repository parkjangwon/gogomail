# ACTIVE_TASK

## TASK-309: POP3 auth policy freshness audit

### 배경

POP3 gateway는 `POP3StoreAdapter.Authenticate`에서 SMTP submission authenticator를
호출해 사용자 인증 및 정책(`must_change_password`, domain/company 상태 등)을
확인한다. 정책이 DB에서 바뀐 뒤 같은 adapter를 재사용해도 이전 인증 결과가
캐시되면 안 된다. 매 로그인마다 authenticator를 다시 호출해 최신 정책을 반영하는
계약을 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 adapter가 첫 로그인 후 두 번째 로그인에서도 authenticator를 다시 호출하는지 검증한다.
- [x] 두 번째 로그인 전에 `must_change_password` 정책이 바뀌면 즉시 거부되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-310: POP3 auth user identity freshness audit
