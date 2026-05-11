'use client';

import { useState, useEffect } from 'react';
import { Folder, MessageSummary, getFolders, getMessages } from '@/lib/api';

export function useMailList(folderId: string) {
  const [folders, setFolders] = useState<Folder[]>([]);
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [foldersLoading, setFoldersLoading] = useState(true);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState('');

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
    setHasMore(false);
    setNextCursor('');
    setMessagesLoading(true);
    getMessages(folderId)
      .then((data) => {
        if (!cancelled) {
          setMessages(data.messages ?? []);
          setHasMore(data.has_more);
          setNextCursor(data.next_cursor);
        }
      })
      .catch(() => { if (!cancelled) setMessages([]); })
      .finally(() => { if (!cancelled) setMessagesLoading(false); });
    return () => { cancelled = true; };
  }, [folderId]);

  return { folders, messages, setMessages, foldersLoading, messagesLoading, hasMore, nextCursor };
}
