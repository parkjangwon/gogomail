import { test, expect, type Page } from "@playwright/test";
import { setupAuthedAdminPage, installLocalAdminSession } from "./helpers";

const BASE_URL = "http://localhost:3001";

async function login(page: Page) {
  await setupAuthedAdminPage(page, { gotoPath: `/companies/default/dashboard` });
  await expect(page.getByRole("heading", { name: /Dashboard|대시보드/ })).toBeVisible({ timeout: 15000 });
}

test.describe("Admin Console", () => {
  test("displays the login page", async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);

    await expect(page.getByRole("heading", { name: "GoGoMail" })).toBeVisible();
    await expect(page.getByText("Admin Console")).toBeVisible();
    await expect(page.getByPlaceholder("admin@system")).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("rejects invalid login", async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);

    await page.getByPlaceholder("admin@system").fill("invalid@example.com");
    await page.locator('input[type="password"]').fill("wrongpassword");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });

  test("returns to a protected page after login", async ({ page }) => {
    await page.goto(`${BASE_URL}/companies/default/audit-logs`);
    await expect(page).toHaveURL(/\/login\?next=/, { timeout: 10_000 });

    // Install mocks while on the login page so API calls are intercepted
    await installLocalAdminSession(page);
    await page.getByPlaceholder("admin@system").fill("admin@system");
    await page.locator('input[type="password"]').fill("admin1234");
    // Button text is locale-dependent; handle both English and Korean
    await page.getByRole("button", { name: /Sign in|로그인/ }).click();

    await page.waitForURL("**/companies/default/audit-logs", { timeout: 20000, waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: /Audit Logs|감사 로그/ })).toBeVisible({ timeout: 10_000 });
  });

  test("displays dashboard and navigation", async ({ page }) => {
    await login(page);

    await expect(page.getByRole("heading", { name: /Dashboard|대시보드/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /Manage Users|사용자 관리/ })).toBeVisible();

    await page.getByRole("button", { name: /Manage Users|사용자 관리/ }).click();
    await page.waitForURL("**/companies/**/users", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: /Users|사용자/ })).toBeVisible();
  });

  test("displays audit logs filters", async ({ page }) => {
    await login(page);
    await page.goto(`${BASE_URL}/companies/default/audit-logs`, { waitUntil: "domcontentloaded" });

    await expect(page.getByRole("heading", { name: /Audit Logs|감사 로그/ })).toBeVisible({ timeout: 10_000 });
    await expect(page.locator("input").first()).toBeVisible({ timeout: 10_000 });
  });

  test("displays reports page", async ({ page }) => {
    await login(page);
    await page.goto(`${BASE_URL}/companies/default/reports`, { waitUntil: "domcontentloaded" });

    await expect(page.getByRole("heading", { name: /Reports|보고서/ })).toBeVisible({ timeout: 10_000 });
    await expect(page.locator("body")).toBeVisible();
  });

  test("handles navigation errors gracefully", async ({ page }) => {
    await page.goto(`${BASE_URL}/nonexistent`, { waitUntil: "networkidle" });

    await expect(page.locator("body")).toBeVisible();
    expect(page.url()).toContain("localhost");
  });

  test("is usable on mobile viewport", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await login(page);

    await expect(page.getByRole("link", { name: /GoGoMail Admin|GGM/ })).toBeVisible();
    await expect(page.locator("body")).toBeVisible();
  });
});

test.describe("Admin Console - Data Operations", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE_URL}/companies/default/audit-logs`, { waitUntil: "domcontentloaded" });
  });

  test("keeps the audit logs page stable while filtering", async ({ page }) => {
    const firstInput = page.locator("input").first();
    if (await firstInput.isVisible()) {
      await firstInput.fill("nonexistent-unique-filter-12345");
      await page.waitForTimeout(300);
    }

    await expect(page.getByRole("heading", { name: /Audit Logs|감사 로그/ })).toBeVisible();
  });
});
