# ACTIVE_TASK

## TASK-082: Domain Settings UI (in progress)

### 배경

TASK-081에서 역할 관리 UI를 회사 범위 훅 기반으로 정리했다. TASK-082는 도메인 설정 화면을 같은 패턴으로 정돈해서, 회사 내 도메인 목록 선택과 도메인별 보안/암호 정책 저장 흐름을 더 일관되게 만든다.

### 구현 대상

Frontend:
- `apps/console/src/app/companies/[id]/tenancy/domain-settings/page.tsx`
  - domain list selection
  - security/password/quota settings
  - save flow through query/mutation hooks
- `apps/console/src/hooks/useDomainSettings.ts`
- `apps/console/src/hooks/useDomains.ts`

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] Domain settings can be loaded and saved through the console
- [ ] Security/password/quota controls remain clearly labeled
- [ ] `docs/CURRENT_STATUS.md` 갱신
- [ ] `docs/backend-roadmap.md` 해당 항목 체크/갱신

### 다음 태스크

TASK-083: API Settings UI
