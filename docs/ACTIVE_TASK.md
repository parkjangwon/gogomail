# ACTIVE_TASK

## ✅ TASK-203: CardDAV/CalDAV PROPPATCH remove element emptiness audit

### 배경

RFC 4918 `PROPPATCH`의 `DAV:remove` instruction은 제거할 property name만 담아야 하며,
`DAV:prop` 안의 property element는 값이나 nested XML을 갖지 않는 빈 element여야 한다.
현재 parser가 remove 대상 property body를 skip하면 malformed remove 요청이 정상 remove나
property-level failure로 처리될 수 있으므로, remove property element emptiness를 엄격히 검사한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH` remove property가 text value를 포함하면 parse error로 거부한다.
- [x] CalDAV `PROPPATCH` remove property가 nested XML child를 포함하면 parse error로 거부한다.
- [x] CardDAV `PROPPATCH` remove property가 text value를 포함하면 parse error로 거부한다.
- [x] CardDAV `PROPPATCH` remove property가 nested XML child를 포함하면 parse error로 거부한다.
- [x] empty remove property는 기존 supported/unsupported/protected property semantics를 유지한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-204: CardDAV/CalDAV PROPPATCH set/remove instruction emptiness audit
