import { test, expect, type Page } from "@playwright/test";

const BASE_URL = "http://localhost:3001";

const menuRoutes = [
  "/dashboard",
  "/tenancy/companies",
  "/tenancy/domains",
  "/tenancy/health",
  "/tenancy/change-history",
  "/users",
  "/admin-users",
  "/tenancy/onboarding",
  "/config/company",
  "/tenancy/domain-settings",
  "/organization/sso",
  "/organization/webhooks",
  "/organization/notification-templates",
  "/organization/signature",
  "/organization/scim-status",
  "/config/user",
  "/mail/message-trace",
  "/mail/flow-logs",
  "/mail/outbox",
  "/mail/delivery-attempts",
  "/mail/routing-rules",
  "/delivery/routes",
  "/delivery/relays",
  "/system/queue",
  "/system/backpressure",
  "/system/health",
  "/access/directory",
  "/access/aliases",
  "/access/delegations",
  "/access/groups",
  "/roles",
  "/audit-logs",
  "/admin-activity",
  "/security/alerts",
  "/security/suppression",
  "/security/dkim-keys",
  "/security/api-keys",
  "/security/api-settings",
  "/security/mfa",
  "/security/ip-access",
  "/security/auth-policy",
  "/security/audit-policy",
  "/security/retention",
  "/security/sessions",
  "/security/rate-limits",
  "/security/dmarc",
  "/security/spam-filter",
  "/security/smtp-policy",
  "/security/posture",
  "/compliance",
  "/compliance/legal-holds",
  "/storage/quota-dashboard",
  "/storage/quota-usage",
  "/storage/quota-alerts",
  "/storage/attachments",
  "/storage/drive",
  "/storage/seat-usage",
  "/analytics/api-usage",
  "/analytics/push",
  "/reports",
] as const;

async function login(page: Page) {
  await page.goto(`${BASE_URL}/login`);
  await page.evaluate(() => localStorage.setItem("locale", "en"));
  await page.getByPlaceholder("admin@system").fill("admin@system");
  await page.locator('input[type="password"]').fill("admin1234");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/companies/**/dashboard", { timeout: 15000, waitUntil: "domcontentloaded" });
}

async function currentCompanyId(page: Page) {
  const href = await page.getByRole("link", { name: /GoGoMail Admin|GGM/ }).getAttribute("href");
  return href ? new URL(href, BASE_URL).pathname.split("/")[2] : "default";
}

test.describe("Admin Console menu inventory", () => {
  test.setTimeout(180000);

  test("renders every sidebar menu route without page or console errors", async ({ page }) => {
    const issues: string[] = [];
    let currentRoute = "login";

    page.on("pageerror", error => {
      issues.push(`${currentRoute}: pageerror: ${error.message}`);
    });
    page.on("console", message => {
      const text = message.text();
      if (
        message.type() === "error" &&
        !text.includes("Failed to load resource") &&
        !text.includes("favicon.ico")
      ) {
        issues.push(`${currentRoute}: console ${message.type()}: ${text}`);
      }
    });

    await login(page);
    const companyId = await currentCompanyId(page);

    for (const route of menuRoutes) {
      const url = `${BASE_URL}/companies/${companyId}${route}`;
      await test.step(route, async () => {
        currentRoute = route;
        const response = await page.goto(url, { waitUntil: "domcontentloaded" });
        await expect(page.locator("body")).toBeVisible();
        await expect(page.getByRole("heading").first()).toBeVisible({ timeout: 10000 });

        const status = response?.status() ?? 0;
        if (status >= 400) issues.push(`${route}: HTTP ${status}`);
        if (page.url().includes("/login")) issues.push(`${route}: redirected to login`);

        const bodyText = await page.locator("body").innerText();
        if (/This page could not be found|404|Application error/i.test(bodyText)) {
          issues.push(`${route}: rendered an error page`);
        }
        if (/\b(?:common|pages|nav|layout|status)\.[a-z0-9_.-]+/i.test(bodyText)) {
          issues.push(`${route}: rendered untranslated message keys`);
        }
        const heading = (await page.getByRole("heading").first().innerText()).trim();
        if (heading === "Title") {
          issues.push(`${route}: rendered placeholder heading "Title"`);
        }
      });
    }

    expect(issues).toEqual([]);
  });

  test("top navigation utilities route to meaningful destinations", async ({ page }) => {
    await login(page);
    const companyId = await currentCompanyId(page);

    await page.getByRole("button", { name: /Alerts|No active alerts|active alert/i }).click();
    await page.waitForURL("**/companies/**/security/alerts", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: /Alerts|Notifications|알림/ })).toBeVisible();

    await page.goto(`${BASE_URL}/companies/${companyId}/organization`);
    await expect(page.getByRole("heading").first()).toBeVisible();
  });
});
