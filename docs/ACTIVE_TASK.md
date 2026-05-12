# ACTIVE_TASK

## ✅ TASK-207: CardDAV/CalDAV collection creation xml:lang PROPFIND compatibility audit

### 배경

TASK-206에서 `PROPPATCH`로 저장한 `DAV:prop xml:lang`은 `PROPFIND`로 회수 가능해졌다.
하지만 collection 생성 경로인 CalDAV `MKCALENDAR`와 CardDAV extended `MKCOL`도
WebDAV property set grammar를 사용하므로, 생성 요청의 `DAV:prop xml:lang` 역시
displayname/description property와 함께 저장되고 이후 collection `PROPFIND`에서 반환되어야 한다.

### 구현 대상

- `internal/caldavgw/repository.go`
- `internal/caldavgw/xml.go`
- `internal/caldavgw/handler.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/repository.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `MKCALENDAR`가 `DAV:prop xml:lang`을 `DAV:displayname`/`CALDAV:calendar-description` 생성 값과 함께 저장한다.
- [x] CalDAV 생성 직후 collection `PROPFIND`가 저장된 language tag를 해당 property의 `xml:lang`으로 반환한다.
- [x] CardDAV extended `MKCOL`이 `DAV:prop xml:lang`을 `DAV:displayname`/`CARDDAV:addressbook-description` 생성 값과 함께 저장한다.
- [x] CardDAV 생성 직후 collection `PROPFIND`가 저장된 language tag를 해당 property의 `xml:lang`으로 반환한다.
- [x] malformed creation `xml:lang` 값은 collection 생성 전에 거부한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-208: CardDAV/CalDAV collection xml:lang migration integration audit
