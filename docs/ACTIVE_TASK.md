# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ TASK-095: 웹메일 E2E 테스트 및 통합 테스트 커버리지

**STATUS: COMPLETE**

### 완료 (2026-05-12)

- `playwright.config.ts`: Chromium 브라우저, baseURL=http://localhost:3003, HTML 리포트
- `package.json`: "test:e2e", "test:e2e:ui" npm 스크립트 추가
- `e2e/auth.spec.ts`: 로그인, 리다이렉트 흐름 (3 tests)
- `e2e/mail-list.spec.ts`: 메일 목록, 네비게이션, 사이드바 (3 tests)
- `e2e/compose.spec.ts`: 모달, 수신자 입력, 제목 입력 (3 tests)
- `e2e/search.spec.ts`: 검색 필드 입력, 초기화 (3 tests)
- `e2e/message-view.spec.ts`: 메시지 클릭, 읽기 창, 폴더 (3 tests)
- `e2e/responsive.spec.ts`: 데스크톱/태블릿/모바일, 리사이즈 (4 tests)
- `e2e/features.spec.ts`: 캘린더, 조직도, 드라이브, 설정 (6 tests)
- `e2e/README.md`: 실행, 작성, CI, 문제 해결 가이드
- **총 25개 E2E 테스트 케이스** 완료 (`pnpm test:e2e --list` 확인)

### 다음 태스크

TASK-096: Webmail 성능 최적화 + 번들 크기 감소 (또는) 백엔드 Phase 5 (Mail Security & Milter)

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
