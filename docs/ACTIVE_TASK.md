# ACTIVE_TASK

## ✅ TASK-182: CalDAV calendar slug/timezone write-through 보강

### 배경

CalDAV XML parser와 repository는 `calendar-slug` 및 `calendar-timezone` 속성을 이미 지원하지만,
핸들러의 `MKCALENDAR`/`PROPPATCH` 경로 일부가 파싱된 값을 repository 요청으로 전달하지 못했다.
또한 PROPPATCH multistatus 응답에서 Apple iCalendar namespace 속성을 직렬화할 prefix가 없어
slug 응답이 실패할 수 있었다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/response.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MKCALENDAR`가 parsed `calendar-timezone`을 `CreateCalendarAtPathRequest`로 전달한다.
- [x] `PROPPATCH`가 parsed `calendar-slug` 및 `calendar-timezone`을 `UpdateCalendarRequest`로 전달한다.
- [x] PROPPATCH multistatus 응답이 `calendar-slug` 및 `calendar-timezone` 값을 반환한다.
- [x] Apple iCalendar namespace prefix를 WebDAV XML serializer에 등록한다.
- [x] handler 회귀 테스트가 slug/timezone 생성 및 수정 흐름을 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-183: CardDAV addressbook-query metadata/search index 고도화
