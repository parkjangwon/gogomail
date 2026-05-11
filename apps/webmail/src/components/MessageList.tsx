'use client';

import { useState, useRef, useEffect } from 'react';
import { MessageSummary, Folder } from '@/lib/api';

type FilterMode = 'all' | 'unread' | 'starred';

const AVATAR_COLORS = ['#2F6EE0', '#0D9488', '#7C3AED', '#EA580C', '#DB2777', '#059669', '#D97706', '#DC2626'];
function avatarColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) & 0xffffff;
  return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

function highlight(text: string, query: string): React.ReactNode {
  if (!query.trim() || !text) return text;
  const words = query.trim().split(/\s+/).filter(Boolean);
  const pattern = new RegExp(`(${words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
  const parts = text.split(pattern);
  return parts.map((part, i) =>
    pattern.test(part)
      ? <mark key={i} style={{ background: '#fef08a', color: 'inherit', borderRadius: '2px', padding: '0 1px' }}>{part}</mark>
      : part
  );
}

function formatDate(receivedAt: string): string {
  const date = new Date(receivedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) {
    return new Intl.DateTimeFormat('ko-KR', {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(date);
  }
  if (diffDays < 7) {
    return new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(date);
  }
  return new Intl.DateTimeFormat('ko-KR', {
    month: 'numeric',
    day: 'numeric',
  }).format(date);
}

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

interface MessageListProps {
  messages: MessageSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  loading?: boolean;
  emptyLabel?: string;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  onStar?: (id: string, starred: boolean) => void;
  onBulkDelete?: (ids: string[]) => void;
  onDeleteMessage?: (id: string) => void;
  onBulkMarkRead?: (ids: string[]) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  isMobile?: boolean;
  onOpenSidebar?: () => void;
  paneWidth?: number;
  fullWidth?: boolean;
  bottomLayout?: boolean;
  onContextMenuMessage?: (id: string, x: number, y: number) => void;
  onMarkAllRead?: () => void;
  emptyFolderLabel?: string;
  onEmptyFolder?: () => void;
  folders?: Folder[];
  onBulkMove?: (ids: string[], folderId: string) => void;
  searchQuery?: string;
  onBulkRestore?: (ids: string[]) => void;
  onBulkLabel?: (ids: string[], color: string | null) => void;
  messageLabels?: Record<string, string>;
}

export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel, hasMore, loadingMore, onLoadMore, onStar, onBulkDelete, onBulkMarkRead, onRefresh, refreshing, isMobile, onOpenSidebar, onContextMenuMessage, onMarkAllRead, emptyFolderLabel, onEmptyFolder, folders, onBulkMove, paneWidth, fullWidth, bottomLayout, searchQuery, onDeleteMessage, onBulkRestore, onBulkLabel, messageLabels = {} }: MessageListProps) {
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const [sortAsc, setSortAsc] = useState(false);
  const [bulkMoveOpen, setBulkMoveOpen] = useState(false);
  const [compact, setCompact] = useState(() => {
    try { return localStorage.getItem('webmail_compact') === '1'; } catch { return false; }
  });
  const lastBulkIndexRef = useRef<number | null>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const pullRef = useRef<{ startY: number } | null>(null);
  const [pullY, setPullY] = useState(0);
  const PULL_THRESHOLD = 64;
  const [quickFilter, setQuickFilter] = useState('');
  const [showQuickFilter, setShowQuickFilter] = useState(false);
  const quickFilterRef = useRef<HTMLInputElement>(null);

  // Scroll selected message into view when selectedId changes (e.g., j/k keyboard nav)
  useEffect(() => {
    if (!selectedId || !scrollContainerRef.current) return;
    const el = scrollContainerRef.current.querySelector<HTMLElement>(`[data-message-id="${selectedId}"]`);
    el?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }, [selectedId]);

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

  const baseFiltered = filterMode === 'unread'
    ? messages.filter((m) => !m.read)
    : filterMode === 'starred'
    ? messages.filter((m) => m.starred)
    : messages;

  const qf = quickFilter.trim().toLowerCase();
  const afterQuickFilter = qf
    ? baseFiltered.filter((m) =>
        m.from_name?.toLowerCase().includes(qf) ||
        m.from_addr.toLowerCase().includes(qf) ||
        m.subject.toLowerCase().includes(qf) ||
        m.preview?.toLowerCase().includes(qf)
      )
    : baseFiltered;

  const sortedBase = sortAsc
    ? [...afterQuickFilter].sort((a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime())
    : afterQuickFilter;

  const [conversationMode, setConversationMode] = useState(() => {
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

  const listWidth = (isMobile || fullWidth || bottomLayout)
    ? { width: '100%', minWidth: 0 }
    : paneWidth
      ? { width: `${paneWidth}px`, minWidth: `${paneWidth}px` }
      : { width: '380px', minWidth: '380px' };
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

  const filterTabs = hasBulk ? (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, background: 'var(--color-accent-subtle)' }}>
      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>{bulkSelected.size}개 선택됨</span>
      {onBulkMarkRead && (
        <button onClick={() => { onBulkMarkRead([...bulkSelected]); clearAll(); }}
          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          읽음
        </button>
      )}
      {onBulkMove && folders && folders.length > 0 && (
        <div style={{ position: 'relative' }}>
          <button
            onClick={() => setBulkMoveOpen((v) => !v)}
            style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
          >
            이동
          </button>
          {bulkMoveOpen && (
            <div style={{
              position: 'absolute',
              top: '100%',
              left: 0,
              marginTop: '4px',
              background: 'var(--color-bg-primary)',
              border: '1px solid var(--color-border-default)',
              borderRadius: '6px',
              boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
              zIndex: 200,
              minWidth: '140px',
              overflow: 'hidden',
            }}>
              {folders.map((f) => (
                <button
                  key={f.id}
                  onClick={() => { onBulkMove([...bulkSelected], f.id); clearAll(); setBulkMoveOpen(false); }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  {f.name}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
      {onBulkRestore && (
        <button onClick={() => { onBulkRestore([...bulkSelected]); clearAll(); }}
          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          복구
        </button>
      )}
      {onBulkLabel && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          {['#ef4444','#f97316','#eab308','#22c55e','#3b82f6','#a855f7'].map((color) => (
            <button
              key={color}
              title={`라벨 지정`}
              onClick={() => { onBulkLabel([...bulkSelected], color); clearAll(); }}
              style={{ width: '14px', height: '14px', borderRadius: '50%', background: color, border: 'none', cursor: 'pointer', flexShrink: 0 }}
            />
          ))}
          <button
            title="라벨 제거"
            onClick={() => { onBulkLabel([...bulkSelected], null); clearAll(); }}
            style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
          >✕</button>
        </div>
      )}
      {onBulkDelete && (
        <button onClick={() => { onBulkDelete([...bulkSelected]); clearAll(); }}
          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}>
          삭제
        </button>
      )}
      <button onClick={clearAll}
        style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
        취소
      </button>
    </div>
  ) : (
    <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
      {isMobile && onOpenSidebar && (
        <button
          aria-label="메뉴 열기"
          onClick={onOpenSidebar}
          style={{ fontSize: '16px', padding: '3px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', marginRight: '4px', lineHeight: 1 }}
        >☰</button>
      )}
      <button
        aria-label="전체 선택"
        onClick={selectAll}
        style={{ fontSize: '13px', padding: '3px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', marginRight: '4px' }}
        title="전체 선택 (Ctrl+A)"
      >☐</button>
      {(['all', 'unread', 'starred'] as FilterMode[]).map((mode) => {
        const label = mode === 'all' ? '전체' : mode === 'unread' ? '안읽음' : '별표';
        const active = filterMode === mode;
        return (
          <button
            key={mode}
            onClick={() => setFilterMode(mode)}
            style={{
              padding: '3px 10px',
              borderRadius: '12px',
              border: active ? 'none' : '1px solid var(--color-border-default)',
              background: active ? 'var(--color-accent)' : 'transparent',
              color: active ? '#fff' : 'var(--color-text-secondary)',
              fontSize: '12px',
              fontWeight: active ? 500 : 400,
              cursor: 'pointer',
              transition: 'all 100ms ease',
            }}
          >
            {label}
          </button>
        );
      })}
      <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', padding: '0 4px', whiteSpace: 'nowrap' }}>
        {filterMode !== 'all' ? `${filteredMessages.length} / ${messages.length}개` : `${filteredMessages.length}개`}
      </span>
      {filteredMessages.some((m) => !m.read) && (
        <button
          aria-label="읽지 않은 메일 선택"
          onClick={() => setBulkSelected(new Set(filteredMessages.filter((m) => !m.read).map((m) => m.id)))}
          title="안읽음 선택"
          style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer' }}
        >
          안읽음 선택
        </button>
      )}
      {emptyFolderLabel && onEmptyFolder && messages.length > 0 && (
        <button
          aria-label={emptyFolderLabel}
          onClick={onEmptyFolder}
          title={emptyFolderLabel}
          style={{
            marginLeft: '4px',
            padding: '3px 8px',
            borderRadius: '4px',
            border: '1px solid rgba(217,79,61,0.4)',
            background: 'transparent',
            color: 'var(--color-destructive)',
            cursor: 'pointer',
            fontSize: '12px',
          }}
        >
          {emptyFolderLabel}
        </button>
      )}
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '4px' }}>
        {showQuickFilter && (
          <input
            ref={quickFilterRef}
            type="text"
            value={quickFilter}
            onChange={(e) => setQuickFilter(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Escape') { setQuickFilter(''); setShowQuickFilter(false); } }}
            placeholder="필터..."
            aria-label="빠른 필터"
            style={{ fontSize: '12px', padding: '3px 8px', borderRadius: '4px', border: '1px solid var(--color-accent)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none', width: '120px' }}
          />
        )}
        <button
          aria-label="빠른 필터 열기"
          title="빠른 필터 (/)  "
          onClick={() => { const next = !showQuickFilter; setShowQuickFilter(next); if (!next) { setQuickFilter(''); } else { setTimeout(() => quickFilterRef.current?.focus(), 50); } }}
          style={{ padding: '3px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: showQuickFilter ? 'var(--color-accent-subtle)' : 'transparent', color: showQuickFilter ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', fontSize: '12px' }}
        >
          {quickFilter ? `🔍 ${messages.length - filteredMessages.length > 0 ? `${filteredMessages.length}/${messages.length}` : ''}` : '🔍'}
        </button>
        <button
          aria-label={conversationMode ? '대화 보기 해제' : '대화 보기'}
          title={conversationMode ? '대화 보기 해제' : '대화 보기'}
          onClick={() => { const next = !conversationMode; setConversationMode(next); try { localStorage.setItem('webmail_conv_mode', next ? '1' : '0'); } catch { /* */ } }}
          style={{ padding: '3px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: conversationMode ? 'var(--color-accent-subtle)' : 'transparent', color: conversationMode ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', fontSize: '12px' }}
        >대화</button>
        <button
          aria-label={compact ? '넓은 보기' : '촘촘한 보기'}
          title={compact ? '넓은 보기' : '촘촘한 보기'}
          onClick={() => { const next = !compact; setCompact(next); try { localStorage.setItem('webmail_compact', next ? '1' : '0'); } catch { /* */ } }}
          style={{ padding: '3px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: compact ? 'var(--color-accent-subtle)' : 'transparent', color: compact ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', fontSize: '12px' }}
        >
          {compact ? '≡' : '☰'}
        </button>
        <button
          aria-label={sortAsc ? '오래된순 정렬 중' : '최신순 정렬 중'}
          title={sortAsc ? '오래된순 — 클릭하면 최신순' : '최신순 — 클릭하면 오래된순'}
          onClick={() => setSortAsc((v) => !v)}
          style={{
            padding: '3px 8px',
            borderRadius: '4px',
            border: '1px solid var(--color-border-default)',
            background: sortAsc ? 'var(--color-accent-subtle)' : 'transparent',
            color: sortAsc ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
            cursor: 'pointer',
            fontSize: '12px',
          }}
        >
          {sortAsc ? '↑ 오래된순' : '↓ 최신순'}
        </button>
        {onMarkAllRead && messages.some((m) => !m.read) && (
          <button
            aria-label="모두 읽음으로 표시"
            onClick={onMarkAllRead}
            title="모두 읽음"
            style={{
              padding: '3px 8px',
              borderRadius: '4px',
              border: '1px solid var(--color-border-default)',
              background: 'transparent',
              color: 'var(--color-text-tertiary)',
              cursor: 'pointer',
              fontSize: '12px',
            }}
          >
            모두 읽음
          </button>
        )}
        {onRefresh && (
          <button
            aria-label="새로고침"
            onClick={onRefresh}
            disabled={refreshing}
            title="새로고침"
            style={{
              padding: '3px 8px',
              borderRadius: '4px',
              border: '1px solid var(--color-border-default)',
              background: 'transparent',
              color: 'var(--color-text-tertiary)',
              cursor: refreshing ? 'not-allowed' : 'pointer',
              fontSize: '13px',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            <span style={{ display: 'inline-block', animation: refreshing ? 'spin 1s linear infinite' : 'none' }}>↻</span>
          </button>
        )}
      </div>
    </div>
  );

  if (filteredMessages.length === 0) {
    return (
      <div data-print="hide" style={{ ...listWidth, height: containerHeight, ...containerBorder, display: 'flex', flexDirection: 'column' }}>
        {filterTabs}
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-tertiary)', fontSize: '14px' }}>
          {emptyLabel ?? (filterMode === 'unread' ? '읽지 않은 메일이 없습니다' : filterMode === 'starred' ? '별표 메일이 없습니다' : '메일이 없습니다')}
        </div>
      </div>
    );
  }

  // Group messages by date
  const groups: { label: string; messages: MessageSummary[] }[] = [];
  const groupOrder = ['오늘', '어제', '지난 7일', '이번 달'];
  const groupMap = new Map<string, MessageSummary[]>();

  for (const msg of filteredMessages) {
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
      {filterTabs}
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
              onHoverDelete={!isMobile ? onDeleteMessage : undefined}
              threadCount={threadCounts[msg.id]}
              labelColor={messageLabels[msg.id]}
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
      </div>
    </div>
  );
}

interface MessageRowProps {
  message: MessageSummary;
  isSelected: boolean;
  isBulkChecked: boolean;
  onSelect: (id: string) => void;
  onStar?: (id: string, starred: boolean) => void;
  onToggleBulk: (id: string, shiftKey?: boolean) => void;
  onContextMenu?: (id: string, x: number, y: number) => void;
  searchQuery?: string;
  compact?: boolean;
  onDelete?: (id: string) => void;
  onHoverDelete?: (id: string) => void;
  threadCount?: number;
  labelColor?: string;
}

function MessageRow({ message, isSelected, isBulkChecked, onSelect, onStar, onToggleBulk, onContextMenu, searchQuery, compact, onDelete, onHoverDelete, threadCount, labelColor }: MessageRowProps) {
  const q = searchQuery ?? '';
  const isUnread = !message.read;
  const swipeRef = useRef<{ startX: number; startY: number } | null>(null);
  const [swipeX, setSwipeX] = useState(0);
  const [hovered, setHovered] = useState(false);

  return (
    <div
      role="listitem"
      data-message-id={message.id}
      style={{ position: 'relative', overflow: 'hidden', borderLeft: labelColor ? `3px solid ${labelColor}` : '3px solid transparent' }}
    >
      {onDelete && swipeX < -20 && (
        <div aria-hidden="true" style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: Math.min(120, -swipeX), background: 'var(--color-destructive)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '13px', fontWeight: 600, pointerEvents: 'none' }}>
          {-swipeX > 70 ? '삭제' : '←'}
        </div>
      )}
      <div
      draggable={!onDelete}
      onDragStart={!onDelete ? (e) => { e.dataTransfer.setData('text/plain', message.id); e.dataTransfer.effectAllowed = 'move'; } : undefined}
      onTouchStart={onDelete ? (e) => { swipeRef.current = { startX: e.touches[0].clientX, startY: e.touches[0].clientY }; } : undefined}
      onTouchMove={onDelete ? (e) => {
        if (!swipeRef.current) return;
        const dx = e.touches[0].clientX - swipeRef.current.startX;
        const dy = e.touches[0].clientY - swipeRef.current.startY;
        if (Math.abs(dy) > Math.abs(dx)) { swipeRef.current = null; return; }
        e.preventDefault();
        setSwipeX(Math.max(-120, Math.min(0, dx)));
      } : undefined}
      onTouchEnd={onDelete ? () => {
        if (swipeX < -70) { onDelete(message.id); }
        setSwipeX(0);
        swipeRef.current = null;
      } : undefined}
      onClick={() => { if (swipeX !== 0) { setSwipeX(0); return; } onSelect(message.id); }}
      onContextMenu={onContextMenu ? (e) => { e.preventDefault(); onContextMenu(message.id, e.clientX, e.clientY); } : undefined}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(message.id);
        }
      }}
      tabIndex={0}
      aria-selected={isSelected}
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: '8px',
        padding: compact ? '5px 16px' : '12px 16px',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: isSelected ? 'var(--color-accent-subtle)' : 'var(--color-bg-primary)',
        cursor: 'pointer',
        transition: 'background 100ms ease, transform 80ms ease',
        position: 'relative',
        transform: `translateX(${swipeX}px)`,
      }}
      onMouseEnter={(e) => {
        setHovered(true);
        if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)';
      }}
      onMouseLeave={(e) => {
        setHovered(false);
        if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)';
      }}
    >
      {/* Hover quick-delete overlay (desktop only) */}
      {hovered && onHoverDelete && !compact && !isBulkChecked && (
        <button
          aria-label="삭제"
          title="삭제"
          onClick={(e) => { e.stopPropagation(); onHoverDelete(message.id); }}
          style={{
            position: 'absolute', right: '8px', top: '50%', transform: 'translateY(-50%)',
            background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-default)',
            borderRadius: '6px', padding: '3px 7px', cursor: 'pointer',
            fontSize: '14px', color: 'var(--color-text-secondary)', zIndex: 2,
            lineHeight: 1,
          }}
        >🗑</button>
      )}
      {/* Checkbox / unread dot — click to toggle bulk selection */}
      <div
        onClick={(e) => { e.stopPropagation(); onToggleBulk(message.id, e.shiftKey); }}
        title={isBulkChecked ? '선택 해제' : '선택'}
        style={{ width: '16px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', marginTop: '3px', cursor: 'pointer' }}
      >
        {isBulkChecked ? (
          <input
            type="checkbox"
            checked
            readOnly
            aria-label="선택됨"
            style={{ cursor: 'pointer', accentColor: 'var(--color-accent)', pointerEvents: 'none' }}
          />
        ) : (
          <div
            aria-hidden="true"
            style={{
              width: '6px',
              height: '6px',
              borderRadius: '50%',
              background: isUnread ? 'var(--color-accent)' : 'transparent',
            }}
          />
        )}
      </div>

      {/* Sender avatar */}
      {!compact && (
        <div aria-hidden="true" style={{
          width: '32px', height: '32px', borderRadius: '50%', flexShrink: 0,
          background: avatarColor(message.from_name || message.from_addr),
          color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '13px', fontWeight: 600, userSelect: 'none',
        }}>
          {(message.from_name || message.from_addr).charAt(0).toUpperCase()}
        </div>
      )}

      {/* Content */}
      <div style={{ flex: 1, minWidth: 0 }}>
        {/* Row 1: sender + date + icons */}
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: '8px', marginBottom: '3px' }}>
          <span
            style={{
              fontSize: '14px',
              fontWeight: isUnread ? 500 : 400,
              color: 'var(--color-text-primary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              flex: 1,
              minWidth: 0,
            }}
          >
            {highlight(message.from_name || message.from_addr, q)}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
            {message.has_attachment && (
              <span aria-label="첨부파일 있음" title="첨부파일" style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>📎</span>
            )}
            <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
              {formatDate(message.received_at)}
            </span>
          </div>
        </div>

        {/* Row 2: subject + preview + star */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div
            style={{
              flex: 1,
              fontSize: '14px',
              color: 'var(--color-text-primary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            <span style={{ fontWeight: isUnread ? 600 : 400 }}>
              {highlight(message.subject || '(제목 없음)', q)}
            </span>
            {threadCount && threadCount > 1 && (
              <span aria-label={`${threadCount}개 메시지`} style={{ marginLeft: '5px', fontSize: '11px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-tertiary)', borderRadius: '10px', padding: '1px 6px', verticalAlign: 'middle', fontWeight: 500 }}>{threadCount}</span>
            )}
            {!compact && message.preview && (
              <span style={{ color: 'var(--color-text-secondary)', fontWeight: 400 }}>
                {' · '}{highlight(message.preview, q)}
              </span>
            )}
          </div>
          {onStar && (
            <button
              aria-label={message.starred ? '별표 해제' : '별표 추가'}
              onClick={(e) => { e.stopPropagation(); onStar(message.id, !message.starred); }}
              style={{
                flexShrink: 0,
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                padding: '2px',
                fontSize: '14px',
                color: message.starred ? '#f59e0b' : 'var(--color-text-tertiary)',
                opacity: message.starred ? 1 : 0.4,
                lineHeight: 1,
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.opacity = '1'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.opacity = message.starred ? '1' : '0.4'; }}
            >
              ★
            </button>
          )}
        </div>
      </div>
      </div>
    </div>
  );
}
