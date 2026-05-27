'use client';

import { useEffect, type Dispatch, type SetStateAction } from 'react';
import { listThreadMessages, searchMessages, type MessageSummary } from '@/lib/api';

interface UseMailThreadMessagesParams {
  selectedThreadId: string | null;
  selectedMessageId: string | null;
  selectedMessageSubject: string | undefined;
  setThreadMessages: Dispatch<SetStateAction<MessageSummary[]>>;
}

/**
 * Fetches thread messages for the currently selected message.
 *
 * When a real thread is selected, it fetches via the thread API.
 * Otherwise falls back to subject-based grouping via search.
 */
export function useMailThreadMessages({
  selectedThreadId,
  selectedMessageId,
  selectedMessageSubject,
  setThreadMessages,
}: UseMailThreadMessagesParams): void {
  useEffect(() => {
    if (selectedThreadId) {
      let cancelled = false;
      listThreadMessages(selectedThreadId)
        .then((msgs) => {
          if (cancelled) return;
          const sorted = [...msgs].sort(
            (a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
          );
          setThreadMessages(sorted);
        })
        .catch(() => { if (!cancelled) setThreadMessages([]); });
      return () => { cancelled = true; };
    }
    // Fallback: subject-based grouping for normal message view
    if (!selectedMessageSubject) { setThreadMessages([]); return; }
    const normalizedSubject = selectedMessageSubject.replace(/^(Re|Fwd?|Fw):\s*/gi, '').trim();
    if (!normalizedSubject) { setThreadMessages([]); return; }
    let cancelled = false;
    searchMessages({ subject: normalizedSubject, limit: 20 })
      .then((res) => {
        if (cancelled) return;
        const sorted = [...(res.messages ?? [])].sort(
          (a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
        );
        setThreadMessages(sorted);
      })
      .catch(() => { if (!cancelled) setThreadMessages([]); });
    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedThreadId, selectedMessageId, selectedMessageSubject, setThreadMessages]);
}
