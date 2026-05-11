'use client';

import { useState, useEffect } from 'react';
import { MessageDetail, getMessage, markRead } from '@/lib/api';

export function useMessage(messageId: string | null) {
  const [message, setMessage] = useState<MessageDetail | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!messageId) {
      setMessage(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    getMessage(messageId)
      .then((data) => { if (!cancelled) setMessage(data); })
      .catch(() => { if (!cancelled) setMessage(null); })
      .finally(() => { if (!cancelled) setLoading(false); });
    markRead(messageId, true).catch(() => undefined);
    return () => { cancelled = true; };
  }, [messageId]);

  return { message, loading };
}
