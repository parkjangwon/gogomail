# ACTIVE_TASK

## TASK-240: POP3 AUTH argument validation audit

### 배경

POP3 `AUTH` 명령은 SASL 메커니즘별 인자 수를 엄격하게 검증해야 한다. 현재
`AUTH PLAIN`은 세 번째 이후 인자를 조용히 무시하고, `AUTH LOGIN`도 잘못 붙은
초기 인자를 무시한 채 continuation 플로우로 진입한다. 잘못된 인자를 즉시
`-ERR syntax error`로 거절하고 세션을 AUTHORIZATION 상태로 유지해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH PLAIN`이 메커니즘+선택 초기응답 외 추가 인자를 거절한다.
- [x] `AUTH LOGIN`이 추가 인자를 거절하고 continuation을 시작하지 않는다.
- [x] AUTH 문법 오류 후 같은 연결에서 정상 USER/PASS 인증이 가능하다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-241: POP3 AUTH cancellation audit
