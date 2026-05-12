# ACTIVE_TASK

## ✅ TASK-198: CardDAV/CalDAV creation XML structural child strictness audit

### 배경

CalDAV `MKCALENDAR`의 non-empty body는 `DAV:set`/`DAV:prop` 구조를 엄격히 요구하도록
정리되었다. CardDAV extended `MKCOL`에도 같은 구조 엄격성이 필요하며,
unknown top-level child 또는 `DAV:set` 내부 unknown child를 skip하면 malformed creation body가
다른 실패 경로로 오인될 수 있다.

### 구현 대상

- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV `MKCOL` non-empty body가 top-level `DAV:set` 외 child를 `400` parse error로 거부한다.
- [x] CardDAV `DAV:set` 내부가 `DAV:prop` 외 child를 `400` parse error로 거부한다.
- [x] rejected structural-child 요청은 address book을 생성하지 않는다.
- [x] 기존 supported/unsupported property-level failure semantics는 유지한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-199: CalDAV/CardDAV PROPPATCH unsupported namespace parity audit
