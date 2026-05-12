# ACTIVE_TASK

## TASK-212: CardDAV/CalDAV collection xml:lang empty-value clearing audit

### 배경

TASK-211에서 PROPPATCH의 absent `xml:lang`은 기존 language tag를 보존하도록
수정되었다. 이제 explicit empty `xml:lang=""`가 language tag를 의도적으로
clear하는 경로를 CalDAV/CardDAV handler와 PostgreSQL repository integration에서
검증한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV handler test가 explicit `xml:lang=""` displayname/description update에서 기존 language tags를 clear한다.
- [x] CardDAV handler test가 explicit `xml:lang=""` displayname/description update에서 기존 language tags를 clear한다.
- [x] CalDAV PostgreSQL integration test가 explicit empty language update를 raw language columns까지 clear한다.
- [x] CardDAV PostgreSQL integration test가 explicit empty language update를 raw language columns까지 clear한다.
- [x] CalDAV/CardDAV parser와 repository validation이 explicit empty language pointer를 보존한다.
- [x] empty stored language response가 `xml:lang` attribute를 생략한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-213: CardDAV/CalDAV collection xml:lang unsupported-property rollback audit
