# ACTIVE_TASK

## TASK-339: POP3 reset after commit failure audit

### 배경

POP3 delete commit 실패 후에도 클라이언트가 `RSET` 의미의 reset을 수행하면 세션 내
삭제 표시와 pending delete 목록은 해제되어야 한다. 실패로 보존된 pending 상태가 reset
이후에도 남아 있으면 후속 commit에서 의도치 않은 삭제가 발생할 수 있으므로 이 경로를
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] bulk delete 실패 후 `ResetDeleted`를 호출한다.
- [x] reset 이후 pending delete 목록과 deleted 표시가 모두 해제되는지 검증한다.
- [x] reset 이후 `CommitDeletes`가 추가 bulk delete를 호출하지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-340: POP3 duplicate mark delete audit
