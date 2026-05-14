'use client';

import { useEffect, useRef } from 'react';
import {
  ArrowPathIcon,
  Bars3Icon,
  BarsArrowDownIcon,
  BarsArrowUpIcon,
  CheckIcon as CheckIconOutline,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  EllipsisVerticalIcon,
  StarIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';
import type { CategoryTab, FilterMode } from './messageListTypes';
import { CATEGORY_TABS } from './messageListTypes';

type MessageListHeaderProps = {
  hasBulk: boolean;
  bulkSelectedSize: number;
  filteredCount: number;
  onBulkMarkRead?: (ids: string[]) => void;
  onBulkStar?: (ids: string[], starred: boolean) => void;
  onBulkMove?: (ids: string[], folderId: string) => void;
  onBulkRestore?: (ids: string[]) => void;
  onBulkLabel?: (ids: string[], color: string | null) => void;
  onBulkDelete?: (ids: string[]) => void;
  folders?: { id: string; name: string }[];
  bulkSelected: Set<string>;
  clearAll: () => void;
  selectAll: () => void;
  bulkMoveOpen: boolean;
  setBulkMoveOpen: (value: boolean) => void;
  isMobile?: boolean;
  onOpenSidebar?: () => void;
  filterMode: FilterMode;
  setFilterMode: (value: FilterMode) => void;
  filterLabel: string | null;
  setFilterLabel: (value: string | null) => void;
  activeLabelColors: string[];
  showFilterDropdown: boolean;
  setShowFilterDropdown: (value: boolean) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  showMoreMenu: boolean;
  setShowMoreMenu: (value: boolean) => void;
  compact: boolean;
  toggleCompact: () => void;
  onMarkAllRead?: () => void;
  emptyFolderLabel?: string;
  onEmptyFolder?: () => void;
  messagesHaveUnread: boolean;
  sortAsc: boolean;
  setSortAsc: (value: boolean) => void;
  pageStart: number;
  pageEnd: number;
  filteredMessagesLength: number;
  hasMore?: boolean;
  onLoadMore?: () => void;
  page: number;
  setPage: (value: number) => void;
  categoryTab: CategoryTab;
  setCategoryTab: (value: CategoryTab) => void;
  categoryUnreadCounts: Partial<Record<CategoryTab, number>>;
  showCategoryTabs: boolean;
};

const FILTER_OPTIONS: { mode: FilterMode; label: string }[] = [
  { mode: 'all', label: '전체' },
  { mode: 'unread', label: '읽지 않음' },
  { mode: 'read', label: '읽음' },
  { mode: 'starred', label: '별표' },
  { mode: 'unstarred', label: '별표 없음' },
  { mode: 'attachment', label: '첨부 파일 있음' },
  { mode: 'noattachment', label: '첨부 파일 없음' },
];

export function MessageListHeader({
  hasBulk,
  bulkSelectedSize,
  filteredCount,
  onBulkMarkRead,
  onBulkStar,
  onBulkMove,
  onBulkRestore,
  onBulkLabel,
  onBulkDelete,
  folders,
  bulkSelected,
  clearAll,
  selectAll,
  bulkMoveOpen,
  setBulkMoveOpen,
  isMobile,
  onOpenSidebar,
  filterMode,
  setFilterMode,
  filterLabel,
  setFilterLabel,
  activeLabelColors,
  showFilterDropdown,
  setShowFilterDropdown,
  onRefresh,
  refreshing,
  showMoreMenu,
  setShowMoreMenu,
  compact,
  toggleCompact,
  onMarkAllRead,
  emptyFolderLabel,
  onEmptyFolder,
  messagesHaveUnread,
  sortAsc,
  setSortAsc,
  pageStart,
  pageEnd,
  filteredMessagesLength,
  hasMore,
  onLoadMore,
  page,
  setPage,
  categoryTab,
  setCategoryTab,
  categoryUnreadCounts,
  showCategoryTabs,
}: MessageListHeaderProps) {
  const filterDropdownRef = useRef<HTMLDivElement>(null);
  const moreMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showFilterDropdown) return;
    function onDown(e: MouseEvent) {
      if (filterDropdownRef.current && !filterDropdownRef.current.contains(e.target as Node)) {
        setShowFilterDropdown(false);
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [setShowFilterDropdown, showFilterDropdown]);

  useEffect(() => {
    if (!showMoreMenu) return;
    function onDown(e: MouseEvent) {
      if (moreMenuRef.current && !moreMenuRef.current.contains(e.target as Node)) {
        setShowMoreMenu(false);
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [setShowMoreMenu, showMoreMenu]);

  const filterTabs = hasBulk ? (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, background: 'var(--color-accent-subtle)' }}>
      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>{bulkSelectedSize}개 선택됨</span>
      {onBulkMarkRead && (
        <button onClick={() => { onBulkMarkRead([...bulkSelected]); clearAll(); }} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          읽음
        </button>
      )}
      {onBulkStar && (
        <>
          <button onClick={() => { onBulkStar([...bulkSelected], true); clearAll(); }} title="별표 추가" style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: '#f59e0b', display: 'inline-flex', alignItems: 'center' }}>
            <StarIconSolid style={{ width: '13px', height: '13px' }} />
          </button>
          <button onClick={() => { onBulkStar([...bulkSelected], false); clearAll(); }} title="별표 제거" style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', alignItems: 'center' }}>
            <StarIcon style={{ width: '13px', height: '13px' }} />
          </button>
        </>
      )}
      {onBulkMove && folders && folders.length > 0 && (
        <div style={{ position: 'relative' }}>
          <button onClick={() => setBulkMoveOpen(!bulkMoveOpen)} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
            이동
          </button>
          {bulkMoveOpen && (
            <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '140px', overflow: 'hidden' }}>
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
        <button onClick={() => { onBulkRestore([...bulkSelected]); clearAll(); }} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          복구
        </button>
      )}
      {onBulkLabel && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          {['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#a855f7'].map((color) => (
            <button
              key={color}
              title="라벨 지정"
              onClick={() => { onBulkLabel([...bulkSelected], color); clearAll(); }}
              style={{ width: '14px', height: '14px', borderRadius: '50%', background: color, border: 'none', cursor: 'pointer', flexShrink: 0 }}
            />
          ))}
          <button
            title="라벨 제거"
            onClick={() => { onBulkLabel([...bulkSelected], null); clearAll(); }}
            style={{ padding: '3px 6px', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
          >
            <XMarkIcon style={{ width: '11px', height: '11px' }} />
          </button>
        </div>
      )}
      {onBulkDelete && (
        <button onClick={() => { onBulkDelete([...bulkSelected]); clearAll(); }} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}>
          삭제
        </button>
      )}
      <button onClick={clearAll} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
        취소
      </button>
    </div>
  ) : (
    <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
      {isMobile && onOpenSidebar && (
        <button aria-label="메뉴 열기" onClick={onOpenSidebar} style={{ padding: '3px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', marginRight: '4px', display: 'inline-flex', alignItems: 'center' }}>
          <Bars3Icon style={{ width: '18px', height: '18px' }} />
        </button>
      )}
      <div ref={filterDropdownRef} style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginRight: '4px', flexShrink: 0 }}>
        <button
          aria-label="전체 선택"
          onClick={() => { bulkSelectedSize === filteredMessagesLength && filteredMessagesLength > 0 ? clearAll() : selectAll(); }}
          title="전체 선택/해제 (Ctrl+A)"
          style={{ padding: '4px 5px', border: '1px solid var(--color-border-default)', borderRight: 'none', borderRadius: '4px 0 0 4px', background: 'transparent', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-tertiary)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <div style={{ width: '14px', height: '14px', borderRadius: '2px', border: `1.5px solid ${bulkSelectedSize > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)'}`, background: bulkSelectedSize === filteredMessagesLength && filteredMessagesLength > 0 ? 'var(--color-accent)' : 'transparent', display: 'flex', alignItems: 'center', justifyContent: 'center', position: 'relative' }}>
            {bulkSelectedSize > 0 && bulkSelectedSize < filteredMessagesLength && (
              <div style={{ width: '8px', height: '1.5px', background: 'var(--color-accent)', borderRadius: '1px' }} />
            )}
            {bulkSelectedSize === filteredMessagesLength && filteredMessagesLength > 0 && (
              <CheckIconOutline style={{ width: '10px', height: '10px', color: '#fff' }} />
            )}
          </div>
        </button>
        <button
          aria-label="필터 선택"
          onClick={() => setShowFilterDropdown(!showFilterDropdown)}
          title="필터"
          style={{ padding: '4px 4px', border: '1px solid var(--color-border-default)', borderRadius: '0 4px 4px 0', background: showFilterDropdown ? 'var(--color-bg-tertiary)' : 'transparent', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', color: 'var(--color-text-tertiary)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { if (!showFilterDropdown) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <ChevronDownIcon style={{ width: '14px', height: '14px' }} />
        </button>
        {showFilterDropdown && (
          <div style={{ position: 'absolute', top: 'calc(100% + 4px)', left: 0, zIndex: 200, background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', minWidth: '160px', padding: '4px 0' }}>
            {FILTER_OPTIONS.map(({ mode, label }) => (
              <button
                key={mode}
                onClick={() => { setFilterMode(mode); setShowFilterDropdown(false); }}
                style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '8px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', textAlign: 'left' }}
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
        <button aria-label="새로고침" onClick={onRefresh} disabled={refreshing} title="새로고침" style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: refreshing ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <ArrowPathIcon style={{ width: '16px', height: '16px', animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
        </button>
      )}
      <div ref={moreMenuRef} style={{ position: 'relative' }}>
        <button aria-label="더 보기" onClick={() => setShowMoreMenu(!showMoreMenu)} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: showMoreMenu ? 'var(--color-bg-tertiary)' : 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} />
        </button>
        {showMoreMenu && (
          <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '2px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '180px', overflow: 'hidden', padding: '4px 0' }}>
            <button onClick={() => { toggleCompact(); setShowMoreMenu(false); }} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
              <span>컴팩트 보기</span>
              <span style={{ width: '28px', height: '16px', borderRadius: '8px', background: compact ? 'var(--color-accent)' : 'var(--color-border-default)', display: 'inline-flex', alignItems: 'center', transition: 'background 150ms ease', flexShrink: 0 }}>
                <span style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#fff', marginLeft: compact ? '14px' : '2px', transition: 'margin-left 150ms ease', display: 'block', boxShadow: '0 1px 3px rgba(0,0,0,0.2)' }} />
              </span>
            </button>
            {onMarkAllRead && messagesHaveUnread && (
              <button onClick={() => { onMarkAllRead(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
                모두 읽음으로 표시
              </button>
            )}
            {emptyFolderLabel && onEmptyFolder && filteredCount > 0 && (
              <button onClick={() => { onEmptyFolder(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
                {emptyFolderLabel}
              </button>
            )}
          </div>
        )}
      </div>
      {filterMode !== 'all' && (
        <span style={{ fontSize: '11px', padding: '2px 8px', borderRadius: '10px', background: 'var(--color-accent-subtle)', color: 'var(--color-accent)', fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
          {filterMode === 'unread' ? '읽지 않음' : filterMode === 'read' ? '읽음' : filterMode === 'starred' ? '별표' : filterMode === 'unstarred' ? '별표 없음' : filterMode === 'attachment' ? '첨부 있음' : '첨부 없음'}
          <button onClick={() => setFilterMode('all')} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, display: 'inline-flex', color: 'var(--color-accent)' }}>
            <XMarkIcon style={{ width: '11px', height: '11px' }} />
          </button>
        </span>
      )}
      {activeLabelColors.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginLeft: '2px' }}>
          {activeLabelColors.map((color) => (
            <button
              key={color}
              title={filterLabel === color ? '라벨 필터 해제' : '이 라벨로 필터'}
              onClick={() => setFilterLabel(filterLabel === color ? null : color)}
              style={{ width: '12px', height: '12px', borderRadius: '50%', background: color, border: filterLabel === color ? '2px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', flexShrink: 0, padding: 0, boxShadow: filterLabel === color ? '0 0 0 1px ' + color : 'none', transition: 'border-color 100ms ease' }}
            />
          ))}
        </div>
      )}
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '2px' }}>
        <button aria-label={sortAsc ? '최신순으로 정렬' : '오래된순으로 정렬'} title={sortAsc ? '오래된순 (클릭: 최신순)' : '최신순 (클릭: 오래된순)'} onClick={() => setSortAsc(!sortAsc)} style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)'; (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
          {sortAsc ? <BarsArrowUpIcon style={{ width: '15px', height: '15px' }} /> : <BarsArrowDownIcon style={{ width: '15px', height: '15px' }} />}
        </button>
        {filteredMessagesLength > 0 && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '0 4px', whiteSpace: 'nowrap' }}>
            {`${pageStart + 1}–${Math.min(pageEnd, filteredMessagesLength)}`}{hasMore ? '+' : ` / ${filteredMessagesLength}`}
          </span>
        )}
        <button aria-label="이전 페이지" onClick={() => setPage(Math.max(0, page - 1))} disabled={page === 0} style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: page === 0 ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center', opacity: page === 0 ? 0.35 : 1 }}>
          <ChevronLeftIcon style={{ width: '16px', height: '16px' }} />
        </button>
        <button aria-label="다음 페이지" onClick={() => {
          const next = page + 1;
          if (next * 50 >= filteredMessagesLength && hasMore && onLoadMore) onLoadMore();
          if (next * 50 < filteredMessagesLength || hasMore) setPage(next);
        }} disabled={!hasMore && (page + 1) * 50 >= filteredMessagesLength} style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: (!hasMore && (page + 1) * 50 >= filteredMessagesLength) ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center', opacity: (!hasMore && (page + 1) * 50 >= filteredMessagesLength) ? 0.35 : 1 }}>
          <ChevronRightIcon style={{ width: '16px', height: '16px' }} />
        </button>
      </div>
    </div>
  );

  const categoryTabsUI = showCategoryTabs ? (
    <div style={{ display: 'flex', gap: '0', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, overflowX: 'auto', scrollbarWidth: 'none' }}>
      {CATEGORY_TABS.map((tab) => {
        const isActive = categoryTab === tab.id;
        const unread = tab.id !== 'all' ? (categoryUnreadCounts[tab.id] ?? 0) : 0;
        return (
          <button
            key={tab.id}
            onClick={() => setCategoryTab(tab.id)}
            style={{ padding: '9px 14px', border: 'none', background: 'none', cursor: 'pointer', fontSize: '13px', fontWeight: isActive ? 600 : 400, color: isActive ? 'var(--color-accent)' : 'var(--color-text-secondary)', borderBottom: isActive ? '2px solid var(--color-accent)' : '2px solid transparent', marginBottom: '-1px', display: 'inline-flex', alignItems: 'center', gap: '5px', whiteSpace: 'nowrap', flexShrink: 0, transition: 'color 120ms, border-color 120ms' }}
          >
            {tab.label}
            {unread > 0 && (
              <span style={{ fontSize: '10px', fontWeight: 700, padding: '0 5px', borderRadius: '10px', background: 'var(--color-accent)', color: '#fff', minWidth: '16px', textAlign: 'center' }}>
                {unread}
              </span>
            )}
          </button>
        );
      })}
    </div>
  ) : null;

  return (
    <>
      {filterTabs}
      {categoryTabsUI}
    </>
  );
}
