import { useState, useCallback, useRef, type Dispatch, type SetStateAction } from 'react';
import { searchMessages, type MessageSummary } from '@/lib/api';
import { AdvancedFilters } from '@/components/Sidebar';
import { parseSearchOperators } from '@/lib/mail/mailPageUtils';
import type { ToastItem } from '@/components/Toast';

interface UseMailSearchParams {
  t: (key: string, values?: Record<string, any>) => string;
  addToast: (message: string, type?: ToastItem['type'], options?: { duration?: number; action?: ToastItem['action'] }) => void;
}

interface UseMailSearchReturn {
  searchQuery: string;
  setSearchQuery: Dispatch<SetStateAction<string>>;
  searchResults: MessageSummary[] | null;
  setSearchResults: Dispatch<SetStateAction<MessageSummary[] | null>>;
  searchLoading: boolean;
  advancedFilters: AdvancedFilters;
  setAdvancedFilters: Dispatch<SetStateAction<AdvancedFilters>>;
  runSearch: (q: string, filters: AdvancedFilters) => Promise<void>;
  handleSearch: (q: string) => void;
}

export function useMailSearch({ addToast: _addToast, t: _t }: UseMailSearchParams): UseMailSearchReturn { // _addToast and _t reserved for future error toasts
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [advancedFilters, setAdvancedFilters] = useState<AdvancedFilters>({});

  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const runSearch = useCallback(async (q: string, filters: AdvancedFilters) => {
    if (!q.trim() && !filters.from && !filters.to && !filters.subject && !filters.since && !filters.until && !filters.has_attachment) {
      setSearchResults(null);
      return;
    }
    setSearchLoading(true);
    try {
      const res = await searchMessages({
        q: q.trim() || undefined,
        from: filters.from || undefined,
        to: filters.to || undefined,
        subject: filters.subject || undefined,
        since: filters.since || undefined,
        until: filters.until || undefined,
        has_attachment: filters.has_attachment || undefined,
        limit: 50,
      });
      setSearchResults(res.messages ?? []);
    } catch {
      setSearchResults([]);
    } finally {
      setSearchLoading(false);
    }
  }, []);

  const handleSearch = useCallback((q: string) => {
    setSearchQuery(q);
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    const { q: plainQ, operators } = parseSearchOperators(q);
    const merged = { ...advancedFilters, ...operators };
    searchDebounceRef.current = setTimeout(() => runSearch(plainQ, merged), 300);
  }, [advancedFilters, runSearch]);

  return {
    searchQuery,
    setSearchQuery,
    searchResults,
    setSearchResults,
    searchLoading,
    advancedFilters,
    setAdvancedFilters,
    runSearch,
    handleSearch,
  };
}
