import { expect, type Page } from '@playwright/test';
import { installMocks, type MockOverrides, SEED_USER_EMAIL } from './mocks';

/**
 * Set the localStorage flags the webmail uses to consider the user
 * authenticated and navigate to /mail.  Does NOT install API mocks.
 */
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

/**
 * Install API mocks + auth localStorage + navigate to /mail.
 * Use this in `beforeEach` for any test that needs the authed mail UI
 * without hitting a real backend.
 */
export async function setupAuthedPage(page: Page, overrides: MockOverrides = {}) {
  await installMocks(page, overrides);
  await page.addInitScript((email) => {
    localStorage.setItem('webmail_authenticated', '1');
    localStorage.setItem('webmail_email', email);
    localStorage.setItem('webmail_login_at', new Date().toISOString());
  }, SEED_USER_EMAIL);
  await page.goto('/mail');
  // If the auth state results in a redirect to /login, accept that — tests
  // that intentionally pass `unauthorized: true` rely on it.  Otherwise
  // wait for the mail UI shell to render.
  if (overrides.unauthorized) {
    await page.waitForURL(/login|mail/, { timeout: 15_000 }).catch(() => null);
    return;
  }
  await page.waitForURL(/\/mail/, { timeout: 15_000 });
  await expect(page.locator('aside[aria-label="메일 탐색"], nav, main').first()).toBeVisible({ timeout: 10_000 });
}

/**
 * Install mocks only (no auth, no navigation) — for tests that drive
 * the login page or test unauthenticated flows.
 */
export async function setupMocksOnly(page: Page, overrides: MockOverrides = {}) {
  await installMocks(page, overrides);
}
