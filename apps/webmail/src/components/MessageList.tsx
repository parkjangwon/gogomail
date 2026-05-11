'use client';

import { useState, useRef, useEffect } from 'react';
import { MessageSummary, Folder } from '@/lib/api';

type FilterMode = 'all' | 'unread' | 'starred';

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
  onBulkMarkRead?: (ids: string[]) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  isMobile?: boolean;
  onOpenSidebar?: () => void;
  paneWidth?: number;
  onContextMenuMessage?: (id: string, x: number, y: number) => void;
  onMarkAllRead?: () => void;
  emptyFolderLabel?: string;
  onEmptyFolder?: () => void;
  folders?: Folder[];
  onBulkMove?: (ids: string[], folderId: string) => void;
  searchQuery?: string;
}

export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel, hasMore, loadingMore, onLoadMore, onStar, onBulkDelete, onBulkMarkRead, onRefresh, refreshing, isMobile, onOpenSidebar, onContextMenuMessage, onMarkAllRead, emptyFolderLabel, onEmptyFolder, folders, onBulkMove, paneWidth, searchQuery }: MessageListProps) {
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const [sortAsc, setSortAsc] = useState(false);
  const [bulkMoveOpen, setBulkMoveOpen] = useState(false);
  const lastBulkIndexRef = useRef<number | null>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

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

  const baseFiltered = filterMode === 'unread'
    ? messages.filter((m) => !m.read)
    : filterMode === 'starred'
    ? messages.filter((m) => m.starred)
    : messages;

  const filteredMessages = sortAsc
    ? [...baseFiltered].sort((a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime())
    : baseFiltered;

  const listWidth = isMobile
    ? { width: '100%', minWidth: 0 }
    : paneWidth
      ? { width: `${paneWidth}px`, minWidth: `${paneWidth}px` }
      : { width: '380px', minWidth: '380px' };

  if (loading) {
    return (
      <div
        data-print="hide"
        style={{
          ...listWidth,
          height: '100%',
          borderRight: '1px solid var(--color-border-subtle)',
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
        title="전체 선택"
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
        {filteredMessages.length}개
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
      <div data-print="hide" style={{ ...listWidth, height: '100%', borderRight: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column' }}>
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
        height: '100%',
        borderRight: '1px solid var(--color-border-subtle)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {filterTabs}
      <div
        ref={scrollContainerRef}
        role="list"
        aria-label="메일 목록"
        style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden' }}
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
}

function MessageRow({ message, isSelected, isBulkChecked, onSelect, onStar, onToggleBulk, onContextMenu, searchQuery }: MessageRowProps) {
  const q = searchQuery ?? '';
  const isUnread = !message.read;

  return (
    <div
      role="listitem"
      data-message-id={message.id}
      onClick={() => onSelect(message.id)}
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
        padding: '12px 16px',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: isSelected ? 'var(--color-accent-subtle)' : 'var(--color-bg-primary)',
        cursor: 'pointer',
        transition: 'background 100ms ease',
        position: 'relative',
      }}
      onMouseEnter={(e) => {
        if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)';
      }}
      onMouseLeave={(e) => {
        if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)';
      }}
    >
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
            {message.preview && (
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
  );
}
