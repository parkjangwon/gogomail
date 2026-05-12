# ACTIVE_TASK

## ✅ TASK-206: CardDAV/CalDAV PROPPATCH xml:lang handling audit

### 배경

RFC 4918 `set` element는 `DAV:prop` scope의 `xml:lang` language tagging information을
property value와 함께 저장하고 이후 `PROPFIND`로 회수 가능해야 한다고 요구한다.
CalDAV/CardDAV collection `PROPPATCH`에서 `DAV:displayname`과 description 계열 property의
`xml:lang`을 파싱, 저장, 응답 직렬화까지 보존한다.

### 구현 대상

- `migrations/0097_dav_collection_property_lang.sql`
- `internal/caldavgw/repository.go`
- `internal/caldavgw/xml.go`
- `internal/caldavgw/handler.go`
- `internal/caldavgw/response.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/repository.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/response.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`가 `DAV:prop xml:lang`을 `DAV:displayname`/`CALDAV:calendar-description` 업데이트와 함께 저장한다.
- [x] CalDAV `PROPFIND`/`PROPPATCH` 성공 응답이 저장된 language tag를 해당 property의 `xml:lang`으로 반환한다.
- [x] CardDAV `PROPPATCH`가 `DAV:prop xml:lang`을 `DAV:displayname`/`CARDDAV:addressbook-description` 업데이트와 함께 저장한다.
- [x] CardDAV `PROPFIND`/`PROPPATCH` 성공 응답이 저장된 language tag를 해당 property의 `xml:lang`으로 반환한다.
- [x] remove는 해당 property value와 language tag를 함께 clear한다.
- [x] malformed `xml:lang` 값은 mutation 전에 거부한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-207: CardDAV/CalDAV collection PROPFIND xml:lang compatibility audit
