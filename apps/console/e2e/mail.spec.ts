import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const MAIL_PAGES = [
  '/companies/default/mail/flow-logs',
  '/companies/default/mail/outbox',
  '/companies/default/mail/delivery-attempts',
  '/companies/default/mail/message-trace',
  '/companies/default/mail/routing-rules',
];

test.describe('Mail pages', () => {
  for (const path of MAIL_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
