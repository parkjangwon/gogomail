# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ PHASE 8 COMPLETE - Backend Foundation

**STATUS: COMPLETED** ✅

Backend Admin Console Phase 8 완료:
- TASK-063 ✅: Admin Console Schema + RBAC + Service Layer
- TASK-064 ✅: Admin Auth & Session (JWT)
- TASK-065 ✅: User Management CRUD
- TASK-066 ✅: Organization Management
- TASK-067 ✅: Audit Logs Level 1+2
- TASK-068 ✅: Identity Provider Abstraction
- TASK-069 ✅: Database Identity Mode
- TASK-070 ✅: LDAP Identity Config & Sync
- TASK-071 ✅: LDAP Sync UI & Logs
- TASK-072 ✅: External RDBMS Config & Sync
- TASK-073 ✅: External RDBMS Sync UI
- TASK-074 ✅: Mail Log Queries & UI
- TASK-075 ✅: Statistics & Dashboard Cache
- TASK-076 ✅: API Metering
- TASK-077 ✅: Audit Policy Config UI
- TASK-078 ✅: Export/Reports (CSV, PDF)

**240 unit tests passing** ✅

---

## 현재 단계

**Phase 9 - Frontend (Admin Console UI) - TASK-079**

### ✅ P0 COMPLETE: Project Scaffolding

- **기술 스택**: Next.js 15, Tailwind CSS v4, Cloudscape Design System
- **아키텍처**: Stateless BFF + httpOnly JWT + React Query
- **완료 항목**:
  - ✅ Next.js 프로젝트 구조 (src/app, src/components, src/hooks, src/lib)
  - ✅ BFF 인증 라우트 (/api/auth/login, logout, refresh)
  - ✅ BFF 범용 프록시 (/api/admin/[...path] → /admin/v1/*)
  - ✅ 미들웨어 (httpOnly 쿠키 인증 가드)
  - ✅ Root Layout + Providers (QueryClientProvider)
  - ✅ Console Layout (Cloudscape AppLayout + SideNav)
  - ✅ 로그인 페이지
  - ✅ 기본 대시보드 페이지
  - ✅ React Query 훅 (useUsers, useDomains, useAuditLogs)
  - ✅ API 클라이언트 유틸리티
  - ✅ 타입 정의 (admin.ts)
  - ✅ Tailwind CSS v4 + Cloudscape 스타일링
  - ✅ 프로젝트 빌드 성공

### ✅ P1 COMPLETE: Core Pages + Navigation

- ✅ Users CRUD 페이지 (DataTable with React Query)
- ✅ Domains 관리 페이지 (Verified status badge)
- ✅ Audit Logs 페이지 (Filterable, export-ready)
- ✅ Organizations 페이지 (stub)
- ✅ Roles 페이지 (stub)
- ✅ Identity Providers 페이지 (stub)
- ✅ Mail Logs 페이지 (stub)
- ✅ Statistics 페이지 (stub)
- ✅ Audit Policy 페이지 (stub)
- ✅ Reports 페이지 (stub)
- ✅ Full navigation working (SideNav routes all functional)
- ✅ Build passing with all pages

### ✅ P2 IN PROGRESS: Advanced Features (1/5 complete)

- ✅ Audit Policy Configuration (Level 1-3 radio, scope checkboxes, React Query)
- [ ] Audit Logs + 필터링 (date range, action filter, admin filter)
- [ ] Identity Provider 설정 (DB/LDAP/RDBMS 탭)
- [ ] 통계 & 차트 (real-time dashboard)
- [ ] CSV/PDF 리포트 다운로드

### P3 TODO: Polish & Production

- [ ] 역할 및 권한 관리
- [ ] 폼 검증 및 에러 처리
- [ ] 유닛 테스트 (Vitest)
- [ ] E2E 테스트 (Playwright)

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
