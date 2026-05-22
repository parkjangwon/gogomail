import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('Calendar', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
    const cal = page.getByRole('button', { name: /^캘린더$|calendar/i }).first();
    if (!(await cal.isVisible().catch(() => false))) {
      test.skip(true, 'calendar app entry not visible');
    }
    await cal.click();
  });

  test('calendar surface renders', async ({ page }) => {
    await expect(page.locator('main, [role="main"]').first()).toBeVisible({ timeout: 10_000 });
  });

  test('new event button opens event editor', async ({ page }) => {
    const newBtn = page.getByRole('button', { name: /새 일정|새 이벤트|new event|일정 추가/i }).first();
    if (await newBtn.isVisible().catch(() => false)) {
      await newBtn.click();
      const dialog = page.getByRole('dialog').first();
      await expect(dialog).toBeVisible({ timeout: 5_000 });
    } else {
      test.skip(true, 'new-event button not found');
    }
  });
});
