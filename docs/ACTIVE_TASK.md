# ACTIVE_TASK

## ✅ TASK-195: CardDAV unsupported namespace response robustness

### 배경

CardDAV `PROPPATCH`와 extended `MKCOL`은 unsupported property를 request metadata로 보존하고
property-level 실패 응답에 포함한다. 그러나 serializer가 알려진 DAV/CardDAV/CalendarServer
namespace만 prefix로 직렬화할 수 있어, 클라이언트가 임의 namespace의 property를 보낼 경우
RFC 실패 응답 대신 내부 오류가 날 수 있다.

### 구현 대상

- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/response.go`
- `internal/carddavgw/response_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV multistatus/mkcol-response serializer가 unknown namespace property를 XML namespace declaration과 함께 직렬화한다.
- [x] PROPPATCH unknown namespace property 실패가 `500` 대신 `207 Multi-Status`로 반환된다.
- [x] MKCOL unknown namespace property 실패가 `500` 대신 property failure 응답으로 반환된다.
- [x] 회귀 테스트가 unknown namespace property 이름이 응답 body에 보존됨을 검증한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-196: CalDAV unsupported namespace response robustness
