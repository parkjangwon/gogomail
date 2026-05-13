# ACTIVE_TASK

## TASK-234: POP3 STLS transaction-state capability audit

### 배경

POP3 `STLS`는 RFC 2595에 따라 인증 전에 TLS 협상을 시작하기 위한 명령이다.
현재 구현은 TLS 설정이 있으면 인증 후 TRANSACTION 상태의 `CAPA`에서도 `STLS`를
광고하고, `STLS` 명령이 인증 후에도 허용될 수 있는 구조다. 인증 후에는 TLS
재협상을 광고하거나 받아들이지 않도록 상태를 고정해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] TLS 설정이 없으면 `CAPA`가 `STLS`를 광고하지 않고 `STLS` 명령은 `-ERR`를 반환한다.
- [x] AUTHORIZATION 상태에서 TLS 설정이 있을 때만 `CAPA`가 `STLS`를 광고한다.
- [x] TRANSACTION 상태 `CAPA`는 TLS 설정이 있어도 `STLS`를 광고하지 않는다.
- [x] TRANSACTION 상태 `STLS` 명령은 `-ERR`를 반환하고 세션을 유지한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-235: POP3 RETR fetch-error response audit
