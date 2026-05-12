# ACTIVE_TASK

## ✅ TASK-200: CardDAV/CalDAV PROPPATCH XML structural strictness audit

### 배경

`PROPPATCH` body의 top-level 구조는 이미 `DAV:set`/`DAV:remove`만 허용하지만,
각 instruction 내부에서 `DAV:prop` 외 structural child를 skip하면 malformed 요청이
조용히 무시될 수 있다. RFC 4918 propertyupdate grammar에 맞춰 instruction 내부 구조도
엄격히 검사한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`의 `DAV:set`/`DAV:remove` 내부가 `DAV:prop` 외 child를 parse error로 거부한다.
- [x] CardDAV `PROPPATCH`의 `DAV:set`/`DAV:remove` 내부가 `DAV:prop` 외 child를 parse error로 거부한다.
- [x] property-level unsupported property failure semantics는 `DAV:prop` 내부에서만 유지된다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-201: CardDAV/CalDAV PROPPATCH property count aggregation audit
