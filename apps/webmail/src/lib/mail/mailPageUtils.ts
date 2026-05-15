import type { MessageSummary, ThreadSummary } from '../api';
import type { AdvancedFilters } from '@/components/Sidebar';

export interface VisibleMailMessagesOptions {
  searchResults: MessageSummary[] | null;
  messages: MessageSummary[];
  threads: ThreadSummary[];
  threadViewEnabled: boolean;
  activeFolderId: string;
  activeFolderSystemType?: string;
  blockedSenders?: string[];
  snoozedMessages?: Record<string, string>;
  pinnedIds?: Set<string>;
  importantIds?: Set<string>;
  focusModeEnabled?: boolean;
  now?: number;
}

export function parseSearchOperators(raw: string): { q: string; operators: AdvancedFilters } {
  let q = raw;
  const operators: AdvancedFilters = {};
  q = q.replace(/\bfrom:(\S+)/gi, (_, val) => { operators.from = val; return ''; });
  q = q.replace(/\bto:(\S+)/gi, (_, val) => { operators.to = val; return ''; });
  q = q.replace(/\bsubject:(?:"([^"]+)"|(\S+))/gi, (_, quoted, plain) => { operators.subject = quoted ?? plain; return ''; });
  q = q.replace(/\bhas:attachment\b/gi, () => { operators.has_attachment = true; return ''; });
  q = q.replace(/\bbefore:(\S+)/gi, (_, val) => { operators.until = val; return ''; });
  q = q.replace(/\bafter:(\S+)/gi, (_, val) => { operators.since = val; return ''; });
  return { q: q.replace(/\s+/g, ' ').trim(), operators };
}

export function buildThreadMessages(threads: ThreadSummary[]): MessageSummary[] {
  return threads.map((t): MessageSummary => ({
    id: t.latest_message_id || t.id,
    subject: t.subject,
    from_addr: t.latest_from_addr,
    from_name: t.latest_from_addr,
    received_at: t.latest_at,
    size: 0,
    read: t.unread_count === 0,
    starred: t.starred,
    has_attachment: t.has_attachment,
    preview: t.preview,
    thread_id: t.id,
    message_count: t.message_count,
    unread_count: t.unread_count,
  }));
}

export function getVisibleMailMessages({
  searchResults,
  messages,
  threads,
  threadViewEnabled,
  activeFolderId,
  activeFolderSystemType,
  blockedSenders = [],
  snoozedMessages = {},
  pinnedIds = new Set<string>(),
  importantIds = new Set<string>(),
  focusModeEnabled = false,
  now = Date.now(),
}: VisibleMailMessagesOptions): MessageSummary[] {
  let visible = searchResults !== null
    ? searchResults
    : threadViewEnabled && threads.length > 0
      ? buildThreadMessages(threads)
      : messages;

  if (activeFolderId !== '__snoozed__') {
    if (blockedSenders.length > 0) {
      visible = visible.filter((m) => {
        const sender = m.from_addr.toLowerCase();
        return !blockedSenders.some((blocked) => sender.includes(blocked.toLowerCase()));
      });
    }
    visible = visible.filter((m) => {
      const snoozedUntil = snoozedMessages[m.id];
      return !snoozedUntil || new Date(snoozedUntil).getTime() <= now;
    });
  }

  if (activeFolderSystemType === 'inbox' && focusModeEnabled) {
    visible = visible.filter((m) => m.starred || pinnedIds.has(m.id) || importantIds.has(m.id));
  }

  return visible;
}

export function getNextMessageId(messages: MessageSummary[], id: string): string | null {
  const idx = messages.findIndex((m) => m.id === id);
  return (messages[idx + 1] ?? messages[idx - 1])?.id ?? null;
}

export function getEmptyFolderLabel(systemType?: string) {
  if (systemType === 'drafts') return '임시 보관된 메일이 없습니다';
  if (systemType === 'sent') return '보낸 메일이 없습니다';
  if (systemType === 'trash') return '휴지통이 비어있습니다';
  if (systemType === 'inbox') return '받은 메일이 없습니다';
  return undefined;
}
