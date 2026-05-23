import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const ACCESS_PAGES = [
  '/companies/default/access/directory',
  '/companies/default/access/aliases',
  '/companies/default/access/delegations',
  '/companies/default/access/groups',
];

test.describe('Access pages', () => {
  for (const path of ACCESS_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
