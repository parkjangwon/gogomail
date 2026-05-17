'use client';

import { useState, useRef, useEffect, useMemo, useCallback, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import { MessageSummary } from '@/lib/api';
import {
  StarIcon,
  ArrowPathIcon,
  XMarkIcon,
  ChevronDownIcon,
  CheckIcon as CheckIconOutline,
  Bars3Icon,
  EllipsisVerticalIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  BarsArrowDownIcon,
  BarsArrowUpIcon,
  BookmarkIcon,
} from '@heroicons/react/24/outline';
import { MessageRow } from './message-list/MessageRow';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';
import { ContactHoverCard } from './message-list/ContactHoverCard';
import { MessageListHeader } from './message-list/MessageListHeader';
import { moveNavFocus } from '@/lib/navKeyboard';
import {
  type CategoryTab,
  type FilterMode,
  type MessageListProps,
  avatarColor,
  getAutoCategory,
  formatDate,
  readingTimeLabel,
  highlight,
  CATEGORY_TABS,
} from './message-list/messageListTypes';

function getDateGroup(receivedAt: string): string {
  const date = new Date(receivedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return '오늘';
  if (diffDays === 1) return '어제';
  if (diffDays < 7) return '지난 7일';
  return '이번 달';
}


export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel, hasMore, loadingMore, onLoadMore, onStar, onBulkDelete, onBulkMarkRead, onRefresh, refreshing, isMobile, onOpenSidebar, onContextMenuMessage, onMarkAllRead, emptyFolderLabel, onEmptyFolder, folders, onBulkMove, paneWidth, fullWidth, bottomLayout, searchQuery, onDeleteMessage, onBulkRestore, onBulkLabel, onBulkStar, onArchiveMessage, onToggleReadMessage, onSnoozeMessage, onPinMessage, pinnedIds = new Set(), importantIds = new Set(), messageLabels = {}, userEmail, showPreview = true, showCategoryTabs = false }: MessageListProps) {
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [filterLabel, setFilterLabel] = useState<string | null>(null);
  const [showFilterDropdown, setShowFilterDropdown] = useState(false);
  const filterDropdownRef = useRef<HTMLDivElement>(null);
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const [hoveredMessageId, setHoveredMessageId] = useState<string | null>(null);
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
  const lastBulkIndexRef = useRef<number | null>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const pullRef = useRef<{ startY: number } | null>(null);
  const [pullY, setPullY] = useState(0);
  const PULL_THRESHOLD = 64;
  const PAGE_SIZE = 50;
  const [page, setPage] = useState(0);
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement>(null);
  const [contactCard, setContactCard] = useState<{ name: string; addr: string; count: number; x: number; y: number } | null>(null);
  const contactCardTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const senderCounts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const msg of messages) m[msg.from_addr] = (m[msg.from_addr] ?? 0) + 1;
    return m;
  }, [messages]);

  const handleAvatarEnter = useCallback((name: string, addr: string, rect: DOMRect) => {
    if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current);
    const count = senderCounts[addr] ?? 1;
    contactCardTimerRef.current = setTimeout(() => {
      setContactCard({ name, addr, count, x: rect.right + 6, y: rect.top - 8 });
    }, 350);
  }, [senderCounts]);
  const handleAvatarLeave = useCallback(() => {
    if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current);
    contactCardTimerRef.current = setTimeout(() => setContactCard(null), 120);
  }, []);

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

  const toggleBulk = (id: string, shiftKey?: boolean) => {
    const idx = filteredMessages.findIndex((m) => m.id === id);
    if (shiftKey && lastBulkIndexRef.current !== null && idx !== -1) {
      const from = Math.min(lastBulkIndexRef.current, idx);
      const to = Math.max(lastBulkIndexRef.current, idx);
      const rangeIds = filteredMessages.slice(from, to + 1).map((m) => m.id);
      setBulkSelected((prev) => {
        const next = new Set(prev);
        rangeIds.forEach((rid) => next.add(rid));
        return next;
      });
    } else {
      setBulkSelected((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id); else next.add(id);
        return next;
      });
      if (idx !== -1) lastBulkIndexRef.current = idx;
    }
  };

  const selectAll = () => setBulkSelected(new Set(filteredMessages.map((m) => m.id)));
  const clearAll = () => { setBulkSelected(new Set()); lastBulkIndexRef.current = null; };
  const getActionMessages = (ids: string[]) => ids
    .map((id) => filteredMessages.find((m) => m.id === id) ?? messages.find((m) => m.id === id))
    .filter((m): m is MessageSummary => Boolean(m));
  const runActionForIds = (ids: string[], action: (id: string) => void, clearAfter = false) => {
    ids.forEach(action);
    if (clearAfter) clearAll();
  };
  const toggleReadForIds = (ids: string[], read: boolean, clearAfter = false) => {
    ids.forEach((id) => onToggleReadMessage?.(id, read));
    if (clearAfter) clearAll();
  };
  const snoozeIdsForOneHour = (ids: string[], clearAfter = false) => {
    const until = new Date(Date.now() + 60 * 60 * 1000);
    ids.forEach((id) => onSnoozeMessage?.(id, until));
    if (clearAfter) clearAll();
  };
  const handleRowKeyDownCapture = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null;
    if (!target) return;
    if (target.closest('button, a, input, textarea, select, [role="button"]')) return;

    const row = target.closest<HTMLElement>('[data-message-id]');
    if (!row) return;

    if (event.key === 'ArrowDown' || event.key === 'j') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'next', 'message-list');
      return;
    }

    if (event.key === 'ArrowUp' || event.key === 'k') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'prev', 'message-list');
      return;
    }

    if (event.key === 'Home') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'first', 'message-list');
      return;
    }

    if (event.key === 'End') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'last', 'message-list');
      return;
    }

    if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      event.stopPropagation();
      const id = row.dataset.messageId;
      if (id) toggleBulk(id, event.shiftKey);
      return;
    }

    if (event.key === 'Enter' || event.key === 'o') {
      event.preventDefault();
      event.stopPropagation();
      const id = row.dataset.messageId;
      if (id) onSelect(id);
    }
  };

  const bulkSize = bulkSelected.size;
  const clearAllRef = useRef(clearAll);
  const selectAllRef = useRef(selectAll);
  useEffect(() => { clearAllRef.current = clearAll; selectAllRef.current = selectAll; });
  useEffect(() => {
    if (bulkSize === 0) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.stopPropagation(); clearAllRef.current(); }
    };
    window.addEventListener('keydown', handler, { capture: true });
    return () => window.removeEventListener('keydown', handler, { capture: true });
  }, [bulkSize]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.key !== 'a') return;
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement).isContentEditable) return;
      e.preventDefault();
      selectAllRef.current();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  const selectedIdRef = useRef(selectedId);
  useEffect(() => { selectedIdRef.current = selectedId; }, [selectedId]);
  useEffect(() => { setPage(0); }, [filterMode, filterLabel]);
  useEffect(() => { setPage(0); }, [messages]);

  const baseFiltered =
    filterMode === 'unread' ? messages.filter((m) => !m.read)
    : filterMode === 'read' ? messages.filter((m) => m.read)
    : filterMode === 'starred' ? messages.filter((m) => m.starred)
    : filterMode === 'unstarred' ? messages.filter((m) => !m.starred)
    : filterMode === 'attachment' ? messages.filter((m) => m.has_attachment)
    : filterMode === 'noattachment' ? messages.filter((m) => !m.has_attachment)
    : messages;

  const afterLabelFilter = filterLabel
    ? baseFiltered.filter((m) => messageLabels[m.id] === filterLabel)
    : baseFiltered;

  const activeLabelColors = [...new Set(messages.map((m) => messageLabels[m.id]).filter(Boolean))];

  const afterCategoryFilter = (showCategoryTabs && categoryTab !== 'all')
    ? afterLabelFilter.filter((m) => getAutoCategory(m.from_addr, m.subject)?.label === categoryTab)
    : afterLabelFilter;

  const categoryUnreadCounts = showCategoryTabs ? (() => {
    const counts: Partial<Record<CategoryTab, number>> = {};
    for (const m of afterLabelFilter) {
      if (m.read) continue;
      const cat = getAutoCategory(m.from_addr, m.subject)?.label as CategoryTab | undefined;
      if (cat) counts[cat] = (counts[cat] ?? 0) + 1;
    }
    return counts;
  })() : {};

  const sortedBase = (() => {
    const base = sortAsc
      ? [...afterCategoryFilter].sort((a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime())
      : afterCategoryFilter;
    if (pinnedIds.size === 0) return base;
    return [...base].sort((a, b) => {
      const aPin = pinnedIds.has(a.id) ? 0 : 1;
      const bPin = pinnedIds.has(b.id) ? 0 : 1;
      return aPin - bPin;
    });
  })();

  const [conversationMode] = useState(() => {
    try { return localStorage.getItem('webmail_conv_mode') !== '0'; } catch { return true; }
  });

  function normalizeSubject(s: string): string {
    return s.replace(/^(re|fwd?)\s*:\s*/gi, '').trim().toLowerCase();
  }

  const { filteredMessages, threadCounts } = (() => {
    if (!conversationMode) return { filteredMessages: sortedBase, threadCounts: {} as Record<string, number> };
    const seen = new Map<string, { msg: MessageSummary; count: number }>();
    for (const msg of sortedBase) {
      const key = normalizeSubject(msg.subject || '');
      const existing = seen.get(key);
      if (!existing) {
        seen.set(key, { msg, count: 1 });
      } else {
        const existingTime = new Date(existing.msg.received_at).getTime();
        const msgTime = new Date(msg.received_at).getTime();
        if (msgTime > existingTime) seen.set(key, { msg, count: existing.count + 1 });
        else seen.set(key, { ...existing, count: existing.count + 1 });
      }
    }
    const msgs = [...seen.values()].map((v) => v.msg);
    const counts: Record<string, number> = {};
    seen.forEach((v) => { counts[v.msg.id] = v.count; });
    return { filteredMessages: msgs, threadCounts: counts };
  })();

  const pageStart = page * PAGE_SIZE;
  const pageEnd = pageStart + PAGE_SIZE;
  const pagedMessages = filteredMessages.slice(pageStart, pageEnd);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) return;
      const bulkIds = [...bulkSelected];
      const ids = bulkIds.length > 0 ? bulkIds : hoveredMessageId ? [hoveredMessageId] : [];
      if (ids.length === 0) return;
      const actionMessages = getActionMessages(ids);
      if (actionMessages.length === 0) return;
      const lowerKey = event.key.toLowerCase();
      const isBulkAction = bulkIds.length > 0;
      const finish = (run: () => void) => {
        event.preventDefault();
        event.stopPropagation();
        event.stopImmediatePropagation();
        run();
      };

      if (lowerKey === 'm' && onToggleReadMessage) {
        const readTarget = actionMessages.some((m) => !m.read);
        finish(() => toggleReadForIds(ids, readTarget, isBulkAction));
        return;
      }
      if (lowerKey === 's' && onStar) {
        const starredTarget = actionMessages.some((m) => !m.starred);
        finish(() => {
          if (isBulkAction && onBulkStar) onBulkStar(ids, starredTarget);
          else ids.forEach((id) => {
            const msg = actionMessages.find((m) => m.id === id);
            if (msg) onStar(id, !msg.starred);
          });
          if (isBulkAction) clearAll();
        });
        return;
      }
      if (lowerKey === 'e' && onArchiveMessage) {
        finish(() => runActionForIds(ids, onArchiveMessage, isBulkAction));
        return;
      }
      if (lowerKey === 'z' && onSnoozeMessage) {
        finish(() => snoozeIdsForOneHour(ids, isBulkAction));
        return;
      }
      if (lowerKey === 'p' && onPinMessage) {
        finish(() => runActionForIds(ids, onPinMessage, isBulkAction));
        return;
      }
      if ((event.key === '#' || event.key === 'Delete') && onDeleteMessage) {
        finish(() => {
          if (isBulkAction && onBulkDelete) {
            onBulkDelete(ids);
            clearAll();
          } else {
            runActionForIds(ids, onDeleteMessage);
          }
        });
      }
    };
    window.addEventListener('keydown', handler, { capture: true });
    return () => window.removeEventListener('keydown', handler, { capture: true });
  }, [bulkSelected, hoveredMessageId, filteredMessages, messages, onToggleReadMessage, onStar, onBulkStar, onArchiveMessage, onSnoozeMessage, onPinMessage, onDeleteMessage, onBulkDelete]);

  const listWidth = (isMobile || fullWidth || bottomLayout || !paneWidth)
    ? { flex: 1, minWidth: 0 }
    : { width: `${paneWidth}px`, minWidth: `${paneWidth}px` };
  const containerHeight = bottomLayout ? '35vh' : '100%';
  const containerBorder: React.CSSProperties = bottomLayout
    ? { borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }
    : { borderRight: '1px solid var(--color-border-subtle)' };

  if (loading) {
    return (
      <div
        data-print="hide"
        style={{
          ...listWidth,
          height: containerHeight,
          ...containerBorder,
          overflowY: 'auto',
          padding: '12px 0',
        }}
      >
        {Array.from({ length: 8 }).map((_, i) => (
          <div
            key={i}
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--color-border-subtle)',
            }}
          >
            <div
              style={{
                height: '14px',
                background: 'var(--color-bg-tertiary)',
                borderRadius: '4px',
                marginBottom: '8px',
                width: `${60 + (i % 3) * 15}%`,
                animation: 'pulse 1.5s ease-in-out infinite',
              }}
            />
            <div
              style={{
                height: '12px',
                background: 'var(--color-bg-tertiary)',
                borderRadius: '4px',
                width: '80%',
                animation: 'pulse 1.5s ease-in-out infinite',
              }}
            />
          </div>
        ))}
      </div>
    );
  }

  const hasBulk = bulkSelected.size > 0;
  const bulkMessages = getActionMessages([...bulkSelected]);
  const bulkReadTarget = bulkMessages.some((m) => !m.read);
  const bulkStarTarget = bulkMessages.some((m) => !m.starred);
  const bulkPinned = bulkMessages.length > 0 && bulkMessages.every((m) => pinnedIds.has(m.id));
  const header = (
    <MessageListHeader
      hasBulk={hasBulk}
      bulkSelectedSize={bulkSelected.size}
      filteredCount={filteredMessages.length}
      onBulkMarkRead={onBulkMarkRead}
      onBulkToggleRead={(ids, read) => toggleReadForIds(ids, read)}
      onBulkStar={onBulkStar}
      onBulkArchive={onArchiveMessage ? (ids) => runActionForIds(ids, onArchiveMessage) : undefined}
      onBulkSnooze={onSnoozeMessage ? (ids, until) => ids.forEach((id) => onSnoozeMessage(id, until)) : undefined}
      onBulkPin={onPinMessage ? (ids) => runActionForIds(ids, onPinMessage) : undefined}
      onBulkMove={onBulkMove}
      onBulkRestore={onBulkRestore}
      onBulkLabel={onBulkLabel}
      onBulkDelete={onBulkDelete}
      folders={folders}
      bulkSelected={bulkSelected}
      clearAll={clearAll}
      selectAll={selectAll}
      bulkMoveOpen={bulkMoveOpen}
      setBulkMoveOpen={setBulkMoveOpen}
      isMobile={isMobile}
      onOpenSidebar={onOpenSidebar}
      filterMode={filterMode}
      setFilterMode={setFilterMode}
      filterLabel={filterLabel}
      setFilterLabel={setFilterLabel}
      activeLabelColors={activeLabelColors}
      showFilterDropdown={showFilterDropdown}
      setShowFilterDropdown={setShowFilterDropdown}
      onRefresh={onRefresh}
      refreshing={refreshing}
      showMoreMenu={showMoreMenu}
      setShowMoreMenu={setShowMoreMenu}
      compact={compact}
      toggleCompact={toggleCompact}
      onMarkAllRead={onMarkAllRead}
      emptyFolderLabel={emptyFolderLabel}
      onEmptyFolder={onEmptyFolder}
      messagesHaveUnread={messages.some((m) => !m.read)}
      sortAsc={sortAsc}
      setSortAsc={(value) => setSortAsc(value)}
      pageStart={pageStart}
      pageEnd={pageEnd}
      filteredMessagesLength={filteredMessages.length}
      hasMore={hasMore}
      onLoadMore={onLoadMore}
      page={page}
      setPage={setPage}
      categoryTab={categoryTab}
      setCategoryTab={setCategoryTab}
      categoryUnreadCounts={categoryUnreadCounts}
      showCategoryTabs={showCategoryTabs}
    />
  );

  if (filteredMessages.length === 0) {
    const isInboxZero = !emptyLabel && filterMode === 'all' && messages.length === 0 && !loading;
    return (
      <div data-print="hide" style={{ ...listWidth, height: containerHeight, ...containerBorder, display: 'flex', flexDirection: 'column' }}>
        {header}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '12px' }}>
          {isInboxZero ? (
            <>
              <div style={{ fontSize: '36px', lineHeight: 1 }}>✓</div>
              <div style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>받은 편지함이 깨끗합니다</div>
              <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', textAlign: 'center', maxWidth: '240px', lineHeight: 1.5 }}>모든 메일을 처리했습니다. 잠시 여유를 즐기세요.</div>
            </>
          ) : (
            <div style={{ color: 'var(--color-text-tertiary)', fontSize: '14px' }}>
              {emptyLabel ?? (filterMode === 'unread' ? '읽지 않은 메일이 없습니다' : filterMode === 'starred' ? '별표 메일이 없습니다' : '메일이 없습니다')}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Group messages by date
  const groups: { label: string; messages: MessageSummary[] }[] = [];
  const groupOrder = ['오늘', '어제', '지난 7일', '이번 달'];
  const groupMap = new Map<string, MessageSummary[]>();

  for (const msg of pagedMessages) {
    const group = getDateGroup(msg.received_at);
    if (!groupMap.has(group)) groupMap.set(group, []);
    groupMap.get(group)!.push(msg);
  }

  const groupOrderDisplay = sortAsc ? [...groupOrder].reverse() : groupOrder;

  for (const label of groupOrderDisplay) {
    if (groupMap.has(label)) {
      groups.push({ label, messages: groupMap.get(label)! });
    }
  }

  // Any remaining groups not in order
  for (const [label, msgs] of groupMap) {
    if (!groupOrderDisplay.includes(label)) {
      groups.push({ label, messages: msgs });
    }
  }

  return (
    <div
      data-print="hide"
      style={{
        ...listWidth,
        height: containerHeight,
        ...containerBorder,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {header}
      {isMobile && pullY > 0 && (
        <div aria-hidden="true" style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          height: `${Math.min(pullY, PULL_THRESHOLD + 20)}px`,
          fontSize: '18px',
          color: pullY >= PULL_THRESHOLD ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
          transition: 'color 150ms ease',
          flexShrink: 0,
        }}>
          {pullY >= PULL_THRESHOLD ? '↺' : '↓'}
        </div>
      )}
      <div
        ref={scrollContainerRef}
        role="list"
        aria-label="메일 목록"
        onKeyDownCapture={handleRowKeyDownCapture}
        style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden', overscrollBehavior: 'contain' }}
        onTouchStart={isMobile && onRefresh ? (e) => {
          if (scrollContainerRef.current?.scrollTop === 0) {
            pullRef.current = { startY: e.touches[0].clientY };
          }
        } : undefined}
        onTouchMove={isMobile && onRefresh ? (e) => {
          if (!pullRef.current) return;
          const dy = e.touches[0].clientY - pullRef.current.startY;
          if (dy > 0) setPullY(Math.min(PULL_THRESHOLD + 20, dy));
          else { pullRef.current = null; setPullY(0); }
        } : undefined}
        onTouchEnd={isMobile && onRefresh ? () => {
          if (pullY >= PULL_THRESHOLD && !refreshing) onRefresh!();
          setPullY(0);
          pullRef.current = null;
        } : undefined}
      >
      {groups.map((group) => (
        <div key={group.label} role="group" aria-label={group.label}>
          <div
            aria-hidden="true"
            style={{
              padding: '12px 16px 4px',
              fontSize: '12px',
              color: 'var(--color-text-tertiary)',
              fontWeight: 500,
              position: 'sticky',
              top: 0,
              zIndex: 1,
              background: 'var(--color-bg-primary)',
              backdropFilter: 'blur(8px)',
            }}
          >
            {group.label}
          </div>
          {group.messages.map((msg) => (
            <MessageRow
              key={msg.id}
              message={msg}
              isSelected={selectedId === msg.id}
              isBulkChecked={bulkSelected.has(msg.id)}
              onSelect={onSelect}
              onStar={onStar}
              onToggleBulk={toggleBulk}
              onContextMenu={onContextMenuMessage}
              searchQuery={searchQuery}
              compact={compact}
              onDelete={isMobile ? onDeleteMessage : undefined}
              onArchiveRow={isMobile ? onArchiveMessage : undefined}
              onHoverDelete={!isMobile ? onDeleteMessage : undefined}
              onHoverArchive={!isMobile ? onArchiveMessage : undefined}
              onHoverToggleRead={!isMobile ? onToggleReadMessage : undefined}
              onHoverSnooze={!isMobile ? onSnoozeMessage : undefined}
              onHoverPin={!isMobile ? onPinMessage : undefined}
              isPinned={pinnedIds.has(msg.id)}
              threadCount={msg.message_count ?? threadCounts[msg.id]}
              labelColor={messageLabels[msg.id]}
              userEmail={userEmail}
              showPreview={showPreview}
              hasNote={noteIds.has(msg.id)}
              isImportant={importantIds.has(msg.id)}
              onAvatarEnter={!isMobile ? handleAvatarEnter : undefined}
              onAvatarLeave={!isMobile ? handleAvatarLeave : undefined}
              onHoverChange={setHoveredMessageId}
            />
          ))}
        </div>
      ))}

      <div ref={sentinelRef} style={{ height: '1px' }} aria-hidden="true" />
      {loadingMore && (
        <div style={{ padding: '12px 16px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
          불러오는 중...
        </div>
      )}
      {contactCard && (
        <ContactHoverCard
          {...contactCard}
          onClose={() => { if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current); setContactCard(null); }}
        />
      )}
      </div>
    </div>
  );
}
