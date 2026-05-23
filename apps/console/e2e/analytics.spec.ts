import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const ANALYTICS_PAGES = [
  '/companies/default/analytics/api-usage',
  '/companies/default/analytics/push',
];

test.describe('Analytics', () => {
  for (const path of ANALYTICS_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
