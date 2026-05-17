import { expect, type Page } from '@playwright/test';

export async function loginAsSeedUser(page: Page) {
  await page.goto('/login');
  await page.getByLabel('이메일').fill('pjw@parkjw.org');
  await page.getByLabel('비밀번호').fill('pass1234');
  await page.getByRole('button', { name: '로그인' }).click();
  await page.waitForURL(/\/mail/, { timeout: 15_000 });
  await expect(page.locator('body')).toBeVisible();
}
