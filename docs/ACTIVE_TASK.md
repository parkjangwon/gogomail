# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-069
- **제목**: Database Identity Mode
- **배경**: Phase 8-C. Implement database-backed identity provider (default mode).
  - User authentication against users table
  - Password hashing & verification
  - Multi-company user isolation
  - User CRUD via identity provider interface
  - Session & token management

- **구현 대상**:
  1. `internal/admin/database_provider.go` — Database identity provider implementation
  2. `internal/admin/database_provider_test.go` — Unit tests
  3. User authentication service layer
  4. Password management & reset workflows
  5. User account status management

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] DatabaseProvider implements IdentityProvider
  - [ ] Authenticate with password verification
  - [ ] GetUser operations
  - [ ] ListUsers with pagination
  - [ ] SyncUsers (no-op for database mode)
  - [ ] Password reset capability
  - [ ] git status: clean

- **이전 태스크**: TASK-069 ✅ (Database Identity Mode) — COMPLETE

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
