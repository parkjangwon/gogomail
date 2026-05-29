import { useEffect } from 'react';
import { markRead, type MessageSummary } from '@/lib/api';

interface UseMailAutoReadParams {
  selectedMessageId: string | null;
  activeFolderId: string;
  activeFolderSystemType: string | undefined;
  findVisibleMessage: (id: string) => MessageSummary | undefined;
  patchVisibleMessages: (ids: string[], patch: Partial<MessageSummary>) => void;
  adjustUnread: (folderId: string, delta: number) => void;
}

export function useMailAutoRead({
  selectedMessageId,
  activeFolderId,
  activeFolderSystemType,
  findVisibleMessage,
  patchVisibleMessages,
  adjustUnread,
}: UseMailAutoReadParams): void {
  // Mark selected message as read locally + server (delay controlled by readMark setting)
  useEffect(() => {
    if (!selectedMessageId || activeFolderSystemType === 'drafts') return;
    let readMark: string;
    try { readMark = (JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as { readMark?: string }).readMark ?? 'instant'; } catch { readMark = 'instant'; }
    if (readMark === 'manual') return;
    const delay = readMark === '2s' ? 2000 : 0;
    let cancelled = false;
    const timer = setTimeout(() => {
      if (cancelled) return;
      const msg = findVisibleMessage(selectedMessageId);
      if (!msg || msg.read) return;
      patchVisibleMessages([selectedMessageId], { read: true });
      adjustUnread(activeFolderId, -1);
      markRead(selectedMessageId, true).catch(() => {}); // fire-and-forget: failure is non-critical
    }, delay);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [selectedMessageId, findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId, activeFolderSystemType]);
}
