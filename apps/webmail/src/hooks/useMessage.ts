'use client';

import { useState, useEffect } from 'react';
import { MessageDetail, getMessage, markRead } from '@/lib/api';
import { ignoreNonCritical } from '@/lib/promise';

const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

const messageCache = new Map<string, CacheEntry<MessageDetail>>();

function getCached(id: string): MessageDetail | null {
  const entry = messageCache.get(id);
  if (!entry) return null;
  if (Date.now() - entry.timestamp > CACHE_TTL_MS) {
    messageCache.delete(id);
    return null;
  }
  return entry.data;
}

function setCached(id: string, data: MessageDetail): void {
  messageCache.set(id, { data, timestamp: Date.now() });
}

export function invalidateMessageCache(id: string) {
  messageCache.delete(id);
}

export function useMessage(messageId: string | null) {
  const [message, setMessage] = useState<MessageDetail | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!messageId) {
      setMessage(null);
      setError(null);
      return;
    }

    const cached = getCached(messageId);
    if (cached) {
      if (!cached.read) ignoreNonCritical(markRead(messageId, true), 'message.markRead.cached');
      setMessage(cached);
      setError(null);
      setIsLoading(false);
      return;
    }

    ignoreNonCritical(markRead(messageId, true), 'message.markRead.load');
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    getMessage(messageId)
      .then((data) => {
        if (!cancelled) {
          setCached(messageId, data);
          setMessage(data);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setMessage(null);
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      })
      .finally(() => { if (!cancelled) setIsLoading(false); });
    return () => { cancelled = true; };
  }, [messageId]);

  return { message, isLoading, error };
}
