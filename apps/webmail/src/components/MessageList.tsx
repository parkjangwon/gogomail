'use client';

import { useState, useRef, useEffect } from 'react';
import { MessageSummary, Folder } from '@/lib/api';
import {
  StarIcon,
  EnvelopeIcon,
  EnvelopeOpenIcon,
  ArchiveBoxIcon,
  TrashIcon,
  PaperClipIcon,
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
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';

type FilterMode = 'all' | 'unread' | 'read' | 'starred' | 'unstarred' | 'attachment' | 'noattachment';

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
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return '방금 전';
  if (diffMins < 60) return `${diffMins}분 전`;
  if (diffHours < 12 && date.getDate() === now.getDate()) return `${diffHours}시간 전`;
  if (diffDays === 0) {
    return new Intl.DateTimeFormat('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false }).format(date);
  }
  if (diffDays < 7) {
    return new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(date);
  }
  if (date.getFullYear() === now.getFullYear()) {
    return new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric' }).format(date);
  }
  return new Intl.DateTimeFormat('ko-KR', { year: 'numeric', month: 'numeric', day: 'numeric' }).format(date);
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
  onBulkStar?: (ids: string[], starred: boolean) => void;
  onArchiveMessage?: (id: string) => void;
  onToggleReadMessage?: (id: string, read: boolean) => void;
  messageLabels?: Record<string, string>;
  userEmail?: string;
  showPreview?: boolean;
}

export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel, hasMore, loadingMore, onLoadMore, onStar, onBulkDelete, onBulkMarkRead, onRefresh, refreshing, isMobile, onOpenSidebar, onContextMenuMessage, onMarkAllRead, emptyFolderLabel, onEmptyFolder, folders, onBulkMove, paneWidth, fullWidth, bottomLayout, searchQuery, onDeleteMessage, onBulkRestore, onBulkLabel, onBulkStar, onArchiveMessage, onToggleReadMessage, messageLabels = {}, userEmail, showPreview = true }: MessageListProps) {
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [filterLabel, setFilterLabel] = useState<string | null>(null);
  const [showFilterDropdown, setShowFilterDropdown] = useState(false);
  const filterDropdownRef = useRef<HTMLDivElement>(null);
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const [sortAsc, setSortAsc] = useState(false);
  const [bulkMoveOpen, setBulkMoveOpen] = useState(false);
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
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'x') return;
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement).isContentEditable) return;
      const id = selectedIdRef.current;
      if (!id) return;
      setBulkSelected((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id); else next.add(id);
        return next;
      });
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

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

  const sortedBase = sortAsc
    ? [...afterLabelFilter].sort((a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime())
    : afterLabelFilter;

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

  const filterTabs = hasBulk ? (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, background: 'var(--color-accent-subtle)' }}>
      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>{bulkSelected.size}개 선택됨</span>
      {onBulkMarkRead && (
        <button onClick={() => { onBulkMarkRead([...bulkSelected]); clearAll(); }}
          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          읽음
        </button>
      )}
      {onBulkStar && (
        <>
          <button onClick={() => { onBulkStar([...bulkSelected], true); clearAll(); }}
            title="별표 추가" style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: '#f59e0b', display: 'inline-flex', alignItems: 'center' }}>
            <StarIconSolid style={{ width: '13px', height: '13px' }} />
          </button>
          <button onClick={() => { onBulkStar([...bulkSelected], false); clearAll(); }}
            title="별표 제거" style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', alignItems: 'center' }}>
            <StarIcon style={{ width: '13px', height: '13px' }} />
          </button>
        </>
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
            style={{ padding: '3px 6px', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
          ><XMarkIcon style={{ width: '11px', height: '11px' }} /></button>
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
          style={{ padding: '3px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', marginRight: '4px', display: 'inline-flex', alignItems: 'center' }}
        ><Bars3Icon style={{ width: '18px', height: '18px' }} /></button>
      )}
      {/* Gmail-style checkbox + filter dropdown */}
      <div ref={filterDropdownRef} style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginRight: '4px', flexShrink: 0 }}>
        {/* Checkbox area */}
        <button
          aria-label="전체 선택"
          onClick={() => { bulkSelected.size === filteredMessages.length && filteredMessages.length > 0 ? clearAll() : selectAll(); }}
          title="전체 선택/해제 (Ctrl+A)"
          style={{ padding: '4px 5px', border: '1px solid var(--color-border-default)', borderRight: 'none', borderRadius: '4px 0 0 4px', background: 'transparent', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-tertiary)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <div style={{
            width: '14px', height: '14px', borderRadius: '2px',
            border: `1.5px solid ${bulkSelected.size > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)'}`,
            background: bulkSelected.size === filteredMessages.length && filteredMessages.length > 0 ? 'var(--color-accent)' : 'transparent',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            position: 'relative',
          }}>
            {bulkSelected.size > 0 && bulkSelected.size < filteredMessages.length && (
              <div style={{ width: '8px', height: '1.5px', background: 'var(--color-accent)', borderRadius: '1px' }} />
            )}
            {bulkSelected.size === filteredMessages.length && filteredMessages.length > 0 && (
              <CheckIconOutline style={{ width: '10px', height: '10px', color: '#fff' }} />
            )}
          </div>
        </button>
        {/* Dropdown arrow */}
        <button
          aria-label="필터 선택"
          onClick={() => setShowFilterDropdown((v) => !v)}
          title="필터"
          style={{ padding: '4px 4px', border: '1px solid var(--color-border-default)', borderRadius: '0 4px 4px 0', background: showFilterDropdown ? 'var(--color-bg-tertiary)' : 'transparent', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', color: 'var(--color-text-tertiary)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { if (!showFilterDropdown) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <ChevronDownIcon style={{ width: '14px', height: '14px' }} />
        </button>
        {/* Filter dropdown menu */}
        {showFilterDropdown && (
          <div
            style={{
              position: 'absolute', top: 'calc(100% + 4px)', left: 0, zIndex: 200,
              background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)',
              borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', minWidth: '160px', padding: '4px 0',
            }}
            onMouseLeave={() => setShowFilterDropdown(false)}
          >
            {([
              { mode: 'all' as FilterMode, label: '전체' },
              { mode: 'unread' as FilterMode, label: '읽지 않음' },
              { mode: 'read' as FilterMode, label: '읽음' },
              { mode: 'starred' as FilterMode, label: '별표' },
              { mode: 'unstarred' as FilterMode, label: '별표 없음' },
              { mode: 'attachment' as FilterMode, label: '첨부 파일 있음' },
              { mode: 'noattachment' as FilterMode, label: '첨부 파일 없음' },
            ]).map(({ mode, label }) => (
              <button
                key={mode}
                onClick={() => { setFilterMode(mode); setShowFilterDropdown(false); }}
                style={{
                  display: 'flex', alignItems: 'center', gap: '8px',
                  width: '100%', padding: '8px 14px', border: 'none',
                  background: 'transparent', color: 'var(--color-text-primary)',
                  fontSize: '13px', cursor: 'pointer', textAlign: 'left',
                }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >
                <span style={{ width: '14px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  {filterMode === mode && <CheckIconOutline style={{ width: '13px', height: '13px', color: 'var(--color-accent)' }} />}
                </span>
                {label}
              </button>
            ))}
          </div>
        )}
      </div>
      {onRefresh && (
        <button
          aria-label="새로고침"
          onClick={onRefresh}
          disabled={refreshing}
          title="새로고침"
          style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: refreshing ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center' }}
        >
          <ArrowPathIcon style={{ width: '16px', height: '16px', animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
        </button>
      )}
      <div ref={moreMenuRef} style={{ position: 'relative' }}>
        <button
          aria-label="더 보기"
          onClick={() => setShowMoreMenu((v) => !v)}
          style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: showMoreMenu ? 'var(--color-bg-tertiary)' : 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
        >
          <EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} />
        </button>
        {showMoreMenu && (
          <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '2px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '180px', overflow: 'hidden', padding: '4px 0' }}>
            <button
              onClick={() => { toggleCompact(); setShowMoreMenu(false); }}
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
            >
              <span>컴팩트 보기</span>
              <span style={{ width: '28px', height: '16px', borderRadius: '8px', background: compact ? 'var(--color-accent)' : 'var(--color-border-default)', display: 'inline-flex', alignItems: 'center', transition: 'background 150ms ease', flexShrink: 0 }}>
                <span style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#fff', marginLeft: compact ? '14px' : '2px', transition: 'margin-left 150ms ease', display: 'block', boxShadow: '0 1px 3px rgba(0,0,0,0.2)' }} />
              </span>
            </button>
            {onMarkAllRead && messages.some((m) => !m.read) && (
              <button
                onClick={() => { onMarkAllRead(); setShowMoreMenu(false); }}
                style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >모두 읽음으로 표시</button>
            )}
            {emptyFolderLabel && onEmptyFolder && messages.length > 0 && (
              <button
                onClick={() => { onEmptyFolder(); setShowMoreMenu(false); }}
                style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >{emptyFolderLabel}</button>
            )}
          </div>
        )}
      </div>
      {filterMode !== 'all' && (
        <span style={{ fontSize: '11px', padding: '2px 8px', borderRadius: '10px', background: 'var(--color-accent-subtle)', color: 'var(--color-accent)', fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
          {filterMode === 'unread' ? '읽지 않음' : filterMode === 'read' ? '읽음' : filterMode === 'starred' ? '별표' : filterMode === 'unstarred' ? '별표 없음' : filterMode === 'attachment' ? '첨부 있음' : '첨부 없음'}
          <button onClick={() => setFilterMode('all')} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, display: 'inline-flex', color: 'var(--color-accent)' }}><XMarkIcon style={{ width: '11px', height: '11px' }} /></button>
        </span>
      )}
      {/* Label color filter dots */}
      {activeLabelColors.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginLeft: '2px' }}>
          {activeLabelColors.map((color) => (
            <button
              key={color}
              title={filterLabel === color ? '라벨 필터 해제' : '이 라벨로 필터'}
              onClick={() => setFilterLabel(filterLabel === color ? null : color)}
              style={{
                width: '12px', height: '12px', borderRadius: '50%', background: color,
                border: filterLabel === color ? '2px solid var(--color-text-primary)' : '2px solid transparent',
                cursor: 'pointer', flexShrink: 0, padding: 0,
                boxShadow: filterLabel === color ? '0 0 0 1px ' + color : 'none',
                transition: 'border-color 100ms ease',
              }}
            />
          ))}
        </div>
      )}
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '2px' }}>
        <button
          aria-label={sortAsc ? '최신순으로 정렬' : '오래된순으로 정렬'}
          title={sortAsc ? '오래된순 (클릭: 최신순)' : '최신순 (클릭: 오래된순)'}
          onClick={() => setSortAsc((v) => !v)}
          style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)'; (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          {sortAsc ? <BarsArrowUpIcon style={{ width: '15px', height: '15px' }} /> : <BarsArrowDownIcon style={{ width: '15px', height: '15px' }} />}
        </button>
        {filteredMessages.length > 0 && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '0 4px', whiteSpace: 'nowrap' }}>
            {`${pageStart + 1}–${Math.min(pageEnd, filteredMessages.length)}`}{hasMore ? '+' : ` / ${filteredMessages.length}`}
          </span>
        )}
        <button
          aria-label="이전 페이지"
          onClick={() => setPage((p) => Math.max(0, p - 1))}
          disabled={page === 0}
          style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: page === 0 ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center', opacity: page === 0 ? 0.35 : 1 }}
        >
          <ChevronLeftIcon style={{ width: '16px', height: '16px' }} />
        </button>
        <button
          aria-label="다음 페이지"
          onClick={() => {
            const next = page + 1;
            if (next * PAGE_SIZE >= filteredMessages.length && hasMore && onLoadMore) onLoadMore();
            if (next * PAGE_SIZE < filteredMessages.length || hasMore) setPage(next);
          }}
          disabled={!hasMore && (page + 1) * PAGE_SIZE >= filteredMessages.length}
          style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: (!hasMore && (page + 1) * PAGE_SIZE >= filteredMessages.length) ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center', opacity: (!hasMore && (page + 1) * PAGE_SIZE >= filteredMessages.length) ? 0.35 : 1 }}
        >
          <ChevronRightIcon style={{ width: '16px', height: '16px' }} />
        </button>
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
              onArchiveRow={isMobile ? onArchiveMessage : undefined}
              onHoverDelete={!isMobile ? onDeleteMessage : undefined}
              onHoverArchive={!isMobile ? onArchiveMessage : undefined}
              onHoverToggleRead={!isMobile ? onToggleReadMessage : undefined}
              threadCount={msg.message_count ?? threadCounts[msg.id]}
              labelColor={messageLabels[msg.id]}
              userEmail={userEmail}
              showPreview={showPreview}
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
  onArchiveRow?: (id: string) => void;
  onHoverDelete?: (id: string) => void;
  onHoverArchive?: (id: string) => void;
  onHoverToggleRead?: (id: string, read: boolean) => void;
  threadCount?: number;
  labelColor?: string;
  userEmail?: string;
  showPreview?: boolean;
}

function MessageRow({ message, isSelected, isBulkChecked, onSelect, onStar, onToggleBulk, onContextMenu, searchQuery, compact, onDelete, onArchiveRow, onHoverDelete, onHoverArchive, onHoverToggleRead, threadCount, labelColor, userEmail, showPreview = true }: MessageRowProps) {
  const q = searchQuery ?? '';
  const isUnread = !message.read;
  const swipeRef = useRef<{ startX: number; startY: number } | null>(null);
  const [swipeX, setSwipeX] = useState(0);
  const [hovered, setHovered] = useState(false);
  const swipeEnabled = onDelete || onArchiveRow;

  return (
    <div
      role="listitem"
      data-message-id={message.id}
      style={{ position: 'relative', overflow: 'hidden', borderLeft: labelColor ? `3px solid ${labelColor}` : '3px solid transparent' }}
    >
      {onArchiveRow && swipeX > 20 && (
        <div aria-hidden="true" style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: Math.min(120, swipeX), background: 'var(--color-accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '13px', fontWeight: 600, pointerEvents: 'none' }}>
          {swipeX > 70 ? '아카이브' : '→'}
        </div>
      )}
      {onDelete && swipeX < -20 && (
        <div aria-hidden="true" style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: Math.min(120, -swipeX), background: 'var(--color-destructive)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '13px', fontWeight: 600, pointerEvents: 'none' }}>
          {-swipeX > 70 ? '삭제' : '←'}
        </div>
      )}
      <div
      draggable={!swipeEnabled}
      onDragStart={!swipeEnabled ? (e) => { e.dataTransfer.setData('text/plain', message.id); e.dataTransfer.effectAllowed = 'move'; } : undefined}
      onTouchStart={swipeEnabled ? (e) => { swipeRef.current = { startX: e.touches[0].clientX, startY: e.touches[0].clientY }; } : undefined}
      onTouchMove={swipeEnabled ? (e) => {
        if (!swipeRef.current) return;
        const dx = e.touches[0].clientX - swipeRef.current.startX;
        const dy = e.touches[0].clientY - swipeRef.current.startY;
        if (Math.abs(dy) > Math.abs(dx)) { swipeRef.current = null; return; }
        e.preventDefault();
        const minX = onDelete ? -120 : 0;
        const maxX = onArchiveRow ? 120 : 0;
        setSwipeX(Math.max(minX, Math.min(maxX, dx)));
      } : undefined}
      onTouchEnd={swipeEnabled ? () => {
        if (swipeX < -70 && onDelete) onDelete(message.id);
        else if (swipeX > 70 && onArchiveRow) onArchiveRow(message.id);
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
        alignItems: 'center',
        gap: '8px',
        padding: compact ? '4px 16px' : '9px 16px',
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
      {/* Unread dot / checkbox */}
      <div
        onClick={(e) => { e.stopPropagation(); onToggleBulk(message.id, e.shiftKey); }}
        title={isBulkChecked ? '선택 해제' : '선택'}
        style={{ width: '16px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', alignSelf: 'center' }}
      >
        {isBulkChecked ? (
          <input type="checkbox" checked readOnly aria-label="선택됨" style={{ cursor: 'pointer', accentColor: 'var(--color-accent)', pointerEvents: 'none' }} />
        ) : (
          <div aria-hidden="true" style={{ width: '6px', height: '6px', borderRadius: '50%', background: isUnread ? 'var(--color-accent)' : 'transparent' }} />
        )}
      </div>

      {/* Sender avatar */}
      {!compact && (
        <div aria-hidden="true" style={{ width: '32px', height: '32px', borderRadius: '50%', flexShrink: 0, background: avatarColor(message.from_name || message.from_addr), color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '13px', fontWeight: 600, userSelect: 'none', alignSelf: 'center', overflow: 'hidden' }}>
          {(() => {
            if (userEmail && message.from_addr === userEmail) {
              try { const av = localStorage.getItem('webmail_avatar'); if (av) return <img src={av} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />; } catch { /* */ }
            }
            return (message.from_name || message.from_addr).charAt(0).toUpperCase();
          })()}
        </div>
      )}

      {/* Sender name + email */}
      <div style={{ width: compact ? '100px' : '130px', flexShrink: 0, minWidth: 0, alignSelf: 'center' }}>
        <div style={{ fontSize: '13px', fontWeight: isUnread ? 600 : 400, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {highlight(message.from_name || message.from_addr, q)}
        </div>
        {!compact && message.from_name && (
          <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginTop: '1px' }}>
            {message.from_addr}
          </div>
        )}
      </div>

      {/* Attachment slot — fixed width so subject aligns consistently */}
      <div style={{ width: '16px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', alignSelf: 'center' }}>
        {message.has_attachment && (
          <PaperClipIcon aria-label="첨부파일" style={{ width: '13px', height: '13px', color: 'var(--color-text-tertiary)' }} />
        )}
      </div>

      {/* Subject + preview */}
      <div style={{ flex: 1, minWidth: 0, overflow: 'hidden', alignSelf: 'center' }}>
        <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: '13px' }}>
          <span style={{ fontWeight: isUnread ? 600 : 400, color: 'var(--color-text-primary)' }}>
            {highlight(message.subject || '(제목 없음)', q)}
          </span>
          {threadCount && threadCount > 1 && (
            <span aria-label={`${threadCount}개 메시지`} style={{ marginLeft: '5px', fontSize: '11px', color: (message.unread_count ?? 0) > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)', background: (message.unread_count ?? 0) > 0 ? 'var(--color-accent-subtle)' : 'var(--color-bg-tertiary)', borderRadius: '10px', padding: '1px 6px', verticalAlign: 'middle', fontWeight: 500 }}>{threadCount}</span>
          )}
          {showPreview && message.preview && (
            <span style={{ color: 'var(--color-text-secondary)', fontWeight: 400 }}>
              {' · '}{highlight(message.preview, q)}
            </span>
          )}
        </div>
      </div>

      {/* Right: date normally, hover-actions on hover */}
      <div style={{ width: '120px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '1px', alignSelf: 'center' }}>
        {hovered ? (
          <>
            {onStar && (
              <button
                aria-label={message.starred ? '별표 해제' : '별표 추가'}
                title={message.starred ? '별표 해제' : '별표 추가'}
                onClick={(e) => { e.stopPropagation(); onStar(message.id, !message.starred); }}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '5px 4px', color: message.starred ? '#f59e0b' : 'var(--color-text-tertiary)', borderRadius: '4px', display: 'inline-flex', alignItems: 'center' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
              >{message.starred ? <StarIconSolid style={{ width: '14px', height: '14px' }} /> : <StarIcon style={{ width: '14px', height: '14px' }} />}</button>
            )}
            {onHoverToggleRead && (
              <button
                aria-label={message.read ? '읽지 않음으로' : '읽음으로'}
                title={message.read ? '읽지 않음으로' : '읽음으로'}
                onClick={(e) => { e.stopPropagation(); onHoverToggleRead(message.id, !message.read); }}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '5px 4px', color: 'var(--color-text-tertiary)', borderRadius: '4px', display: 'inline-flex', alignItems: 'center' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
              >{message.read ? <EnvelopeOpenIcon style={{ width: '14px', height: '14px' }} /> : <EnvelopeIcon style={{ width: '14px', height: '14px' }} />}</button>
            )}
            {onHoverArchive && (
              <button
                aria-label="아카이브"
                title="아카이브"
                onClick={(e) => { e.stopPropagation(); onHoverArchive(message.id); }}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '5px 4px', color: 'var(--color-text-tertiary)', borderRadius: '4px', display: 'inline-flex', alignItems: 'center' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
              ><ArchiveBoxIcon style={{ width: '14px', height: '14px' }} /></button>
            )}
            {onHoverDelete && (
              <button
                aria-label="삭제"
                title="삭제"
                onClick={(e) => { e.stopPropagation(); onHoverDelete(message.id); }}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '5px 4px', color: 'var(--color-text-tertiary)', borderRadius: '4px', display: 'inline-flex', alignItems: 'center' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
              ><TrashIcon style={{ width: '14px', height: '14px' }} /></button>
            )}
          </>
        ) : (
          <>
            {message.starred && <StarIconSolid style={{ width: '12px', height: '12px', color: '#f59e0b', marginRight: '2px', flexShrink: 0 }} />}
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}
              title={new Intl.DateTimeFormat('ko-KR', { dateStyle: 'full', timeStyle: 'short' }).format(new Date(message.received_at))}>
              {formatDate(message.received_at)}
            </span>
          </>
        )}
      </div>
      </div>
    </div>
  );
}
