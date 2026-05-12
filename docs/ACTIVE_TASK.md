# ACTIVE_TASK

## ✅ TASK-180: CardDAV addressbook-multiget 배치 조회 고도화

### 배경

CardDAV `addressbook-multiget`은 요청 href를 순회하며 contact object마다 `LookupContactObject`를 호출했다.
클라이언트가 주소록 초기 로딩이나 캐시 재검증에서 많은 `.vcf` href를 보내면 요청 크기에 비례해 DB 왕복이 증가한다.

이번 라운드는 요청 href를 검증한 뒤 주소록+객체명 그룹으로 dedupe하고,
repository가 `VALUES` 기반 배치 조회로 active contact object를 한 번에 가져오도록 정리했다.
응답은 기존 RFC 6352/WebDAV multistatus 동작처럼 원 요청 순서와 중복 href, per-href 404를 보존한다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/repository.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `addressbook-multiget` valid href들을 주소록+객체명 기준으로 dedupe해 배치 조회한다.
- [x] 배치 store가 있을 때 per-href `LookupContactObject` 반복 호출을 제거한다.
- [x] 응답 순서, duplicate href 응답, missing href 404 multistatus를 유지한다.
- [x] repository 배치 조회는 안전한 청크 크기로 동적 SQL 파라미터 폭주를 방지한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-181: CardDAV write/delete lock contention 및 sync marker 단일 쿼리 고도화
