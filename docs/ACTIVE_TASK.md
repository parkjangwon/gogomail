# ACTIVE_TASK

## TASK-243: POP3 implicit TLS listener documentation/runtime audit

### 배경

문서는 POP3S implicit TLS를 지원한다고 설명하지만 런타임에는 POP3 listener와
STLS만 연결되어 있다. POP3S 주소를 선택적으로 설정해 같은 POP3 서버 인스턴스가
TLS listener도 함께 serve하도록 만들고, plain POP3와 POP3S가 연결 제한과
maildrop 잠금을 공유하게 한다.

### 구현 대상

- `internal/config/*`
- `internal/app/run.go`
- `internal/app/run_test.go`
- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `GOGOMAIL_POP3S_ADDR` / YAML `pop3s_addr`를 설정할 수 있다.
- [x] POP3S listener가 TLS 설정을 요구하고 같은 POP3 server 인스턴스를 공유한다.
- [x] implicit TLS 연결에서는 `STLS`가 CAPA에 광고되지 않는다.
- [x] `go test ./internal/config ./internal/app ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-244: SMTP inbound domain policy multi-recipient audit
