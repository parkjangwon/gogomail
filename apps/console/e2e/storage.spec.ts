import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const STORAGE_PAGES = [
  '/companies/default/storage/quota-dashboard',
  '/companies/default/storage/quota-usage',
  '/companies/default/storage/quota-alerts',
  '/companies/default/storage/attachments',
  '/companies/default/storage/drive',
  '/companies/default/storage/seat-usage',
  '/companies/default/storage/reconciliation',
];

test.describe('Storage / Quotas', () => {
  for (const path of STORAGE_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
