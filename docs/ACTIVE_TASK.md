# ACTIVE_TASK

## TASK-256: CardDAV vCard payload validation audit

### 배경

CardDAV contact object 저장 경계는 vCard 구조, UID, FN, 크기를 검증하지만
LF-only 또는 혼합 줄바꿈 본문을 CRLF 기반 vCard처럼 받아들일 수 있다. 저장되는
주소록 객체가 RFC식 CRLF content line 계약을 따르도록 PUT/Repository 공통
검증을 강화한다.

### 구현 대상

- `internal/carddavgw/metadata.go`
- `internal/carddavgw/metadata_test.go`
- `internal/carddavgw/repository_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CardDAV vCard 저장 검증이 LF-only 및 혼합 줄바꿈을 거절한다.
- [x] metadata/repository 회귀 테스트가 CRLF 라인 엔딩 정책을 커버한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-257: CardDAV address-data UID projection audit
