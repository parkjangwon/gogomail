# ACTIVE_TASK

## ✅ TASK-189: CardDAV PROPPATCH unsupported/protected property multistatus

### 배경

CardDAV `PROPPATCH`가 unsupported property를 조용히 무시하거나 protected property 제거를
일반 `400` parse error로 반환했다.
WebDAV RFC 4918의 PROPPATCH semantics에 맞춰 property별 multistatus 실패를 반환하고,
요청 전체를 atomic하게 실패시켜 부분 업데이트처럼 보이지 않도록 한다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/xml_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] PROPPATCH parser가 unsupported property와 protected property remove 시도를 request metadata로 보존한다.
- [x] unsupported/protected property가 포함되면 repository update를 호출하지 않고 atomic하게 실패한다.
- [x] 실패 응답은 `207 Multi-Status`이며 unsupported/protected property는 `403 Forbidden` propstat로 반환한다.
- [x] 같은 요청의 otherwise mutable property는 `424 Failed Dependency` propstat로 반환한다.
- [x] 회귀 테스트가 unsupported/protected 혼합 요청에서 address book property가 변경되지 않음을 검증한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-190: CardDAV MKCOL request body property semantics audit
