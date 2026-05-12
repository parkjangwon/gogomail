# ACTIVE_TASK

## ✅ TASK-205: CardDAV/CalDAV PROPPATCH prop child cardinality audit

### 배경

RFC 4918 grammar는 `DAV:set`과 `DAV:remove`를 각각 `(prop)`로 정의하므로,
instruction 하나에는 정확히 하나의 `DAV:prop` child만 허용된다. 현재 parser는 여러
`DAV:prop` 블록을 한 instruction 안에서 허용하므로, RFC grammar에 맞춰 두 번째
`DAV:prop`을 malformed request로 거부한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`가 한 `DAV:set` instruction 안의 두 번째 `DAV:prop`을 parse error로 거부한다.
- [x] CalDAV `PROPPATCH`가 한 `DAV:remove` instruction 안의 두 번째 `DAV:prop`을 parse error로 거부한다.
- [x] CardDAV `PROPPATCH`가 한 `DAV:set` instruction 안의 두 번째 `DAV:prop`을 parse error로 거부한다.
- [x] CardDAV `PROPPATCH`가 한 `DAV:remove` instruction 안의 두 번째 `DAV:prop`을 parse error로 거부한다.
- [x] 여러 instruction 각각에 하나의 `DAV:prop`이 있는 기존 정상 요청은 유지한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-206: CardDAV/CalDAV PROPPATCH xml:lang handling audit
