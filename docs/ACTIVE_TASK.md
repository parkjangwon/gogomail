# ACTIVE_TASK

## TASK-247: SMTP submitted object rollback audit

### 배경

Authenticated Submission도 `.eml` 객체를 storage에 먼저 기록한 뒤 submitted
recorder/쿼터 경로를 호출한다. receive 경로와 같은 이유로, DB 기록 전 stored hook
또는 recorder가 실패하면 object storage에 제출 원문이 고아 객체로 남을 수 있다.

### 구현 대상

- `internal/smtp/submission.go`
- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] submitted stored hook 실패 시 방금 저장한 `.eml` 객체를 삭제한다.
- [x] submitted recorder 실패 시 방금 저장한 `.eml` 객체를 삭제한다.
- [x] submitted mailbox full/quota 실패 시 방금 저장한 `.eml` 객체를 삭제한다.
- [x] `go test ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-248: IMAP auth policy enforcement audit
