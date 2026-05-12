# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 🔄 TASK-095: 웹메일 E2E 테스트 및 통합 테스트 커버리지

**STATUS: IN_PROGRESS**

### 목표

webmail UI가 Phase 3 완료되었으므로, Playwright E2E 테스트를 추가해 사용자 워크플로우를 자동으로 검증한다.
주요 사용 시나리오(메일 조회, 검색, 작성, 회신, 첨부, 캘린더, 조직도)에 대한 통합 테스트를 구성한다.

### 구현 대상

1. `apps/webmail/e2e/` — Playwright 테스트 스위트
   - Auth flow: login, logout, session persistence
   - Mail list: pagination, sorting, filtering (unread, starred, folder)
   - Message view: open, star, read/unread, reply/forward
   - Compose: to/cc/bcc input, org picker, file attach, send, draft save
   - Search: keyword search, filter operators (from:, to:, subject:, has:attachment)
   - Calendar: create event from ICS attachment, subscribe to calendar
   - Directory: search org, view contact details
   - Drive: browse, attach to compose
   - Settings: preferences, profile, password
2. `playwright.config.ts` — E2E 환경 설정 (baseURL, timeout, retries)
3. `package.json` — `"test:e2e": "playwright test"` script

### 완료 조건

- [ ] Playwright 설정 완료 (baseURL=http://localhost:3002)
- [ ] `pnpm test:e2e` 실행 가능
- [ ] Auth 테스트 (로그인, 세션, 로그아웃)
- [ ] Mail list 기본 시나리오 (load, filter, sort)
- [ ] Compose → Send 전체 워크플로우
- [ ] Search 필터 검증
- [ ] Org picker 통합 테스트
- [ ] 최소 10개 E2E 케이스 커버
- [ ] 모든 E2E 테스트 통과
- [ ] docs/CURRENT_STATUS.md 갱신

### 다음 태스크

TASK-096: Webmail 성능 최적화 및 번들 크기 감소 (또는) 백엔드 Phase 5 (Mail Security & Milter)

---

## ✅ TASK-094: 조직도 수신자 피커 + 그룹 자동완성

**STATUS: COMPLETE**

### 완료 (2026-05-12)

- `internal/httpapi/carddav.go`: `/api/v1/contacts/autocomplete` 그룹/조직 검색 추가 (`PrincipalKindGroup`, `PrincipalKindOrganization`) — `type: "group"` 배지 반환
- `apps/webmail/src/lib/api.ts`: `ContactSuggestion.type` 필드 추가
- `apps/webmail/src/components/RecipientChips.tsx`: 드롭다운에 "그룹" 배지 표시
- `apps/webmail/src/components/ComposeModal.tsx`: 조직도 피커 추가 — To/Cc/Bcc 각 필드에 UsersIcon 버튼, 검색+멀티셀렉트+아바타 패널, 선택된 사용자 수신자 필드에 추가

---

## ✅ TASK-090: 수신 차단 + 부재중 자동 응답 백엔드 강제 적용

**STATUS: COMPLETE**
