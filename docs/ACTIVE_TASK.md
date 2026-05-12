# ACTIVE_TASK

## ✅ TASK-188: WebDAV If header conditional request support

### 배경

CalDAV/CardDAV는 `If-Match`, `If-None-Match`, date conditional headers를 지원하지만
WebDAV RFC 4918의 `If` header 자체는 평가하지 않았다.
현재 서버는 lock token 저장소가 없으므로 positive state-token은 실패시키고,
ETag 조건 list를 기존 object/collection ETag precondition과 통합해 안전하게 지원한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] CalDAV object GET/HEAD/PUT/DELETE가 WebDAV `If` ETag condition list를 평가한다.
- [x] CalDAV calendar collection PROPPATCH/DELETE/MKCALENDAR precondition 경로가 WebDAV `If` ETag condition list를 평가한다.
- [x] CardDAV contact object GET/HEAD/PUT/DELETE가 WebDAV `If` ETag condition list를 평가한다.
- [x] CardDAV address-book collection PROPPATCH/DELETE/MKCOL precondition 경로가 WebDAV `If` ETag condition list를 평가한다.
- [x] positive lock/state-token 조건은 현재 lock store가 없으므로 실패하며, `Not` 조건은 RFC condition semantics에 따라 평가된다.
- [x] 실패한 `If` precondition은 request body read 전에 `412 Precondition Failed`로 반환된다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-189: CardDAV PROPPATCH/MKCOL RFC response semantics audit
