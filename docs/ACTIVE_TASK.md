# ACTIVE_TASK

## ✅ TASK-197: CardDAV/CalDAV creation XML body presence semantics audit

### 배경

CalDAV `MKCALENDAR`는 absent body를 호환 목적으로 허용하지만, 공백뿐인 non-empty body는
유효한 XML `mkcalendar` body가 아니다. CardDAV extended `MKCOL`도 address-book 생성은
명시적인 XML body가 필요하며, 공백뿐인 body를 일반 empty-body 경로처럼 처리하면
클라이언트 오류가 모호해진다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `internal/caldavgw/handler_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV absent `MKCALENDAR` body는 기존 호환 생성 경로를 유지한다.
- [x] CalDAV whitespace-only non-empty `MKCALENDAR` body는 `400`으로 거부하고 calendar를 생성하지 않는다.
- [x] CardDAV whitespace-only non-empty `MKCOL` body는 `400`으로 거부하고 address book을 생성하지 않는다.
- [x] parser 단위 테스트가 absent body와 whitespace-only body를 구분한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-198: CardDAV/CalDAV creation XML structural child strictness audit
