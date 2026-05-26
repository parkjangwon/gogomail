import { useState, useCallback } from 'react';
import type { ToastItem } from '@/components/Toast';

export interface UseMailLabelsParams {
  addToast: (message: string, type?: ToastItem['type'], options?: { duration?: number; action?: ToastItem['action'] }) => void;
  t: (key: string, values?: Record<string, unknown>) => string;
}

export function useMailLabels({ addToast, t }: UseMailLabelsParams) {
  const [messageLabels, setMessageLabels] = useState<Record<string, string>>(() => {
    try { return JSON.parse(localStorage.getItem('webmail_labels') ?? '{}'); } catch { return {}; }
  });

  const [pinnedIds, setPinnedIds] = useState<Set<string>>(() => {
    try { return new Set<string>(JSON.parse(localStorage.getItem('webmail_pinned') ?? '[]') as string[]); } catch { return new Set(); }
  });

  const [importantIds, setImportantIds] = useState<Set<string>>(() => {
    try { return new Set<string>(JSON.parse(localStorage.getItem('webmail_important') ?? '[]') as string[]); } catch { return new Set(); }
  });

  const handlePin = useCallback((id: string) => {
    setPinnedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      try { localStorage.setItem('webmail_pinned', JSON.stringify([...next])); } catch { /* */ }
      return next;
    });
  }, []);

  const handleImportant = useCallback((id: string) => {
    setImportantIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      try { localStorage.setItem('webmail_important', JSON.stringify([...next])); } catch { /* */ }
      return next;
    });
  }, []);

  const setLabel = useCallback((id: string, color: string | null) => {
    setMessageLabels((prev) => {
      const next = { ...prev };
      if (color) next[id] = color; else delete next[id];
      try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
  }, []);

  const handleBulkLabel = useCallback((ids: string[], color: string | null) => {
    setMessageLabels((prev) => {
      const next = { ...prev };
      for (const id of ids) { if (color) next[id] = color; else delete next[id]; }
      try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
    addToast(color ? t('misc.mailPage.labelAdded', { count: ids.length }) : t('misc.mailPage.labelRemoved', { count: ids.length }), 'info');
  }, [addToast, t]);

  return {
    messageLabels,
    setMessageLabels,
    pinnedIds,
    setPinnedIds,
    importantIds,
    setImportantIds,
    handlePin,
    handleImportant,
    setLabel,
    handleBulkLabel,
  };
}
