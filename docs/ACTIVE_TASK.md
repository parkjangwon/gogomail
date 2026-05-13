# ACTIVE_TASK

## TASK-330: POP3 message size conversion audit

### 배경

POP3 mailbox는 message size를 `int`로 노출하지만 maildb summary는 `int64`를 사용한다.
음수 크기나 현재 플랫폼의 `int` 범위를 초과하는 값이 그대로 변환되면 LIST/STAT 응답이
음수 또는 overflow 값이 될 수 있으므로, POP3 adapter 경계에서 size를 정규화한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 message size 변환 helper를 추가한다.
- [x] 음수/0/양수 size 정규화를 테스트한다.
- [x] `int` 범위 초과 size가 `max int`로 clamp되는지 테스트한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-331: POP3 message size adapter coverage audit
