# ACTIVE_TASK

## TASK-239: POP3 CAPA semantics audit

### 배경

POP3 `CAPA` 응답은 세션 상태에 따라 현재 사용할 수 있는 기능만 광고해야 한다.
현재 TRANSACTION 상태에서도 인증 전용 `USER`/`SASL` 기능을 계속 노출해 클라이언트
자동 구성과 RFC 2449 의미론이 어긋난다. 공통 기능과 인증 전용 기능을 분리하고,
기본 서버 식별/로그인 지연 정보를 명시해 CAPA 응답을 더 예측 가능하게 만든다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] AUTHORIZATION 상태 CAPA는 `USER`/`SASL` 인증 기능을 광고한다.
- [x] TRANSACTION 상태 CAPA는 `USER`/`SASL` 인증 전용 기능을 광고하지 않는다.
- [x] CAPA가 `IMPLEMENTATION` 및 `LOGIN-DELAY 0` 같은 안정적인 서버 메타 기능을 포함한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-240: POP3 AUTH argument validation audit
