# ACTIVE_TASK

## TASK-086: Admin Console Frontend (Phase 2)

### 배경

Phase 8 프론트엔드 Phase 1 (TASK-085)에서 로그인, 대시보드, 사용자 목록이 구현됨.
Phase 2는 조직 관리, 도메인 설정, 감사 로그, API 설정, 알림 설정 등 운영 관리 페이지 구현.

### 구현 대상

Frontend (`apps/console`):
- `src/app/companies/[id]/organization/page.tsx` - 조직 구조 관리, 멤버 할당/제거
- `src/app/companies/[id]/tenancy/domains/page.tsx` - 도메인 목록 및 설정
- `src/app/companies/[id]/audit-logs/page.tsx` - 감사 로그 검색 및 필터
- `src/app/companies/[id]/security/api-settings/page.tsx` - API 키, Rate Limit, CIDR Allowlist
- `src/app/companies/[id]/alerts/page.tsx` - Alerts & Notifications 설정 (TASK-084 backend)
- 각 페이지별 `useX.ts` hooks 추가/갱신

### 완료 조건

- [x] `pnpm -C apps/console type-check` 통과
- [x] 조직 관리 페이지 동작 확인
- [x] 도메인 관리 페이지 동작 확인
- [x] 감사 로그 조회 및 필터 동작
- [x] API 설정 페이지 동작 (API 키 관리, Rate Limit)
- [x] 알림 설정 페이지 동작 (threshold 설정, channels 관리)
- [x] docs/CURRENT_STATUS.md 갱신
- [x] docs/backend-roadmap.md 해당 항목 체크

### 검증

- `pnpm -C apps/console type-check`
- `pnpm -C apps/console build`

### 다음 태스크

TASK-087: Admin Console Frontend (Phase 3)
