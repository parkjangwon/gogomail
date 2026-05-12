# ACTIVE_TASK

## ✅ TASK-202: CardDAV/CalDAV PROPPATCH duplicate property semantics audit

### 배경

RFC 4918 `PROPPATCH`는 instruction을 document order로 처리하며 결과는 property 단위로
보고한다. 같은 property가 여러 instruction에 반복 등장할 때 최종 mutation은 마지막
instruction을 반영하되, 성공/실패 `propstat` 응답은 중복 property name을 반복하지 않도록
정규화한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `PROPPATCH`가 같은 mutable property의 반복 instruction에서 document-order 최종 값을 적용한다.
- [x] CardDAV `PROPPATCH`가 같은 mutable property의 반복 instruction에서 document-order 최종 값을 적용한다.
- [x] CalDAV 성공 및 failure dependency `propstat`가 중복 property name을 반복하지 않는다.
- [x] CardDAV 성공 및 failure dependency `propstat`가 중복 property name을 반복하지 않는다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-203: CardDAV/CalDAV PROPPATCH remove element emptiness audit
