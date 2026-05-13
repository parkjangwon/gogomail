# ACTIVE_TASK

## TASK-360: POP3 AUTH PLAIN challenge invalid base64 capability audit

### 배경

POP3 `AUTH PLAIN`은 초기 응답 없이 challenge 방식으로도 credential을 받을 수 있다.
continuation 입력이 invalid base64여도 서버는 authorization 상태를 유지해야 하므로
inline 입력과 별도로 CAPA와 일반 로그인 흐름을 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH PLAIN` challenge continuation의 invalid base64가 `-ERR invalid base64`를 반환하는지 검증한다.
- [x] continuation 오류 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 오류 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-361: POP3 AUTH PLAIN challenge invalid format capability audit
