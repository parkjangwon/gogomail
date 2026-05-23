import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const SYSTEM_PAGES = [
  '/companies/default/system/queue',
  '/companies/default/system/backpressure',
  '/companies/default/system/health',
];

test.describe('System pages', () => {
  for (const path of SYSTEM_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
