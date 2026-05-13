# ACTIVE_TASK

## TASK-315: POP3 auth credential validation consolidation audit

### 배경

POP3 credential 검증은 username trim/empty/CRLF 거부와 password CRLF 거부를
Authenticate 경계에서 적용한다. 이 조건이 본문에 직접 흩어져 있으면 이후 SASL/USER
흐름이 추가될 때 일부 검증만 재사용되는 실수가 생길 수 있으므로, credential 검증을
작은 helper로 분리하고 직접 테스트로 계약을 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 username trim/empty/CRLF 검증을 helper로 통합한다.
- [x] POP3 password CRLF 검증을 helper로 통합한다.
- [x] username/password helper 단위 테스트로 정상/오류 케이스를 고정한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-316: POP3 username normalization passthrough audit
