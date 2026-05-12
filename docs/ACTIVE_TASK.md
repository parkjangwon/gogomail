# ACTIVE_TASK

## ✅ TASK-204: CardDAV/CalDAV PROPPATCH set/remove instruction emptiness audit

### 배경

RFC 4918 `propertyupdate`의 `DAV:set`/`DAV:remove` instruction은 각각 하나 이상의
`DAV:prop` child를 포함해야 실질적인 property update 지시가 된다. 빈 instruction이 전체
요청의 다른 property 때문에 조용히 허용되지 않도록 각 instruction 단위에서 `DAV:prop`
존재를 엄격히 검사한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`가 빈 `DAV:set` instruction을 parse error로 거부한다.
- [x] CalDAV `PROPPATCH`가 빈 `DAV:remove` instruction을 parse error로 거부한다.
- [x] CardDAV `PROPPATCH`가 빈 `DAV:set` instruction을 parse error로 거부한다.
- [x] CardDAV `PROPPATCH`가 빈 `DAV:remove` instruction을 parse error로 거부한다.
- [x] 한 instruction에 `DAV:prop`이 있으면 기존 supported/unsupported/protected property semantics를 유지한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-205: CardDAV/CalDAV PROPPATCH prop child cardinality audit
