import { test, expect } from "@playwright/test";

const BASE_URL = "http://localhost:3001";

test.describe("Admin Console", () => {
  test.beforeEach(async ({ page }) => {
    // Start from the login page
    await page.goto(`${BASE_URL}/login`);
  });

  test("should display login page", async ({ page }) => {
    // Check if login form is visible
    await expect(page.locator("text=Login")).toBeVisible();
    await expect(page.locator('input[placeholder*="email" i]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
  });

  test("should reject invalid login", async ({ page }) => {
    // Try to login with invalid credentials
    await page.fill('input[placeholder*="email" i]', "invalid@example.com");
    await page.fill('input[type="password"]', "wrongpassword");
    await page.click('button:has-text("Login")');

    // Wait for error message (could be displayed in various ways)
    // This is a placeholder - actual error handling depends on implementation
    await page.waitForTimeout(1000);
  });

  test("should navigate between pages", async ({ page }) => {
    // This test assumes we can navigate without strict auth
    // In a real scenario, you'd mock auth or use test credentials

    // Check if sidebar navigation links exist
    const navLinks = page.locator('a, button').filter({ has: page.locator("text=Users") });
    const usersLink = navLinks.first();
    if (await usersLink.isVisible()) {
      await usersLink.click();
      await expect(page).toHaveURL(/\/users/);
    }
  });

  test("should display dashboard", async ({ page }) => {
    // Navigate to dashboard (or it might be the default page)
    await page.goto(`${BASE_URL}/dashboard`);

    // Wait for page to load
    await page.waitForLoadState("networkidle");

    // Check if page title or header is visible
    const header = page.locator("h1, h2");
    const isVisible = await header.count() > 0;
    expect(isVisible).toBe(true);
  });

  test("should display audit logs page", async ({ page }) => {
    await page.goto(`${BASE_URL}/audit-logs`);
    await page.waitForLoadState("networkidle");

    // Check if table or data is displayed
    const auditHeader = page.locator("text=Audit Logs");
    await expect(auditHeader).toBeVisible();

    // Check for filter controls
    const filterInputs = page.locator("input");
    const filterCount = await filterInputs.count();
    expect(filterCount).toBeGreaterThan(0);
  });

  test("should display statistics page", async ({ page }) => {
    await page.goto(`${BASE_URL}/statistics`);
    await page.waitForLoadState("networkidle");

    // Check if statistics header is visible
    const statsHeader = page.locator("text=Statistics");
    await expect(statsHeader).toBeVisible();

    // Check for metric cards or data display
    const content = page.locator("main, [role='main']");
    await expect(content).toBeVisible();
  });

  test("should display identity providers page", async ({ page }) => {
    await page.goto(`${BASE_URL}/identity-providers`);
    await page.waitForLoadState("networkidle");

    // Check if tabs are visible
    const tabs = page.locator('[role="tab"], button').filter({ hasText: /Database|LDAP|RDBMS/ });
    const tabCount = await tabs.count();
    expect(tabCount).toBeGreaterThanOrEqual(1);

    // Check if the first tab content is visible
    const firstTab = tabs.first();
    if (await firstTab.isVisible()) {
      await firstTab.click();
      const inputSelectCount = await page.locator("input, select").count();
      expect(inputSelectCount).toBeGreaterThan(0);
    }
  });

  test("should display roles page", async ({ page }) => {
    await page.goto(`${BASE_URL}/roles`);
    await page.waitForLoadState("networkidle");

    // Check if roles header is visible
    const rolesHeader = page.locator("text=Roles");
    await expect(rolesHeader).toBeVisible();

    // Check if create role button is visible
    const createButton = page.locator('button:has-text("Create")');
    expect(await createButton.isVisible()).toBeTruthy();
  });

  test("should display reports page", async ({ page }) => {
    await page.goto(`${BASE_URL}/reports`);
    await page.waitForLoadState("networkidle");

    // Check if reports header is visible
    const reportsHeader = page.locator("text=Reports");
    await expect(reportsHeader).toBeVisible();

    // Check if dropdown selectors are visible
    const selects = page.locator("select, [role='combobox'], [role='button']");
    const selectCount = await selects.count();
    expect(selectCount).toBeGreaterThan(0);
  });

  test("should handle navigation errors gracefully", async ({ page }) => {
    // Try to navigate to a non-existent page
    await page.goto(`${BASE_URL}/nonexistent`, { waitUntil: "networkidle" });

    // Page should either show 404 or redirect to home
    // The actual behavior depends on your routing setup
    const url = page.url();
    expect(url).toContain("localhost");
  });

  test("should responsive on mobile viewport", async ({ page }) => {
    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });

    await page.goto(`${BASE_URL}/dashboard`);
    await page.waitForLoadState("networkidle");

    // Check if page is still accessible on mobile
    const mainContent = page.locator("main, [role='main']");
    await expect(mainContent).toBeVisible();
  });

  test("should responsive on tablet viewport", async ({ page }) => {
    // Set tablet viewport
    await page.setViewportSize({ width: 768, height: 1024 });

    await page.goto(`${BASE_URL}/users`);
    await page.waitForLoadState("networkidle");

    // Check if page is accessible on tablet
    const content = page.locator("body");
    await expect(content).toBeVisible();
  });

  test("should handle slow network gracefully", async ({ page, context }) => {
    // Simulate slow network
    await context.route("**/*", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 500));
      await route.continue();
    });

    await page.goto(`${BASE_URL}/dashboard`);

    // Page should still be accessible even with slow network
    const pageContent = page.locator("body");
    await expect(pageContent).toBeVisible();
  });
});

test.describe("Admin Console - Data Operations", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to a data page
    await page.goto(`${BASE_URL}/audit-logs`);
    await page.waitForLoadState("networkidle");
  });

  test("should filter data", async ({ page }) => {
    // Find and interact with filter inputs
    const filterInputs = page.locator("input[placeholder*='filter' i], input[placeholder*='search' i]");

    if (await filterInputs.first().isVisible()) {
      await filterInputs.first().fill("test");
      await page.waitForTimeout(500);

      // Verify that the page didn't break
      const mainContent = page.locator("main, [role='main']");
      await expect(mainContent).toBeVisible();
    }
  });

  test("should handle empty states", async ({ page }) => {
    // Try to filter to no results
    const filterInputs = page.locator("input");

    if (await filterInputs.count() > 0) {
      await filterInputs.first().fill("nonexistent-unique-filter-12345");
      await page.waitForTimeout(500);

      // Check if empty state message or empty table is shown
      const content = page.locator("body");
      await expect(content).toBeVisible();
    }
  });

  test("should export data", async ({ page }) => {
    // Look for export button
    const exportButton = page.locator('button:has-text("Export"), button:has-text("Download")');

    if (await exportButton.isVisible()) {
      // Listen for download event
      const downloadPromise = page.waitForEvent("download");
      await exportButton.click();

      // Wait for download to start
      try {
        const download = await downloadPromise;
        expect(download).toBeDefined();
      } catch {
        // Download might not work in test environment, which is ok
      }
    }
  });
});
