# ACTIVE_TASK

## TASK-336: POP3 reset restores content access audit

### 배경

POP3 `RSET`은 세션 내 삭제 표시를 해제한다. `DELE` 후 content 접근이 차단되더라도
`ResetDeleted` 이후에는 같은 메시지의 본문을 다시 읽을 수 있어야 하므로, reset 이후
content lazy-load 경로가 복구되는지 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 삭제 표시 후 `ResetDeleted`를 호출한다.
- [x] reset 이후 `MessageContentWithError`가 오류 없이 content를 반환하는지 검증한다.
- [x] reset 이후 반환 content가 실제 저장된 원문을 포함하는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-337: POP3 commit clears pending deletes audit
