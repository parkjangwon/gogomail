# ACTIVE_TASK

## ✅ TASK-199: CalDAV/CardDAV PROPPATCH unsupported namespace parity audit

### 배경

CardDAV `PROPPATCH`는 unsupported/protected property를 request metadata로 보존하고
`207 Multi-Status`에서 실패 property와 dependent property를 분리해 반환한다.
CalDAV `PROPPATCH`도 RFC 4918 atomic property update semantics에 맞춰 arbitrary namespace의
unsupported property와 protected remove 시도를 같은 방식으로 처리해야 한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH` parser가 unsupported property와 protected property remove 시도를 request metadata로 보존한다.
- [x] unsupported/protected property가 포함되면 repository update를 호출하지 않고 atomic하게 실패한다.
- [x] 실패 응답은 `207 Multi-Status`이며 unsupported/protected property는 `403 Forbidden`, mutable dependent property는 `424 Failed Dependency`로 반환한다.
- [x] arbitrary namespace property가 `500` 없이 응답 body에 보존된다.
- [x] 회귀 테스트가 실패 요청에서 calendar property가 변경되지 않음을 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-200: CardDAV/CalDAV PROPPATCH XML structural strictness audit
