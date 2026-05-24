'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { Folder, MessageSummary, getFolders, getMessages } from '@/lib/api';

export type RefreshIntervalSeconds = 30 | 60 | 300;

const VIRTUAL_ALL_FOLDER_ID = '__all__';

function isExternallyLoadedVirtualFolder(folderId: string): boolean {
  return folderId.startsWith('__') && folderId !== VIRTUAL_ALL_FOLDER_ID;
}

function backendFolderId(folderId: string): string {
  return folderId === VIRTUAL_ALL_FOLDER_ID ? '' : folderId;
}

export function useMailList(folderId: string, refreshIntervalSeconds: RefreshIntervalSeconds) {
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
    if (!folderId) {
      setMessages([]);
      setHasMore(false);
      setNextCursor('');
      nextCursorRef.current = '';
      setMessagesLoading(false);
      return;
    }
    if (isExternallyLoadedVirtualFolder(folderId)) {
      // Filtered virtual folders are loaded externally via searchMessages.
      // All Mail uses the normal messages endpoint without a folder_id.
      setMessages([]);
      setHasMore(false);
      setNextCursor('');
      nextCursorRef.current = '';
      return;
    }
    let cancelled = false;
    setMessages([]);
    setHasMore(false);
    setNextCursor('');
    nextCursorRef.current = '';
    setMessagesLoading(true);
    getMessages(backendFolderId(folderId))
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
    if (!folderId || isExternallyLoadedVirtualFolder(folderId)) return;
    const cursor = nextCursorRef.current;
    if (!cursor) return;
    setLoadingMore(true);
    try {
      const data = await getMessages(backendFolderId(folderId), cursor);
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

  // Poll for new messages using the user's Settings interval.
  useEffect(() => {
    if (!folderId || isExternallyLoadedVirtualFolder(folderId)) return;
    const id = setInterval(async () => {
      try {
        const data = await getMessages(backendFolderId(folderId));
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
          return [...incoming, ...prev];
        });
      } catch {
        // ignore poll errors silently
      }
    }, refreshIntervalSeconds * 1000);
    return () => clearInterval(id);
  }, [folderId, refreshIntervalSeconds]);

  const [refreshing, setRefreshing] = useState(false);

  const refresh = useCallback(async () => {
    if (!folderId || isExternallyLoadedVirtualFolder(folderId) || refreshing) return;
    setRefreshing(true);
    try {
      const [fData, mData] = await Promise.all([getFolders(), getMessages(backendFolderId(folderId))]);
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
