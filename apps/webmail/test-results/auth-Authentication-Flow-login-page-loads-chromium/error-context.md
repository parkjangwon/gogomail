# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: auth.spec.ts >> Authentication Flow >> login page loads
- Location: e2e/auth.spec.ts:4:3

# Error details

```
Error: expect(page).toHaveTitle(expected) failed

Expected pattern: /login|웹메일/i
Received string:  "GoGoMail"
Timeout: 5000ms

Call log:
  - Expect "toHaveTitle" with timeout 5000ms
    6 × unexpected value "GoGoMail"
    - unexpected value "GoGoMail (6)"
    7 × unexpected value "GoGoMail"

```

```yaml
- alert
- navigation "앱 전환":
  - button "메일 (읽지 않음 6)" [pressed]: "6"
  - button "캘린더"
  - button "연락처"
  - button "드라이브"
  - button "설정"
- complementary "메일 탐색":
  - button "계정 메뉴": 사용자
  - button "사이드바 접기"
  - button "편지 쓰기"
  - navigation:
    - button "모든 편지함"
    - button "별표 편지함"
    - button "중요 메일"
    - button "읽지 않은 메일"
    - button "첨부 편지함"
    - button "다시 알림"
    - button "핀 고정"
    - button "할 일"
    - button "받은 편지함 읽지 않은 메일 6개": 받은 편지함 6
    - button "보낸 편지함"
    - button "임시 보관함"
    - button "스팸 편지함"
    - button "휴지통"
    - text: 개인 편지함
    - button "+ 편지함 추가"
- button "전체 선택"
- button "필터 선택"
- button "새로고침"
- button "더 보기"
- button "오래된순으로 정렬"
- text: 1–10 / 10
- button "이전 페이지" [disabled]
- button "다음 페이지" [disabled]
- button "전체"
- button "알림"
- button "뉴스레터 1"
- button "주문"
- button "청구서"
- list "메일 목록":
  - group "오늘":
    - listitem: kim.chulsoo@parkjw.org kim.chulsoo@parkjw.org [개발팀] 5월 스프린트 킥오프 일정 공유 5시간 전
    - listitem: "lee.younghee@parkjw.org lee.younghee@parkjw.org Re: PR #247 코드 리뷰 요청 8시간 전"
    - listitem: newsletter@devnews.kr newsletter@devnews.kr 주간 뉴스레터 - 5월 2주차뉴스레터 22:19
    - listitem: colleague@example.com colleague@example.com 프로젝트 미팅 일정 공유 21:49
    - listitem: support@gogomail.dev support@gogomail.dev GoGoMail에 오신 것을 환영합니다 20:49
  - group "어제":
    - listitem: park.minjun@parkjw.org park.minjun@parkjw.org Q2 마케팅 캠페인 협업 요청 월
  - group "지난 7일":
    - listitem: choi.junho@parkjw.org choi.junho@parkjw.org 5월 인사평가 일정 및 자가평가 제출 안내 일
    - listitem: jung.sooyeon@parkjw.org jung.sooyeon@parkjw.org [전체] 5월 타운홀 미팅 일정 안내 토
    - listitem: han.jiyeon@parkjw.org han.jiyeon@parkjw.org 클라우드 인프라 비용 최적화 제안 금
    - listitem: kim.chulsoo@parkjw.org kim.chulsoo@parkjw.org 신규 서비스 런칭 계획 최종 검토 요청 목
- region "메일 읽기":
  - main "메일 읽기":
    - paragraph: 메시지를 선택하세요
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test';
  2  | 
  3  | test.describe('Authentication Flow', () => {
  4  |   test('login page loads', async ({ page }) => {
  5  |     await page.goto('/login');
> 6  |     await expect(page).toHaveTitle(/login|웹메일/i);
     |                        ^ Error: expect(page).toHaveTitle(expected) failed
  7  |     await expect(page.locator('input[type="email"]')).toBeVisible();
  8  |     await expect(page.locator('input[type="password"]')).toBeVisible();
  9  |   });
  10 | 
  11 |   test('redirect to login when not authenticated', async ({ page }) => {
  12 |     await page.goto('/mail');
  13 |     // Should redirect to /login if not authenticated
  14 |     await page.waitForURL(/login|auth/, { timeout: 5000 }).catch(() => null);
  15 |     // Or stay on mail page if dev mode allows
  16 |     const url = page.url();
  17 |     expect(url).toMatch(/login|auth|mail/i);
  18 |   });
  19 | 
  20 |   test('homepage redirects appropriately', async ({ page }) => {
  21 |     await page.goto('/');
  22 |     const url = page.url();
  23 |     // Should redirect to /login or /mail
  24 |     expect(url).toMatch(/login|mail/i);
  25 |   });
  26 | });
  27 | 
```