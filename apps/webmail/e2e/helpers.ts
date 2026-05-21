import { expect, type Page } from '@playwright/test';

export async function loginAsSeedUser(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('webmail_authenticated', '1');
    localStorage.setItem('webmail_email', 'pjw@parkjw.org');
    localStorage.setItem('webmail_login_at', new Date().toISOString());
  });
  await page.goto('/mail');
  await page.waitForURL(/\/mail/, { timeout: 15_000 });
  await expect(page.locator('body')).toBeVisible();
}
