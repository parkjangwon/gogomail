/**
 * Helpers for the admin console E2E suite.
 *
 * The canonical entry point is `setupAuthedAdminPage(page, overrides?)`
 * which installs the mock API + auth localStorage + navigates to a
 * default page.  Tests can override the start path via `gotoPath`.
 *
 * The legacy `installLocalAdminSession(page)` from the original suite is
 * preserved (re-exported from mocks.ts) so the existing spec files keep
 * working unchanged.
 */
import { expect, type Page } from '@playwright/test';
import { installMocks, type MockOverrides, DEFAULT_COMPANY_ID } from './mocks';

export { installMocks, installLocalAdminSession } from './mocks';
export type { MockOverrides } from './mocks';

export interface SetupOptions extends MockOverrides {
  /** Path to navigate to after auth is installed. Defaults to the company dashboard. */
  gotoPath?: string;
  /** Skip navigation entirely — useful for tests that drive /login themselves. */
  noNavigate?: boolean;
  /** Locale to seed in localStorage (default: 'en' for stable English assertions). */
  locale?: 'en' | 'ko' | 'ja' | 'zh-CN';
}

/**
 * Install API mocks + auth localStorage/cookie + navigate to a sensible
 * default page.  Use this in `beforeEach` for any authed-console test.
 */
export async function setupAuthedAdminPage(page: Page, options: SetupOptions = {}) {
  const { gotoPath, noNavigate, locale = 'en', ...overrides } = options;
  await installMocks(page, overrides);

  // Seed locale + auth markers BEFORE first navigation so React state matches.
  await page.addInitScript((loc) => {
    try {
      window.localStorage.setItem('locale', loc);
      window.localStorage.setItem('console_authenticated', '1');
      window.localStorage.setItem('console_user_email', 'admin@system');
      window.localStorage.setItem('console_login_at', new Date().toISOString());
    } catch {
      /* ignore */
    }
  }, locale);

  // Also set a cookie so middleware that checks cookies sees an authed session.
  try {
    await page.context().addCookies([
      {
        name: 'admin_session',
        value: 'test-session',
        domain: 'localhost',
        path: '/',
        httpOnly: false,
        sameSite: 'Lax',
      },
    ]);
  } catch {
    /* ignore */
  }

  if (noNavigate) return;

  const target = gotoPath ?? `/companies/${DEFAULT_COMPANY_ID}/dashboard`;
  await page.goto(target, { waitUntil: 'domcontentloaded' });

  if (overrides.unauthorized) {
    // Tests that intentionally force 401 expect a redirect to /login.
    await page.waitForURL(/\/login|companies/, { timeout: 15_000 }).catch(() => null);
    return;
  }

  await expect(page.locator('body')).toBeVisible();
}

/**
 * Install mocks only (no auth, no navigation) — for tests that drive
 * the login page or test unauthenticated flows.
 */
export async function setupMocksOnly(page: Page, overrides: MockOverrides = {}) {
  await installMocks(page, overrides);
}
