# ACTIVE_TASK

## TASK-326: POP3 message page cursor error audit

### 배경

POP3 Authenticate는 INBOX message page를 모두 읽기 위해 cursor를 decode한다. page가
`HasMore`를 반환했지만 cursor payload가 maildb cursor 규칙을 통과하지 못하면 다음
page를 잘못 조회하거나 무한 반복하면 안 되고, `decode inbox cursor` 오류로 실패해야
한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] UUID 형식이 아닌 message ID가 포함된 multi-page INBOX fixture를 구성한다.
- [x] cursor decode 실패가 `decode inbox cursor` 오류로 전파되는지 검증한다.
- [x] cursor decode 실패 시 첫 page 조회 이후 추가 page 조회가 발생하지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-327: POP3 missing cursor guard audit
