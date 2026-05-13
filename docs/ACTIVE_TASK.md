# ACTIVE_TASK

## TASK-254: CardDAV company suspension policy audit

### 배경

CardDAV Basic Auth는 공통 submission 인증기를 재사용한다. 이 인증 경로가 사용자와
도메인 상태만 확인하고 회사 상태를 확인하지 않으면, 회사가 중지된 뒤에도 CardDAV
및 동일 인증 경로를 쓰는 프로토콜 인증이 열릴 수 있다. 회사 상태를 공통 인증
쿼리에서 닫아 테넌트 정지 정책을 일관되게 적용한다.

### 구현 대상

- `internal/maildb/submission.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] submission 인증이 회사 상태를 조인하고 `active` 회사만 허용한다.
- [x] PostgreSQL 회귀 테스트가 중지된 회사의 인증 거절을 커버한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-255: CalDAV/CardDAV password-change policy audit
