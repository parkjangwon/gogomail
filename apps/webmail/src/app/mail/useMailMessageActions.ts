import { useCallback, useRef, type Dispatch, type SetStateAction } from 'react';
import {
  deleteMessage,
  restoreMessage,
  bulkRestoreMessages,
  starMessage,
  markRead,
  moveMessage,
  bulkMarkRead,
  bulkMoveMessages,
  setPreferences,
  type MessageSummary,
  type Folder,
  type ThreadSummary,
} from '@/lib/api';
import {
  buildThreadMessages,
  getNextMessageId,
  patchThreadsForMessages,
  shouldHideMessageAfterSnooze,
} from '@/lib/mail/mailPageUtils';
import type { ToastItem } from '@/components/Toast';

interface UseMailMessageActionsParams {
  messages: MessageSummary[];
  searchResults: MessageSummary[] | null;
  threads: ThreadSummary[];
  threadMessages: MessageSummary[];
  selectedMessageId: string | null;
  activeFolderId: string;
  activeFolderSystemType: string | undefined;
  folders: Folder[];
  setMessages: Dispatch<SetStateAction<MessageSummary[]>>;
  setSearchResults: Dispatch<SetStateAction<MessageSummary[] | null>>;
  setThreadMessages: Dispatch<SetStateAction<MessageSummary[]>>;
  setThreads: Dispatch<SetStateAction<ThreadSummary[]>>;
  setSelectedMessageId: (id: string | null) => void;
  adjustUnread: (folderId: string, delta: number) => void;
  addToast: (
    message: string,
    type?: ToastItem['type'],
    options?: { duration?: number; action?: ToastItem['action'] },
  ) => void;
  setSpamDialogMessageId: (id: string | null) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function useMailMessageActions(params: UseMailMessageActionsParams) {
  const {
    messages,
    searchResults,
    threads,
    threadMessages,
    selectedMessageId,
    activeFolderId,
    activeFolderSystemType,
    folders,
    setMessages,
    setSearchResults,
    setThreadMessages,
    setThreads,
    setSelectedMessageId,
    adjustUnread,
    addToast,
    setSpamDialogMessageId,
    t,
  } = params;

  const pendingDeletesRef = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const patchVisibleMessages = useCallback(
    (ids: string[], patch: Partial<MessageSummary>) => {
      const idSet = new Set(ids);
      const applyPatch = (items: MessageSummary[]) =>
        items.map((m) => (idSet.has(m.id) ? { ...m, ...patch } : m));
      setMessages(applyPatch);
      setSearchResults((prev) => (prev ? applyPatch(prev) : prev));
      setThreadMessages(applyPatch);
      setThreads((prev) => patchThreadsForMessages(prev, ids, patch));
    },
    [setMessages, setThreadMessages, setThreads],
  );

  // Remove messages from all visible sources so delete/archive/move take immediate effect.
  // Must also filter threads because visibleMessages uses buildThreadMessages(threads) when
  // threadViewEnabled is true — threads use (latest_message_id || thread.id) as the display id.
  const removeVisibleMessages = useCallback(
    (ids: string[]) => {
      const idSet = new Set(ids);
      const filterFn = (prev: MessageSummary[]) => prev.filter((m) => !idSet.has(m.id));
      setMessages(filterFn);
      setSearchResults((prev) => (prev ? filterFn(prev) : prev));
      setThreadMessages(filterFn);
      setThreads((prev) => prev.filter((t) => !idSet.has(t.latest_message_id || t.id)));
    },
    [setMessages, setThreadMessages, setThreads],
  );

  const findVisibleMessage = useCallback(
    (id: string) =>
      messages.find((m) => m.id === id) ??
      searchResults?.find((m) => m.id === id) ??
      threadMessages.find((m) => m.id === id) ??
      buildThreadMessages(threads).find((m) => m.id === id),
    [messages, searchResults, threadMessages, threads],
  );

  const countUnreadVisible = useCallback(
    (ids: string[]) =>
      ids.reduce((count, id) => count + (findVisibleMessage(id)?.read === false ? 1 : 0), 0),
    [findVisibleMessage],
  );

  const getNextId = useCallback(
    (id: string): string | null => getNextMessageId(messages, id),
    [messages],
  );

  const handleMarkUnread = useCallback(async () => {
    if (!selectedMessageId) return;
    patchVisibleMessages([selectedMessageId], { read: false });
    adjustUnread(activeFolderId, 1);
    addToast(t('misc.mailPage.markedUnread'), 'info');
    markRead(selectedMessageId, false).catch(() => {
      patchVisibleMessages([selectedMessageId], { read: true });
      adjustUnread(activeFolderId, -1);
    });
  }, [selectedMessageId, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleMarkRead = useCallback(async () => {
    if (!selectedMessageId) return;
    const msg = findVisibleMessage(selectedMessageId);
    if (msg?.read) return;
    patchVisibleMessages([selectedMessageId], { read: true });
    adjustUnread(activeFolderId, -1);
    addToast(t('misc.mailPage.markedRead'), 'info');
    markRead(selectedMessageId, true).catch(() => {
      patchVisibleMessages([selectedMessageId], { read: false });
      adjustUnread(activeFolderId, 1);
    });
  }, [selectedMessageId, findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleToggleReadMessage = useCallback(
    (id: string, read: boolean) => {
      const prev = findVisibleMessage(id);
      if (!prev || prev.read === read) return;
      patchVisibleMessages([id], { read });
      adjustUnread(activeFolderId, read ? -1 : 1);
      markRead(id, read).catch(() => {
        patchVisibleMessages([id], { read: !read });
        adjustUnread(activeFolderId, read ? 1 : -1);
      });
    },
    [findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId],
  );

  const handleDeleteById = useCallback(
    (id: string) => {
      const msgToDelete =
        messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
      if (msgToDelete && !msgToDelete.read) adjustUnread(activeFolderId, -1);
      const nextId = getNextId(id);
      // Snapshot the thread entry before removal so undo can restore it.
      const threadToRestore = threads.find((t) => (t.latest_message_id || t.id) === id);
      removeVisibleMessages([id]);
      if (selectedMessageId === id) setSelectedMessageId(nextId);

      const inTrash = activeFolderSystemType === 'trash';
      const trashFolder = inTrash ? null : folders.find((f) => f.system_type === 'trash');

      const timer = setTimeout(() => {
        pendingDeletesRef.current.delete(id);
        if (inTrash || !trashFolder) {
          // Already in trash → permanent delete
          deleteMessage(id).catch(() => {});
        } else {
          // Move to trash (soft delete)
          moveMessage(id, trashFolder.id).catch(() => {});
        }
      }, 5000);
      pendingDeletesRef.current.set(id, timer);

      addToast(t('misc.mailPage.deleted'), 'info', {
        duration: 5000,
        action: {
          label: t('misc.mailPage.undo'),
          onClick: () => {
            const timer = pendingDeletesRef.current.get(id);
            if (timer) {
              clearTimeout(timer);
              pendingDeletesRef.current.delete(id);
            }
            if (msgToDelete) {
              setMessages((prev) => [msgToDelete, ...prev]);
              setSearchResults((prev) => (prev ? [msgToDelete, ...prev] : prev));
            }
            if (threadToRestore) {
              setThreads((prev) => [threadToRestore, ...prev]);
            }
          },
        },
      });
    },
    [
      messages,
      searchResults,
      threads,
      selectedMessageId,
      activeFolderId,
      activeFolderSystemType,
      folders,
      removeVisibleMessages,
      setMessages,
      addToast,
      getNextId,
      adjustUnread,
    ],
  );

  const handleDelete = useCallback(() => {
    if (!selectedMessageId) return;
    handleDeleteById(selectedMessageId);
  }, [selectedMessageId, handleDeleteById]);

  const handleBulkDelete = useCallback(
    async (ids: string[]) => {
      const unreadDeleteCount = countUnreadVisible(ids);
      if (unreadDeleteCount > 0) adjustUnread(activeFolderId, -unreadDeleteCount);
      removeVisibleMessages(ids);
      if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
      const inTrash = activeFolderSystemType === 'trash';
      const trashFolder = inTrash ? null : folders.find((f) => f.system_type === 'trash');
      let failed = 0;
      if (inTrash || !trashFolder) {
        // Already in trash → permanent delete
        const results = await Promise.allSettled(ids.map((id) => deleteMessage(id)));
        failed = results.filter((r) => r.status === 'rejected').length;
      } else {
        // Move to trash (soft delete)
        try {
          await bulkMoveMessages(ids, trashFolder.id);
        } catch {
          failed = ids.length;
        }
      }
      if (failed > 0) {
        addToast(
          t('misc.mailPage.bulkDeleteMixed', { ok: ids.length - failed, failed }),
          'error',
        );
      } else {
        addToast(t('misc.mailPage.bulkDeleted', { count: ids.length }));
      }
    },
    [
      selectedMessageId,
      countUnreadVisible,
      adjustUnread,
      activeFolderId,
      activeFolderSystemType,
      folders,
      removeVisibleMessages,
      addToast,
    ],
  );

  const handleRestore = useCallback(
    async (id: string) => {
      const nextId = getNextId(id);
      removeVisibleMessages([id]);
      if (selectedMessageId === id) setSelectedMessageId(nextId);
      try {
        await restoreMessage(id);
        addToast(t('misc.mailPage.restored'));
      } catch {
        addToast(t('misc.mailPage.restoreFailed'), 'error');
      }
    },
    [selectedMessageId, getNextId, removeVisibleMessages, addToast],
  );

  const handleBulkRestore = useCallback(
    async (ids: string[]) => {
      removeVisibleMessages(ids);
      if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
      try {
        await bulkRestoreMessages(ids);
        addToast(t('misc.mailPage.bulkRestored', { count: ids.length }));
      } catch {
        addToast(t('misc.mailPage.restoreFailed'), 'error');
      }
    },
    [selectedMessageId, removeVisibleMessages, addToast],
  );

  // Restore archived messages back to inbox (archive uses move, not the trash restore API)
  const handleRestoreFromArchive = useCallback(
    (id: string) => {
      const inboxFolder = folders.find((f) => f.system_type === 'inbox');
      if (!inboxFolder) return;
      const msg =
        messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
      const nextId = getNextId(id);
      removeVisibleMessages([id]);
      if (selectedMessageId === id) setSelectedMessageId(nextId);
      addToast(t('misc.mailPage.restored'), 'info');
      void moveMessage(id, inboxFolder.id).catch(() => {
        if (msg) {
          setMessages((prev) => [msg, ...prev]);
          setSearchResults((prev) => (prev ? [msg, ...prev] : prev));
        }
        addToast(t('misc.mailPage.restoreFailed'), 'error');
      });
    },
    [
      folders,
      messages,
      searchResults,
      selectedMessageId,
      getNextId,
      removeVisibleMessages,
      setMessages,
      addToast,
    ],
  );

  const handleBulkRestoreFromArchive = useCallback(
    async (ids: string[]) => {
      const inboxFolder = folders.find((f) => f.system_type === 'inbox');
      if (!inboxFolder) return;
      removeVisibleMessages(ids);
      if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
      await Promise.allSettled(ids.map((id) => moveMessage(id, inboxFolder.id)));
      addToast(t('misc.mailPage.bulkRestored', { count: ids.length }));
    },
    [folders, selectedMessageId, removeVisibleMessages, addToast],
  );

  const handleBulkMarkRead = useCallback(
    async (ids: string[]) => {
      const unreadCount = countUnreadVisible(ids);
      patchVisibleMessages(ids, { read: true });
      if (unreadCount > 0) adjustUnread(activeFolderId, -unreadCount);
      try {
        await bulkMarkRead(ids, true);
        addToast(t('misc.mailPage.bulkMarkedRead', { count: ids.length }), 'info');
      } catch {
        patchVisibleMessages(ids, { read: false });
        if (unreadCount > 0) adjustUnread(activeFolderId, unreadCount);
        addToast(t('misc.mailPage.markReadFailed'), 'error');
      }
    },
    [countUnreadVisible, patchVisibleMessages, adjustUnread, activeFolderId, addToast],
  );

  const handleBulkStar = useCallback(
    async (ids: string[], starred: boolean) => {
      patchVisibleMessages(ids, { starred });
      await Promise.allSettled(ids.map((id) => starMessage(id, starred)));
      addToast(
        starred
          ? t('misc.mailPage.starAdded', { count: ids.length })
          : t('misc.mailPage.starRemoved', { count: ids.length }),
        'info',
      );
    },
    [patchVisibleMessages, addToast],
  );

  const handleMarkAllRead = useCallback(async () => {
    const unreadIds = messages.filter((m) => !m.read).map((m) => m.id);
    if (unreadIds.length === 0) return;
    patchVisibleMessages(unreadIds, { read: true });
    adjustUnread(activeFolderId, -unreadIds.length);
    try {
      await bulkMarkRead(unreadIds, true);
      addToast(t('misc.mailPage.bulkMarkedRead', { count: unreadIds.length }), 'info');
    } catch {
      patchVisibleMessages(unreadIds, { read: false });
      adjustUnread(activeFolderId, unreadIds.length);
      addToast(t('misc.mailPage.markReadFailed'), 'error');
    }
  }, [messages, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleArchiveById = useCallback(
    (id: string) => {
      const archiveFolder = folders.find((f) => f.system_type === 'archive');
      if (!archiveFolder) return;
      const msgToArchive =
        messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
      if (msgToArchive && !msgToArchive.read) adjustUnread(activeFolderId, -1);
      const nextId = getNextId(id);
      const threadToRestore = threads.find((t) => (t.latest_message_id || t.id) === id);
      removeVisibleMessages([id]);
      if (selectedMessageId === id) setSelectedMessageId(nextId);
      addToast(t('misc.mailPage.archived'), 'info', {
        action: {
          label: t('misc.mailPage.undo'),
          onClick: () => {
            if (msgToArchive) {
              setMessages((prev) => [msgToArchive, ...prev]);
              setSearchResults((prev) => (prev ? [msgToArchive, ...prev] : prev));
              if (!msgToArchive.read) adjustUnread(activeFolderId, 1);
            }
            if (threadToRestore) setThreads((prev) => [threadToRestore, ...prev]);
          },
        },
      });
      void moveMessage(id, archiveFolder.id).catch(() => {
        if (msgToArchive) {
          setMessages((prev) => [msgToArchive, ...prev]);
          setSearchResults((prev) => (prev ? [msgToArchive, ...prev] : prev));
        }
      });
    },
    [
      folders,
      getNextId,
      removeVisibleMessages,
      setMessages,
      selectedMessageId,
      messages,
      searchResults,
      threads,
      addToast,
      adjustUnread,
      activeFolderId,
    ],
  );

  const handleArchive = useCallback(() => {
    if (!selectedMessageId) return;
    handleArchiveById(selectedMessageId);
  }, [selectedMessageId, handleArchiveById]);

  const handleSpam = useCallback(() => {
    if (!selectedMessageId) return;
    setSpamDialogMessageId(selectedMessageId);
  }, [selectedMessageId, setSpamDialogMessageId]);

  const executeSpam = useCallback(
    (id: string, opts: { blockSender: boolean; blockDomain: boolean }) => {
      const spamFolder = folders.find(
        (f) => f.system_type === 'spam' || f.system_type === 'junk',
      );
      if (!spamFolder) return;
      const spamMsg =
        messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
      if (spamMsg && !spamMsg.read) adjustUnread(activeFolderId, -1);
      const nextId = getNextId(id);
      const threadToRestore = threads.find((t) => (t.latest_message_id || t.id) === id);
      removeVisibleMessages([id]);
      setSelectedMessageId(nextId);
      // Block sender/domain if requested
      if (opts.blockSender) {
        try {
          const blocked: string[] = JSON.parse(
            localStorage.getItem('webmail_blocked_senders') ?? '[]',
          );
          const fromAddr = spamMsg?.from_addr ?? '';
          const toBlock: string[] = [fromAddr];
          if (opts.blockDomain && fromAddr.includes('@')) {
            toBlock.push('@' + fromAddr.split('@')[1]);
          }
          const next = [...new Set([...blocked, ...toBlock.filter(Boolean)])];
          localStorage.setItem('webmail_blocked_senders', JSON.stringify(next));
          // Record timestamp metadata for newly blocked addresses
          const meta: Record<string, string> = JSON.parse(
            localStorage.getItem('webmail_blocked_meta') ?? '{}',
          );
          const now = new Date().toISOString();
          toBlock.filter(Boolean).forEach((a) => {
            if (!meta[a]) meta[a] = now;
          });
          localStorage.setItem('webmail_blocked_meta', JSON.stringify(meta));
          void setPreferences({ blocked_senders: next });
        } catch {
          /* ignore */
        }
      }
      addToast(t('misc.mailPage.movedToSpam'), 'info', {
        action: {
          label: t('misc.mailPage.undo'),
          onClick: () => {
            if (spamMsg) {
              setMessages((prev) => [spamMsg, ...prev]);
              setSearchResults((prev) => (prev ? [spamMsg, ...prev] : prev));
              if (!spamMsg.read) adjustUnread(activeFolderId, 1);
            }
            if (threadToRestore) setThreads((prev) => [threadToRestore, ...prev]);
          },
        },
      });
      void moveMessage(id, spamFolder.id).catch(() => {
        if (spamMsg) {
          setMessages((prev) => [spamMsg, ...prev]);
          setSearchResults((prev) => (prev ? [spamMsg, ...prev] : prev));
        }
        addToast(t('misc.mailPage.moveFailed'), 'error');
      });
    },
    [
      folders,
      getNextId,
      removeVisibleMessages,
      setMessages,
      addToast,
      messages,
      searchResults,
      threads,
      adjustUnread,
      activeFolderId,
    ],
  );

  const handleBlockSender = useCallback(
    (addr: string) => {
      if (!addr) return;
      try {
        const blocked: string[] = JSON.parse(
          localStorage.getItem('webmail_blocked_senders') ?? '[]',
        );
        if (blocked.includes(addr)) return;
        const next = [...blocked, addr];
        localStorage.setItem('webmail_blocked_senders', JSON.stringify(next));
        // Record timestamp metadata
        const meta: Record<string, string> = JSON.parse(
          localStorage.getItem('webmail_blocked_meta') ?? '{}',
        );
        meta[addr] = new Date().toISOString();
        localStorage.setItem('webmail_blocked_meta', JSON.stringify(meta));
        void setPreferences({ blocked_senders: next });
      } catch {
        /* ignore */
      }
      addToast(t('misc.mailPage.senderBlocked', { addr }), 'info');
    },
    [addToast],
  );

  const handleNotSpam = useCallback(() => {
    if (!selectedMessageId) return;
    const inboxFolder = folders.find((f) => f.system_type === 'inbox');
    if (!inboxFolder) return;
    const id = selectedMessageId;
    const notSpamMsg =
      messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (notSpamMsg && !notSpamMsg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    void moveMessage(id, inboxFolder.id)
      .then(() => {
        removeVisibleMessages([id]);
        setSelectedMessageId(nextId);
        addToast(t('misc.mailPage.movedToInbox'), 'info');
      })
      .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
  }, [
    selectedMessageId,
    folders,
    getNextId,
    removeVisibleMessages,
    messages,
    searchResults,
    adjustUnread,
    activeFolderId,
    addToast,
  ]);

  const handleMove = useCallback(
    async (folderId: string) => {
      if (!selectedMessageId) return;
      const id = selectedMessageId;
      const msg =
        messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
      const nextId = getNextId(id);
      removeVisibleMessages([id]);
      setSelectedMessageId(nextId);
      moveMessage(id, folderId)
        .then(() => addToast(t('misc.mailPage.moved')))
        .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
    },
    [
      selectedMessageId,
      getNextId,
      removeVisibleMessages,
      messages,
      searchResults,
      adjustUnread,
      activeFolderId,
      addToast,
    ],
  );

  const handleStar = useCallback(
    async (id: string, starred: boolean) => {
      patchVisibleMessages([id], { starred });
      starMessage(id, starred).catch(() => {
        patchVisibleMessages([id], { starred: !starred });
      });
    },
    [patchVisibleMessages],
  );

  const handleSnooze = useCallback(
    (id: string, until: Date) => {
      try {
        const stored: Record<string, string> = JSON.parse(
          localStorage.getItem('webmail_snoozed') ?? '{}',
        );
        stored[id] = until.toISOString();
        localStorage.setItem('webmail_snoozed', JSON.stringify(stored));
      } catch {
        /* ignore */
      }
      if (shouldHideMessageAfterSnooze(activeFolderId)) {
        setMessages((prev) => prev.filter((m) => m.id !== id));
        if (selectedMessageId === id) setSelectedMessageId(null);
      }
      addToast(
        t('misc.mailPage.snoozeNotifyAt', {
          time: until.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' }),
        }),
        'info',
        { duration: 4000 },
      );
    },
    [activeFolderId, selectedMessageId, setMessages, addToast],
  );

  return {
    pendingDeletesRef,
    patchVisibleMessages,
    removeVisibleMessages,
    findVisibleMessage,
    countUnreadVisible,
    getNextId,
    handleMarkUnread,
    handleMarkRead,
    handleToggleReadMessage,
    handleDeleteById,
    handleDelete,
    handleBulkDelete,
    handleRestore,
    handleBulkRestore,
    handleRestoreFromArchive,
    handleBulkRestoreFromArchive,
    handleBulkMarkRead,
    handleBulkStar,
    handleMarkAllRead,
    handleArchiveById,
    handleArchive,
    handleSpam,
    executeSpam,
    handleBlockSender,
    handleNotSpam,
    handleMove,
    handleStar,
    handleSnooze,
  };
}
