# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: verify-i18n.spec.ts >> Verify Korean i18n translations on Dashboard
- Location: e2e/verify-i18n.spec.ts:3:1

# Error details

```
TimeoutError: page.waitForURL: Timeout 15000ms exceeded.
=========================== logs ===========================
waiting for navigation to "**/dashboard" until "load"
============================================================
```

# Page snapshot

```yaml
- generic [active] [ref=e1]:
  - button "Open Next.js Dev Tools" [ref=e7] [cursor=pointer]:
    - img [ref=e8]
  - alert [ref=e13]
  - generic [ref=e15]:
    - generic [ref=e16]:
      - heading "GoGoMail" [level=1] [ref=e17]
      - paragraph [ref=e18]: Admin Console
    - generic [ref=e19]:
      - generic [ref=e23]:
        - group [ref=e25]:
          - generic [ref=e27]:
            - img
          - generic [ref=e29]: Invalid credentials
        - button [ref=e31] [cursor=pointer]:
          - generic [ref=e32]:
            - img
      - generic [ref=e33]:
        - generic [ref=e34]:
          - generic [ref=e35]: Email Address
          - textbox "Email Address" [ref=e40]:
            - /placeholder: admin@system
        - generic [ref=e41]:
          - generic [ref=e42]: Password
          - textbox "Password" [ref=e47]: admin1234
        - button "Sign in" [ref=e48] [cursor=pointer]
      - generic [ref=e49]:
        - paragraph [ref=e51]: Demo Credentials
        - generic [ref=e52]: admin@system / admin1234
    - generic [ref=e53]: © 2026 GoGoMail Inc. All rights reserved.
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test';
  2  | 
  3  | test('Verify Korean i18n translations on Dashboard', async ({ page }) => {
  4  |   const BASE_URL = 'http://localhost:3001';
  5  | 
  6  |   // Login
  7  |   await page.goto(`${BASE_URL}/login`);
  8  |   await page.fill('input[type="email"]', 'admin@system');
  9  |   await page.fill('input[type="password"]', 'admin1234');
  10 |   await page.click('button:has-text("Sign in")');
> 11 |   await page.waitForURL('**/dashboard', { timeout: 15000 });
     |              ^ TimeoutError: page.waitForURL: Timeout 15000ms exceeded.
  12 | 
  13 |   // Change language to Korean
  14 |   await page.click('button:visible >> nth=0');
  15 |   await page.waitForTimeout(200);
  16 |   
  17 |   // Find and click Korean option
  18 |   const koreanOption = page.locator('text=한국어');
  19 |   if (await koreanOption.isVisible()) {
  20 |     await koreanOption.click();
  21 |     await page.waitForTimeout(1000);
  22 |   }
  23 | 
  24 |   // Get page content
  25 |   const pageText = await page.textContent('body');
  26 | 
  27 |   // Check Korean translations
  28 |   console.log('\n📋 검증 중:');
  29 |   const checks = [
  30 |     { text: '대시보드', label: 'Dashboard' },
  31 |     { text: '시스템 지표', label: 'System Metrics' },
  32 |     { text: '총 사용자', label: 'Total Users' },
  33 |     { text: '활성 도메인', label: 'Active Domains' },
  34 |     { text: '빠른 작업', label: 'Quick Actions' },
  35 |   ];
  36 | 
  37 |   let passCount = 0;
  38 |   for (const check of checks) {
  39 |     const found = pageText?.includes(check.text) || false;
  40 |     const status = found ? '✅' : '❌';
  41 |     console.log(`${status} ${check.text} (${check.label})`);
  42 |     if (found) passCount++;
  43 |   }
  44 | 
  45 |   console.log(`\n결과: ${passCount}/${checks.length} 통과\n`);
  46 | 
  47 |   // Assert at least some Korean text is present
  48 |   expect(pageText).toContain('대시보드');
  49 |   expect(pageText).toContain('시스템 지표');
  50 | });
  51 | 
```