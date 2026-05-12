# ACTIVE_TASK

## ✅ TASK-184: CardDAV addressbook-query metadata/search index 고도화

### 배경

CardDAV `addressbook-query`는 RFC 6352 필터 파싱과 Go 기반 vCard 정밀 매칭을 수행하지만,
실행 경로는 주소록의 모든 활성 객체를 순회하며 각 vCard 본문을 파싱했다.
기존 `lower(vcard::text)` trigram index가 있었지만 REPORT 경로에서 활용되지 않았으므로,
안전한 positive ASCII `text-match`를 후보 검색 조건으로 사용하고 최종 판정은 기존 RFC 매칭 함수에 맡긴다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `internal/carddavgw/repository_discovery.go`
- `internal/carddavgw/repository_discovery_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] `addressbook-query`가 안전한 `text-match` 필터에서 indexed candidate walker를 우선 사용한다.
- [x] repository candidate walker가 `user_id/addressbook_id/status` scope와 `lower(vcard::text) LIKE`를 함께 적용한다.
- [x] 후보 결과는 기존 `contactObjectMatchesFilter`로 재검증해 property-specific, param-filter, collation 결과 정합성을 보존한다.
- [x] negated/non-ASCII/불안전 filter shape는 기존 broad walker로 fallback한다.
- [x] handler 회귀 테스트가 fast path 사용, false positive 제거, fallback 동작을 검증한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-185: CalDAV calendar-query time-range 후보 인덱스 고도화
