import { test, expect } from '@playwright/test';

test('Verify Korean i18n translations on Dashboard', async ({ page }) => {
  const BASE_URL = 'http://localhost:3001';

  // Login
  await page.goto(`${BASE_URL}/login`);
  await page.fill('input[type="email"]', 'admin@system');
  await page.fill('input[type="password"]', 'admin1234');
  await page.click('button:has-text("Sign in")');
  await page.waitForURL('**/dashboard', { timeout: 15000 });

  // Change language to Korean
  await page.click('button:visible >> nth=0');
  await page.waitForTimeout(200);
  
  // Find and click Korean option
  const koreanOption = page.locator('text=한국어');
  if (await koreanOption.isVisible()) {
    await koreanOption.click();
    await page.waitForTimeout(1000);
  }

  // Get page content
  const pageText = await page.textContent('body');

  // Check Korean translations
  console.log('\n📋 검증 중:');
  const checks = [
    { text: '대시보드', label: 'Dashboard' },
    { text: '시스템 지표', label: 'System Metrics' },
    { text: '총 사용자', label: 'Total Users' },
    { text: '활성 도메인', label: 'Active Domains' },
    { text: '빠른 작업', label: 'Quick Actions' },
  ];

  let passCount = 0;
  for (const check of checks) {
    const found = pageText?.includes(check.text) || false;
    const status = found ? '✅' : '❌';
    console.log(`${status} ${check.text} (${check.label})`);
    if (found) passCount++;
  }

  console.log(`\n결과: ${passCount}/${checks.length} 통과\n`);

  // Assert at least some Korean text is present
  expect(pageText).toContain('대시보드');
  expect(pageText).toContain('시스템 지표');
});
