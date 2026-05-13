# ACTIVE_TASK

## TASK-328: POP3 page cursor pagination audit

### 배경

POP3 Authenticate는 INBOX 전체를 로드하기 위해 maildb page cursor를 반복 사용한다.
많은 메시지를 가진 계정에서 cursor가 잘못 이어지면 일부 메시지가 누락되거나 중복될 수
있으므로, multi-page fixture가 실제로 올바른 cursor sequence를 사용해 3 page를 읽는지
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository가 page cursor 인자를 기록한다.
- [x] 450개 메시지 fixture가 정확히 3 page 조회로 로드되는지 검증한다.
- [x] 두 번째/세 번째 page cursor가 직전 page 마지막 메시지 ID를 기준으로 이어지는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-329: POP3 empty inbox pagination audit
