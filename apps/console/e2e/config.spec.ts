import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const CONFIG_PAGES = [
  '/companies/default/config/company',
  '/companies/default/config/domain',
  '/companies/default/config/user',
];

test.describe('Configuration pages', () => {
  for (const path of CONFIG_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
