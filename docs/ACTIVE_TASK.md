# ACTIVE_TASK

## TASK-087: Admin Console Frontend (Phase 3)

### 배경

Phase 2에서 조직/도메인/감사로그/API/알림 설정 페이지가 완료됨.
Phase 3는 운영 관리, 모니터링, 규정 준수 관리 등 추가 고급 기능 페이지 구현.

### 구현 대상

Frontend (`apps/console`):
- `src/app/companies/[id]/compliance/page.tsx` - 규정 준수 현황 대시보드
- `src/app/companies/[id]/compliance/legal-holds/page.tsx` - 법적 보류 관리
- `src/app/companies/[id]/delivery/relays/page.tsx` - SMTP 릴레이/스마트호스트 관리
- `src/app/companies/[id]/delivery/routes/page.tsx` - 배송 라우팅 규칙 관리
- `src/app/companies/[id]/monitoring/page.tsx` - 시스템 모니터링 대시보드
- `src/app/companies/[id]/system/backpressure/page.tsx` - 백프레셔 상태 조회
- `src/app/companies/[id]/system/health/page.tsx` - 시스템 상태 점검
- `src/app/companies/[id]/system/queue/page.tsx` - 큐 통계 및 재시도 관리
- `src/app/companies/[id]/admin-activity/page.tsx` - 관리자 활동 로그
- E2E 테스트 추가 (admin console 전체 workflow)

### 완료 조건

- [x] `pnpm -C apps/console type-check` 통과
- [x] 규정 준수 관리 페이지 동작 확인
- [x] 배송 라우팅 페이지 동작 확인
- [x] 모니터링 및 시스템 상태 페이지 동작 확인
- [x] 관리자 활동 로그 조회
- [x] E2E 테스트 작성 및 통과
- [x] docs/CURRENT_STATUS.md 갱신
- [x] docs/backend-roadmap.md 해당 항목 체크

### 검증

- `pnpm -C apps/console type-check`
- `pnpm -C apps/console build`
- `pnpm -C apps/console test` (E2E tests)

### 다음 태스크

TASK-088: Admin Console Frontend (Phase 4) / Mail Infrastructure Hardening
