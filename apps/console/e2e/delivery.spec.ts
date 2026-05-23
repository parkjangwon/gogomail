import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const DELIVERY_PAGES = [
  '/companies/default/delivery/routes',
  '/companies/default/delivery/relays',
];

test.describe('Delivery pages', () => {
  for (const path of DELIVERY_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
