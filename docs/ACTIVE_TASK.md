# ACTIVE_TASK

## TASK-245: SMTP object storage orphan rollback audit

### 배경

SMTP receive 경로는 수신자별 `.eml` 객체를 object storage에 먼저 기록한 뒤 DB
recorder/쿼터 경로를 호출한다. recorder가 실패하거나 mailbox full을 반환하면
DB에는 메시지가 없는데 object storage에는 원문 객체가 남을 수 있다. storage
write 이후 commit 지점까지 실패하면 해당 객체를 best-effort로 삭제해야 한다.

### 구현 대상

- `internal/smtp/receiver.go`
- `internal/smtp/receiver_test.go`
- `internal/smtp/receiver_mailboxfull_extra_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] recorder 실패 시 방금 저장한 `.eml` 객체를 삭제한다.
- [x] mailbox full/quota 실패 시에도 방금 저장한 `.eml` 객체를 삭제한다.
- [x] stored 이벤트 실패 시 DB 기록 전 저장 객체를 삭제한다.
- [x] `go test ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-246: SMTP submission sender alias authorization audit
