# ACTIVE_TASK

## TASK-083: API Settings UI (in progress)

### 배경

TASK-082에서 도메인 설정 화면을 훅 기반으로 정돈했다. TASK-083은 API key / rate limit / CIDR allowlist를 다루는 API Settings 화면을 같은 방식으로 노출해서, 보안 섹션 내에서 도메인별 API 제어를 바로 조정할 수 있게 한다.

### 구현 대상

Frontend:
- `apps/console/src/app/companies/[id]/security/api-settings/page.tsx`
  - domain selector
  - rate limit / allowlist / API key requirement controls
- `apps/console/src/hooks/useAPISettings.ts`
- `apps/console/src/hooks/useDomains.ts`
- `apps/console/src/components/Sidebar.tsx` if a navigation link is needed

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] API settings can be loaded and saved from the console
- [ ] Rate limit and allowlist controls are clearly labeled
- [ ] `docs/CURRENT_STATUS.md` 갱신
- [ ] `docs/backend-roadmap.md` 해당 항목 체크/갱신

### 다음 태스크

TASK-084: Alerts & Notifications
