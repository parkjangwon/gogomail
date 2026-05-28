'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { moveMessage, MessageSummary } from '@/lib/api';
import { AdvancedFilters, VIRTUAL_ALL, VIRTUAL_SNOOZED } from '@/components/Sidebar';
import { useMailList } from '@/hooks/useMailList';
import { useMessage } from '@/hooks/useMessage';
import { useIsMobile } from '@/hooks/useIsMobile';
import { useIsOnline } from '@/hooks/useIsOnline';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ComposeModal } from '@/components/ComposeModal';
import { ToastContainer, ToastItem } from '@/components/Toast';
import { ShortcutHelp } from '@/components/ShortcutHelp';
import { ContextMenu } from '@/components/ContextMenu';
import { PencilSquareIcon } from '@heroicons/react/24/outline';
import { AppIconBar, AppId } from '@/components/AppIconBar';
import { CalendarView } from '@/components/CalendarView';
import { ContactsView } from '@/components/ContactsView';
import { SettingsView } from '@/components/SettingsView';
import { type SectionId } from '@/components/settings-view/settingsViewConfig';
import { DriveView } from '@/components/DriveView';
import { SpotlightSearch } from '@/components/SpotlightSearch';
import { MFASetupPromptModal } from '@/components/MFASetupPromptModal';
import { SpamReportDialog } from '@/components/spam/SpamReportDialog';
import { MailWarningBanners } from './MailWarningBanners';
import { SpamFolderBanner } from './SpamFolderBanner';
import { MailDMModal } from './MailDMModal';
import { useMailMessageActions } from './useMailMessageActions';
import { useDMModal } from './useDMModal';
import { useMailLabels } from './useMailLabels';
import { useMailSession } from './useMailSession';
import { useMailSearch } from './useMailSearch';
import { useMailLayout } from './useMailLayout';
import { useMailToasts } from './useMailToasts';
import { useMailSettings } from './useMailSettings';
import { useMailThreads } from './useMailThreads';
import { useMailCompose } from './useMailCompose';
import { useMailNav } from './useMailNav';
import { useMailKeyboardShortcuts } from './useMailKeyboardShortcuts';
import { useMailBadge } from './useMailBadge';
import { useMailNotifications } from './useMailNotifications';
import { useMailFilterRules } from './useMailFilterRules';
import { useMailServiceWorker } from './useMailServiceWorker';
import { useMailComposeGate } from './useMailComposeGate';
import { useMailAutoRead } from './useMailAutoRead';
import { useMailTimers } from './useMailTimers';
import { useMailThreadMessages } from './useMailThreadMessages';
import { useMailFolderOps } from './useMailFolderOps';
import { printMessage } from '@/lib/mail/printMessage';
import {
  getEmptyFolderLabel,
  getVisibleMailMessages,
} from '@/lib/mail/mailPageUtils';
import { MailReadingPanel } from './MailReadingPanel';
import { useNotifications } from '@/lib/notifications/store';
import {
  getDefaultDMModalRect,
  type DMModalRect,
  type DMResizeEdge,
} from './mailPageHelpers';

export default function MailPage() {
  const router = useRouter();
  const t = useTranslations();
  const tNotif = useTranslations('notifications');
  const { push: pushNotification } = useNotifications();

  const { composeContext, openCompose, closeCompose, pendingCompose, setPendingCompose } = useMailCompose();
  const { activeApp, setActiveApp, activeFolderId, setActiveFolderId, selectedMessageId, setSelectedMessageId } = useMailNav();
  const { toasts, setToasts, addToast, dismissToast } = useMailToasts();
  const [showShortcuts, setShowShortcuts] = useState(false);
  const {
    mobileSidebarOpen, setMobileSidebarOpen,
    sidebarCollapsed, setSidebarCollapsed,
    sidebarWidth, setSidebarWidth,
    readingPaneWidth, setReadingPaneWidth,
    swipeDeltaX, setSwipeDeltaX,
    swipeTouchStartRef,
  } = useMailLayout();
  const [contextMenu, setContextMenu] = useState<{ id: string; x: number; y: number } | null>(null);

  const {
    badgeCountMode, setBadgeCountMode,
    refreshIntervalSeconds, setRefreshIntervalSeconds,
    threadNotificationOverrides, setThreadNotificationOverrides,
    wmSettings, setWmSettings,
    settingsInitialSection, setSettingsInitialSection,
  } = useMailSettings();
  const [showSpotlight, setShowSpotlight] = useState(false);
  const [spotlightMoveId, setSpotlightMoveId] = useState<string | null>(null);
  const [spamDialogMessageId, setSpamDialogMessageId] = useState<string | null>(null);

  const threadViewEnabled = true; // thread view always on (toggle removed)

  const isMobile = useIsMobile();
  const gPrefixRef = useRef(false);
  const isOnline = useIsOnline();

  // Extracted hooks
  const {
    searchQuery, setSearchQuery,
    searchResults, setSearchResults,
    searchLoading,
    advancedFilters, setAdvancedFilters,
    handleSearch,
  } = useMailSearch({ t, addToast });

  const {
    showDMModal, setShowDMModal,
    dmModalRect, setDMModalRect,
    dmUnreadCount, setDMUnreadCount,
    startDMModalResize, startDMModalDrag,
  } = useDMModal({ isMobile });

  const {
    messageLabels, setMessageLabels,
    pinnedIds, setPinnedIds,
    importantIds, setImportantIds,
    handlePin, handleImportant,
    setLabel, handleBulkLabel,
  } = useMailLabels({ addToast, t });

  const {
    userEmail, setUserEmail,
    mustChangePassword, setMustChangePassword,
    sessionWarning, setSessionWarning,
    handleLogout,
  } = useMailSession({ router, t });

  const { folders, messages, setMessages, foldersLoading, messagesLoading, setMessagesLoading, hasMore, loadingMore, loadMore, adjustUnread, refresh, refreshing } =
    useMailList(activeFolderId, refreshIntervalSeconds);

  const {
    virtualRefreshKey, setVirtualRefreshKey,
    threadMessages, setThreadMessages,
    threads, setThreads,
    threadRefreshKey, setThreadRefreshKey,
  } = useMailThreads({
    activeFolderId,
    foldersLoading,
    setMessages: (msgs) => setMessages(() => msgs),
    setMessagesLoading,
  });

  const { message: selectedMessage, loading: messageLoading } =
    useMessage(selectedMessageId);

  // selectedMessageSummary: the MessageSummary row that was clicked (may carry thread_id)
  const selectedMessageSummary = (threadViewEnabled && threads.length > 0)
    ? threads.find((t) => (t.latest_message_id || t.id) === selectedMessageId) ?? null
    : null;
  const selectedThreadId = selectedMessageSummary?.id ?? null;
  const selectedNotificationThreadId = selectedThreadId ?? selectedMessage?.thread_id ?? selectedMessage?.id ?? '';
  const selectedThreadMuted = !!selectedNotificationThreadId && threadNotificationOverrides[selectedNotificationThreadId]?.enabled === false;

  // Thread messages: fetch via thread API when a thread is selected, or fall back
  // to subject-based grouping for normal message view.
  useMailThreadMessages({
    selectedThreadId,
    selectedMessageId,
    selectedMessageSubject: selectedMessage?.subject,
    activeFolderId,
    setThreadMessages,
  });

  // Set default folder to inbox UUID once folders are loaded, and recover from stale saved IDs.
  useEffect(() => {
    if (folders.length === 0 || activeFolderId.startsWith('__')) return;
    const inbox = folders.find((f) => f.system_type === 'inbox') ?? folders[0];
    if (!activeFolderId || !folders.some((f) => f.id === activeFolderId)) {
      if (inbox) setActiveFolderId(inbox.id);
    }
  }, [folders, activeFolderId]);

  // Update document title + favicon badge according to the selected badge mode.
  useMailBadge({ folders, badgeCountMode });


  const activeFolderSystemType = folders.find((f) => f.id === activeFolderId)?.system_type;

  const {
    pendingDeletesRef,
    patchVisibleMessages,
    removeVisibleMessages,
    findVisibleMessage,
    countUnreadVisible,
    getNextId,
    handleMarkUnread,
    handleMarkRead,
    handleToggleReadMessage,
    handleDeleteById,
    handleDelete,
    handleBulkDelete,
    handleRestore,
    handleBulkRestore,
    handleRestoreFromArchive,
    handleBulkRestoreFromArchive,
    handleBulkMarkRead,
    handleBulkStar,
    handleMarkAllRead,
    handleArchiveById,
    handleArchive,
    handleSpam,
    executeSpam,
    handleBlockSender,
    handleNotSpam,
    handleMove,
    handleStar,
    handleSnooze,
  } = useMailMessageActions({
    messages,
    searchResults,
    threads,
    threadMessages,
    selectedMessageId,
    activeFolderId,
    activeFolderSystemType,
    folders,
    setMessages,
    setSearchResults,
    setThreadMessages,
    setThreads,
    setSelectedMessageId,
    adjustUnread,
    addToast,
    setSpamDialogMessageId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    t: t as (key: string, values?: Record<string, any>) => string,
  });

  useMailComposeGate({ selectedMessage, activeFolderSystemType, pendingCompose, openCompose, setSelectedMessageId, setPendingCompose });
  useMailAutoRead({ selectedMessageId, activeFolderId, activeFolderSystemType, findVisibleMessage, patchVisibleMessages, adjustUnread });

  const handleSelectFolder = useCallback((id: string) => {
    setActiveFolderId(id);
    setSelectedMessageId(null);
    setSearchResults(null);
    setSearchQuery('');
    setAdvancedFilters({});
  }, []);

  const handleSelectMessage = useCallback((id: string) => {
    setSelectedMessageId(id);
  }, []);

  const handleGlobalEscape = useCallback(() => {
    if (composeContext) return false;
    if (showSpotlight) {
      setShowSpotlight(false);
      setSpotlightMoveId(null);
      return true;
    }
    if (contextMenu) {
      setContextMenu(null);
      return true;
    }
    if (showShortcuts) {
      setShowShortcuts(false);
      return true;
    }
    if (mobileSidebarOpen) {
      setMobileSidebarOpen(false);
      return true;
    }
    if (showDMModal) {
      setShowDMModal(false);
      return true;
    }
    if (selectedMessageId) {
      setSelectedMessageId(null);
      return true;
    }
    return false;
  }, [composeContext, showSpotlight, contextMenu, showShortcuts, mobileSidebarOpen, showDMModal, selectedMessageId]);


  // Persist last-selected message per folder

  // Keyboard shortcuts (skip when typing in input/textarea/contenteditable)
  useMailKeyboardShortcuts({
    messages,
    searchResults,
    selectedMessageId,
    selectedMessage,
    composeContext,
    showShortcuts,
    showSpotlight,
    activeApp,
    activeFolderSystemType,
    folders,
    isMobile,
    messageLabels,
    importantIds,
    gPrefixRef,
    handleDelete,
    handleArchive,
    handleSpam,
    handleMarkRead,
    handleMarkUnread,
    handleStar,
    handleMove,
    handlePin,
    handleImportant,
    handleSnooze,
    setLabel,
    openCompose,
    setSelectedMessageId,
    setActiveApp,
    setShowSpotlight,
    setSpotlightMoveId,
    setShowDMModal,
    setShowShortcuts,
    setSidebarCollapsed,
    handleSelectFolder,
    handleGlobalEscape,
    addToast,
    t,
  });

  // Unified refresh: works for both real folders (useMailList) and virtual folders.
  const isVirtualFolder = activeFolderId.startsWith('__') && activeFolderId !== VIRTUAL_ALL;
  const handleRefresh = useCallback(() => {
    if (isVirtualFolder) {
      setVirtualRefreshKey((k) => k + 1);
    } else {
      refresh();
      // threadViewEnabled is always true; visibleMessages uses buildThreadMessages(threads).
      // Bumping threadRefreshKey re-triggers the thread fetch effect (with proper cancellation).
      setThreadRefreshKey((k) => k + 1);
    }
  }, [isVirtualFolder, refresh]);

  const { refreshRef } = useMailServiceWorker({ refreshIntervalSeconds, onRefresh: handleRefresh });

  const { handleToggleThreadMute } = useMailNotifications({
    messages,
    activeFolderId,
    selectedNotificationThreadId,
    selectedThreadMuted,
    threadNotificationOverrides,
    setThreadNotificationOverrides,
    pushNotification,
    t,
    tNotif,
  });

  useMailFilterRules({
    messages,
    setMessages,
    setMessageLabels,
    folders,
    adjustUnread,
    activeFolderId,
    setLabel,
    t,
  });

  useMailTimers({ messages, addToast, t, refresh });

  const { handleDropMessage, handleCreateFolder, handleRenameFolder, handleDeleteFolder } =
    useMailFolderOps({
      activeFolderId,
      setActiveFolderId,
      messages,
      setMessages,
      selectedMessageId,
      setSelectedMessageId,
      refresh,
      addToast,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      t: t as (key: string, values?: Record<string, any>) => string,
    });

  if (foldersLoading) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          background: 'var(--color-bg-primary)',
          color: 'var(--color-text-tertiary)',
          fontSize: '14px',
          gap: '10px',
        }}
      >
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
          style={{ animation: 'spin 1s linear infinite' }}
        >
          <path d="M21 12a9 9 0 1 1-6.219-8.56" />
        </svg>
        <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
        {t('misc.mailPage.loading')}
      </div>
    );
  }

  const visibleMessages = (() => {
    let blockedSenders: string[] = [];
    let snoozedMessages: Record<string, string> = {};
    let focusModeEnabled = false;
    try {
      if (activeFolderId !== VIRTUAL_SNOOZED) {
        blockedSenders = JSON.parse(localStorage.getItem('webmail_blocked_senders') ?? '[]');
        snoozedMessages = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
      }
      if (activeFolderSystemType === 'inbox') {
        focusModeEnabled = localStorage.getItem('webmail_focus_mode') === '1';
      }
    } catch { /* ignore */ }
    return getVisibleMailMessages({
      searchResults,
      messages,
      threads,
      threadViewEnabled,
      activeFolderId,
      activeFolderSystemType,
      blockedSenders,
      snoozedMessages,
      pinnedIds,
      importantIds,
      focusModeEnabled,
    });
  })();
  const resolvedDMModalRect = dmModalRect ?? getDefaultDMModalRect();

  return (
    <div
      style={{
        display: 'flex',
        height: '100vh',
        overflow: 'hidden',
        background: 'var(--color-bg-primary)',
      }}
    >
      <MailWarningBanners
        mustChangePassword={mustChangePassword}
        sessionWarning={sessionWarning}
        isOnline={isOnline}
        onDismissPasswordWarning={() => { localStorage.removeItem('webmail_must_change_password'); setMustChangePassword(false); }}
        onLogout={handleLogout}
        onDismissSessionWarning={() => setSessionWarning(null)}
        tClose={t('misc.mailPage.close')}
        tMustChangePassword={t('misc.mailPage.mustChangePassword')}
        tLoginAgain={t('misc.mailPage.loginAgain')}
        tOffline={t('misc.mailPage.offline')}
      />

      <AppIconBar
        activeApp={activeApp}
        onChangeApp={setActiveApp}
        mailUnread={folders.reduce((s, f) => s + (f.unread ?? 0), 0)}
        dmUnread={dmUnreadCount}
        dmOpen={showDMModal}
        onOpenDM={() => setShowDMModal((open) => !open)}
      />

      {activeApp === 'mail' ? (
        <>
          <Sidebar
            folders={folders}
            activeFolderId={activeFolderId}
            onSelectFolder={(id) => { handleSelectFolder(id); setMobileSidebarOpen(false); }}
            onCompose={() => { openCompose({ intent: 'new' }); setMobileSidebarOpen(false); }}
            onComposeInNewWindow={() => window.open('/compose', '_blank', 'width=620,height=720,menubar=no,toolbar=no,resizable=yes')}
            userName={userEmail || t('misc.mailPage.defaultUser')}
            userEmailAddress={userEmail || undefined}
            width={sidebarWidth}
            onLogout={handleLogout}
            isMobile={isMobile}
            isOpen={mobileSidebarOpen}
            onClose={() => setMobileSidebarOpen(false)}
            collapsed={sidebarCollapsed}
            onToggleCollapse={() => setSidebarCollapsed((v) => !v)}
            onDropMessage={handleDropMessage}
            onCreateFolder={handleCreateFolder}
            onRenameFolder={handleRenameFolder}
            onDeleteFolder={handleDeleteFolder}
          />

          {/* Sidebar drag-resize handle */}
          {!isMobile && !sidebarCollapsed && (
            <div
              aria-hidden="true"
              style={{ width: '4px', flexShrink: 0, cursor: 'col-resize', position: 'relative', zIndex: 10, transition: 'background 150ms ease' }}
              onMouseDown={(e) => {
                e.preventDefault();
                const startX = e.clientX;
                const startWidth = sidebarWidth;
                let lastWidth = startWidth;
                const onMove = (ev: MouseEvent) => {
                  lastWidth = Math.min(360, Math.max(160, startWidth + ev.clientX - startX));
                  setSidebarWidth(lastWidth);
                };
                const onUp = () => {
                  document.removeEventListener('mousemove', onMove);
                  document.removeEventListener('mouseup', onUp);
                  try { localStorage.setItem('webmail_sidebar_width', String(lastWidth)); } catch { /* */ }
                };
                document.addEventListener('mousemove', onMove);
                document.addEventListener('mouseup', onUp);
              }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-accent)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
            />
          )}

          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>

            {/* Spam folder info banner */}
            <SpamFolderBanner
              activeFolderSystemType={activeFolderSystemType}
              messages={messages}
              folders={folders}
              removeVisibleMessages={removeVisibleMessages}
              setSelectedMessageId={setSelectedMessageId}
              handleBulkDelete={handleBulkDelete}
              addToast={addToast}
              t={t}
            />

            <MessageList
              messages={visibleMessages}
              selectedId={selectedMessageId}
              onSelect={handleSelectMessage}
              loading={searchResults !== null ? searchLoading : messagesLoading}
              emptyLabel={searchResults !== null ? (searchQuery ? t('misc.mailPage.searchEmptyQuery', { query: searchQuery }) : t('misc.mailPage.searchEmpty')) : getEmptyFolderLabel(activeFolderSystemType, t, activeFolderId)}
              hasMore={searchResults === null ? hasMore : false}
              loadingMore={loadingMore}
              onLoadMore={loadMore}
              onStar={handleStar}
              onBulkDelete={handleBulkDelete}
              onBulkMarkRead={handleBulkMarkRead}
              folders={folders}
              onBulkMove={async (ids, folderId) => {
                removeVisibleMessages(ids);
                if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
                await Promise.allSettled(ids.map((id) => moveMessage(id, folderId)));
                addToast(t('misc.mailPage.bulkMoved', { count: ids.length }));
              }}
              onRefresh={handleRefresh}
              refreshing={refreshing || (isVirtualFolder && messagesLoading)}
              isMobile={isMobile}
              onOpenSidebar={() => setMobileSidebarOpen(true)}
              onContextMenuMessage={(id, x, y) => setContextMenu({ id, x, y })}
              onMarkAllRead={activeFolderSystemType !== 'trash' ? handleMarkAllRead : undefined}
              searchQuery={searchResults !== null ? searchQuery : undefined}
              emptyFolderLabel={activeFolderSystemType === 'trash' ? t('misc.mailPage.emptyTrashAction') : undefined}
              onEmptyFolder={activeFolderSystemType === 'trash' ? () => handleBulkDelete(messages.map((m) => m.id)) : undefined}
              onDeleteMessage={handleDeleteById}
              onArchiveMessage={activeFolderSystemType !== 'archive' && activeFolderSystemType !== 'trash' ? handleArchiveById : undefined}
              onToggleReadMessage={handleToggleReadMessage}
              onSnoozeMessage={activeFolderSystemType !== 'trash' ? handleSnooze : undefined}
              onPinMessage={handlePin}
              pinnedIds={pinnedIds}
              importantIds={importantIds}
              onBulkRestore={activeFolderSystemType === 'trash' ? handleBulkRestore : activeFolderSystemType === 'archive' ? handleBulkRestoreFromArchive : undefined}
              onBulkLabel={handleBulkLabel}
              onBulkStar={handleBulkStar}
              messageLabels={messageLabels}
              userEmail={userEmail || undefined}
              showPreview={wmSettings.showPreview}
              showCategoryTabs={activeFolderSystemType === 'inbox' || activeFolderId === VIRTUAL_ALL}
            />

          </div>{/* end mail layout wrapper */}
        </>
      ) : activeApp === 'calendar' ? (
        <CalendarView />
      ) : activeApp === 'contacts' ? (
        <ContactsView onCompose={(email) => openCompose({ intent: 'new', to: email })} />
      ) : activeApp === 'drive' ? (
        <DriveView />
      ) : activeApp === 'settings' ? (
        <SettingsView userEmail={userEmail || undefined} userName={userEmail || undefined} initialSection={settingsInitialSection} />
      ) : null}

      {showDMModal && (
        <MailDMModal
          isMobile={isMobile}
          rect={resolvedDMModalRect}
          userEmail={userEmail || undefined}
          onUnreadChange={setDMUnreadCount}
          onClose={() => setShowDMModal(false)}
          onComposeToAddress={(email) => openCompose({ intent: 'new', to: email, focusSubjectOnOpen: true })}
          onStartWindowDrag={startDMModalDrag}
          onStartResize={startDMModalResize}
        />
      )}

      <MFASetupPromptModal
        onGoToSettings={() => {
          setSettingsInitialSection('security');
          setActiveApp('settings');
        }}
      />

      {/* Slide-in reading pane overlay */}
      <MailReadingPanel
        selectedMessageId={selectedMessageId}
        selectedMessage={selectedMessage}
        messages={messages}
        searchResults={searchResults}
        threads={threads}
        isMobile={isMobile}
        readingPaneWidth={readingPaneWidth}
        setReadingPaneWidth={setReadingPaneWidth}
        messageLoading={messageLoading}
        folders={folders}
        activeFolderSystemType={activeFolderSystemType}
        wmSettings={wmSettings}
        swipeDeltaX={swipeDeltaX}
        setSwipeDeltaX={setSwipeDeltaX}
        swipeTouchStartRef={swipeTouchStartRef}
        onClose={() => setSelectedMessageId(null)}
        onSelectMessage={handleSelectMessage}
        onOpenCompose={openCompose}
        onArchive={activeFolderSystemType !== 'archive' && activeFolderSystemType !== 'trash' && activeFolderSystemType !== 'spam' && activeFolderSystemType !== 'junk' ? handleArchive : undefined}
        onSpam={folders.some((f) => f.system_type === 'spam' || f.system_type === 'junk') && activeFolderSystemType !== 'spam' && activeFolderSystemType !== 'junk' && activeFolderSystemType !== 'trash' ? handleSpam : undefined}
        onNotSpam={activeFolderSystemType === 'spam' || activeFolderSystemType === 'junk' ? handleNotSpam : undefined}
        onDelete={handleDelete}
        onMove={handleMove}
        onPrint={selectedMessage ? () => printMessage(selectedMessage, t) : undefined}
        onRestore={selectedMessageId && (activeFolderSystemType === 'trash' || activeFolderSystemType === 'archive') ? () => activeFolderSystemType === 'archive' ? handleRestoreFromArchive(selectedMessageId) : handleRestore(selectedMessageId) : undefined}
        onComposeToAddress={(address) => openCompose({ intent: 'new', to: address })}
        onBlockSender={handleBlockSender}
        onSnooze={activeFolderSystemType !== 'trash' ? handleSnooze : undefined}
        onToggleRead={selectedMessageId ? () => { const m = findVisibleMessage(selectedMessageId); if (m?.read) handleMarkUnread(); else void handleMarkRead(); } : undefined}
        isRead={selectedMessageId ? findVisibleMessage(selectedMessageId)?.read : undefined}
        onStar={selectedMessageId ? () => { const m = findVisibleMessage(selectedMessageId); if (m) handleStar(m.id, !m.starred); } : undefined}
        isStarred={selectedMessageId ? findVisibleMessage(selectedMessageId)?.starred : undefined}
        onToggleThreadMute={selectedNotificationThreadId ? handleToggleThreadMute : undefined}
        isThreadMuted={selectedThreadMuted}
        threadMessages={threadMessages.length > 1 ? threadMessages : undefined}
        onSelectThread={handleSelectMessage}
        userEmail={userEmail || undefined}
        addToast={addToast}
      />

      {/* Spam Report Dialog */}
      {spamDialogMessageId && (() => {
        const spamTargetMsg = findVisibleMessage(spamDialogMessageId);
        const fromAddr = spamTargetMsg?.from_addr ?? '';
        const fromName = spamTargetMsg?.from_name ?? '';
        return (
          <SpamReportDialog
            fromAddr={fromAddr}
            fromName={fromName}
            onConfirm={(opts) => {
              const id = spamDialogMessageId;
              setSpamDialogMessageId(null);
              executeSpam(id, opts);
            }}
            onCancel={() => setSpamDialogMessageId(null)}
          />
        );
      })()}

      {composeContext && (
        <ComposeModal
          intent={composeContext.intent}
          sourceMessage={composeContext.source}
          draftMessage={composeContext.draft}
          initialTo={composeContext.to}
          initialSubject={composeContext.initialSubject}
          initialBody={composeContext.initialBody}
          focusSubjectOnOpen={composeContext.focusSubjectOnOpen}
          userEmail={userEmail}
          isMobile={isMobile}
          onClose={closeCompose}
          onArchiveSource={(composeContext.intent === 'reply' || composeContext.intent === 'reply_all') && composeContext.source
            ? () => handleArchiveById(composeContext.source!.id)
            : undefined}
          onAfterSend={() => { setTimeout(() => refreshRef.current(), 1500); }}
        />
      )}

      {/* Mobile FAB — compose button when sidebar is hidden */}
      {isMobile && !selectedMessageId && !composeContext && (

        <button
          aria-label={t('misc.mailPage.composeMail')}
          onClick={() => openCompose({ intent: 'new' })}
          style={{
            position: 'fixed',
            bottom: '24px',
            right: '20px',
            zIndex: 200,
            width: '52px',
            height: '52px',
            borderRadius: '50%',
            background: 'var(--color-accent)',
            color: '#fff',
            border: 'none',
            boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            transition: 'background 100ms ease, transform 100ms ease',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent-hover)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent)'; }}
        ><PencilSquareIcon style={{ width: '24px', height: '24px' }} /></button>
      )}

      {contextMenu && (() => {
        const ctxMsg = findVisibleMessage(contextMenu.id);
        return (
          <ContextMenu
            x={contextMenu.x}
            y={contextMenu.y}
            onClose={() => setContextMenu(null)}
            items={[
              {
                label: t('misc.mailPage.ctx.reply'),
                onClick: () => {
                  handleSelectMessage(contextMenu.id);
                  setPendingCompose({ intent: 'reply', messageId: contextMenu.id });
                },
              },
              {
                label: t('misc.mailPage.ctx.forward'),
                onClick: () => {
                  handleSelectMessage(contextMenu.id);
                  setPendingCompose({ intent: 'forward', messageId: contextMenu.id });
                },
              },
              {
                label: ctxMsg?.starred ? t('misc.mailPage.ctx.unstar') : t('misc.mailPage.ctx.star'),
                onClick: () => ctxMsg && handleStar(contextMenu.id, !ctxMsg.starred),
              },
              ctxMsg?.read
                ? {
                    label: t('misc.mailPage.ctx.markUnread'),
                    onClick: () => handleToggleReadMessage(contextMenu.id, false),
                  }
                : {
                    label: t('misc.mailPage.ctx.markRead'),
                    onClick: () => handleToggleReadMessage(contextMenu.id, true),
                  },
              {
                label: t('misc.mailPage.ctx.label'),
                children: ([
                  { color: '#ef4444', name: t('misc.mailPage.ctx.labelRed') },
                  { color: '#f97316', name: t('misc.mailPage.ctx.labelOrange') },
                  { color: '#eab308', name: t('misc.mailPage.ctx.labelYellow') },
                  { color: '#22c55e', name: t('misc.mailPage.ctx.labelGreen') },
                  { color: '#3b82f6', name: t('misc.mailPage.ctx.labelBlue') },
                  { color: '#a855f7', name: t('misc.mailPage.ctx.labelPurple') },
                ]).map(({ color, name }) => ({
                  label: `${messageLabels[contextMenu.id] === color ? '✓ ' : '   '}${name}`,
                  onClick: () => setLabel(contextMenu.id, messageLabels[contextMenu.id] === color ? null : color),
                })),
              },
              {
                label: t('misc.mailPage.ctx.moveToFolder'),
                children: folders
                  .filter((f) => f.id !== activeFolderId && f.system_type !== 'drafts')
                  .map((f) => ({
                    label: f.name,
                    onClick: () => {
                      const msg = findVisibleMessage(contextMenu.id);
                      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
                      removeVisibleMessages([contextMenu.id]);
                      if (selectedMessageId === contextMenu.id) setSelectedMessageId(null);
                      moveMessage(contextMenu.id, f.id)
                        .then(() => addToast(t('misc.mailPage.movedTo', { name: f.name })))
                        .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
                    },
                  })),
              },
              { separator: true } as { separator: true; label: string; onClick: () => void },
              {
                label: t('misc.mailPage.ctx.delete'),
                danger: true,
                onClick: () => handleDeleteById(contextMenu.id),
              },
            ]}
          />
        );
      })()}

      {showSpotlight && (
        <SpotlightSearch
          onClose={() => { setShowSpotlight(false); setSpotlightMoveId(null); }}
          folders={folders}
          onSelectFolder={(id) => { handleSelectFolder(id); setShowSpotlight(false); setSpotlightMoveId(null); }}
          onCompose={() => { openCompose({ intent: 'new' }); setShowSpotlight(false); }}
          onComposeToAddress={(email) => { openCompose({ intent: 'new', to: email }); setShowSpotlight(false); }}
          onSelectMessage={(id) => { handleSelectMessage(id); setShowSpotlight(false); }}
          onOpenCalendar={() => { setActiveApp('calendar'); setShowSpotlight(false); }}
          onOpenDrive={() => { setActiveApp('drive'); setShowSpotlight(false); }}
          onOpenSettings={(sectionId?: SectionId) => { if (sectionId) setSettingsInitialSection(sectionId); setActiveApp('settings'); setShowSpotlight(false); }}
          onOpenNotifications={() => { setShowSpotlight(false); window.dispatchEvent(new CustomEvent('toggleNotificationCenter')); }}
          onSearch={(q) => { handleSearch(q); setActiveApp('mail'); setShowSpotlight(false); }}
          onComposeWithTemplate={(t) => { openCompose({ intent: 'new', initialSubject: t.subject, initialBody: t.body }); setShowSpotlight(false); }}
          movingMessageId={spotlightMoveId ?? undefined}
          onMoveMessage={(folderId: string) => {
            handleMove(folderId);
            setShowSpotlight(false);
            setSpotlightMoveId(null);
          }}
        />
      )}

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
      {showShortcuts && <ShortcutHelp onClose={() => setShowShortcuts(false)} />}

    </div>
  );
}
