# ACTIVE_TASK

## TASK-346: POP3 QUIT success connection close audit

### 배경

POP3 `QUIT`이 삭제 커밋에 성공하면 서버는 `+OK` 응답 후 연결을 종료해야 한다.
응답만 검증하면 세션이 열린 채 남는 회귀를 놓칠 수 있으므로 실제 TCP 연결에서
EOF까지 확인한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 성공하는 삭제 커밋 이후 `QUIT`이 `+OK`를 반환하는지 검증한다.
- [x] 성공한 `QUIT` 이후 같은 TCP 연결에서 추가 라인을 읽을 수 없는지 검증한다.
- [x] 연결 종료 검증이 무한 대기하지 않도록 클라이언트 read deadline을 둔다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-347: POP3 AUTH QUIT connection close audit
