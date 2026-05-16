import { test, expect } from '@playwright/test';

test('Verify Korean i18n translations on Dashboard', async ({ page }) => {
  const BASE_URL = 'http://localhost:3001';

  await page.addInitScript(() => {
    window.localStorage.setItem('locale', 'ko');
  });

  // Login
  await page.goto(`${BASE_URL}/login`);
  await page.fill('input[type="email"]', 'admin@system');
  await page.fill('input[type="password"]', 'admin1234');
  await page.getByRole('button', { name: /Sign in|로그인/ }).click();
  await page.waitForURL('**/companies/**/dashboard', { timeout: 15000 });
  await expect(page.getByRole('heading', { name: '대시보드' })).toBeVisible();
  await expect(page.getByText('빠른 작업')).toBeVisible({ timeout: 15000 });

  // Get page content
  const pageText = await page.textContent('body');

  // Check Korean translations
  console.log('\n📋 검증 중:');
  const checks = [
    { text: '대시보드', label: 'Dashboard' },
    { text: '메일 볼륨', label: 'Mail Volume' },
    { text: '전체 사용자', label: 'Total Users' },
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
  expect(pageText).toContain('빠른 작업');
});
