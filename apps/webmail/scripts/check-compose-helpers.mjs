import assert from 'node:assert/strict';
import { composeCloseSaveButtonAriaLabel } from '../src/lib/composeCloseSaveButtonAriaLabel.ts';
import { composeCloseSaveButtonLabel } from '../src/lib/composeCloseSaveButtonLabel.ts';
import { composeCloseSavePrompt } from '../src/lib/composeCloseSavePrompt.ts';
import { composeSendButtonLabel } from '../src/lib/composeSendButtonLabel.ts';
import { toDateTimeLocalValue } from '../src/lib/dateTimeLocal.ts';
import { normalizeEmailTemplates } from '../src/lib/emailTemplates.ts';
import { buildQuoteHTML, invalidRecipientAddresses, parseAddressList, parseToPickerItems, pickerItemsToString } from '../src/lib/mail-address.ts';

const koMessages = {
  'misc.compose.savePrompt': '임시저장 후 닫으시겠습니까?',
  'misc.compose.savePromptScheduled': '예약 설정을 포함해 임시저장 후 닫으시겠습니까?',
  'misc.compose.closeSaveLabel': '임시저장',
  'misc.compose.closeSaveSaving': '저장 중...',
  'misc.compose.closeSaveAria': '임시저장 후 작성창 닫기',
  'misc.compose.closeSaveAriaInProgress': '임시저장 중입니다',
  'misc.compose.sendScheduledLabel': '예약 전송',
  'misc.compose.sendScheduled': '예약됨 ✓',
};
const t = (key) => koMessages[key] ?? key;

// Keep these assertions focused on pure compose helpers that are easy to
// regress through copy or datetime formatting changes.
assert.equal(toDateTimeLocalValue(new Date(2026, 0, 2, 3, 4)), '2026-01-02T03:04');
assert.equal(toDateTimeLocalValue(new Date(2026, 10, 12, 13, 45)), '2026-11-12T13:45');

assert.equal(composeCloseSavePrompt(false, t), '임시저장 후 닫으시겠습니까?');
assert.equal(composeCloseSavePrompt(true, t), '예약 설정을 포함해 임시저장 후 닫으시겠습니까?');
assert.equal(composeCloseSaveButtonLabel(false, t), '임시저장');
assert.equal(composeCloseSaveButtonLabel(true, t), '저장 중...');
assert.equal(composeCloseSaveButtonAriaLabel(false, t), '임시저장 후 작성창 닫기');
assert.equal(composeCloseSaveButtonAriaLabel(true, t), '임시저장 중입니다');

assert.equal(composeSendButtonLabel({ sending: false, sent: false, scheduled: true, uploading: false }, t), '예약 전송');
assert.equal(composeSendButtonLabel({ sending: false, sent: true, scheduled: true, uploading: false }, t), '예약됨 ✓');

const normalizedTemplates = normalizeEmailTemplates([
  { name: ' Follow up ', subject: ' Next step ', body: '<p>Thanks</p>' },
  { name: 'follow up', subject: 'Duplicate should be ignored', body: '' },
  { name: '', subject: 'Blank names are skipped', body: '' },
]);
assert.equal(normalizedTemplates.length, 1);
assert.match(normalizedTemplates[0].id, /^template-/);
assert.deepEqual(
  {
    name: normalizedTemplates[0].name,
    subject: normalizedTemplates[0].subject,
    body: normalizedTemplates[0].body,
  },
  { name: 'Follow up', subject: 'Next step', body: '<p>Thanks</p>' }
);

assert.deepEqual(
  parseAddressList('Alice <alice@example.com>, org:123e4567-e89b-12d3-a456-426614174000'),
  [
    { name: 'Alice', address: 'alice@example.com' },
    { address: 'org:123e4567-e89b-12d3-a456-426614174000' },
  ]
);
assert.deepEqual(
  invalidRecipientAddresses('ok@example.com', 'bad address', 'org:123e4567-e89b-12d3-a456-426614174000'),
  ['bad address']
);

const pickerItems = parseToPickerItems('Alice <alice@example.com>, addressbook:123e4567-e89b-12d3-a456-426614174000');
assert.equal(pickerItemsToString(pickerItems), 'Alice <alice@example.com>, addressbook:123e4567-e89b-12d3-a456-426614174000');
assert.ok(
  buildQuoteHTML('reply', {
    id: '1',
    subject: 'Hello',
    preview: '',
    from_addr: 'sender@example.com',
    from_name: 'Sender',
    received_at: '2026-01-02T03:04:00.000Z',
    size: 1,
    has_attachment: false,
    read: false,
    starred: false,
    message_id: '<msg@example.com>',
    to_addrs: [],
    cc_addrs: [],
    bcc_addrs: [],
    flags: {},
    storage_path: '/mail/1.eml',
    text_body: 'First line\nSecond line',
  }).includes('보낸 사람:')
);

console.log('compose datetime, send button, and close-save helper checks passed');
