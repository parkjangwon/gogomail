import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config.  All specs default to running against the dev server
 * at http://localhost:3003.  API calls are intercepted via page.route()
 * (see e2e/mocks.ts) so a real backend is not required.
 *
 * Cross-browser coverage: chromium (default), firefox, webkit.  Run a
 * single project with --project=chromium for fast iteration.
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'html',
  use: {
    baseURL: 'http://localhost:3003',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:3003',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
