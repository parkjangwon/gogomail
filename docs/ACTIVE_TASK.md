# ACTIVE_TASK

## ✅ TASK-193: CalDAV/CardDAV creation response cache-control audit

### 배경

CalDAV `MKCALENDAR` RFC 4791은 응답에 `Cache-Control: no-cache`를 요구하고,
RFC 5689 extended `MKCOL` 예제 역시 creation 응답을 cache하지 않도록 다룬다.
현재 생성 성공/실패 응답은 `no-store`만 사용하므로, 기존 보수적 캐시 차단은 유지하면서
`no-cache`도 명시해 RFC 문구와 클라이언트 기대를 함께 만족시킨다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `MKCALENDAR` 성공 응답이 `no-cache`와 `no-store`를 모두 포함한다.
- [x] CalDAV `MKCALENDAR` property failure `207` 응답이 `no-cache`와 `no-store`를 모두 포함한다.
- [x] CardDAV extended `MKCOL` 성공 응답이 `no-cache`와 `no-store`를 모두 포함한다.
- [x] CardDAV extended `MKCOL` property failure 응답이 `no-cache`와 `no-store`를 모두 포함한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-194: CalDAV/CardDAV MKCOL/MKCALENDAR XML content-type validation
