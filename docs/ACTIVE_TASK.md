# ACTIVE_TASK

## TASK-236: POP3 mailbox pagination audit

### 배경

mailservice 메시지 목록 API는 페이지 크기를 서버 최대값으로 정규화한다. POP3
어댑터가 INBOX를 한 번만 조회하면 메일함에 서버 최대 페이지보다 많은 메시지가
있을 때 POP3 클라이언트가 일부 메시지만 볼 수 있다. POP3 세션 생성 시 INBOX
전체를 안정적인 커서 페이지네이션으로 수집해야 한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 adapter가 INBOX 메시지를 `MessageListMaxLimit` 크기 페이지로 끝까지 조회한다.
- [x] 커서가 있는 다음 페이지를 디코드해 이어서 조회하고, 조회 순서를 유지한다.
- [x] 200개 초과 메시지 메일함에 대한 회귀 테스트를 추가한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-237: POP3 per-user exclusive maildrop lock audit
