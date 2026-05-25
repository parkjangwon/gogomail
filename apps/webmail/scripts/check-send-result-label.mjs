/**
 * Unit tests for src/lib/sendResultLabel.ts
 *
 * Run with: node --experimental-strip-types scripts/check-send-result-label.mjs
 */
import assert from 'node:assert/strict';

import {
  sendStatusLabel,
  deliveryStatusLabel,
  bounceStatusLabel,
  formatSendResultLabel,
} from '../src/lib/sendResultLabel.ts';

// ── sendStatusLabel ───────────────────────────────────────────────────────────
assert.equal(sendStatusLabel('sent'), 'Send requested', 'sent without t()');
assert.equal(sendStatusLabel('scheduled'), 'Scheduled', 'scheduled without t()');
assert.equal(sendStatusLabel('failed'), 'Send failed', 'failed without t()');
assert.equal(sendStatusLabel('queued'), 'Queued', 'queued without t()');
assert.equal(sendStatusLabel(undefined), 'Queued', 'undefined treated as queued');
assert.equal(sendStatusLabel('unknown_status'), 'unknown_status', 'unknown status returned as-is');

// with translation function
const t = (k) => `[${k}]`;
assert.equal(sendStatusLabel('sent', t), '[misc.sendResult.sent]', 'uses t() when provided');
assert.equal(sendStatusLabel('scheduled', t), '[misc.sendResult.scheduled]', 'scheduled with t()');

// ── deliveryStatusLabel ───────────────────────────────────────────────────────
assert.equal(deliveryStatusLabel('delivered'), 'Delivered', 'delivered without t()');
assert.equal(deliveryStatusLabel('deferred'), 'Retrying', 'deferred → Retrying');
assert.equal(deliveryStatusLabel('failed'), 'Delivery failed', 'failed delivery');
assert.equal(deliveryStatusLabel('pending'), 'Delivery pending', 'pending delivery');
assert.equal(deliveryStatusLabel(undefined), 'Delivery pending', 'undefined → pending');
assert.equal(deliveryStatusLabel('bounce'), 'bounce', 'unknown status as-is');

assert.equal(deliveryStatusLabel('delivered', t), '[misc.sendResult.deliverDelivered]', 'delivered with t()');

// ── bounceStatusLabel ─────────────────────────────────────────────────────────
assert.equal(bounceStatusLabel('bounced'), 'Bounced', 'bounced without t()');
assert.equal(bounceStatusLabel(undefined), '', 'undefined → empty');
assert.equal(bounceStatusLabel(''), '', 'empty → empty');

// ── formatSendResultLabel ─────────────────────────────────────────────────────
// null result → empty string
assert.equal(formatSendResultLabel(null), '', 'null result → empty');

// formatSendResultLabel composes Send + Delivery + Bounce labels
assert.equal(
  formatSendResultLabel({ send_status: 'sent', delivery_status: 'delivered', bounce_status: null }),
  'Send: Send requested · Delivery: Delivered',
  'sent + delivered result'
);

assert.equal(
  formatSendResultLabel({ send_status: 'queued', delivery_status: 'pending', bounce_status: null }),
  'Send: Queued · Delivery: Delivery pending',
  'queued + pending result'
);

assert.equal(
  formatSendResultLabel({ send_status: 'failed', delivery_status: 'failed', bounce_status: 'bounced' }),
  'Send: Send failed · Delivery: Delivery failed · Bounce: Bounced',
  'failed + bounced result'
);

// result with no bounce_status
assert.equal(
  formatSendResultLabel({ send_status: 'scheduled', delivery_status: 'pending', bounce_status: '' }),
  'Send: Scheduled · Delivery: Delivery pending',
  'scheduled result (empty bounce omitted)'
);

console.log('✅ check-send-result-label: all assertions passed');
