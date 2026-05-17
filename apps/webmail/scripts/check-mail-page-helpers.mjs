import assert from 'node:assert/strict';
import {
  buildThreadMessages,
  getEmptyFolderLabel,
  getNextMessageId,
  getVisibleMailMessages,
  patchThreadsForMessages,
  parseSearchOperators,
  shouldHideMessageAfterSnooze,
} from '../src/lib/mail/mailPageUtils.ts';

const parsed = parseSearchOperators('from:alice subject:"Quarterly Update" has:attachment before:2026-06-01 after:2026-05-01 hello world');
assert.equal(parsed.q, 'hello world');
assert.deepEqual(parsed.operators, {
  from: 'alice',
  subject: 'Quarterly Update',
  has_attachment: true,
  until: '2026-06-01',
  since: '2026-05-01',
});

const threads = buildThreadMessages([
  {
    id: 'thread-1',
    subject: 'Thread',
    latest_message_id: 'msg-2',
    latest_from_addr: 'sender@example.com',
    latest_at: '2026-05-15T00:00:00.000Z',
    unread_count: 0,
    starred: true,
    has_attachment: false,
    preview: 'preview',
    message_count: 2,
  },
]);
assert.equal(threads[0].id, 'msg-2');
assert.equal(threads[0].thread_id, 'thread-1');

const patchedReadThreads = patchThreadsForMessages([
  {
    id: 'thread-1',
    subject: 'Thread',
    latest_message_id: 'msg-2',
    latest_from_addr: 'sender@example.com',
    latest_at: '2026-05-15T00:00:00.000Z',
    unread_count: 1,
    starred: false,
    has_attachment: false,
    preview: 'preview',
    message_count: 2,
  },
], ['msg-2'], { read: true, starred: true });
assert.equal(patchedReadThreads[0].unread_count, 0);
assert.equal(patchedReadThreads[0].starred, true);

const patchedUnreadThreads = patchThreadsForMessages(patchedReadThreads, ['msg-2'], { read: false });
assert.equal(patchedUnreadThreads[0].unread_count, 1);

const visible = getVisibleMailMessages({
  searchResults: null,
  messages: [
    { id: 'a', subject: 'A', from_addr: 'blocked@example.com', from_name: 'Blocked', received_at: '2026-05-15T00:00:00.000Z', size: 1, read: false, starred: false, has_attachment: false, preview: '' },
    { id: 'b', subject: 'B', from_addr: 'ok@example.com', from_name: 'Ok', received_at: '2026-05-15T00:00:00.000Z', size: 1, read: false, starred: true, has_attachment: false, preview: '' },
  ],
  threads: [],
  threadViewEnabled: false,
  activeFolderId: 'inbox',
  activeFolderSystemType: 'inbox',
  blockedSenders: ['blocked@'],
  snoozedMessages: { a: '2026-05-16T00:00:00.000Z' },
  pinnedIds: new Set(['b']),
  importantIds: new Set(),
  focusModeEnabled: true,
  now: Date.parse('2026-05-15T12:00:00.000Z'),
});
assert.deepEqual(visible.map((m) => m.id), ['b']);

const virtualImportantVisible = getVisibleMailMessages({
  searchResults: null,
  messages: [
    { id: 'a', subject: 'A', from_addr: 'a@example.com', from_name: 'A', received_at: '2026-05-15T00:00:00.000Z', size: 1, read: false, starred: false, has_attachment: false, preview: '' },
    { id: 'b', subject: 'B', from_addr: 'b@example.com', from_name: 'B', received_at: '2026-05-15T00:00:00.000Z', size: 1, read: false, starred: false, has_attachment: false, preview: '' },
  ],
  threads: [],
  threadViewEnabled: false,
  activeFolderId: '__important__',
  importantIds: new Set(['b']),
});
assert.deepEqual(virtualImportantVisible.map((m) => m.id), ['b']);

assert.equal(shouldHideMessageAfterSnooze('inbox'), true);
assert.equal(shouldHideMessageAfterSnooze('__snoozed__'), false);

assert.equal(getNextMessageId([{ id: 'a' }, { id: 'b' }, { id: 'c' }], 'b'), 'c');
assert.equal(getEmptyFolderLabel('trash'), '휴지통이 비어있습니다');

console.log('mail page helper checks passed');
