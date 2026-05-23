import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const ORG_PAGES = [
  '/companies/default/organization',
  '/companies/default/organization/sso',
  '/companies/default/organization/webhooks',
  '/companies/default/organization/notification-templates',
  '/companies/default/organization/signature',
  '/companies/default/organization/scim-status',
  '/companies/default/organization/idp-config',
];

test.describe('Organization pages', () => {
  for (const path of ORG_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
