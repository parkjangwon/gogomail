# ACTIVE_TASK

## TASK-267: IMAP mailbox status consistency audit

### 배경

IMAP `SELECT`, `STATUS`, `LIST RETURN STATUS`는 mailbox message count뿐 아니라
`UIDNEXT`와 `HIGHESTMODSEQ`도 함께 노출한다. DB에 active message가 있지만 아직
IMAP UID가 lazy 할당되지 않은 경우 EXISTS에는 포함되면서 UID 상태 예측값에는
빠져 클라이언트가 낮은 `UIDNEXT`를 볼 수 있다.

### 구현 대상

- `internal/maildb/messages.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/imap_uid_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] folder 집계에 IMAP UID 미배정 active message 수를 포함한다.
- [x] mailbox status 변환 시 미배정 메시지를 `UIDNEXT`/`HIGHESTMODSEQ` 예측값에 반영한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-268: IMAP lazy UID assignment ordering audit
