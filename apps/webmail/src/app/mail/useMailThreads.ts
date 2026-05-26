'use client';

import { useState, useEffect } from 'react';
import {
  getMessages,
  getMessage,
  listThreads,
  type MessageSummary,
  type MessageDetail,
  type ThreadSummary,
} from '@/lib/api';
import {
  VIRTUAL_STARRED,
  VIRTUAL_UNREAD,
  VIRTUAL_ATTACHMENTS,
  VIRTUAL_SNOOZED,
  VIRTUAL_PINNED,
  VIRTUAL_IMPORTANT,
  VIRTUAL_ALL,
} from '@/components/Sidebar';

interface UseMailThreadsParams {
  activeFolderId: string;
  foldersLoading: boolean;
  setMessages: (msgs: MessageSummary[]) => void;
  setMessagesLoading: (loading: boolean) => void;
}

export function useMailThreads({
  activeFolderId,
  foldersLoading,
  setMessages,
  setMessagesLoading,
}: UseMailThreadsParams) {
  // Incrementing this triggers the virtual-folder load effect to re-run without changing folder.
  const [virtualRefreshKey, setVirtualRefreshKey] = useState(0);
  const [threadMessages, setThreadMessages] = useState<MessageSummary[]>([]);
  const [threads, setThreads] = useState<ThreadSummary[]>([]);
  // Incrementing this triggers the thread-fetch effect to re-run without changing folder.
  const [threadRefreshKey, setThreadRefreshKey] = useState(0);

  // Virtual folder message loading.
  // __starred__, __unread__, __attachments__: use the messages API directly with
  // server-side filters (starred/read/has_attachment) so we never miss messages
  // due to a small page limit.
  // __pinned__, __important__, __snoozed__: stored in localStorage so
  // we fetch each stored ID directly via getMessage.
  useEffect(() => {
    if (!activeFolderId.startsWith('__') || activeFolderId === VIRTUAL_ALL) return;
    let cancelled = false;
    setMessagesLoading(true);

    const load = async (): Promise<MessageSummary[]> => {
      // Backend caps list requests at 200; exceeding it returns 400.
      const MAX = 200;
      if (activeFolderId === VIRTUAL_STARRED) {
        const data = await getMessages('', '', MAX, { starred: true });
        return data.messages ?? [];
      }
      if (activeFolderId === VIRTUAL_UNREAD) {
        const data = await getMessages('', '', MAX, { read: false });
        return data.messages ?? [];
      }
      if (activeFolderId === VIRTUAL_ATTACHMENTS) {
        const data = await getMessages('', '', MAX, { has_attachment: true });
        return data.messages ?? [];
      }
      // localStorage-based virtual folders: fetch each stored ID directly so we
      // never miss messages that fall outside any arbitrary page-size limit.
      const fetchByIds = async (ids: string[]): Promise<MessageSummary[]> => {
        if (ids.length === 0) return [];
        const results = await Promise.allSettled(ids.slice(0, 50).map((id) => getMessage(id)));
        return results
          .filter((r): r is PromiseFulfilledResult<MessageDetail> => r.status === 'fulfilled')
          .map((r) => r.value);
      };
      if (activeFolderId === VIRTUAL_SNOOZED) {
        try {
          const snoozed: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
          const now = Date.now();
          const ids = Object.entries(snoozed)
            .filter(([, until]) => new Date(until).getTime() > now)
            .map(([id]) => id);
          return fetchByIds(ids);
        } catch { return []; }
      }
      if (activeFolderId === VIRTUAL_PINNED) {
        try {
          const pinned: string[] = JSON.parse(localStorage.getItem('webmail_pinned') ?? '[]');
          return fetchByIds(pinned);
        } catch { return []; }
      }
      if (activeFolderId === VIRTUAL_IMPORTANT) {
        try {
          const important: string[] = JSON.parse(localStorage.getItem('webmail_important') ?? '[]');
          return fetchByIds(important);
        } catch { return []; }
      }
      return [];
    };

    load()
      .then((msgs) => { if (!cancelled) { setMessages(msgs); setMessagesLoading(false); } })
      .catch(() => { if (!cancelled) setMessagesLoading(false); });

    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeFolderId, foldersLoading, virtualRefreshKey]);

  // Thread view: fetch threads when folder changes or threadRefreshKey bumps.
  // threadRefreshKey is incremented by handleRefresh so periodic/manual refresh updates the list.
  useEffect(() => {
    if (!activeFolderId || activeFolderId.startsWith('__')) {
      setThreads([]);
      return;
    }
    let cancelled = false;
    listThreads({ folder_id: activeFolderId, limit: 50 })
      .then((r) => { if (!cancelled) setThreads(r.threads ?? []); })
      .catch(() => {});
    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeFolderId, threadRefreshKey]);

  return {
    virtualRefreshKey,
    setVirtualRefreshKey,
    threadMessages,
    setThreadMessages,
    threads,
    setThreads,
    threadRefreshKey,
    setThreadRefreshKey,
  };
}
