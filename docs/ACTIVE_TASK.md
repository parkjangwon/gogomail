# ACTIVE_TASK

## TASK-321: POP3 inbox folder casing audit

### 배경

POP3는 사용자의 INBOX를 mailbox로 노출한다. folder `SystemType`은 DB 마이그레이션,
관리자 API, 복구 경로에 따라 casing이 달라질 수 있으므로 POP3 adapter는 `inbox`를
대소문자 구분 없이 찾아야 한다. 이 동작이 회귀하지 않도록 인증 후 INBOX 식별 경로를
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `SystemType=INBOX` folder를 POP3 INBOX로 식별하는지 검증한다.
- [x] casing이 다른 INBOX folder에서도 mailbox message count가 로드되는지 검증한다.
- [x] casing이 다른 INBOX folder에서도 inbox page 조회가 정규화된 user ID로 수행되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-322: POP3 inbox folder first-match audit
