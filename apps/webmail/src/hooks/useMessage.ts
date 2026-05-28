'use client';

import { useState, useEffect } from 'react';
import { MessageDetail, getMessage, markRead } from '@/lib/api';

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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!messageId) {
      setMessage(null);
      setError(null);
      return;
    }

    const cached = getCached(messageId);
    if (cached) {
      if (!cached.read) markRead(messageId, true).catch(() => undefined);
      setMessage(cached);
      setError(null);
      setLoading(false);
      return;
    }

    markRead(messageId, true).catch(() => undefined);
    let cancelled = false;
    setLoading(true);
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
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [messageId]);

  return { message, loading, error };
}
