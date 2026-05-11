'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { Folder, MessageSummary, getFolders, getMessages } from '@/lib/api';

export function useMailList(folderId: string) {
  const [folders, setFolders] = useState<Folder[]>([]);
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [foldersLoading, setFoldersLoading] = useState(true);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState('');
  const nextCursorRef = useRef('');

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
    nextCursorRef.current = '';
    setMessagesLoading(true);
    getMessages(folderId)
      .then((data) => {
        if (!cancelled) {
          setMessages(data.messages ?? []);
          setHasMore(data.has_more);
          setNextCursor(data.next_cursor);
          nextCursorRef.current = data.next_cursor;
        }
      })
      .catch(() => { if (!cancelled) setMessages([]); })
      .finally(() => { if (!cancelled) setMessagesLoading(false); });
    return () => { cancelled = true; };
  }, [folderId]);

  const loadMore = useCallback(async () => {
    const cursor = nextCursorRef.current;
    if (!cursor) return;
    setLoadingMore(true);
    try {
      const data = await getMessages(folderId, cursor);
      setMessages((prev) => [...prev, ...(data.messages ?? [])]);
      setHasMore(data.has_more);
      setNextCursor(data.next_cursor);
      nextCursorRef.current = data.next_cursor;
    } catch {
      // ignore
    } finally {
      setLoadingMore(false);
    }
  }, [folderId]);

  const adjustUnread = useCallback((folderId: string, delta: number) => {
    setFolders((prev) =>
      prev.map((f) => f.id === folderId ? { ...f, unread: Math.max(0, f.unread + delta) } : f)
    );
  }, []);

  // Poll for new messages every 30s
  useEffect(() => {
    if (!folderId) return;
    const id = setInterval(async () => {
      try {
        const data = await getMessages(folderId);
        setMessages((prev) => {
          const existingIds = new Set(prev.map((m) => m.id));
          const incoming = (data.messages ?? []).filter((m) => !existingIds.has(m.id));
          if (incoming.length === 0) return prev;
          // Prepend new messages and update unread count
          setFolders((fs) =>
            fs.map((f) =>
              f.id === folderId
                ? { ...f, unread: f.unread + incoming.filter((m) => !m.read).length }
                : f
            )
          );
          // Browser notification for first unread new message
          const firstUnread = incoming.find((m) => !m.read);
          if (firstUnread && typeof Notification !== 'undefined') {
            const notify = () => {
              const title = `새 메일: ${firstUnread.from_name || firstUnread.from_addr}`;
              const body = firstUnread.subject || '(제목 없음)';
              new Notification(title, { body, icon: '/favicon.ico' });
            };
            if (Notification.permission === 'granted') notify();
            else if (Notification.permission === 'default') {
              Notification.requestPermission().then((p) => { if (p === 'granted') notify(); });
            }
          }
          return [...incoming, ...prev];
        });
      } catch {
        // ignore poll errors silently
      }
    }, 30_000);
    return () => clearInterval(id);
  }, [folderId]);

  const [refreshing, setRefreshing] = useState(false);

  const refresh = useCallback(async () => {
    if (!folderId || refreshing) return;
    setRefreshing(true);
    try {
      const [fData, mData] = await Promise.all([getFolders(), getMessages(folderId)]);
      setFolders(fData.folders);
      setMessages(mData.messages ?? []);
      setHasMore(mData.has_more);
      setNextCursor(mData.next_cursor);
      nextCursorRef.current = mData.next_cursor;
    } catch {
      // ignore
    } finally {
      setRefreshing(false);
    }
  }, [folderId, refreshing]);

  return { folders, messages, setMessages, foldersLoading, messagesLoading, loadingMore, hasMore, nextCursor, loadMore, adjustUnread, refresh, refreshing };
}
