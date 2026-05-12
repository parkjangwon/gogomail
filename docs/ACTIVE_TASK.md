# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ TASK-098: 사용자 웹메일 베타 안정화 — API base-path 정합성 + 베타 시드 데이터

**STATUS: COMPLETE**

### 배경

목표는 사용자 웹메일 베타서비스다. 관리자 콘솔 이후 사용자 웹메일 프론트엔드가 진행 중이며, 프론트엔드와 백엔드 기능이 실제로 연결되어 동작해야 한다.

현재 가장 먼저 해소해야 할 베타 블로커는 웹메일 API base-path 드리프트다.

- 웹메일 브라우저 코드는 `/api/mail/...`을 호출한다.
- 백엔드 CardDAV/Directory 라우트는 `/api/mail/...`로 등록되어 있다.
- 기존 Mail API와 OpenAPI 문서에는 `/api/v1/...` 라우트가 여전히 존재한다.
- 따라서 프록시는 기능 영역별로 의도된 backend base path를 명확히 선택해야 한다.

### 구현 대상

- `apps/webmail/src/app/api/mail/[...path]/route.ts`
- `apps/webmail/src/components/OrgPickerModal.tsx`
- `scripts/seed_dev_beta.sh`
- `scripts/seed_dev_data.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `/api/mail/addressbooks`, `/api/mail/contacts`, `/api/mail/directory` 프록시가 backend `/api/mail/...`로 전달된다.
- [x] 기존 Mail API 요청은 backend `/api/v1/...` 전달을 유지한다.
- [x] 조직도 피커가 사용자 소속 조직을 기본 선택하고 부모 체인을 확장한다.
- [x] Docker PostgreSQL 컨테이너에 풍부한 어드민/웹메일 베타 데이터를 넣는 실행 스크립트가 있다.
- [x] 시드 데이터는 조직도, 주소록, 사용자, 메일 목록 테스트에 충분하다.
- [x] 디자인 토큰/레이아웃/시각 톤은 변경하지 않는다.
- [x] 표준 경로 드리프트를 문서화한다.
- [x] 관련 검증 통과 후 기능 단위 커밋.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-099: 사용자 웹메일 베타 안정화 — 핵심 메일 화면/작성 흐름 점검 및 보강

---

## ✅ TASK-099: 사용자 웹메일 베타 안정화 — 핵심 메일 화면/작성 흐름 점검

**STATUS: COMPLETE**

### 배경

TASK-098에서 웹메일 API base-path 정합성과 베타 시드 실행 경로를 안정화했다.
다음 베타 목표는 사용자 웹메일의 기본 사용 흐름이 실제 데이터로 자연스럽게 동작하도록 만드는 것이다.

디자인은 현재 상태를 유지한다. 시각 톤, 레이아웃, 디자인 토큰을 갑자기 바꾸지 않는다.

### 구현 대상

- `apps/webmail/src/app/mail/page.tsx`
- `apps/webmail/src/components/ComposeModal.tsx`
- `apps/webmail/src/components/MessageList.tsx`
- `apps/webmail/src/components/ReadingPane.tsx`
- 관련 API helper 또는 문서

### 완료 조건

- [x] 베타 seed 데이터 기준으로 로그인 후 메일 목록/읽기/작성 기본 흐름을 점검한다.
- [x] 프론트엔드-백엔드 API 계약 불일치가 있으면 표준/기존 계약을 우선해 수정한다.
- [x] 실제 폴더 선택 전 또는 가상 폴더 상태에서 잘못된 일반 메시지 목록 API 호출을 방지한다.
- [x] 디자인 톤은 유지한다.
- [x] 변경 내용과 검증 결과를 `docs/CURRENT_STATUS.md`에 기록한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-100: 사용자 웹메일 베타 안정화 — 작성/전송/초안 흐름 계약 점검

---

## ✅ TASK-100: 사용자 웹메일 베타 안정화 — 작성/전송/초안 흐름 계약 점검

**STATUS: COMPLETE**

### 배경

메일 작성은 사용자 웹메일 베타의 핵심 경로다. UI는 reply-all 같은 사용자 편의 동작을 제공하지만, 백엔드 compose 계약은 표준적으로 `new`, `reply`, `forward`만 허용한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `reply_all` UI 동작이 백엔드 계약에서 허용되는 `reply` intent로 정규화된다.
- [x] 초안 자동저장/수동저장/닫기 전 저장/전송 payload가 같은 intent 정규화 경로를 사용한다.
- [x] 초안 저장 수신자 필드는 전송과 같은 주소 파서로 처리한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-101: 사용자 웹메일 베타 안정화 — 첨부파일/드라이브 첨부 계약 점검

---

## ✅ TASK-101: 사용자 웹메일 베타 안정화 — 첨부파일/드라이브 첨부 계약 점검

**STATUS: COMPLETE**

### 배경

사용자 웹메일 베타에서 첨부파일은 작성/초안/전송 흐름의 핵심 기능이다.
백엔드 초안 계약은 `attachment_ids`를 지원하므로, 프론트엔드 초안 저장 경로도 전송 경로와 동일하게 준비된 첨부 ID를 보존해야 한다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 초안 저장 payload 타입이 `attachment_ids`를 지원한다.
- [x] 자동저장/수동저장/닫기 전 저장에서 업로드 완료된 첨부 ID가 초안에 포함된다.
- [x] 업로드 중이거나 실패한 첨부는 초안 `attachment_ids`에 포함하지 않는다.
- [x] 전송 경로의 기존 첨부 동작을 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-102: 사용자 웹메일 베타 안정화 — 첨부파일 업로드/전송 실패 상태 UX 점검

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
