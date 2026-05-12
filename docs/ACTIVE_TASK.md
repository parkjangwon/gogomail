# ACTIVE_TASK

## ✅ TASK-194: CalDAV/CardDAV MKCOL/MKCALENDAR XML content-type validation

### 배경

RFC 5689 extended `MKCOL`은 XML body가 있을 때 XML `Content-Type`을 요구하고,
CalDAV `MKCALENDAR`도 body가 포함되면 XML로 해석된다.
현재 생성 핸들러는 body를 파싱하기 전에 `Content-Type`을 검증하지 않는다.
호환성을 위해 빈 body 또는 header가 없는 기존 클라이언트는 유지하되,
명시적으로 잘못된/중복된 `Content-Type`은 생성 전에 거부한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `MKCALENDAR`가 non-empty body의 non-XML `Content-Type`을 `415`로 거부한다.
- [x] CardDAV extended `MKCOL`이 non-empty body의 non-XML `Content-Type`을 `415`로 거부한다.
- [x] 중복 또는 malformed `Content-Type`은 body parse/create 전에 `400`으로 거부한다.
- [x] header가 없는 호환 요청과 `application/xml; charset=utf-8` 요청은 기존 성공 경로를 유지한다.
- [x] 회귀 테스트가 rejected media-type 요청에서 collection이 생성되지 않음을 검증한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-195: CardDAV unsupported namespace response robustness
