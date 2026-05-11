'use client';

import { useState, useEffect } from 'react';
import { MessageDetail, getMessage, markRead } from '@/lib/api';

const messageCache = new Map<string, MessageDetail>();

export function invalidateMessageCache(id: string) {
  messageCache.delete(id);
}

export function useMessage(messageId: string | null) {
  const [message, setMessage] = useState<MessageDetail | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!messageId) {
      setMessage(null);
      return;
    }

    markRead(messageId, true).catch(() => undefined);

    if (messageCache.has(messageId)) {
      setMessage(messageCache.get(messageId)!);
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);
    getMessage(messageId)
      .then((data) => {
        if (!cancelled) {
          messageCache.set(messageId, data);
          setMessage(data);
        }
      })
      .catch(() => { if (!cancelled) setMessage(null); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [messageId]);

  return { message, loading };
}
