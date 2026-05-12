# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 🔄 TASK-090: 수신 차단 + 부재중 자동 응답 백엔드 강제 적용

**STATUS: COMPLETE**

### 목표

웹메일 프론트엔드에서 설정할 수 있는 수신 차단(blocked senders)과 부재중 자동 응답(vacation auto-reply)을 백엔드 mail.stored 이벤트 핸들러로 실제 강제 적용한다.

### 구현 대상

1. `internal/inboundfilter/handler.go` — mail.stored 이벤트 핸들러
   - 수신 차단: envelope_from이 blocked_senders에 있으면 trash 폴더로 이동
   - 부재중 자동 응답: vacation.enabled + 날짜 범위 확인 → SendText로 자동 응답
   - 자동 응답 루프 방지: mailer-daemon, noreply, postmaster 필터링
   - 인메모리 rate limiter: 사용자당 발신자별 7일 1회
2. `internal/inboundfilter/handler_test.go` — 단위 테스트
3. `internal/app/run.go` — mail.stored 핸들러에 등록

### 완료 조건

- [ ] `go test ./internal/inboundfilter/...` 통과
- [ ] `go test ./...` 통과
- [ ] blocked sender → trash 이동 테스트
- [ ] vacation 활성 + 날짜 범위 내 → 자동 응답 전송 테스트
- [ ] vacation 비활성 → 자동 응답 없음 테스트
- [ ] 루프 방지 (mailer-daemon) 테스트

---

## ✅ TASK-089: 웹메일 클라이언트 UI (Webmail Client Frontend — Phase 3: Power Features)

**STATUS: COMPLETE**

### 구현 완료 (2026-05-12) — Phase 3 진행 중

**Phase 0–2 완료 항목:**
- 3-pane 레이아웃, 디자인 토큰, 다크/라이트 모드, i18n (ko/en/ja/zh-CN)
- API 통합 (TASK-090): 실제 메일 로드, HTML 렌더링, DOMPurify 새니타이징
- 회신/전달 (TASK-091): 제목/수신자 프리필, 인라인 회신 UI
- 검색 (TASK-092): 키워드 검색, 상세 검색 (from/to/subject/date/attachment)
- 사이드바: 폴더 목록, 읽지 않음 뱃지, 사용자 메뉴, 프로필 사진
- 설정 모달: 서명 편집기, 테마, 알림, 보안(세션 취소), 단축키, 고급
- Drive 뷰: 파일 브라우저, 업로드/삭제/공유, 이메일 첨부 연동
- 조직도 뷰: People Directory, 검색, 상세 패널
- 캘린더 뷰: 월/주/일, ICS 파서, CalDAV 이벤트 생성

**Phase 3 완료 항목 (2026-05-12):**
- ✅ ReadingPane: 별표 토글 버튼 (toolbar)
- ✅ ReadingPane: 읽음/안읽음 토글 (more menu)
- ✅ ReadingPane: 연락처에 추가 버튼 (sender row)
- ✅ ReadingPane: ICS 캘린더 초대 감지 → "캘린더에 추가" 카드 UI
- ✅ AppIconBar: 메일 읽지않음 뱃지 (99+ 처리)
- ✅ ComposeModal: Drive 첨부 파일 피커 (폴더 탐색, 브레드크럼)
- ✅ ComposeModal: 이모지 피커 (6카테고리, 120개)
- ✅ ComposeModal: 클립보드 이미지 붙여넣기 → 첨부파일 변환
- ✅ SearchBar: 받는사람(to:) 연산자 + 고급 필터 입력 활성화
- ✅ 아카이브/스팸: 낙관적 UI + 실행취소 토스트
- ✅ 이메일 인쇄: 새 창 팝업, 서식 있는 HTML 출력

**Phase 3 완료 항목 (2026-05-12, 세션 2):**
- ✅ 별표받은/안읽음 필터 탭 (Sidebar VIRTUAL_STARRED + MessageList filterMode)
- ✅ 임시저장 자동저장 (ComposeModal triggerAutoSave debounce 3s)
- ✅ 폴더 이동 (ReadingPane more menu + MessageList bulk move)
- ✅ 메시지 선택 + 일괄 작업 (bulkSelected, bulk read/star/move/delete)
- ✅ 반응형 레이아웃 (useIsMobile, mobile slide panel, swipe-back gesture)
- ✅ 이메일 별칭 선택 (ComposeModal From: 드롭다운, listUserAddresses API)
- ✅ 서버 사이드 환경설정 (GET/PUT /api/v1/preferences, users.settings jsonb)
- ✅ 사용자 프로필/비밀번호 API (GET/PATCH /api/v1/me, POST /api/v1/me/password)

### 진행 로그 (2026-05-12)

- 조직도 뷰: `GET /api/v1/directory/users?q=&limit=` 엔드포인트 추가 (`internal/httpapi/carddav.go`)
- 조직도 뷰: `OrgChartView.tsx` — People Directory (검색, 사용자 카드 그리드, 상세 패널, 메일 쓰기)
- 캘린더 뷰: `CalendarView.tsx` — 월/주/일 뷰, 이벤트 표시, ICS 파서 (CalendarAgent 완성)
- `api.ts`: `listDirectoryUsers()`, `Calendar/CalendarObject` 인터페이스, `listCalendars/listCalendarObjects()` 추가
- `page.tsx`: OrgChartPlaceholder → OrgChartView, CalendarPlaceholder → CalendarView 교체

### 진행 로그 (2026-05-11)

- TASK-090: API 통합 완료 — field name 불일치 전면 수정 (from_addr, read, received_at 등)
- TASK-090: HTML 이메일 렌더링 — EML 파서 HTML body 추출, DOMPurify 새니타이징
- TASK-090: dev 모드 bypass — GOGOMAIL_DEV_USER_ID로 JWT 없이 테스트 가능

### 핫픽스 적용 (2026-05-11)
- `internal/maildb/admin.go`: `NULLIF($N, 0)` → `NULLIF($N::bigint, 0)` — pgx int4 타입 추론 오버플로우 수정 (도메인/사용자/회사 quota 5곳)
- `apps/console` 도메인 모달 React key prop 경고 수정
- 웹메일 next-intl 인프라 추가 (ko/en/ja/zh-CN)
- `internal/maildb/mail_flow_logs.go`: `references` PostgreSQL 예약어 → `"references"` 쿼터 수정 (3곳)
- `apps/console` admin console CRUD 오류 일괄 수정:
  - access/* 페이지: company_id 파라미터 누락, 응답 키 오류, 필드명 대소문자 불일치 수정
  - analytics/* 페이지: 잘못된 API 엔드포인트 URL 수정
  - config/company: useParams + 올바른 URL 패턴 적용
  - config/domain, config/user: 도메인/사용자 선택 후 설정 조회 방식으로 재작성
  - security/api-keys: 도메인 선택 후 키 조회, 생성, 삭제 기능 구현

### 배경

Admin Console (TASK-088)이 완성되었고, 이제 최종 사용자가 사용할 메인 웹메일 클라이언트를 개발한다.
설계 철학: **Notion Mail 스타일의 깔끔하고 미니멀한, 콘텐츠 중심 UI**
참조: `docs/DESIGN.md` (디자인 언어, 컬러 토큰, 컴포넌트 패턴, i18n 등)

### 구현 대상

**앱 구조:**
```
apps/webmail/
  src/
    app/                          ← Next.js 15 App Router
      layout.tsx                  ← 루트 레이아웃, 테마 프로바이더
      page.tsx                    ← 리다이렉트 to /mail
      auth/
        login/page.tsx            ← 로그인 페이지
      mail/
        layout.tsx                ← 3-pane 레이아웃
        page.tsx                  ← 수신함 (inbox)
        [messageId]/page.tsx       ← 메일 읽기/상세
    components/
      layout/
        Shell.tsx                 ← 3-pane 컨테이너
        Sidebar.tsx               ← 좌측 사이드바
        MessageList.tsx           ← 메시지 목록
        ReadingPane.tsx           ← 읽기 화면
      compose/
        ComposeWindow.tsx          ← 플로팅 작성 모달
      common/
        ThemeToggle.tsx            ← 다크/라이트 모드 토글
        LocaleSelector.tsx         ← 언어 선택
    styles/
      design-tokens.css            ← 컬러/타이포그래피/스페이싱
    hooks/
      useMailList.ts              ← 메일 목록 조회
      useMessage.ts               ← 단일 메일 상세
      useTheme.ts                 ← 테마 관리
      useLocale.ts                ← i18n 로캘 관리
    lib/
      api.ts                      ← API 프록시 (Mail API → backend)
      auth.ts                     ← 인증 토큰 관리
      sanitize.ts                 ← HTML 새니타이징 (메일 본문)
    messages/
      ko.json                     ← 한국어 번역
      en.json                     ← 영어 번역
      ja.json                     ← 일본어 번역
      zh-CN.json                  ← 중국어(간체) 번역
```

**Phase 0 범위:**
1. 프로젝트 셋업 (Next.js 15, TypeScript, Tailwind v4, shadcn/ui 통합)
2. 디자인 토큰 구현 (색상, 타이포그래피, 스페이싱 CSS 변수)
3. 루트 레이아웃 + 테마/다크모드 시스템
4. 3-pane 쉘 레이아웃 (sidebar + message list + reading pane)
5. 사이드바 UI (네비게이션, 계정 선택기, 검색)
6. 메일 목록 화면 (스켈레톤, 상태 핸들링)
7. 메일 읽기 화면 (기본 구조, HTML 새니타이징 준비)
8. 로그인 페이지 (스타일링만, 통합은 TASK-090)
9. i18n 셋업 (next-intl, 4 로캘, 필수 키만)
10. 기본 접근성 (포커스, ARIA, 키보드 네비게이션)

**NOT in Phase 0:**
- 실제 메일 로드/렌더링 (API 통합은 TASK-090)
- 작성/회신/전달 기능 (TASK-091)
- 라벨/필터/검색 (TASK-092)
- 모바일 반응형 (기본 구조만)
- 고급 애니메이션 (기본 전환만)
- 캘린더/연락처/드라이브 (향후 Phase)

### 완료 조건

- [ ] `pnpm install` (apps/webmail) 완료, 의존성 정상 설치
- [ ] `pnpm build` (apps/webmail) 성공 (TS strict, no errors)
- [ ] `pnpm dev` (apps/webmail) 시작, http://localhost:3002 접근 가능
- [ ] 루트 레이아웃: 테마 토글 제공, 다크/라이트 모드 동작
- [ ] 3-pane 레이아웃: sidebar + list + pane 배치 정확, 반응형 768px 이상
- [ ] 사이드바: 계정 선택기, 네비게이션, 뱃지, 검색 입력 스타일링 완료
- [ ] 메일 목록: 5개 스켈레톤 행, 날짜 그룹 라벨 (오늘/어제/지난 7일)
- [ ] 메일 읽기: 제목, 발신자, 수신자, 액션 바, HTML 렌더 영역 레이아웃
- [ ] 로그인 페이지: 폼 스타일링 (제출은 미실시)
- [ ] i18n: `next-intl` 통합, ko/en 기본 키 번역 (common, mail, settings)
- [ ] 포커스 링: 모든 interactive 요소에 visible focus ring
- [ ] 스크린샷: 라이트 모드 + 다크 모드 각각 (Shell, MessageList, ReadingPane, Login)
- [ ] git add + commit (코드, 스크린샷, docs 전부) + push

### 루프 절차

```
1. 이 파일 읽기
2. pnpm create next-app@15 apps/webmail
3. Tailwind v4 + shadcn/ui 통합
4. design-tokens.css 구현 (color, typography, spacing)
5. Root layout: theme provider, toggle UI
6. Shell layout: sidebar + list + pane 3-pane
7. Sidebar: nav structure, account picker, badges
8. MessageList: skeleton rows, grouping labels
9. ReadingPane: header (actions) + metadata + body area
10. LoginPage: form styling
11. next-intl: config, messages/{locale}.json, provider setup
12. pnpm build 성공 & pnpm dev 실행 확인
13. 라이트/다크 모드 스크린샷 4개
14. git add + commit + push
15. NEXT_STEPS.md에서 다음 항목 ACTIVE_TASK.md로
```

### 우선순위

1. **높음 (Phase 0 필수)**
   - 3-pane 레이아웃 (Shell의 기초)
   - 디자인 토큰 (색상, 타이포그래피, 스페이싱)
   - 다크/라이트 모드 (first-class)
   - i18n (4 로캘 지원)

2. **중간 (Phase 0 완성도)**
   - 사이드바 (정보 밀도, 스타일)
   - 메일 목록 (행 높이, 계층화)
   - 메일 읽기 (레이아웃 뼈대)

3. **낮음 (Polish, Phase 1로 미루기)**
   - 애니메이션 (기본 전환만)
   - 반응형 (768px 이상만 지원)
   - 모바일 (TASK-093)

### 참고 자료

- `docs/DESIGN.md` — 전체 디자인 언어 (필독)
- `docs/backend-roadmap.md` — 전체 로드맵
- Memory: `user_design_preferences.md` — 사용자의 웹메일 UX 철학

### 다음 단계

TASK-090: 웹메일 클라이언트 API 통합 (메일 로드, 구성)

### 프론트엔드 게이트

**이 작업은 프론트엔드 개발을 트리거한다. Phase 0 구현 후 사용자에게 스크린샷과 진행 상황을 보여주되, TASK-090은 사용자 승인 없이 진행한다.**
