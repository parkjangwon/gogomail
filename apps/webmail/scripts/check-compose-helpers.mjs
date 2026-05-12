import assert from 'node:assert/strict';
import { composeCloseSaveButtonAriaLabel } from '../src/lib/composeCloseSaveButtonAriaLabel.ts';
import { composeCloseSaveButtonLabel } from '../src/lib/composeCloseSaveButtonLabel.ts';
import { composeCloseSavePrompt } from '../src/lib/composeCloseSavePrompt.ts';
import { composeSendButtonLabel } from '../src/lib/composeSendButtonLabel.ts';
import { toDateTimeLocalValue } from '../src/lib/dateTimeLocal.ts';

// Keep these assertions focused on pure compose helpers that are easy to
// regress through copy or datetime formatting changes.
assert.equal(toDateTimeLocalValue(new Date(2026, 0, 2, 3, 4)), '2026-01-02T03:04');
assert.equal(toDateTimeLocalValue(new Date(2026, 10, 12, 13, 45)), '2026-11-12T13:45');

assert.equal(composeCloseSavePrompt(false), '임시저장 후 닫으시겠습니까?');
assert.equal(composeCloseSavePrompt(true), '예약 설정을 포함해 임시저장 후 닫으시겠습니까?');
assert.equal(composeCloseSaveButtonLabel(false), '임시저장');
assert.equal(composeCloseSaveButtonLabel(true), '저장 중...');
assert.equal(composeCloseSaveButtonAriaLabel(false), '임시저장 후 작성창 닫기');
assert.equal(composeCloseSaveButtonAriaLabel(true), '임시저장 중입니다');

assert.equal(composeSendButtonLabel({ sending: false, sent: false, scheduled: true, uploading: false }), '예약 전송');
assert.equal(composeSendButtonLabel({ sending: false, sent: true, scheduled: true, uploading: false }), '예약됨 ✓');

console.log('compose datetime, send button, and close-save helper checks passed');
