# ACTIVE_TASK

## ✅ TASK-201: CardDAV/CalDAV PROPPATCH property count aggregation audit

### 배경

`PROPPATCH` parser는 각 `DAV:prop` 블록 안에서만 property 개수를 제한한다.
여러 `DAV:set`/`DAV:remove` instruction 또는 여러 `DAV:prop` 블록으로 나뉜 요청이
전체 `MaxWebDAVProperties` 한도를 우회하지 못하도록 요청 전체 기준의 property count를
집계한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`가 여러 `DAV:prop` 블록에 걸친 전체 property 수를 집계해 `MaxWebDAVProperties` 초과를 거부한다.
- [x] CardDAV `PROPPATCH`가 여러 `DAV:prop` 블록에 걸친 전체 property 수를 집계해 `MaxWebDAVProperties` 초과를 거부한다.
- [x] supported, unsupported, protected property 모두 전체 한도에 포함된다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-202: CardDAV/CalDAV PROPPATCH duplicate property semantics audit
