# ACTIVE_TASK

## TASK-394: POP3 DELE invalid command sequence helper cleanup

### 배경

POP3 `DELE` 이후 pending delete 보존 회귀들이 공통적으로 `LIST 1` 실패와 `STAT`
카운트 유지 검증을 반복한다. 같은 검증을 헬퍼로 모아 중복을 줄이고 이후 테스트가
같은 기준을 쓰도록 정리한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] pending delete가 `LIST 1`에서 숨겨지고 `STAT`에서 제외되는지 확인하는 공통 헬퍼를 추가한다.
- [x] `NOOP`, `CAPA`, unknown/empty command 회귀가 공통 헬퍼를 사용하도록 정리한다.
- [x] `AUTH`, `USER/PASS`, `STLS` 거부 회귀가 공통 헬퍼를 사용하도록 정리한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-395: POP3 DELE RSET clears pending delete audit
