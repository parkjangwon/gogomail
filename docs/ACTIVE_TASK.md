# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ TASK-088-I18N: Admin Console 다국화 (Internationalization)

**STATUS: FOUNDATION COMPLETE - READY FOR PAGE-BY-PAGE TRANSLATION**

### 배경

Admin Console (TASK-088)이 완성되었으나 모든 텍스트가 영어로만 되어 있다.
웹메일 클라이언트 개발 전에 admin console도 다국어 지원 (한글/영어/일본어/중국어)을 추가한다.
기존 Cloudscape 컴포넌트를 활용하되, 커스텀 UI도 다국화 처리한다.

### 구현 대상

**파일 구조:**
```
apps/admin/
  src/
    messages/
      ko.json          ← 한국어 번역
      en.json          ← 영어 (기본)
      ja.json          ← 일본어
      zh-CN.json       ← 중국어(간체)
    i18n/
      config.ts        ← next-intl 설정
      routing.ts       ← 로캘 라우팅 규칙
    app/
      layout.tsx       ← i18n provider 추가
      [locale]/
        page.tsx       ← 리다이렉트
        companies/
          [id]/
            layout.tsx      ← 각 페이지 동적 locale
            dashboard/page.tsx
            users/page.tsx
            admin-users/page.tsx
            ...
    hooks/
      useI18n.ts       ← i18n 헬퍼 훅
```

**번역 대상:**
1. 네비게이션 메뉴 (좌측 사이드바 항목)
2. 페이지 제목 + 설명
3. 테이블 헤더 (Users, Domains, API Keys 등)
4. 버튼 라벨 (Add, Edit, Delete, Save, Cancel 등)
5. 폼 라벨 + 플레이스홀더
6. 에러 메시지 + 성공 메시지
7. 스테이터스 배지 텍스트
8. 모달 제목 + 내용
9. 확인 대화상자 텍스트
10. 페이지네이션, 필터, 검색 관련 텍스트

**언어:**
- **한국어 (ko)** — 기본/우선
- **영어 (en)** — 기존 영문
- **일본어 (ja)** — 추가
- **중국어 간체 (zh-CN)** — 추가

**메시지 키 구조 (예시):**
```json
{
  "common": {
    "save": "저장",
    "cancel": "취소",
    "delete": "삭제",
    "edit": "편집",
    "add": "추가",
    "search": "검색",
    "loading": "로딩 중...",
    "error": "오류가 발생했습니다",
    "success": "성공"
  },
  "nav": {
    "dashboard": "대시보드",
    "users": "사용자",
    "admin_users": "관리자",
    "audit_logs": "감사 로그",
    "mail_logs": "메일 로그",
    "domains": "도메인",
    "monitoring": "모니터링",
    "settings": "설정"
  },
  "dashboard": {
    "title": "대시보드",
    "description": "시스템 개요 및 주요 지표",
    "system_metrics": "시스템 지표",
    "total_users": "총 사용자",
    "active_domains": "활성 도메인",
    ...
  },
  ...
}
```

### 완료 조건

- [ ] `next-intl` 설치 + 설정 (`i18n/config.ts`)
- [ ] 라우팅 구조 변경: `/companies/[id]/...` → `/[locale]/companies/[id]/...`
- [ ] messages/*.json 4개 언어 파일 생성 (공통 키 + 페이지별 키)
- [ ] 루트 레이아웃 i18n provider 통합
- [ ] Admin Layout에 언어 선택 드롭다운 추가 (우측 상단)
- [ ] 모든 페이지 text hardcode → `useTranslations()` 훅으로 변경
- [ ] 테이블 헤더 + 라벨 + 버튼 모두 i18n 처리
- [ ] 날짜/시간 포매팅: `Intl.DateTimeFormat` 활용
- [ ] 언어 변경 즉시 반영 (페이지 리로드 없음)
- [ ] 로캘 선택 localStorage에 저장
- [ ] `pnpm build` 성공 (TS strict, no errors)
- [ ] 브라우저에서 한글/영어/일본어/중국어 모두 테스트
- [ ] 스크린샷: 한글 + 영어 각 1개 (대시보드)
- [ ] git add + commit + push

### 루프 절차

```
1. next-intl 설치
2. i18n/config.ts 작성 (로캘, 메시지 구조)
3. i18n/routing.ts 작성 (path-based routing)
4. messages/{ko,en,ja,zh-CN}.json 모든 키 정의
5. 루트 레이아웃: i18n provider 감싸기
6. AdminLayout: 언어 선택 드롭다운 추가
7. 모든 페이지에서 hardcode text → useTranslations()
8. 테이블, 폼, 모달, 메시지 모두 i18n 처리
9. 날짜 포매팅 Intl.DateTimeFormat로 변경
10. pnpm build 성공 확인
11. 브라우저 4개 언어 테스트
12. 스크린샷 (한글 + 영어)
13. git add + commit + push
14. ACTIVE_TASK.md 갱신
15. 다음 task: TASK-089 (웹메일 UI)
```

### 참고 자료

- `docs/DESIGN.md` — 웹메일 UI 설계 (i18n 섹션)
- next-intl 공식 문서: https://next-intl-docs.vercel.app/
- Intl.DateTimeFormat: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat

### 다음 단계

TASK-089: 웹메일 클라이언트 UI (Phase 0)
