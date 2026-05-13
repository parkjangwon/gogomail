# ACTIVE_TASK

## TASK-427: SMTP inbound domain policy auth reset audit

### 배경

SMTP inbound 수신에서 `Logout`으로 인증 상태가 초기화되면 envelope, DSN 옵션,
recipient-domain 정책 누적 상태도 함께 초기화되어야 한다. 재인증 후 성공 트랜잭션이 이전
인증 세션의 DSN metadata나 domain size 제한을 물려받으면 안 된다.

### 구현 대상

- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 인증된 세션에서 DSN 옵션을 포함한 d1+d2 혼합 도메인 트랜잭션 후 `Logout`을 수행하는 세션을 구성한다.
- [x] `Logout` 후 재인증 전 `MAIL`이 거절되고, 재인증 후 d1-only `DATA`가 이전 d2 size 제한에 막히지 않는지 검증한다.
- [x] 기록된 성공 메시지에 `Logout` 이전 DSN envelope/recipient 옵션이 남지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-428: IMAP message UID row-lock audit
