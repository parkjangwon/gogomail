# ACTIVE_TASK

## TASK-238: POP3 configured connection limit audit

### 배경

`GOGOMAIL_POP3_MAX_CONNECTIONS` 설정은 config 모델에는 존재하지만 실제 POP3
서버의 accept loop에 적용되지 않는다. 운영자가 POP3 연결 상한을 설정해도 런타임
과부하를 막지 못하므로, 서버에 연결 슬롯을 추가하고 env/YAML 설정과 검증 경로가
동일하게 동작하도록 연결해야 한다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `internal/app/run.go`
- `internal/config/*`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 server가 `MaxConnections` 초과 연결을 `-ERR` 후 닫는다.
- [x] 닫힌 연결의 슬롯이 해제되어 후속 연결이 가능하다.
- [x] `GOGOMAIL_POP3_MAX_CONNECTIONS` 및 YAML `pop3_max_connections`가 검증/런타임에 반영된다.
- [x] `go test ./internal/pop3d ./internal/config ./internal/app` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-239: POP3 CAPA semantics audit
