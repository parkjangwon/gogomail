import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Monitoring', () => {
  test('monitoring page renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/monitoring' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
