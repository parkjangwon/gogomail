# ACTIVE_TASK

## TASK-340: POP3 duplicate mark delete audit

### 배경

POP3 클라이언트가 같은 메시지에 `DELE`를 반복하더라도 pending delete 목록에는 같은
message ID가 한 번만 들어가야 한다. 중복 pending은 commit 시 중복 bulk delete 요청을
만들 수 있으므로, duplicate mark delete 경로를 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 같은 index에 `MarkDeleted`를 두 번 호출한다.
- [x] pending delete 목록에 message ID가 한 번만 들어가는지 검증한다.
- [x] commit 시 bulk delete 요청에도 message ID가 한 번만 포함되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-341: POP3 deleted UIDL visibility audit
