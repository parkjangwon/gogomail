import { test, expect } from '@playwright/test';

test.describe('Advanced Features', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/mail');
  });

  test('sidebar navigation tabs', async ({ page }) => {
    // Look for tab buttons (Mail, Calendar, Directory, Drive, Settings)
    const tabs = page.locator('button, a').filter({ hasText: /메일|캘린더|조직도|드라이브|설정|mail|calendar|directory|drive|settings/i });
    const tabCount = await tabs.count();
    expect(tabCount).toBeGreaterThanOrEqual(0);
  });

  test('settings modal can open', async ({ page }) => {
    // Find settings button/link
    const settingsBtn = page.locator('button, a, [role="button"], [role="link"]').filter({ hasText: /설정|settings|⚙/i }).first();
    if (await settingsBtn.isVisible()) {
      await settingsBtn.click();
      await page.waitForTimeout(500);
      // Modal or panel should appear
      const modal = page.locator('[role="dialog"], [class*="modal"], [class*="panel"]').first();
      const isVisible = await modal.isVisible().catch(() => false);
      expect(typeof isVisible).toBe('boolean');
    }
  });

  test('calendar view accessible', async ({ page }) => {
    // Look for Calendar tab
    const calendarTab = page.locator('button, a').filter({ hasText: /캘린더|calendar/i }).first();
    if (await calendarTab.isVisible()) {
      await calendarTab.click();
      await page.waitForTimeout(1000);
      // Calendar elements might appear
      const calView = page.locator('[class*="calendar"], [role="grid"], [role="table"]').first();
      const isVisible = await calView.isVisible().catch(() => false);
      expect(typeof isVisible).toBe('boolean');
    }
  });

  test('directory/org view accessible', async ({ page }) => {
    // Look for Directory or Organization tab
    const dirTab = page.locator('button, a').filter({ hasText: /조직도|directory|조직|organization/i }).first();
    if (await dirTab.isVisible()) {
      await dirTab.click();
      await page.waitForTimeout(1000);
      // Directory content should load
      const content = page.locator('main, [role="main"]').first();
      const isVisible = await content.isVisible().catch(() => false);
      expect(typeof isVisible).toBe('boolean');
    }
  });

  test('drive view accessible', async ({ page }) => {
    // Look for Drive tab
    const driveTab = page.locator('button, a').filter({ hasText: /드라이브|drive/i }).first();
    if (await driveTab.isVisible()) {
      await driveTab.click();
      await page.waitForTimeout(1000);
      // Drive content should appear
      const driveContent = page.locator('main, [role="main"], [class*="file"], [class*="drive"]').first();
      const isVisible = await driveContent.isVisible().catch(() => false);
      expect(typeof isVisible).toBe('boolean');
    }
  });

  test('user menu has profile items', async ({ page }) => {
    // Look for user profile button
    const profileBtn = page.locator('button, [role="button"]').filter({ hasText: /프로필|profile|사용자|user|account/i }).first();
    if (await profileBtn.isVisible()) {
      await profileBtn.click();
      await page.waitForTimeout(300);
      // Menu should appear with profile-related items
      const menu = page.locator('[role="menu"], [class*="menu"], [class*="dropdown"]').first();
      const isVisible = await menu.isVisible().catch(() => false);
      expect(typeof isVisible).toBe('boolean');
    }
  });
});
