'use client';

import { useCallback, type Dispatch, type SetStateAction } from 'react';
import {
  createFolder,
  renameFolder,
  deleteFolder,
  moveMessage,
  type MessageSummary,
} from '@/lib/api';
import type { ToastItem } from '@/components/Toast';

interface UseMailFolderOpsParams {
  activeFolderId: string;
  setActiveFolderId: Dispatch<SetStateAction<string>>;
  messages: MessageSummary[];
  setMessages: Dispatch<SetStateAction<MessageSummary[]>>;
  selectedMessageId: string | null;
  setSelectedMessageId: (id: string | null) => void;
  refresh: () => void;
  addToast: (
    message: string,
    type?: ToastItem['type'],
  ) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function useMailFolderOps({
  activeFolderId,
  setActiveFolderId,
  messages,
  setMessages,
  selectedMessageId,
  setSelectedMessageId,
  refresh,
  addToast,
  t,
}: UseMailFolderOpsParams) {
  const handleDropMessage = useCallback(
    (messageId: string, folderId: string) => {
      setMessages((prev) => prev.filter((m) => m.id !== messageId));
      if (selectedMessageId === messageId) setSelectedMessageId(null);
      moveMessage(messageId, folderId)
        .then(() => addToast(t('misc.mailPage.moved')))
        .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
    },
    [selectedMessageId, setMessages, setSelectedMessageId, addToast, t],
  );

  const handleCreateFolder = useCallback(
    async (name: string) => {
      try {
        await createFolder(name);
        refresh();
        addToast(t('misc.mailPage.folderCreated', { name }));
      } catch {
        addToast(t('misc.mailPage.folderCreateFailed'), 'error');
      }
    },
    [refresh, addToast, t],
  );

  const handleRenameFolder = useCallback(
    async (id: string, name: string) => {
      try {
        await renameFolder(id, name);
        refresh();
        addToast(t('misc.mailPage.folderRenamed'));
      } catch {
        addToast(t('misc.mailPage.folderRenameFailed'), 'error');
      }
    },
    [refresh, addToast, t],
  );

  const handleDeleteFolder = useCallback(
    async (id: string) => {
      try {
        await deleteFolder(id);
        if (activeFolderId === id) setActiveFolderId('');
        refresh();
        addToast(t('misc.mailPage.folderDeleted'));
      } catch {
        addToast(t('misc.mailPage.folderDeleteFailed'), 'error');
      }
    },
    [activeFolderId, setActiveFolderId, refresh, addToast, t],
  );

  return { handleDropMessage, handleCreateFolder, handleRenameFolder, handleDeleteFolder };
}
