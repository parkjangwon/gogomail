# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-063
- **제목**: Admin Console Schema + RBAC + Custom Roles
- **배경**: Phase 8-A 첫 번째 태스크. Admin Console의 기초 데이터 모델과 권한 시스템 구현.
  - 관리자 역할 정의 (builtin + custom)
  - 역할별 권한 매트릭스
  - 사용자-역할 할당
  - 감시 정책 설정 (Audit Level, Retention, Masking)

- **구현 대상**:
  1. `internal/admin/models.go` — Admin role, permission, audit policy types
  2. `migrations/00XX_admin_console_schema.sql` — DB schema (9개 테이블)
  3. `internal/admin/repository.go` — Admin role CRUD, permission queries
  4. `internal/admin/service.go` — Service layer (validation, business logic)
  5. `internal/admin/service_test.go` — Unit tests
  6. `docs/ADMIN_CONSOLE_SPEC.md` — ✅ 완성된 스펙 문서

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] Schema validation (migration 문법 확인)
  - [ ] Admin role CRUD 동작 확인
  - [ ] Custom role 생성/수정/삭제 동작 확인
  - [ ] Permission matrix 쿼리 동작 확인
  - [ ] Audit policy 설정 동작 확인
  - [ ] docs/ADMIN_CONSOLE_SPEC.md 반영 완료
  - [ ] git status: clean (docs/ADMIN_CONSOLE_SPEC.md, migrations, internal/admin 포함)

- **다음 태스크**: TASK-064 (Admin Auth & Session)

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
