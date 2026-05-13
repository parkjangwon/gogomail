# ACTIVE_TASK

## TASK-259: CardDAV addressbook-query candidate optimization audit

### 배경

CardDAV `addressbook-query`는 안전한 positive text-match에서만 후보 walker를
사용해 전체 주소록 스캔을 줄인다. 후보 텍스트가 `%`, `_`, `\` 같은 SQL LIKE
메타문자를 포함하면 구현별 이스케이프 차이로 최적화 경계가 과하게 넓어질 수
있으므로, 이 경우에는 broad walker로 폴백한다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 후보 텍스트에 LIKE 메타문자가 있으면 candidate walker 최적화를 사용하지 않는다.
- [x] handler 회귀 테스트가 wildcard 후보 텍스트의 broad walker 폴백을 커버한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-260: CardDAV sync-token retention audit
