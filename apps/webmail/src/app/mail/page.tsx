'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { deleteMessage, starMessage, markRead, moveMessage, bulkMarkRead, searchMessages, ComposeIntent, MessageDetail, MessageSummary } from '@/lib/api';
import { AdvancedFilters } from '@/components/Sidebar';
import { useMailList } from '@/hooks/useMailList';
import { useMessage } from '@/hooks/useMessage';
import { useIsMobile } from '@/hooks/useIsMobile';
import { useIsOnline } from '@/hooks/useIsOnline';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ReadingPane } from '@/components/ReadingPane';
import { ComposeModal } from '@/components/ComposeModal';
import { ThemeToggle } from '@/components/ThemeToggle';
import { LocaleSelector } from '@/components/common/LocaleSelector';
import { ToastContainer, ToastItem } from '@/components/Toast';
import { ShortcutHelp } from '@/components/ShortcutHelp';
import { ContextMenu } from '@/components/ContextMenu';

export default function MailPage() {
  const router = useRouter();

  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [userEmail, setUserEmail] = useState('');
  const [composeContext, setComposeContext] = useState<{
    intent: ComposeIntent;
    source?: MessageDetail;
    draft?: MessageDetail;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [advancedFilters, setAdvancedFilters] = useState<AdvancedFilters>({});
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [showShortcuts, setShowShortcuts] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [contextMenu, setContextMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const isMobile = useIsMobile();
  const gPrefixRef = useRef(false);
  const isOnline = useIsOnline();

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

  // Update document title with total unread count
  useEffect(() => {
    const totalUnread = folders.reduce((sum, f) => sum + (f.unread ?? 0), 0);
    document.title = totalUnread > 0 ? `GoGoMail (${totalUnread})` : 'GoGoMail';
  }, [folders]);

  const [mustChangePassword, setMustChangePassword] = useState(false);

  // Check auth on mount, load email
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) { router.push('/login'); return; }
    setUserEmail(localStorage.getItem('webmail_email') ?? '');
    if (localStorage.getItem('webmail_must_change_password') === '1') {
      setMustChangePassword(true);
    }
  }, [router]);

  const handleLogout = useCallback(() => {
    localStorage.removeItem('webmail_token');
    localStorage.removeItem('webmail_email');
    localStorage.removeItem('webmail_must_change_password');
    router.push('/login');
  }, [router]);

  const activeFolderSystemType = folders.find((f) => f.id === activeFolderId)?.system_type;

  // When a draft message loads, open it in compose instead of ReadingPane
  useEffect(() => {
    if (!selectedMessage || activeFolderSystemType !== 'drafts') return;
    setComposeContext({ intent: 'new', draft: selectedMessage });
    setSelectedMessageId(null);
  }, [selectedMessage, activeFolderSystemType]);

  // Mark selected message as read locally + update sidebar badge (skip drafts)
  useEffect(() => {
    if (!selectedMessageId || activeFolderSystemType === 'drafts') return;
    setMessages((prev) => {
      const msg = prev.find((m) => m.id === selectedMessageId);
      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
      return prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m));
    });
  }, [selectedMessageId, setMessages, adjustUnread, activeFolderId, activeFolderSystemType]);

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

  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleSearch = useCallback((q: string) => {
    setSearchQuery(q);
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    searchDebounceRef.current = setTimeout(() => runSearch(q, advancedFilters), 300);
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

  const handleMarkAllRead = useCallback(async () => {
    const unreadIds = messages.filter((m) => !m.read).map((m) => m.id);
    if (unreadIds.length === 0) return;
    setMessages((prev) => prev.map((m) => ({ ...m, read: true })));
    adjustUnread(activeFolderId, -unreadIds.length);
    bulkMarkRead(unreadIds, true).catch(() => {});
    addToast(`${unreadIds.length}개를 읽음으로 표시했습니다`, 'info');
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

      // g+key two-key folder navigation
      if (gPrefixRef.current) {
        gPrefixRef.current = false;
        const systemTypeMap: Record<string, string> = { i: 'inbox', s: 'sent', d: 'drafts', t: 'trash' };
        const target = systemTypeMap[e.key];
        if (target) {
          const folder = folders.find((f) => f.system_type === target);
          if (folder) { e.preventDefault(); handleSelectFolder(folder.id); return; }
        }
      }

      switch (e.key) {
        case 'g':
          gPrefixRef.current = true;
          setTimeout(() => { gPrefixRef.current = false; }, 1000);
          return;
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
        case 'n':
          if (!composeContext) { e.preventDefault(); setComposeContext({ intent: 'new' }); }
          break;
        case 'u':
          if (selectedMessageId && !composeContext) handleMarkUnread();
          break;
        case 's': {
          if (selectedMessageId && !composeContext) {
            const msg = messages.find((m) => m.id === selectedMessageId);
            if (msg) handleStar(selectedMessageId, !msg.starred);
          }
          break;
        }
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
        case '/': {
          e.preventDefault();
          const searchInput = document.querySelector<HTMLInputElement>('[aria-label="메일 검색"]');
          searchInput?.focus();
          break;
        }
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
      {mustChangePassword && (
        <div
          role="status"
          aria-live="polite"
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            zIndex: 500,
            background: '#b45309',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px 40px',
            fontWeight: 500,
          }}
        >
          보안을 위해 비밀번호를 변경해 주세요.
          <button
            onClick={() => { localStorage.removeItem('webmail_must_change_password'); setMustChangePassword(false); }}
            style={{ marginLeft: '12px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >닫기</button>
        </div>
      )}

      {!isOnline && (
        <div
          role="status"
          aria-live="polite"
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            zIndex: 500,
            background: '#b45309',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px',
            fontWeight: 500,
          }}
        >
          오프라인 상태입니다. 네트워크 연결을 확인하세요.
        </div>
      )}

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
          emptyLabel={searchResults !== null ? (searchQuery ? `"${searchQuery}" 검색 결과가 없습니다` : '검색 결과가 없습니다') : (() => {
            const f = folders.find((f) => f.id === activeFolderId);
            const t = f?.system_type;
            if (t === 'drafts') return '임시 보관된 메일이 없습니다';
            if (t === 'sent') return '보낸 메일이 없습니다';
            if (t === 'trash') return '휴지통이 비어있습니다';
            if (t === 'inbox') return '받은 메일이 없습니다';
            return undefined;
          })()}
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
          onContextMenuMessage={(id, x, y) => setContextMenu({ id, x, y })}
          onMarkAllRead={activeFolderSystemType !== 'trash' ? handleMarkAllRead : undefined}
          emptyFolderLabel={activeFolderSystemType === 'trash' ? '휴지통 비우기' : undefined}
          onEmptyFolder={activeFolderSystemType === 'trash' ? () => handleBulkDelete(messages.map((m) => m.id)) : undefined}
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
          draftMessage={composeContext.draft}
          onClose={() => setComposeContext(null)}
        />
      )}

      {/* Mobile FAB — compose button when sidebar is hidden */}
      {isMobile && !selectedMessageId && !composeContext && (
        <button
          aria-label="새 메일 작성"
          onClick={() => setComposeContext({ intent: 'new' })}
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
            fontSize: '24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            transition: 'background 100ms ease, transform 100ms ease',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent-hover)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent)'; }}
        >✏</button>
      )}

      {contextMenu && (() => {
        const ctxMsg = messages.find((m) => m.id === contextMenu.id);
        return (
          <ContextMenu
            x={contextMenu.x}
            y={contextMenu.y}
            onClose={() => setContextMenu(null)}
            items={[
              {
                label: '답장',
                onClick: () => {
                  handleSelectMessage(contextMenu.id);
                  if (selectedMessage?.id === contextMenu.id) {
                    setComposeContext({ intent: 'reply', source: selectedMessage });
                  }
                },
              },
              {
                label: '전달',
                onClick: () => {
                  handleSelectMessage(contextMenu.id);
                  if (selectedMessage?.id === contextMenu.id) {
                    setComposeContext({ intent: 'forward', source: selectedMessage });
                  }
                },
              },
              {
                label: ctxMsg?.starred ? '별표 해제' : '별표 추가',
                onClick: () => ctxMsg && handleStar(contextMenu.id, !ctxMsg.starred),
              },
              {
                label: '읽지 않음으로',
                onClick: () => {
                  setMessages((prev) =>
                    prev.map((m) => (m.id === contextMenu.id ? { ...m, read: false } : m))
                  );
                  adjustUnread(activeFolderId, 1);
                  markRead(contextMenu.id, false).catch(() => {});
                },
              },
              {
                label: '삭제',
                danger: true,
                onClick: async () => {
                  try {
                    await deleteMessage(contextMenu.id);
                    setMessages((prev) => prev.filter((m) => m.id !== contextMenu.id));
                    if (selectedMessageId === contextMenu.id) setSelectedMessageId(null);
                    addToast('메일을 삭제했습니다');
                  } catch {
                    addToast('삭제에 실패했습니다', 'error');
                  }
                },
              },
            ]}
          />
        );
      })()}

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
