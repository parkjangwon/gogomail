# ACTIVE_TASK

## TASK-338: POP3 commit failure preserves pending deletes audit

### 배경

POP3 `QUIT` 시점의 delete commit이 DB bulk delete 실패를 만나면 pending delete 목록을
지우면 안 된다. 실패 후 pending이 보존되어야 세션 종료 처리나 상위 계층이 재시도/오류
처리를 판단할 수 있으므로, 실패 경로의 상태 보존을 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository에 bulk delete 오류 주입을 추가한다.
- [x] bulk delete 실패 시 `CommitDeletes`가 오류를 반환하는지 검증한다.
- [x] bulk delete 실패 후 pending delete 목록과 deleted 표시가 보존되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-339: POP3 reset after commit failure audit
