# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-061
- **제목**: 조직도 백엔드 완성 — 데이터베이스 스키마 + 서비스 레이어 + HTTP API
- **배경**: Phase 8-A. 사용자/관리자 콘솔에서 조직 구조(부서, 팀, 계층)를 관리하고,
  LDAP와 동기화하는 기능. 

- **구현 완료**:
  1. ✓ `migrations/0079_organization_structure.sql` — 스키마 (3개 테이블 + 인덱스)
  2. ✓ `internal/orgchart/models.go` — OrganizationUnit, OrganizationMember, SyncLog, OrganizationHierarchy
  3. ✓ `internal/orgchart/repository.go` — CRUD + sync 로깅 (8개 메서드)
  4. ✓ `internal/orgchart/service.go` — 비즈니스 로직 + LDAP 동기화 오케스트레이션 + 인터페이스 기반 설계
  5. ✓ `internal/orgchart/repository_test.go` — repository 테스트 placeholder
  6. ✓ `internal/orgchart/service_test.go` — service 테스트 (12개 테스트 통과)
  7. ✓ `internal/httpapi/orgchart.go` — HTTP API 엔드포인트 (9개 엔드포인트)
  8. ✓ `internal/httpapi/orgchart_test.go` — HTTP API 테스트 (8개 테스트 통과)

- **완료 확인**:
  - [x] `go test ./...` 통과: 5469 tests passed
  - [x] orgchart: 12 service tests ✓
  - [x] httpapi: 8 orgchart endpoint tests ✓
  - [x] HTTP API 9개 엔드포인트:
     - GET /admin/v1/organization/units (company_id 쿼리)
     - GET /admin/v1/organization/units/{id}
     - POST /admin/v1/organization/units
     - PUT /admin/v1/organization/units/{id}
     - DELETE /admin/v1/organization/units/{id}
     - GET /admin/v1/organization/hierarchy (company_id 쿼리)
     - POST /admin/v1/organization/members (unitID, userID, role)
     - DELETE /admin/v1/organization/members/{id}
     - POST /admin/v1/organization/sync (company_id 쿼리)
  - [x] Service layer with RepositoryInterface abstraction for testability
  - [x] X-Admin-Token authentication on all endpoints

- **다음 태스크**: TASK-062 (Spam filter hardening via Milter)

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기 ✓
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
