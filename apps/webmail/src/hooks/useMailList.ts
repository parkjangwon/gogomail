'use client';

import { useState, useEffect } from 'react';
import { Folder, MessageSummary, getFolders, getMessages } from '@/lib/api';

export function useMailList(folderId: string) {
  const [folders, setFolders] = useState<Folder[]>([]);
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [foldersLoading, setFoldersLoading] = useState(true);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [total, setTotal] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setFoldersLoading(true);
    getFolders()
      .then((data) => { if (!cancelled) setFolders(data.folders); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setFoldersLoading(false); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setMessages([]);
    setMessagesLoading(true);
    getMessages(folderId)
      .then((data) => {
        if (!cancelled) {
          setMessages(data.messages);
          setTotal(data.total);
        }
      })
      .catch(() => { if (!cancelled) setMessages([]); })
      .finally(() => { if (!cancelled) setMessagesLoading(false); });
    return () => { cancelled = true; };
  }, [folderId]);

  return { folders, messages, setMessages, foldersLoading, messagesLoading, total };
}
