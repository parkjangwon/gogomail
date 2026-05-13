# ACTIVE_TASK

## TASK-270: POP3 delete commit idempotency audit

### 배경

POP3 UPDATE phase는 `QUIT`에서 표시된 삭제를 실제 저장소에 커밋한다.
일반 경로의 `DELE`는 중복 표시를 막지만, 커밋 경계가 pending ID 목록을
그대로 bulk delete에 넘기면 내부 재시도/상태 오염 시 같은 메시지 ID가
중복 삭제 요청으로 전달될 수 있다. 저장소 삭제는 멱등적이어야 하지만,
프로토콜 어댑터도 UPDATE 요청을 정규화해 감사/이벤트/저장소 작업량을
안정적으로 유지해야 한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 pending delete ID를 커밋 직전에 trim/empty-skip/de-duplicate 한다.
- [x] 중복/공백 pending ID가 bulk delete 요청으로 전파되지 않는 회귀 테스트를 추가한다.
- [x] 성공한 커밋 뒤 pending delete 목록이 비워지는 동작을 유지한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-271: IMAP append lazy UID ordering audit
