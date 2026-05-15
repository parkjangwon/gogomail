# ACTIVE_TASK

## TASK-081: Role Management UI (in progress)

### 배경

TASK-080 정리 이후 역할 관리 화면을 회사 범위와 백엔드 역할 목록 계약에 더 가깝게 맞춘다. 현재 페이지는 단순 목록/생성 중심이므로, builtin/custom 역할을 구분해서 보여주고 회사 범위 생성 흐름을 훅 기반으로 일관되게 유지한다.

### 구현 대상

Frontend:
- `apps/console/src/app/companies/[id]/roles/page.tsx`
  - builtin/custom 역할 구분 표시
  - 회사 범위 목록/생성 흐름 유지
- `apps/console/src/hooks/useRoles.ts`
  - company-scoped query/mutation 정리
- `apps/console/src/messages/*`
  - role management labels

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] Role list/create가 회사 범위로 동작
- [ ] Builtin 역할과 custom 역할이 구분되어 보임
- [ ] `docs/CURRENT_STATUS.md` 갱신
- [ ] `docs/backend-roadmap.md` 해당 항목 체크/갱신

### 다음 태스크

TASK-082: Domain Settings UI
