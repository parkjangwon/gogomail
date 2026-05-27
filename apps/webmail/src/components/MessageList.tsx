'use client';

import { useState, useRef, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { MessageSummary } from '@/lib/api';
import { MessageRow } from './message-list/MessageRow';
import { ContactHoverCard } from './message-list/ContactHoverCard';
import { MessageListHeader } from './message-list/MessageListHeader';
import { useMessageListSelection } from './message-list/useMessageListSelection';
import { useContactHoverCard } from './message-list/useContactHoverCard';
import { useMessageListState } from './message-list/useMessageListState';
import {
  type CategoryTab,
  type MessageListProps,
  getAutoCategory,
} from './message-list/messageListTypes';
import { DateGroupKey, getDateGroup } from './message-list/messageListHelpers';

export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel, hasMore, loadingMore, onLoadMore, onStar, onBulkDelete, onBulkMarkRead, onRefresh, refreshing, isMobile, onOpenSidebar, onContextMenuMessage, onMarkAllRead, emptyFolderLabel, onEmptyFolder, folders, onBulkMove, paneWidth, fullWidth, bottomLayout, searchQuery, onDeleteMessage, onBulkRestore, onBulkLabel, onBulkStar, onArchiveMessage, onToggleReadMessage, onSnoozeMessage, onPinMessage, pinnedIds = new Set(), importantIds = new Set(), messageLabels = {}, userEmail, showPreview = true, showCategoryTabs = false }: MessageListProps) {
  const t = useTranslations('mailListFull');
  const {
    filterMode, setFilterMode,
    filterLabel, setFilterLabel,
    showFilterDropdown, setShowFilterDropdown,
    filterDropdownRef,
    sortAsc, setSortAsc,
    bulkMoveOpen, setBulkMoveOpen,
    categoryTab, setCategoryTab,
    noteIds,
    compact,
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
  } = useMessageListState({ messages, selectedId, onLoadMore, hasMore, isMobile, onRefresh, refreshing });

  const selectedIdRef = useRef(selectedId);
  useEffect(() => { selectedIdRef.current = selectedId; }, [selectedId]);

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

  // --- Custom hooks (called after filteredMessages is computed, before any early returns) ---
  const {
    bulkSelected,
    toggleBulk,
    selectAll,
    clearAll,
    getActionMessages,
    handleRowKeyDownCapture,
    hoveredMessageIdRef,
  } = useMessageListSelection({
    filteredMessages,
    messages,
    onSelect,
    onToggleReadMessage,
    onArchiveMessage,
    onSnoozeMessage,
    onPinMessage,
    onDeleteMessage,
    onBulkDelete,
  });

  const { contactCard, handleAvatarEnter, handleAvatarLeave, closeContactCard } = useContactHoverCard(messages);

  const runActionForIds = (ids: string[], action: (id: string) => void, clearAfter = false) => {
    ids.forEach(action);
    if (clearAfter) clearAll();
  };
  const toggleReadForIds = (ids: string[], read: boolean, clearAfter = false) => {
    ids.forEach((id) => onToggleReadMessage?.(id, read));
    if (clearAfter) clearAll();
  };

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
  const folderLabelById = new Map((folders ?? []).map((folder) => [folder.id, folder.system_type === 'inbox'
    ? t('folder.inbox')
    : folder.system_type === 'sent'
      ? t('folder.sent')
      : folder.system_type === 'drafts'
        ? t('folder.drafts')
        : folder.system_type === 'trash'
          ? t('folder.trash')
          : folder.system_type === 'spam' || folder.system_type === 'junk'
            ? t('folder.spam')
            : folder.system_type === 'archive'
              ? t('folder.archive')
              : folder.name]));
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
      bulkReadTarget={bulkReadTarget}
      bulkStarTarget={bulkStarTarget}
      bulkPinned={bulkPinned}
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
              <div style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('inboxZeroTitle')}</div>
              <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', textAlign: 'center', maxWidth: '240px', lineHeight: 1.5 }}>{t('inboxZeroDesc')}</div>
            </>
          ) : (
            <div style={{ color: 'var(--color-text-tertiary)', fontSize: '14px' }}>
              {emptyLabel ?? (filterMode === 'unread' ? t('emptyNoUnread') : filterMode === 'starred' ? t('emptyNoStarred') : t('emptyDefault'))}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Group messages by date
  const groups: { key: string; label: string; messages: MessageSummary[] }[] = [];
  const groupOrder: DateGroupKey[] = ['today', 'yesterday', 'lastWeek', 'thisMonth'];
  const groupMap = new Map<DateGroupKey, MessageSummary[]>();

  for (const msg of pagedMessages) {
    const group = getDateGroup(msg.received_at);
    if (!groupMap.has(group)) groupMap.set(group, []);
    groupMap.get(group)!.push(msg);
  }

  const groupOrderDisplay = sortAsc ? [...groupOrder].reverse() : groupOrder;

  for (const key of groupOrderDisplay) {
    if (groupMap.has(key)) {
      groups.push({ key, label: t(`dateGroup.${key}`), messages: groupMap.get(key)! });
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
        aria-label={t('listAria')}
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
        <div key={group.key} role="group" aria-label={group.label}>
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
              folderLabel={folderLabelById.get(msg.folder_id)}
              onAvatarEnter={!isMobile ? handleAvatarEnter : undefined}
              onAvatarLeave={!isMobile ? handleAvatarLeave : undefined}
              onHoverChange={(id) => { hoveredMessageIdRef.current = id; }}
            />
          ))}
        </div>
      ))}

      <div ref={sentinelRef} style={{ height: '1px' }} aria-hidden="true" />
      {loadingMore && (
        <div style={{ padding: '12px 16px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
          {t('loadingMore')}
        </div>
      )}
      {contactCard && (
        <ContactHoverCard
          {...contactCard}
          onClose={closeContactCard}
        />
      )}
      </div>
    </div>
  );
}
