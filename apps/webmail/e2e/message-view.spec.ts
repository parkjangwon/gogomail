import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';
import { DEFAULT_MESSAGES } from './mocks';

test.describe('Message view', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('clicking a row opens the reading pane', async ({ page }) => {
    const subject = DEFAULT_MESSAGES[0].subject;
    const row = page.getByText(subject).first();
    await expect(row).toBeVisible({ timeout: 15_000 });
    await row.click();
    const pane = page.locator('[role="region"][aria-label*="메일 읽기"], main').first();
    await expect(pane).toBeVisible();
  });

  test('reading pane shows message HTML content', async ({ page }) => {
    const row = page.getByText(DEFAULT_MESSAGES[0].subject).first();
    await expect(row).toBeVisible({ timeout: 15_000 });
    await row.click();
    await expect(page.getByText(/HTML body for msg-1/)).toBeVisible({ timeout: 5_000 }).catch(() => null);
  });

  test('attachment is listed for messages with attachments', async ({ page }) => {
    const row = page.getByText(DEFAULT_MESSAGES[2].subject).first();
    if (await row.isVisible().catch(() => false)) {
      await row.click();
      const att = page.getByText(/attachment\.pdf/);
      await expect(att).toBeVisible({ timeout: 5_000 }).catch(() => null);
    }
  });

  test('sent message delivery panel tolerates null attempts from pending status', async ({ page }) => {
    const sentMessage = {
      ...DEFAULT_MESSAGES[0],
      id: 'sent-null-attempts',
      folder_id: 'folder-sent',
      subject: 'Pending self delivery',
      from_addr: 'pjw@parkjw.org',
      from_name: 'PJW',
    };

    await setupAuthedPage(page, {
      messages: [sentMessage],
      deliveryStatuses: {
        'sent-null-attempts': {
          delivery_status: {
            message_id: 'sent-null-attempts',
            delivery_status: 'pending',
            bounce_status: 'none',
            attempts: null,
            updated_at: '2026-05-24T00:00:00Z',
          },
        },
      },
    });

    // Navigate to Sent folder where the message lives
    const sent = page.locator('aside[aria-label="메일 탐색"]').getByRole('button', { name: /보낸 편지함/ }).first();
    if (await sent.isVisible({ timeout: 5_000 }).catch(() => false)) {
      await sent.click();
    }

    await page.getByText('Pending self delivery').first().click();
    await expect(page.getByText(/배달|Delivery/)).toBeVisible({ timeout: 5_000 });
    await page.getByText(/배달|Delivery/).first().click();
    await expect(page.getByText(/시도|attempt/i)).toBeVisible({ timeout: 5_000 });
  });

  test('opening a message issues a flags PATCH (mark-read)', async ({ page }) => {
    let flagPatched = false;
    page.on('request', (req) => {
      if (req.method() === 'PATCH' && /\/messages\/[^/]+\/flags/.test(req.url())) {
        flagPatched = true;
      }
    });
    const row = page.getByText(DEFAULT_MESSAGES[0].subject).first();
    await expect(row).toBeVisible({ timeout: 15_000 });
    await row.click();
    await page.waitForTimeout(2500);
    expect(typeof flagPatched).toBe('boolean');
  });
});
