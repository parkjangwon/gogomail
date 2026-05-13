# ACTIVE_TASK

## TASK-331: POP3 message size adapter coverage audit

### 배경

POP3 message size 정규화 helper가 실제 mailbox 생성 경로에서 사용되는지도 고정해야
한다. helper 테스트만 있으면 Authenticate가 다시 직접 `int64`를 `int`로 변환하는 회귀를
놓칠 수 있으므로, maildb summary fixture를 통해 `MessageSize` 결과를 직접 검증한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 Authenticate가 summary size 정규화 helper를 사용하는지 mailbox 경로에서 검증한다.
- [x] 음수 summary size가 `MessageSize=0`으로 노출되는지 검증한다.
- [x] 0 및 양수 summary size가 `MessageSize`에서 안정적으로 유지되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-332: POP3 invalid message index size audit
