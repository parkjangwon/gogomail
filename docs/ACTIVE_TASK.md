# ACTIVE_TASK

## TASK-334: POP3 invalid content index audit

### 배경

POP3 RETR/TOP 경로는 message content 조회를 사용한다. 잘못된 index가 전달되면
`MessageContent`는 빈 문자열을 반환하고, 오류를 노출하는 `MessageContentWithError`는
명확한 오류를 반환해야 한다. 두 경로가 서로 일관되게 invalid index를 처리하는지
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MessageContent(-1)` 및 out-of-range index가 빈 문자열을 반환하는지 검증한다.
- [x] `MessageContentWithError(-1)`이 오류를 반환하는지 검증한다.
- [x] `MessageContentWithError(MessageCount())`가 오류를 반환하는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-335: POP3 deleted content access audit
