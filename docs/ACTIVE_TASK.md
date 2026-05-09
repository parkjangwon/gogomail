# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-064
- **제목**: Admin Auth & Session — JWT, login, refresh
- **배경**: Phase 8-B. Admin console의 인증 및 세션 관리.
  - JWT 토큰 발급 및 검증
  - Admin login endpoint (email + password)
  - Token refresh mechanism
  - Session timeout & revocation

- **구현 대상**:
  1. `internal/admin/auth.go` — JWT utility functions, token generation/validation
  2. `internal/admin/auth_test.go` — Unit tests for JWT operations
  3. `internal/admin/session.go` — Session management, timeout handling
  4. API routes for admin login/logout/refresh
  5. Middleware for JWT validation

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] JWT token generation with expiry
  - [ ] Token refresh endpoint 동작 확인
  - [ ] Admin login with email/password 동작 확인
  - [ ] Session timeout & revocation 동작 확인
  - [ ] JWT validation middleware 동작 확인
  - [ ] git status: clean

- **이전 태스크**: TASK-063 ✅ (Admin Console Schema + RBAC) — COMPLETE

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
