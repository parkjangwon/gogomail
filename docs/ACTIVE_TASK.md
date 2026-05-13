# ACTIVE_TASK

## TASK-249: IMAP connection deadline audit

### 배경

IMAP 게이트웨이는 장기 TCP 세션을 유지하지만 명령 읽기, 응답 쓰기, IDLE 대기
구간에 명시적인 deadline 설정이 없었다. 느린 클라이언트나 끊어진 연결이 고루틴과
connection slot을 오래 점유하지 않도록 프로토콜 서버와 런타임 설정을 함께 묶는다.

### 구현 대상

- `internal/imapgw/server.go`
- `internal/imapgw/server_test.go`
- `internal/config/config.go`
- `internal/config/config_file.go`
- `internal/config/validate.go`
- `internal/config/*_test.go`
- `internal/app/run.go`
- `internal/app/run_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] IMAP 서버 옵션이 read/write/idle timeout을 표현하고 음수 값을 거절한다.
- [x] IMAP 서버 루프가 명령 읽기, 응답 쓰기, STARTTLS handshake, IDLE 대기 전에 deadline을 갱신한다.
- [x] 환경 변수와 YAML 설정에서 IMAP timeout 값을 로드하고 검증한다.
- [x] 앱 런타임이 설정된 IMAP timeout 값을 프로토콜 서버에 전달한다.
- [x] `go test ./internal/imapgw ./internal/config ./internal/app` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-250: IMAP capability and session policy audit
