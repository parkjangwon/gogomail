# ACTIVE_TASK

## TASK-348: POP3 connection-close test helper cleanup

### 배경

POP3 연결 종료 회귀 테스트가 늘어나면서 데드라인 설정과 greeting 검증 로직이
반복되기 시작했다. 종료 경로 테스트의 신뢰성을 유지하면서 다음 테스트를 쉽게
추가할 수 있도록 전용 헬퍼로 정리한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] read deadline을 설정하고 greeting을 검증하는 POP3 테스트 연결 헬퍼를 추가한다.
- [x] authorization 상태 `QUIT` 연결 종료 테스트가 새 헬퍼를 사용하도록 정리한다.
- [x] transaction 상태 `QUIT` 연결 종료 테스트가 새 헬퍼를 사용하도록 정리한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-349: POP3 STLS failure connection close audit
