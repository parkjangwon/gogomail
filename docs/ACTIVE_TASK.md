# ACTIVE_TASK

## TASK-327: POP3 missing cursor guard audit

### 배경

POP3 Authenticate는 page가 `HasMore=true`를 반환하면 반드시 다음 cursor가 있어야
한다. maildb page 생성은 마지막 message에 cursor 재료가 부족하면 빈 cursor를 만들 수
있으므로, adapter가 이를 그대로 decode하면 같은 첫 page를 반복할 위험이 있다. missing
cursor를 명시 오류로 처리해 무한 반복을 차단한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `HasMore=true`이지만 `NextCursor`가 비어 있는 page를 `missing inbox cursor` 오류로 거부한다.
- [x] missing cursor 상황이 첫 page 조회 이후 반복 조회로 이어지지 않는지 검증한다.
- [x] missing cursor 상황에서도 선택된 INBOX folder ID가 유지되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-328: POP3 page cursor pagination audit
