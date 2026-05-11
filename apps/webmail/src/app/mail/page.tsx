'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { deleteMessage, starMessage, markRead, moveMessage, bulkMarkRead, searchMessages, ComposeIntent, MessageDetail, MessageSummary } from '@/lib/api';
import { AdvancedFilters } from '@/components/Sidebar';
import { useMailList } from '@/hooks/useMailList';
import { useMessage } from '@/hooks/useMessage';
import { useIsMobile } from '@/hooks/useIsMobile';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ReadingPane } from '@/components/ReadingPane';
import { ComposeModal } from '@/components/ComposeModal';
import { ThemeToggle } from '@/components/ThemeToggle';
import { LocaleSelector } from '@/components/common/LocaleSelector';
import { ToastContainer, ToastItem } from '@/components/Toast';
import { ShortcutHelp } from '@/components/ShortcutHelp';

export default function MailPage() {
  const router = useRouter();

  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [userEmail, setUserEmail] = useState('');
  const [composeContext, setComposeContext] = useState<{
    intent: ComposeIntent;
    source?: MessageDetail;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [advancedFilters, setAdvancedFilters] = useState<AdvancedFilters>({});
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [showShortcuts, setShowShortcuts] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const isMobile = useIsMobile();

  const addToast = useCallback((message: string, type: ToastItem['type'] = 'success') => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { id, message, type }]);
  }, []);
  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const { folders, messages, setMessages, foldersLoading, messagesLoading, hasMore, loadingMore, loadMore, adjustUnread, refresh, refreshing } =
    useMailList(activeFolderId);

  // Set default folder to inbox UUID once folders are loaded
  useEffect(() => {
    if (activeFolderId || folders.length === 0) return;
    const inbox = folders.find((f) => f.system_type === 'inbox') ?? folders[0];
    if (inbox) setActiveFolderId(inbox.id);
  }, [folders, activeFolderId]);

  const { message: selectedMessage, loading: messageLoading } =
    useMessage(selectedMessageId);

  // Check auth on mount, load email
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) { router.push('/login'); return; }
    setUserEmail(localStorage.getItem('webmail_email') ?? '');
  }, [router]);

  const handleLogout = useCallback(() => {
    localStorage.removeItem('webmail_token');
    localStorage.removeItem('webmail_email');
    router.push('/login');
  }, [router]);

  // Mark selected message as read locally + update sidebar badge
  useEffect(() => {
    if (!selectedMessageId) return;
    setMessages((prev) => {
      const msg = prev.find((m) => m.id === selectedMessageId);
      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
      return prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m));
    });
  }, [selectedMessageId, setMessages, adjustUnread, activeFolderId]);

  const handleMarkUnread = useCallback(async () => {
    if (!selectedMessageId) return;
    setMessages((prev) =>
      prev.map((m) => (m.id === selectedMessageId ? { ...m, read: false } : m))
    );
    adjustUnread(activeFolderId, 1);
    addToast('읽지 않음으로 표시했습니다', 'info');
    markRead(selectedMessageId, false).catch(() => {
      setMessages((prev) =>
        prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m))
      );
      adjustUnread(activeFolderId, -1);
    });
  }, [selectedMessageId, setMessages, adjustUnread, activeFolderId, addToast]);

  const runSearch = useCallback(async (q: string, filters: AdvancedFilters) => {
    if (!q.trim() && !filters.from && !filters.since && !filters.until && !filters.has_attachment) {
      setSearchResults(null);
      return;
    }
    setSearchLoading(true);
    try {
      const res = await searchMessages({
        q: q.trim() || undefined,
        from: filters.from || undefined,
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
    runSearch(q, advancedFilters);
  }, [advancedFilters, runSearch]);

  const handleFilterChange = useCallback((filters: AdvancedFilters) => {
    setAdvancedFilters(filters);
    runSearch(searchQuery, filters);
  }, [searchQuery, runSearch]);

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

  const handleDelete = useCallback(async () => {
    if (!selectedMessageId) return;
    try {
      await deleteMessage(selectedMessageId);
      setMessages((prev) => prev.filter((m) => m.id !== selectedMessageId));
      setSelectedMessageId(null);
      addToast('메일을 삭제했습니다');
    } catch {
      addToast('삭제에 실패했습니다', 'error');
    }
  }, [selectedMessageId, setMessages, addToast]);

  const handleBulkDelete = useCallback(async (ids: string[]) => {
    setMessages((prev) => prev.filter((m) => !ids.includes(m.id)));
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    await Promise.allSettled(ids.map((id) => deleteMessage(id)));
    addToast(`${ids.length}개 삭제했습니다`);
  }, [selectedMessageId, setMessages, addToast]);

  const handleBulkMarkRead = useCallback(async (ids: string[]) => {
    const unreadCount = messages.filter((m) => ids.includes(m.id) && !m.read).length;
    setMessages((prev) => prev.map((m) => ids.includes(m.id) ? { ...m, read: true } : m));
    if (unreadCount > 0) adjustUnread(activeFolderId, -unreadCount);
    bulkMarkRead(ids, true).catch(() => {});
    addToast(`${ids.length}개를 읽음으로 표시했습니다`, 'info');
  }, [messages, setMessages, adjustUnread, activeFolderId, addToast]);

  const handleMove = useCallback(async (folderId: string) => {
    if (!selectedMessageId) return;
    const id = selectedMessageId;
    setMessages((prev) => prev.filter((m) => m.id !== id));
    setSelectedMessageId(null);
    moveMessage(id, folderId)
      .then(() => addToast('메일을 이동했습니다'))
      .catch(() => addToast('이동에 실패했습니다', 'error'));
  }, [selectedMessageId, setMessages, addToast]);

  const handleStar = useCallback(async (id: string, starred: boolean) => {
    setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred } : m)));
    starMessage(id, starred).catch(() => {
      setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred: !starred } : m)));
    });
  }, [setMessages]);

  // Keyboard shortcuts (skip when typing in input/textarea/contenteditable)
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      const list = searchResults ?? messages;
      const currentIdx = list.findIndex((m) => m.id === selectedMessageId);

      switch (e.key) {
        case 'j': {
          const next = list[currentIdx + 1];
          if (next) setSelectedMessageId(next.id);
          break;
        }
        case 'k': {
          const prev = list[currentIdx - 1];
          if (prev) setSelectedMessageId(prev.id);
          break;
        }
        case 'c':
          if (!composeContext) { e.preventDefault(); setComposeContext({ intent: 'new' }); }
          break;
        case 'r':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'reply', source: selectedMessage });
          }
          break;
        case 'f':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'forward', source: selectedMessage });
          }
          break;
        case '#':
        case 'Delete':
          if (selectedMessageId && !composeContext) handleDelete();
          break;
        case '?':
          setShowShortcuts((v) => !v);
          break;
        case 'Escape':
          if (showShortcuts) setShowShortcuts(false);
          else if (composeContext) setComposeContext(null);
          else setSelectedMessageId(null);
          break;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [messages, searchResults, selectedMessageId, selectedMessage, composeContext, showShortcuts, handleDelete]);

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
        로딩 중...
      </div>
    );
  }

  return (
    <div
      style={{
        display: 'flex',
        height: '100vh',
        overflow: 'hidden',
        background: 'var(--color-bg-primary)',
      }}
    >
      <Sidebar
        folders={folders}
        activeFolderId={activeFolderId}
        onSelectFolder={(id) => { handleSelectFolder(id); setMobileSidebarOpen(false); }}
        onCompose={() => { setComposeContext({ intent: 'new' }); setMobileSidebarOpen(false); }}
        onSearch={handleSearch}
        searchQuery={searchQuery}
        advancedFilters={advancedFilters}
        onAdvancedFilterChange={handleFilterChange}
        userName={userEmail || '사용자'}
        onLogout={handleLogout}
        isMobile={isMobile}
        isOpen={mobileSidebarOpen}
        onClose={() => setMobileSidebarOpen(false)}
      />

      {(!isMobile || !selectedMessageId) && (
        <MessageList
          messages={searchResults ?? messages}
          selectedId={selectedMessageId}
          onSelect={handleSelectMessage}
          loading={searchResults !== null ? searchLoading : messagesLoading}
          emptyLabel={searchResults !== null ? (searchQuery ? `"${searchQuery}" 검색 결과가 없습니다` : '검색 결과가 없습니다') : undefined}
          hasMore={searchResults === null ? hasMore : false}
          loadingMore={loadingMore}
          onLoadMore={loadMore}
          onStar={handleStar}
          onBulkDelete={handleBulkDelete}
          onBulkMarkRead={handleBulkMarkRead}
          onRefresh={refresh}
          refreshing={refreshing}
          isMobile={isMobile}
          onOpenSidebar={() => setMobileSidebarOpen(true)}
        />
      )}

      {(!isMobile || selectedMessageId) && (
        <ReadingPane
          message={selectedMessage}
          folders={folders}
          onDelete={handleDelete}
          onReply={() => selectedMessage && setComposeContext({ intent: 'reply', source: selectedMessage })}
          onReplyAll={() => selectedMessage && setComposeContext({ intent: 'reply_all', source: selectedMessage })}
          onForward={() => selectedMessage && setComposeContext({ intent: 'forward', source: selectedMessage })}
          onMarkUnread={handleMarkUnread}
          onMove={handleMove}
          onPrint={() => window.print()}
          loading={messageLoading}
          onBack={isMobile ? () => setSelectedMessageId(null) : undefined}
          isStarred={messages.find((m) => m.id === selectedMessageId)?.starred}
          onStar={selectedMessageId ? (starred) => handleStar(selectedMessageId, starred) : undefined}
        />
      )}

      {composeContext && (
        <ComposeModal
          intent={composeContext.intent}
          sourceMessage={composeContext.source}
          onClose={() => setComposeContext(null)}
        />
      )}

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
      {showShortcuts && <ShortcutHelp onClose={() => setShowShortcuts(false)} />}

      {/* Controls: locale + theme, top-right */}
      <div
        style={{
          position: 'fixed',
          top: '14px',
          right: '16px',
          zIndex: 50,
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <LocaleSelector />
        <ThemeToggle inline />
      </div>
    </div>
  );
}
