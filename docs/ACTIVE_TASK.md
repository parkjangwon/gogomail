# ACTIVE_TASK

## ✅ TASK-190: CardDAV MKCOL request body property semantics audit

### 배경

CardDAV extended `MKCOL`이 request body의 unsupported property와 지원하지 않는
`DAV:resourcetype` 값을 조용히 무시한 뒤 `201 Created`를 반환할 수 있었다.
RFC 5689 extended MKCOL semantics에 맞춰 property 설정 실패를 `DAV:mkcol-response`
property status로 반환하고, 실패 시 address book을 생성하지 않도록 atomicity를 보장한다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/response.go`
- `internal/carddavgw/types.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] MKCOL parser가 supported, unsupported, invalid `resourcetype` metadata를 보존한다.
- [x] unsupported property가 포함되면 repository create를 호출하지 않고 atomic하게 실패한다.
- [x] 지원하지 않는 `resourcetype` 값이 포함되면 `DAV:valid-resourcetype` 실패로 반환한다.
- [x] 실패 응답은 `DAV:mkcol-response` XML이며 실패 property는 `403 Forbidden`, 의존 property는 `424 Failed Dependency` propstat로 반환한다.
- [x] OPTIONS `DAV` discovery가 RFC 5689 `extended-mkcol` 토큰을 광고한다.
- [x] 회귀 테스트가 unsupported/invalid resourcetype 요청에서 address book이 생성되지 않음을 검증한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-191: CalDAV MKCALENDAR property failure multistatus
