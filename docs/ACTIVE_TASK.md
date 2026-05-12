# ACTIVE_TASK

## ✅ TASK-196: CalDAV unsupported namespace response robustness

### 배경

CalDAV `MKCALENDAR` property failure와 WebDAV multistatus 응답도 알려진 namespace만
직렬화할 수 있으면, 임의 namespace의 unsupported property가 들어왔을 때 RFC 실패 응답 대신
내부 오류로 빠질 수 있다. CardDAV와 동일하게 unknown namespace property를 응답 XML에 보존한다.

### 구현 대상

- `internal/caldavgw/handler_test.go`
- `internal/caldavgw/response.go`
- `internal/caldavgw/response_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV multistatus/mkcalendar-response serializer가 unknown namespace property를 XML namespace declaration과 함께 직렬화한다.
- [x] MKCALENDAR unknown namespace property 실패가 `500` 대신 `207 Multi-Status`로 반환된다.
- [x] 회귀 테스트가 unknown namespace property 이름이 응답 body에 보존됨을 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-197: CardDAV/CalDAV creation XML body presence semantics audit
