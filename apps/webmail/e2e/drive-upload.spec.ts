import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

function makeTempFile(name: string, sizeBytes: number): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogomail-drive-upload-'));
  const filePath = path.join(dir, name);
  const fd = fs.openSync(filePath, 'w');
  try {
    fs.writeSync(fd, Buffer.from('drive-upload-e2e'));
    fs.ftruncateSync(fd, sizeBytes);
  } finally {
    fs.closeSync(fd);
  }
  return filePath;
}

test.describe('Drive upload queue', () => {
  test.setTimeout(120_000);

  test('accepts three files in one batch and shows them in the modal', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('이메일').fill('pjw@parkjw.org');
    await page.getByLabel('비밀번호').fill('pass1234');
    await page.getByRole('button', { name: '로그인' }).click();
    await page.waitForURL(/\/mail/, { timeout: 15_000 });

    await page.getByRole('button', { name: '드라이브' }).click();
    await expect(page.getByRole('button', { name: '업로드' }).or(page.getByRole('button', { name: /업로드 창/ }))).toBeVisible();

    const files = [
      makeTempFile('drive-upload-a.bin', 9 * 1024 * 1024),
      makeTempFile('drive-upload-b.bin', 9 * 1024 * 1024),
      makeTempFile('drive-upload-c.bin', 9 * 1024 * 1024),
    ];

    await page.locator('input[type="file"]').first().setInputFiles(files);

    const dialog = page.getByRole('dialog', { name: '파일 업로드' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('3개 선택')).toBeVisible();
    await expect(dialog.getByText('3개 파일')).toBeVisible();
    await expect(dialog.getByText('동시 3/3')).toBeVisible();

    await expect(dialog.locator('div[title="drive-upload-a.bin"]')).toBeVisible();
    await expect(dialog.locator('div[title="drive-upload-b.bin"]')).toBeVisible();
    await expect(dialog.locator('div[title="drive-upload-c.bin"]')).toBeVisible();

    await dialog.getByRole('button', { name: '취소' }).first().click();
  });
});
