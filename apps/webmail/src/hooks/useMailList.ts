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

// refreshIntervalSeconds is used by the caller (mail page) to drive the
// periodic refresh via its own setInterval; it is no longer used inside
// this hook to avoid a double-poll race with the page-level refresh.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function useMailList(folderId: string, refreshIntervalSeconds: RefreshIntervalSeconds) {
  const [folders, setFolders] = useState<Folder[]>([]);
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [isFoldersLoading, setIsFoldersLoading] = useState(true);
  const [isMessagesLoading, setIsMessagesLoading] = useState(false);
  const [foldersError, setFoldersError] = useState<Error | null>(null);
  const [messagesError, setMessagesError] = useState<Error | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState('');
  const nextCursorRef = useRef('');

  useEffect(() => {
    let cancelled = false;
    setIsFoldersLoading(true);
    setFoldersError(null);
    getFolders()
      .then((data) => { if (!cancelled) setFolders(data.folders); })
      .catch((err) => { if (!cancelled) setFoldersError(err instanceof Error ? err : new Error(String(err))); })
      .finally(() => { if (!cancelled) setIsFoldersLoading(false); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!folderId) {
      setMessages([]);
      setHasMore(false);
      setNextCursor('');
      nextCursorRef.current = '';
      setIsMessagesLoading(false);
      return;
    }
    if (isExternallyLoadedVirtualFolder(folderId)) {
      // Filtered virtual folders are loaded externally (page-level effect).
      // All Mail uses the normal messages endpoint without a folder_id.
      // Reset loading to false so a stale true from a previous mid-load regular
      // folder doesn't leave the skeleton spinner showing forever.
      setMessages([]);
      setHasMore(false);
      setNextCursor('');
      nextCursorRef.current = '';
      setIsMessagesLoading(false);
      return;
    }
    let cancelled = false;
    setMessages([]);
    setHasMore(false);
    setNextCursor('');
    nextCursorRef.current = '';
    setIsMessagesLoading(true);
    setMessagesError(null);
    getMessages(backendFolderId(folderId))
      .then((data) => {
        if (!cancelled) {
          setMessages(data.messages ?? []);
          setHasMore(data.has_more);
          setNextCursor(data.next_cursor);
          nextCursorRef.current = data.next_cursor;
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setMessages([]);
          setMessagesError(err instanceof Error ? err : new Error(String(err)));
        }
      })
      .finally(() => { if (!cancelled) setIsMessagesLoading(false); });
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
      prev.map((f) => f.id === folderId ? { ...f, unread: Math.max(0, (f.unread ?? 0) + delta) } : f)
    );
  }, []);

  const [refreshing, setRefreshing] = useState(false);
  // Track in-flight status via ref so refresh() never captures a stale
  // closure of `refreshing` state — changing `refreshing` recreates the
  // useCallback, but between setRefreshing(true) and the next render the
  // OLD closure is still in refreshRef, making concurrent refreshes possible.
  const refreshingRef = useRef(false);
  // Keep folderId accessible inside async refresh without it being a dep.
  const folderIdRef = useRef(folderId);
  useEffect(() => { folderIdRef.current = folderId; }, [folderId]);

  const refresh = useCallback(async () => {
    const currentFolderId = folderIdRef.current;
    if (!currentFolderId || isExternallyLoadedVirtualFolder(currentFolderId) || refreshingRef.current) return;
    refreshingRef.current = true;
    setRefreshing(true);
    try {
      const [fData, mData] = await Promise.all([getFolders(), getMessages(backendFolderId(currentFolderId))]);
      // Verify folder hasn't changed while fetch was in flight.
      if (folderIdRef.current !== currentFolderId) return;
      setFolders(fData.folders);
      setMessages(mData.messages ?? []);
      setHasMore(mData.has_more);
      setNextCursor(mData.next_cursor);
      nextCursorRef.current = mData.next_cursor;
    } catch {
      // ignore network / auth errors
    } finally {
      refreshingRef.current = false;
      setRefreshing(false);
    }
  // No deps: uses refs for all mutable values to keep the callback stable.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { folders, messages, setMessages, isFoldersLoading, isMessagesLoading, setIsMessagesLoading, loadingMore, hasMore, nextCursor, loadMore, adjustUnread, refresh, refreshing, foldersError, messagesError };
}
