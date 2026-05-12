# ACTIVE_TASK

## ✅ TASK-187: CalDAV calendar-timezone VTIMEZONE RFC 정합성

### 배경

CalDAV `calendar-timezone` property는 클라이언트가 VTIMEZONE payload로 해석할 수 있어야 하지만,
응답은 저장된 Olson TZID 문자열을 그대로 반환했다.
또한 timezone service가 생성한 iCalendar 본문은 `X-WR-CALDESC`/`X-PUBLISHED-LL`을
`END:VCALENDAR` 뒤에 붙여 malformed calendar가 될 수 있었다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/response.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/client_fixtures_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] `calendar-timezone` property 응답이 저장된 TZID를 VTIMEZONE iCalendar payload로 직렬화한다.
- [x] timezone service의 generated calendar properties가 `END:VCALENDAR` 앞에 위치한다.
- [x] 저장소 canonical timezone 값은 기존 TZID 형식을 유지해 write path와 time.LoadLocation 연동을 보존한다.
- [x] Apple iCal fixture가 `calendar-timezone` VTIMEZONE payload를 검증한다.
- [x] handler 회귀 테스트가 VTIMEZONE calendar property 위치를 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-188: WebDAV If header conditional request support audit
