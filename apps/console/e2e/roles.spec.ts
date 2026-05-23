import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Roles', () => {
  test('roles list renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/roles' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
