# ACTIVE_TASK

## TASK-395: POP3 DELE RSET clears pending delete audit

### 배경

POP3 `RSET`은 pending delete를 해제해야 한다. 기존 visibility 회귀를 pending delete
해제 헬퍼와 연결해 `LIST`와 `STAT` 기준으로 삭제 표시가 실제로 복구되는지 더
명확히 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] pending delete가 해제되어 `LIST 1`과 `STAT`이 복구되는지 확인하는 공통 헬퍼를 추가한다.
- [x] `DELE 1` 후 `RSET`이 `LIST 1`을 다시 성공시키는지 검증한다.
- [x] `DELE 1` 후 `RSET`이 `STAT`에서 전체 메시지 수를 다시 반환하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-396: POP3 QUIT success commits pending delete audit
