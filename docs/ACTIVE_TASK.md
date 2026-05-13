# ACTIVE_TASK

## TASK-287: IMAP event broker event type normalization audit

### 배경

IMAP mailbox event broker는 user/mailbox identity를 정규화하지만, event type은
빈 값만 검사한다. `" exists "`처럼 공백이 섞인 type은 publish 검증을 통과한 뒤
서버의 event switch에서 처리되지 않을 수 있고, 알 수 없는 type도 브로커를
지나갈 수 있다. 브로커 입구에서 event type을 trim하고 지원되는 type만 허용해
producer 오류가 조용히 유실되지 않게 해야 한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] IMAP event broker 발행 event type이 trim된 값으로 fanout된다.
- [x] 지원되지 않는 event type은 publish 단계에서 에러로 거부된다.
- [x] 공백이 섞인 event type과 unsupported type 테스트를 추가한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-288: IMAP event broker slow-subscriber metrics audit
