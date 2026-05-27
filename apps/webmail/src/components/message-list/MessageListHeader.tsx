'use client';

import { useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';
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
  XMarkIcon,
} from '@heroicons/react/24/outline';
import type { CategoryTab, FilterMode } from './messageListTypes';
import { CATEGORY_TABS } from './messageListTypes';
import { MessageListBulkToolbar } from './MessageListBulkToolbar';

type MessageListHeaderProps = {
  hasBulk: boolean;
  bulkSelectedSize: number;
  filteredCount: number;
  onBulkMarkRead?: (ids: string[]) => void;
  onBulkToggleRead?: (ids: string[], read: boolean) => void;
  onBulkStar?: (ids: string[], starred: boolean) => void;
  onBulkArchive?: (ids: string[]) => void;
  onBulkSnooze?: (ids: string[], until: Date) => void;
  onBulkPin?: (ids: string[]) => void;
  onBulkMove?: (ids: string[], folderId: string) => void;
  onBulkRestore?: (ids: string[]) => void;
  onBulkLabel?: (ids: string[], color: string | null) => void;
  onBulkDelete?: (ids: string[]) => void;
  folders?: { id: string; name: string; system_type?: string }[];
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
  bulkReadTarget?: boolean;
  bulkStarTarget?: boolean;
  bulkPinned?: boolean;
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

const FILTER_OPTIONS: { mode: FilterMode; labelKey: string }[] = [
  { mode: 'all', labelKey: 'filter.all' },
  { mode: 'unread', labelKey: 'filter.unread' },
  { mode: 'read', labelKey: 'filter.read' },
  { mode: 'starred', labelKey: 'filter.starred' },
  { mode: 'unstarred', labelKey: 'filter.unstarred' },
  { mode: 'attachment', labelKey: 'filter.attachment' },
  { mode: 'noattachment', labelKey: 'filter.noattachment' },
];

export function MessageListHeader({
  hasBulk,
  bulkSelectedSize,
  filteredCount,
  onBulkMarkRead,
  onBulkToggleRead,
  onBulkStar,
  onBulkArchive,
  onBulkSnooze,
  onBulkPin,
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
  bulkReadTarget = true,
  bulkStarTarget = true,
  bulkPinned = false,
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
  const t = useTranslations('mailListFull');

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
    <MessageListBulkToolbar
      bulkSelected={bulkSelected}
      bulkSelectedSize={bulkSelectedSize}
      clearAll={clearAll}
      onBulkMarkRead={onBulkMarkRead}
      onBulkToggleRead={onBulkToggleRead}
      onBulkStar={onBulkStar}
      onBulkArchive={onBulkArchive}
      onBulkSnooze={onBulkSnooze}
      onBulkPin={onBulkPin}
      onBulkMove={onBulkMove}
      onBulkRestore={onBulkRestore}
      onBulkLabel={onBulkLabel}
      onBulkDelete={onBulkDelete}
      folders={folders}
      bulkMoveOpen={bulkMoveOpen}
      setBulkMoveOpen={setBulkMoveOpen}
      bulkReadTarget={bulkReadTarget}
      bulkStarTarget={bulkStarTarget}
      bulkPinned={bulkPinned}
    />
  ) : (
    <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
      {isMobile && onOpenSidebar && (
        <button aria-label={t('header.openMenu')} onClick={onOpenSidebar} style={{ padding: '3px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', marginRight: '4px', display: 'inline-flex', alignItems: 'center' }}>
          <Bars3Icon style={{ width: '18px', height: '18px' }} />
        </button>
      )}
      <div ref={filterDropdownRef} style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginRight: '4px', flexShrink: 0 }}>
        <button
          aria-label={t('header.selectAllAria')}
          onClick={() => { bulkSelectedSize === filteredMessagesLength && filteredMessagesLength > 0 ? clearAll() : selectAll(); }}
          title={t('header.selectAllTitle')}
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
          aria-label={t('header.filterAria')}
          onClick={() => setShowFilterDropdown(!showFilterDropdown)}
          title={t('header.filterTitle')}
          style={{ padding: '4px 4px', border: '1px solid var(--color-border-default)', borderRadius: '0 4px 4px 0', background: showFilterDropdown ? 'var(--color-bg-tertiary)' : 'transparent', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', color: 'var(--color-text-tertiary)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { if (!showFilterDropdown) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <ChevronDownIcon style={{ width: '14px', height: '14px' }} />
        </button>
        {showFilterDropdown && (
          <div style={{ position: 'absolute', top: 'calc(100% + 4px)', left: 0, zIndex: 200, background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', minWidth: '160px', padding: '4px 0' }}>
            {FILTER_OPTIONS.map(({ mode, labelKey }) => (
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
                {t(labelKey)}
              </button>
            ))}
          </div>
        )}
      </div>
      {onRefresh && (
        <button aria-label={t('header.refreshAria')} onClick={onRefresh} disabled={refreshing} title={t('header.refreshTitle')} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: refreshing ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <ArrowPathIcon style={{ width: '16px', height: '16px', animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
        </button>
      )}
      <div ref={moreMenuRef} style={{ position: 'relative' }}>
        <button aria-label={t('header.moreAria')} onClick={() => setShowMoreMenu(!showMoreMenu)} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: showMoreMenu ? 'var(--color-bg-tertiary)' : 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} />
        </button>
        {showMoreMenu && (
          <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '2px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '180px', overflow: 'hidden', padding: '4px 0' }}>
            <button onClick={() => { toggleCompact(); setShowMoreMenu(false); }} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
              <span>{t('header.compactView')}</span>
              <span style={{ width: '28px', height: '16px', borderRadius: '8px', background: compact ? 'var(--color-accent)' : 'var(--color-border-default)', display: 'inline-flex', alignItems: 'center', transition: 'background 150ms ease', flexShrink: 0 }}>
                <span style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#fff', marginLeft: compact ? '14px' : '2px', transition: 'margin-left 150ms ease', display: 'block', boxShadow: '0 1px 3px rgba(0,0,0,0.2)' }} />
              </span>
            </button>
            {onMarkAllRead && messagesHaveUnread && (
              <button onClick={() => { onMarkAllRead(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 16px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
                {t('header.markAllRead')}
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
          {filterMode === 'unread' ? t('filter.unread') : filterMode === 'read' ? t('filter.read') : filterMode === 'starred' ? t('filter.starred') : filterMode === 'unstarred' ? t('filter.unstarred') : filterMode === 'attachment' ? t('filter.attachmentShort') : t('filter.noattachmentShort')}
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
              title={filterLabel === color ? t('header.removeLabelFilter') : t('header.filterByLabel')}
              onClick={() => setFilterLabel(filterLabel === color ? null : color)}
              style={{ width: '12px', height: '12px', borderRadius: '50%', background: color, border: filterLabel === color ? '2px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', flexShrink: 0, padding: 0, boxShadow: filterLabel === color ? '0 0 0 1px ' + color : 'none', transition: 'border-color 100ms ease' }}
            />
          ))}
        </div>
      )}
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '2px' }}>
        <button aria-label={sortAsc ? t('header.sortNewestAria') : t('header.sortOldestAria')} title={sortAsc ? t('header.sortNewestTitle') : t('header.sortOldestTitle')} onClick={() => setSortAsc(!sortAsc)} style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)'; (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>
          {sortAsc ? <BarsArrowUpIcon style={{ width: '15px', height: '15px' }} /> : <BarsArrowDownIcon style={{ width: '15px', height: '15px' }} />}
        </button>
        {filteredMessagesLength > 0 && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '0 4px', whiteSpace: 'nowrap' }}>
            {`${pageStart + 1}–${Math.min(pageEnd, filteredMessagesLength)}`}{hasMore ? '+' : ` / ${filteredMessagesLength}`}
          </span>
        )}
        <button aria-label={t('header.prevPage')} onClick={() => setPage(Math.max(0, page - 1))} disabled={page === 0} style={{ padding: '4px 6px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: page === 0 ? 'not-allowed' : 'pointer', display: 'inline-flex', alignItems: 'center', opacity: page === 0 ? 0.35 : 1 }}>
          <ChevronLeftIcon style={{ width: '16px', height: '16px' }} />
        </button>
        <button aria-label={t('header.nextPage')} onClick={() => {
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
