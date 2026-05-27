'use client';

import { useState, useRef, useEffect } from 'react';
import { MessageSummary } from '@/lib/api';
import { type FilterMode, type CategoryTab } from './messageListTypes';

export const PULL_THRESHOLD = 64;
export const PAGE_SIZE = 50;

interface UseMessageListStateParams {
  messages: MessageSummary[];
  selectedId: string | null;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isMobile?: boolean;
  onRefresh?: () => void;
  refreshing?: boolean;
}

export function useMessageListState({
  messages,
  selectedId,
  onLoadMore,
  hasMore,
  isMobile,
  onRefresh,
  refreshing,
}: UseMessageListStateParams) {
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [filterLabel, setFilterLabel] = useState<string | null>(null);
  const [showFilterDropdown, setShowFilterDropdown] = useState(false);
  const filterDropdownRef = useRef<HTMLDivElement>(null);
  const [sortAsc, setSortAsc] = useState(false);
  const [bulkMoveOpen, setBulkMoveOpen] = useState(false);
  const [categoryTab, setCategoryTab] = useState<CategoryTab>('all');
  const [noteIds, setNoteIds] = useState<Set<string>>(() => {
    try { return new Set(Object.keys(JSON.parse(localStorage.getItem('webmail_notes') ?? '{}'))); } catch { return new Set(); }
  });
  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key !== 'webmail_notes') return;
      try { setNoteIds(new Set(Object.keys(JSON.parse(e.newValue ?? '{}')))); } catch { /* */ }
    }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);
  const [compact, setCompact] = useState(() => {
    try { return localStorage.getItem('webmail_compact') === '1'; } catch { return false; }
  });
  const toggleCompact = () => setCompact((v) => {
    const next = !v;
    try { localStorage.setItem('webmail_compact', next ? '1' : '0'); } catch { /* */ }
    return next;
  });
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const pullRef = useRef<{ startY: number } | null>(null);
  const [pullY, setPullY] = useState(0);
  const [page, setPage] = useState(0);
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement>(null);

  // Scroll selected message into view when selectedId changes (e.g., j/k keyboard nav)
  useEffect(() => {
    if (!selectedId || !scrollContainerRef.current) return;
    const el = scrollContainerRef.current.querySelector<HTMLElement>(`[data-message-id="${selectedId}"]`);
    el?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }, [selectedId]);

  useEffect(() => {
    if (!showFilterDropdown) return;
    function onDown(e: MouseEvent) {
      if (filterDropdownRef.current && !filterDropdownRef.current.contains(e.target as Node)) {
        setShowFilterDropdown(false);
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [showFilterDropdown]);

  useEffect(() => {
    if (!showMoreMenu) return;
    function onDown(e: MouseEvent) {
      if (moreMenuRef.current && !moreMenuRef.current.contains(e.target as Node)) {
        setShowMoreMenu(false);
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [showMoreMenu]);

  useEffect(() => {
    if (!sentinelRef.current || !hasMore || !onLoadMore) return;
    const observer = new IntersectionObserver(
      ([entry]) => { if (entry.isIntersecting) onLoadMore(); },
      { threshold: 0.1 }
    );
    observer.observe(sentinelRef.current);
    return () => observer.disconnect();
  }, [hasMore, onLoadMore, messages.length]);

  useEffect(() => { setPage(0); }, [filterMode, filterLabel]);
  useEffect(() => { setPage(0); }, [messages]);

  return {
    filterMode, setFilterMode,
    filterLabel, setFilterLabel,
    showFilterDropdown, setShowFilterDropdown,
    filterDropdownRef,
    sortAsc, setSortAsc,
    bulkMoveOpen, setBulkMoveOpen,
    categoryTab, setCategoryTab,
    noteIds, setNoteIds,
    compact, setCompact,
    toggleCompact,
    page, setPage,
    showMoreMenu, setShowMoreMenu,
    moreMenuRef,
    sentinelRef,
    scrollContainerRef,
    pullRef,
    pullY, setPullY,
    PULL_THRESHOLD,
    PAGE_SIZE,
  };
}
