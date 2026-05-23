import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { loginAsSeedUser } from './helpers';

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
    await loginAsSeedUser(page);

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

  test('accepts a multi-file drop as a single batch', async ({ page, browserName }) => {
    // WebKit's synthetic DragEvent does not populate DataTransfer.files
    // through `items.add(File)`, so the drop handler never sees the files.
    test.skip(browserName === 'webkit', 'WebKit drag/drop simulation gap');
    await loginAsSeedUser(page);

    await page.getByRole('button', { name: '드라이브' }).click();

    await page.locator('[data-testid="drive-drop-surface"]').evaluate((node) => {
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(new File(['alpha'], 'drop-a.txt', { type: 'text/plain' }));
      dataTransfer.items.add(new File(['beta'], 'drop-b.txt', { type: 'text/plain' }));
      dataTransfer.items.add(new File(['gamma'], 'drop-c.txt', { type: 'text/plain' }));

      node.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
      node.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
    });

    const dialog = page.locator('[data-testid="drive-upload-modal"]');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('3개 선택')).toBeVisible();
    await expect(dialog.locator('div[title="drop-a.txt"]')).toBeVisible();
    await expect(dialog.locator('div[title="drop-b.txt"]')).toBeVisible();
    await expect(dialog.locator('div[title="drop-c.txt"]')).toBeVisible();

    await dialog.getByRole('button', { name: '취소' }).first().click();
  });

  test('accepts multi-file drops while the upload modal is open', async ({ page, browserName }) => {
    test.skip(browserName === 'webkit', 'WebKit drag/drop simulation gap');
    await loginAsSeedUser(page);

    await page.getByRole('button', { name: '드라이브' }).click();

    const firstBatch = page.locator('input[type="file"]').first();
    await firstBatch.setInputFiles([
      { name: 'seed.txt', mimeType: 'text/plain', buffer: Buffer.from('seed') },
    ]);

    const modal = page.locator('[data-testid="drive-upload-modal"]');
    await expect(modal).toBeVisible();

    await modal.evaluate((node) => {
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(new File(['one'], 'modal-a.txt', { type: 'text/plain' }));
      dataTransfer.items.add(new File(['two'], 'modal-b.txt', { type: 'text/plain' }));
      dataTransfer.items.add(new File(['three'], 'modal-c.txt', { type: 'text/plain' }));

      node.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
      node.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
    });

    await expect(modal.getByText('3개 선택')).toBeVisible();
    await expect(modal.locator('div[title="seed.txt"]')).toBeVisible();
    await expect(modal.locator('div[title="modal-a.txt"]')).toBeVisible();
    await expect(modal.locator('div[title="modal-b.txt"]')).toBeVisible();
    await expect(modal.locator('div[title="modal-c.txt"]')).toBeVisible();
  });

  test('keeps the selected app after refresh', async ({ page }) => {
    await loginAsSeedUser(page);

    const driveTab = page.getByRole('button', { name: '드라이브', exact: true });
    await driveTab.click();
    await expect(driveTab).toHaveAttribute('aria-pressed', 'true');

    await page.reload();
    await expect(page.getByRole('button', { name: '드라이브', exact: true })).toHaveAttribute('aria-pressed', 'true');
  });
});
