# ACTIVE_TASK

## ✅ TASK-178: CardDAV sync-collection 증분 응답 단일 조회 및 truncation 정합성

### 배경

CalDAV/CardDAV RFC 완성도와 성능 고도화 라운드의 첫 작업으로 CardDAV 증분 동기화 핫 패스를 정리했다.
기존 CardDAV `sync-collection` 증분 응답은 변경 목록 조회 후 각 변경 객체마다 `LookupContactObject`를 반복해
변경량에 비례한 N+1 조회를 만들었다.

또한 증분 변경이 `limit`을 초과할 때 일반 텍스트 오류로 빠져 snapshot truncation과 다른 응답 형태가 발생했다.
RFC 6578/WebDAV 클라이언트가 동일한 precondition 응답으로 처리할 수 있도록 정합성을 맞췄다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/repository.go`
- `internal/carddavgw/types.go`
- `internal/carddavgw/handler_test.go`
- `migrations/0096_carddav_sync_changes_covering_index.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] CardDAV `sync-collection` 증분 응답에서 변경 레코드와 active contact object를 단일 조인 경로로 조회한다.
- [x] 변경 객체 응답에서 per-change `LookupContactObject` 반복 호출을 제거한다.
- [x] 삭제/미발견 contact object는 기존 `404 Not Found` multistatus 응답을 유지한다.
- [x] 증분 변경 truncation을 snapshot truncation과 같은 RFC 6578/WebDAV XML precondition 응답으로 매핑한다.
- [x] sync marker/증분 변경 조회용 커버링 인덱스를 추가한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-179: CalDAV calendar-query time-range limit 정확성 및 고속화
