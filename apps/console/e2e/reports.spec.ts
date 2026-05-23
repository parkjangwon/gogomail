import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Reports', () => {
  test('reports page renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/reports' });
    await expect(page.getByRole('heading', { name: /Reports|보고서/ })).toBeVisible({ timeout: 10_000 });
  });
});
