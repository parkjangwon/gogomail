# ACTIVE_TASK

## TASK-269: POP3 message listing consistency audit

### 배경

POP3 `STAT`, `LIST`, `RETR`는 같은 maildrop snapshot의 message size를
일관되게 보고해야 한다. `LIST`/`STAT`는 mailbox metadata size를 쓰는데
`RETR`가 content 문자열 길이를 다시 계산하면 line-ending 정규화나 저장소
표현 차이로 octet count가 서로 달라질 수 있다.

### 구현 대상

- `internal/pop3d/pop3d.go`
- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `RETR`의 octet count가 `LIST`/`STAT`와 같은 `MessageSize` 기준을 사용한다.
- [x] LF-only content에서도 `LIST`와 `RETR`가 같은 size를 알리는 회귀 테스트를 추가한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-270: POP3 delete commit idempotency audit
