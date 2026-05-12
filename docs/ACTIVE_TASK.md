# ACTIVE_TASK

## ✅ TASK-191: CalDAV MKCALENDAR property failure multistatus

### 배경

CalDAV `MKCALENDAR` request body의 unsupported/protected creation property나
지원하지 않는 property 값이 일반 `400` 또는 silent skip으로 처리될 수 있다.
RFC 4791 `MKCALENDAR` semantics에 맞춰 property별 실패를 `mkcalendar-response`
multistatus body로 반환하고, 실패 시 calendar collection을 생성하지 않도록 한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/response.go`
- `internal/caldavgw/xml.go`
- `internal/caldavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] MKCALENDAR parser가 supported, unsupported, invalid/protected property metadata를 보존한다.
- [x] unsupported/protected/invalid creation property가 포함되면 repository create를 호출하지 않고 atomic하게 실패한다.
- [x] 실패 응답은 `mkcalendar-response` XML이며 실패 property는 적절한 `403`/`409`, 의존 property는 `424 Failed Dependency` propstat로 반환한다.
- [x] 회귀 테스트가 실패 요청에서 calendar collection이 생성되지 않음을 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-192: CardDAV/CalDAV creation body strictness audit
