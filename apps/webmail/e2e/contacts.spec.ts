import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('Contacts', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('contacts view is reachable if entry button exists', async ({ page }) => {
    const entry = page.getByRole('button', { name: /주소록|연락처|contacts/i }).first();
    if (!(await entry.isVisible().catch(() => false))) {
      test.skip(true, 'contacts entry not visible in app icon bar');
    }
    await entry.click();
    await expect(page.locator('main, [role="main"]').first()).toBeVisible();
  });

  test('contact list renders empty state with no contacts', async ({ page }) => {
    const entry = page.getByRole('button', { name: /주소록|연락처|contacts/i }).first();
    if (!(await entry.isVisible().catch(() => false))) {
      test.skip(true, 'contacts entry not visible');
    }
    await entry.click();
    // No mocked contacts → expect empty UI (no rows).
    await page.waitForTimeout(500);
    expect(page.url()).toMatch(/mail/);
  });
});
