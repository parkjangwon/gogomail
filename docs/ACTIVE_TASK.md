# ACTIVE_TASK

## ✅ TASK-192: CardDAV/CalDAV creation body strictness audit

### 배경

CardDAV extended `MKCOL`과 CalDAV `MKCALENDAR`의 기본 생성 구현이
빈 body 또는 body 안의 필수 `DAV:set`/resource-type 신호가 없는 요청을
너무 관대하게 생성 성공으로 처리할 수 있다.
RFC 5689/RFC 6352/RFC 4791의 creation body semantics에 맞춰 지원하는 생성 형태와
지원하지 않는 일반 collection 생성 형태를 명확히 분리한다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV address-book creation requires an extended MKCOL body with `DAV:resourcetype` including `DAV:collection` and `CARDDAV:addressbook`.
- [x] CardDAV empty-body/generic MKCOL requests do not create address books accidentally.
- [x] CalDAV `MKCALENDAR` with a non-empty body requires the RFC 4791 `DAV:set` shape instead of ignoring unknown children.
- [x] Regression tests prove rejected creation-body shapes do not create collections.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-193: CalDAV/CardDAV creation response cache-control audit
