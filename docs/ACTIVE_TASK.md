# ACTIVE_TASK

## TASK-381: POP3 transaction STLS denial session audit

### 배경

POP3 transaction 상태에서는 `STLS`를 허용하지 않아야 한다. 이미 wire-level 회귀가
있으므로 중복 테스트를 추가하지 않고, 기존 커버리지가 명확한 `-ERR` 메시지와
세션 유지 조건을 충족하는지 재검증한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `TestPOP3STLSDeniedInTransactionStateKeepsSessionUsable`가 transaction `STLS`의 `-ERR` 거부를 검증한다.
- [x] 같은 회귀가 거부 이후 `NOOP` 성공을 검증한다.
- [x] 같은 회귀가 거부 이후 `STAT` 성공을 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-382: POP3 transaction CAPA stability audit
