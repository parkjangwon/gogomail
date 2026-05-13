# ACTIVE_TASK

## TASK-311: POP3 auth user identity trimming audit

### 배경

POP3 gateway는 authenticator가 반환한 user ID를 mail service 조회와 maildrop
lock key의 신뢰 경계로 사용한다. DB/identity adapter가 앞뒤 공백이 섞인 user ID를
반환하더라도 POP3 세션은 정규화된 user ID만 사용해야 하며, folder/page 조회와
maildrop lock key가 서로 다른 형태로 갈라지면 안 된다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 공백이 포함된 authenticated user ID가 maildrop lock key에서 trim되는지 검증한다.
- [x] 공백이 포함된 authenticated user ID가 folder 조회 전에 trim되는지 검증한다.
- [x] 공백이 포함된 authenticated user ID가 inbox page 조회 전에 trim되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-312: POP3 auth empty user identity rejection audit
