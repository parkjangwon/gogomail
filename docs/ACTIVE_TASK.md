# ACTIVE_TASK

## TASK-393: POP3 DELE invalid command sequence preserves pending delete docs audit

### 배경

POP3 `DELE` 이후 상태 보존은 정상 no-op 계열과 오류 계열 명령 모두에서 중요하다.
이미 추가된 wire-level 회귀들이 서로 어떤 명령군을 덮는지 문서에 명확히 남겨
중복 테스트 없이 감사 가능하게 만든다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 정상 조회/no-op 계열: `NOOP`, `CAPA`가 pending delete를 보존하는 테스트를 확인한다.
- [x] 파서/unknown 오류 계열: 빈 명령과 unknown command가 pending delete를 보존하는 테스트를 확인한다.
- [x] 재인증/STLS 거부 계열: `AUTH`, `USER/PASS`, `STLS` 거부가 pending delete를 보존하는 테스트를 확인한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-394: POP3 DELE invalid command sequence helper cleanup
