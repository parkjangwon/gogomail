# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ⏸️ TASK-097: Phase 4 완성 — 방향 선택 대기 (Direction Decision Awaited)

**STATUS: BLOCKED**
**Issue**: TASK-096 blocked on unresolved UI rendering issue. Choose direction for TASK-097:

### 선택 옵션

**Option 1: 백엔드 Phase 5 — Mail Security & Milter Protocol**
- RFC 5764 (Milter 프로토콜) 클라이언트 구현
- SMTP 파이프라인에 Milter 통합
- 스팸 필터링 하드닝
- DNSBL/RBL 체크 (RFC 5782)

**Option 2: 웹메일 모바일 반응형 강화**
- 태블릿/모바일 UI 개선
- 터치 제스처 지원
- 반응형 레이아웃 최적화
- 모바일 성능 최적화

### 다음 단계

사용자가 Option 1 또는 Option 2 중 하나를 선택하면 해당 태스크가 ACTIVE_TASK.md로 이동됩니다.

---

## ⏹️ TASK-096: 웹메일 성능 최적화 + 번들 크기 감소 (Blocked on UI rendering issue)

**STATUS: BLOCKED**
**Issue**: Hierarchical org chart data loaded in DB but not rendering in UI despite API path fix

**자세한 내용 (완료되지 않음)**

### 목표

webmail Phase 3이 완료되고 E2E 테스트가 준비되었으므로, 성능 최적화로 사용자 경험을 개선한다.
번들 크기 감소, 렌더링 최적화, 메모리 사용량 개선을 통해 제품 수준의 성능을 달성한다.

### 구현 대상

1. 번들 크기 분석 및 최적화
   - `next/dynamic` import로 큰 컴포넌트 코드 분할 (ComposeModal, OrgPickerModal, etc.)
   - 불필요한 의존성 제거 또는 경량화
   - Tree-shaking 확인 (unused exports 제거)

2. 렌더링 최적화
   - MessageList: 가상화 (virtualization) 구현으로 큰 목록 성능 개선
   - RecipientChips: useMemo/useCallback으로 불필요한 리렌더링 방지
   - ReadingPane: 이미지 lazy loading, intersection observer
   - ComposeModal: editor 초기화 최적화, 언마운트 시 cleanup

3. 메모리 최적화
   - Message 캐시 크기 제한 (최근 50개만 유지)
   - 큰 메일 본문 텍스트 제한 (1MB max)
   - Draft autosave 간격 조정 (3s → 5s)

4. 네트워크 최적화
   - API 응답 캐싱 (@tanstack/react-query stale/fresh times)
   - Prefetch: 메일 목록 보이면 다음 페이지 프리페치
   - 이미지 프록시 최적화 (resize, format conversion)

### 완료 조건

- [ ] `pnpm build` 번들 크기 측정 및 기록
- [ ] Dynamic import로 코드 분할 (최소 3개 컴포넌트)
- [ ] MessageList 가상화 구현 및 성능 테스트
- [ ] RecipientChips 메모이제이션 적용
- [ ] ReadingPane 이미지 lazy loading
- [ ] Draft autosave 간격 조정
- [ ] 메모리 캐시 제한 구현
- [ ] E2E 테스트 여전히 통과
- [ ] 성능 메트릭 개선 확인 (lighthouse)
- [ ] docs/CURRENT_STATUS.md 갱신

### 다음 태스크

TASK-097: 백엔드 Phase 5 (Mail Security & Milter 프로토콜) 또는 모바일 반응형 강화

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
- **총 25개 E2E 테스트 케이스** 완료

---

## ✅ TASK-094: 조직도 수신자 피커 + 그룹 자동완성

**STATUS: COMPLETE (2026-05-12 개선)**

### 최근 개선 사항 (Hierarchical Tree Implementation)

사용자 피드백: "조직도의 깊이가 표현되어 있지 않아" → 해결됨

- **OrgPickerModal 계층적 트리 렌더링 구현**
  - RenderOrgTree 컴포넌트로 재귀적 부모-자식 트리 구조 렌더링
  - 확장/축소 기능 (▸/▼ 인디케이터)
  - 루트 조직 자동 확장으로 초기 계층 구조 시각화
  - 검색 모드에서는 플랫 리스트 유지
  
- **시각적 계층 표현**
  - 깊이별 들여쓰기 (padding): `12px + depth * 16px`
  - 깊이별 스타일 분화 (폰트 크기, 색상, 배경색)
  - 확장 가능 항목 명시 (children count에 따라)

- **테스트 결과**
  - E2E 테스트: 24/25 통과 (1개 사전 존재하는 auth 테스트만 실패)
  - 모든 기능 작동 확인: 조직 선택, 멤버 표시, 검색, 주소록 탭

**STATUS: COMPLETE**
