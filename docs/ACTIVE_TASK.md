# ACTIVE_TASK

## TASK-436: IMAP mailbox subscription deleted-folder noselect audit

### 배경

IMAP LSUB 호환 동작에서는 구독된 mailbox가 사라져도 구독 이름을 noselect/non-existing
항목으로 유지할 수 있어야 한다. 기존 테스트는 system folder를 DB에서 직접 삭제하는 경로만
검증했으므로 실제 user folder `DeleteFolder` 경로에서도 구독 이름이 보존되고, 존재하지
않는 mailbox로 표시되며, retained name으로 unsubscribe할 수 있는지 Postgres 통합 테스트로
고정한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] user folder를 구독한 뒤 `DeleteFolder`로 삭제하는 실제 경로를 검증한다.
- [x] 삭제 후 `ListSubscribedIMAPMailboxes`가 retained name을 noselect/non-existing 항목으로 반환하는지 검증한다.
- [x] retained subscription name으로 `UnsubscribeIMAPMailbox`가 성공하는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresIMAPMailboxSubscriptionPersistsAfterDeleteFolder` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-437: IMAP subscription name normalization audit
