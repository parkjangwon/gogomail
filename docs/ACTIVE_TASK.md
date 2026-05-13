# ACTIVE_TASK

## TASK-337: POP3 commit clears pending deletes audit

### 배경

POP3 `QUIT` 시점의 delete commit은 pending delete 목록을 한 번만 반영하고 성공 후
pending 상태를 비워야 한다. 성공한 commit 뒤에도 pending이 남아 있으면 재시도나 중복
QUIT 처리에서 같은 message ID가 반복 삭제될 수 있으므로, 성공 후 no-op 동작까지
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository가 bulk delete 호출 횟수를 기록한다.
- [x] `CommitDeletes` 성공 후 mailbox pending delete 목록이 비는지 검증한다.
- [x] 두 번째 `CommitDeletes`가 추가 bulk delete를 호출하지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-338: POP3 commit failure preserves pending deletes audit
