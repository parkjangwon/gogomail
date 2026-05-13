# ACTIVE_TASK

## TASK-335: POP3 deleted content access audit

### 배경

POP3 `DELE`로 세션 내 삭제 표시된 메시지는 이후 RETR/TOP 본문 조회에서 다시 노출되면
안 된다. `MessageContent`는 빈 문자열을 반환하고, 오류를 노출하는
`MessageContentWithError`는 명확한 오류를 반환해야 하므로 deleted content 접근을
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 삭제 표시된 메시지의 `MessageContent`가 빈 문자열을 반환하는지 검증한다.
- [x] 삭제 표시된 메시지의 `MessageContentWithError`가 오류를 반환하는지 검증한다.
- [x] 삭제 표시 후 content access가 기존 lazy-load 상태에 의존하지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-336: POP3 reset restores content access audit
