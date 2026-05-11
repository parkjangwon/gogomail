'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { deleteMessage, restoreMessage, bulkRestoreMessages, starMessage, markRead, moveMessage, bulkMarkRead, searchMessages, sendMessage, ComposeIntent, MessageDetail, MessageSummary } from '@/lib/api';
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
import { AccentPicker } from '@/components/AccentPicker';

export default function MailPage() {
  const router = useRouter();

  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [userEmail, setUserEmail] = useState('');
  const [composeContext, setComposeContext] = useState<{
    intent: ComposeIntent;
    source?: MessageDetail;
    draft?: MessageDetail;
    to?: string;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [advancedFilters, setAdvancedFilters] = useState<AdvancedFilters>({});
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [showShortcuts, setShowShortcuts] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [contextMenu, setContextMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [pendingCompose, setPendingCompose] = useState<{ intent: 'reply' | 'forward'; messageId: string } | null>(null);
  const [listPaneWidth, setListPaneWidth] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_list_pane_width') ?? '380', 10) || 380; } catch { return 380; }
  });
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null);
  const [readingPanePosition, setReadingPanePosition] = useState<'right' | 'bottom'>(() => {
    try { return (localStorage.getItem('webmail_pane_position') ?? 'right') as 'right' | 'bottom'; } catch { return 'right'; }
  });
  const isMobile = useIsMobile();
  const gPrefixRef = useRef(false);
  const isOnline = useIsOnline();

  const pendingDeletesRef = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const addToast = useCallback((message: string, type: ToastItem['type'] = 'success', options?: { duration?: number; action?: ToastItem['action'] }) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { id, message, type, ...options }]);
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

  // Update document title + favicon badge with total unread count
  useEffect(() => {
    const totalUnread = folders.reduce((sum, f) => sum + (f.unread ?? 0), 0);
    document.title = totalUnread > 0 ? `GoGoMail (${totalUnread})` : 'GoGoMail';

    // Draw favicon with optional badge on 32x32 canvas
    try {
      const size = 32;
      const canvas = document.createElement('canvas');
      canvas.width = size; canvas.height = size;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      // Envelope icon
      ctx.fillStyle = '#6366f1';
      ctx.beginPath();
      ctx.roundRect(2, 6, 28, 20, 3);
      ctx.fill();
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.moveTo(2, 8); ctx.lineTo(16, 18); ctx.lineTo(30, 8);
      ctx.strokeStyle = '#fff'; ctx.lineWidth = 2; ctx.stroke();
      // Badge
      if (totalUnread > 0) {
        const label = totalUnread > 99 ? '99+' : String(totalUnread);
        const badgeR = label.length > 2 ? 9 : 7;
        const bx = size - badgeR - 1, by = badgeR + 1;
        ctx.fillStyle = '#ef4444';
        ctx.beginPath(); ctx.arc(bx, by, badgeR, 0, Math.PI * 2); ctx.fill();
        ctx.fillStyle = '#fff';
        ctx.font = `bold ${label.length > 2 ? 7 : 9}px sans-serif`;
        ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.fillText(label, bx, by + 0.5);
      }
      let link = document.querySelector<HTMLLinkElement>('link[rel~="icon"]');
      if (!link) { link = document.createElement('link'); link.rel = 'icon'; document.head.appendChild(link); }
      link.href = canvas.toDataURL('image/png');
    } catch { /* canvas not supported */ }
  }, [folders]);

  const [mustChangePassword, setMustChangePassword] = useState(false);
  const [sessionWarning, setSessionWarning] = useState<string | null>(null);

  // Check auth on mount, load email
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) { router.push('/login'); return; }
    setUserEmail(localStorage.getItem('webmail_email') ?? '');
    if (localStorage.getItem('webmail_must_change_password') === '1') {
      setMustChangePassword(true);
    }
  }, [router]);

  // Session expiry warning: check every 60s, warn when < 10 min left
  useEffect(() => {
    function check() {
      const expiresAt = localStorage.getItem('webmail_token_expires_at');
      if (!expiresAt) { setSessionWarning(null); return; }
      const msLeft = new Date(expiresAt).getTime() - Date.now();
      if (msLeft <= 0) { setSessionWarning('세션이 만료되었습니다. 다시 로그인해 주세요.'); return; }
      const minsLeft = Math.floor(msLeft / 60000);
      if (minsLeft < 10) setSessionWarning(`세션이 ${minsLeft}분 후 만료됩니다.`);
      else setSessionWarning(null);
    }
    check();
    const id = setInterval(check, 60000);
    return () => clearInterval(id);
  }, []);

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

  useEffect(() => {
    if (!pendingCompose || !selectedMessage || selectedMessage.id !== pendingCompose.messageId) return;
    setComposeContext({ intent: pendingCompose.intent, source: selectedMessage });
    setPendingCompose(null);
  }, [pendingCompose, selectedMessage]);

  // Mark selected message as read locally + server after 1.5s (skip drafts)
  useEffect(() => {
    if (!selectedMessageId || activeFolderSystemType === 'drafts') return;
    let cancelled = false;
    const timer = setTimeout(() => {
      if (cancelled) return;
      setMessages((prev) => {
        const msg = prev.find((m) => m.id === selectedMessageId);
        if (msg && !msg.read) {
          adjustUnread(activeFolderId, -1);
          markRead(selectedMessageId, true).catch(() => {});
        }
        return prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m));
      });
    }, 1500);
    return () => { cancelled = true; clearTimeout(timer); };
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

  const handleDeleteById = useCallback((id: string) => {
    const msgToDelete = messages.find((m) => m.id === id);
    const currentIdx = messages.findIndex((m) => m.id === id);
    const nextMsg = messages[currentIdx + 1] ?? messages[currentIdx - 1];
    setMessages((prev) => prev.filter((m) => m.id !== id));
    if (selectedMessageId === id) setSelectedMessageId(nextMsg?.id ?? null);

    const timer = setTimeout(() => {
      pendingDeletesRef.current.delete(id);
      deleteMessage(id).catch(() => {});
    }, 5000);
    pendingDeletesRef.current.set(id, timer);

    addToast('메일을 삭제했습니다', 'info', {
      duration: 5000,
      action: {
        label: '실행 취소',
        onClick: () => {
          const t = pendingDeletesRef.current.get(id);
          if (t) { clearTimeout(t); pendingDeletesRef.current.delete(id); }
          if (msgToDelete) setMessages((prev) => [msgToDelete, ...prev]);
        },
      },
    });
  }, [messages, selectedMessageId, setMessages, addToast]);

  const handleDelete = useCallback(() => {
    if (!selectedMessageId) return;
    handleDeleteById(selectedMessageId);
  }, [selectedMessageId, handleDeleteById]);

  const handleBulkDelete = useCallback(async (ids: string[]) => {
    setMessages((prev) => prev.filter((m) => !ids.includes(m.id)));
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    const results = await Promise.allSettled(ids.map((id) => deleteMessage(id)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed > 0) {
      addToast(`${ids.length - failed}개 삭제, ${failed}개 실패`, 'error');
    } else {
      addToast(`${ids.length}개 삭제했습니다`);
    }
  }, [selectedMessageId, setMessages, addToast]);

  const handleRestore = useCallback(async (id: string) => {
    setMessages((prev) => prev.filter((m) => m.id !== id));
    if (selectedMessageId === id) setSelectedMessageId(null);
    try { await restoreMessage(id); addToast('메일을 복구했습니다'); }
    catch { addToast('복구에 실패했습니다', 'error'); }
  }, [selectedMessageId, setMessages, addToast]);

  const handleBulkRestore = useCallback(async (ids: string[]) => {
    setMessages((prev) => prev.filter((m) => !ids.includes(m.id)));
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    try { await bulkRestoreMessages(ids); addToast(`${ids.length}개를 복구했습니다`); }
    catch { addToast('복구에 실패했습니다', 'error'); }
  }, [selectedMessageId, setMessages, addToast]);

  const handleBulkMarkRead = useCallback(async (ids: string[]) => {
    const unreadCount = messages.filter((m) => ids.includes(m.id) && !m.read).length;
    setMessages((prev) => prev.map((m) => ids.includes(m.id) ? { ...m, read: true } : m));
    if (unreadCount > 0) adjustUnread(activeFolderId, -unreadCount);
    try {
      await bulkMarkRead(ids, true);
      addToast(`${ids.length}개를 읽음으로 표시했습니다`, 'info');
    } catch {
      setMessages((prev) => prev.map((m) => ids.includes(m.id) ? { ...m, read: false } : m));
      if (unreadCount > 0) adjustUnread(activeFolderId, unreadCount);
      addToast('읽음 표시에 실패했습니다', 'error');
    }
  }, [messages, setMessages, adjustUnread, activeFolderId, addToast]);

  const handleMarkAllRead = useCallback(async () => {
    const unreadIds = messages.filter((m) => !m.read).map((m) => m.id);
    if (unreadIds.length === 0) return;
    setMessages((prev) => prev.map((m) => ({ ...m, read: true })));
    adjustUnread(activeFolderId, -unreadIds.length);
    try {
      await bulkMarkRead(unreadIds, true);
      addToast(`${unreadIds.length}개를 읽음으로 표시했습니다`, 'info');
    } catch {
      setMessages((prev) => prev.map((m) => unreadIds.includes(m.id) ? { ...m, read: false } : m));
      adjustUnread(activeFolderId, unreadIds.length);
      addToast('읽음 표시에 실패했습니다', 'error');
    }
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

  // Persist list pane width and pane layout
  useEffect(() => {
    localStorage.setItem('webmail_list_pane_width', String(listPaneWidth));
  }, [listPaneWidth]);
  useEffect(() => {
    localStorage.setItem('webmail_pane_position', readingPanePosition);
  }, [readingPanePosition]);

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
        case 'a':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'reply_all', source: selectedMessage });
          }
          break;
        case 'f':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'forward', source: selectedMessage });
          }
          break;
        case 'e': {
          if (selectedMessageId && !composeContext) {
            const archiveFolder = folders.find((f) => f.system_type === 'archive');
            if (archiveFolder) {
              void moveMessage(selectedMessageId, archiveFolder.id).then(() => {
                setMessages((prev) => prev.filter((m) => m.id !== selectedMessageId));
                setSelectedMessageId(null);
              }).catch(() => {});
            }
          }
          break;
        }
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
        case '[':
          if (!composeContext) setSidebarCollapsed((v) => !v);
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

  const refreshRef = useRef(refresh);
  useEffect(() => { refreshRef.current = refresh; }, [refresh]);
  useEffect(() => {
    const id = setInterval(() => {
      if (document.visibilityState === 'visible') refreshRef.current();
    }, 30_000);
    return () => clearInterval(id);
  }, []);

  // Request notification permission once on mount
  useEffect(() => {
    if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
      Notification.requestPermission();
    }
  }, []);

  // Detect new unread messages after refresh and notify
  const seenMsgIdsRef = useRef<Set<string> | null>(null);
  useEffect(() => {
    if (messages.length === 0) return;
    if (seenMsgIdsRef.current === null) {
      seenMsgIdsRef.current = new Set(messages.map((m) => m.id));
      return;
    }
    const newUnread = messages.filter((m) => !m.read && !seenMsgIdsRef.current!.has(m.id));
    messages.forEach((m) => seenMsgIdsRef.current!.add(m.id));
    if (newUnread.length > 0 && typeof Notification !== 'undefined' && Notification.permission === 'granted' && document.visibilityState !== 'visible') {
      const title = newUnread.length === 1
        ? `새 메일: ${newUnread[0].from_name || newUnread[0].from_addr}`
        : `새 메일 ${newUnread.length}개`;
      const n = new Notification(title, {
        body: newUnread.length === 1 ? (newUnread[0].subject || '(제목 없음)') : undefined,
        icon: '/favicon.ico',
      });
      n.onclick = () => window.focus();
    }
  }, [messages]);

  // Reset seen IDs when folder changes (avoid false notifications on folder switch)
  useEffect(() => { seenMsgIdsRef.current = null; }, [activeFolderId]);

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

      {sessionWarning && (
        <div
          role="alert"
          style={{
            position: 'fixed',
            top: mustChangePassword ? '33px' : 0,
            left: 0,
            right: 0,
            zIndex: 499,
            background: '#92400e',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px 40px',
            fontWeight: 500,
          }}
        >
          {sessionWarning}
          <button
            onClick={handleLogout}
            style={{ marginLeft: '12px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >다시 로그인</button>
          <button
            onClick={() => setSessionWarning(null)}
            style={{ marginLeft: '8px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
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
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed((v) => !v)}
        onDropMessage={(messageId, folderId) => {
          setMessages((prev) => prev.filter((m) => m.id !== messageId));
          if (selectedMessageId === messageId) setSelectedMessageId(null);
          moveMessage(messageId, folderId)
            .then(() => addToast('메일을 이동했습니다'))
            .catch(() => addToast('이동에 실패했습니다', 'error'));
        }}
      />

      <div style={{ flex: 1, display: 'flex', flexDirection: (!isMobile && readingPanePosition === 'bottom') ? 'column' : 'row', overflow: 'hidden', minWidth: 0 }}>

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
          folders={folders}
          onBulkMove={async (ids, folderId) => {
            setMessages((prev) => prev.filter((m) => !ids.includes(m.id)));
            if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
            await Promise.allSettled(ids.map((id) => moveMessage(id, folderId)));
            addToast(`${ids.length}개를 이동했습니다`);
          }}
          onRefresh={refresh}
          refreshing={refreshing}
          isMobile={isMobile}
          onOpenSidebar={() => setMobileSidebarOpen(true)}
          onContextMenuMessage={(id, x, y) => setContextMenu({ id, x, y })}
          onMarkAllRead={activeFolderSystemType !== 'trash' ? handleMarkAllRead : undefined}
          paneWidth={(!isMobile && readingPanePosition === 'right') ? listPaneWidth : undefined}
          fullWidth={!isMobile && readingPanePosition === 'bottom'}
          bottomLayout={!isMobile && readingPanePosition === 'bottom'}
          searchQuery={searchResults !== null ? searchQuery : undefined}
          emptyFolderLabel={activeFolderSystemType === 'trash' ? '휴지통 비우기' : undefined}
          onEmptyFolder={activeFolderSystemType === 'trash' ? () => handleBulkDelete(messages.map((m) => m.id)) : undefined}
          onDeleteMessage={handleDeleteById}
          onBulkRestore={activeFolderSystemType === 'trash' ? handleBulkRestore : undefined}
        />
      )}

      {!isMobile && readingPanePosition === 'right' && (
        <div
          aria-hidden="true"
          onMouseDown={(e) => {
            dragRef.current = { startX: e.clientX, startWidth: listPaneWidth };
            const onMove = (ev: MouseEvent) => {
              if (!dragRef.current) return;
              const delta = ev.clientX - dragRef.current.startX;
              setListPaneWidth(Math.max(240, Math.min(600, dragRef.current.startWidth + delta)));
            };
            const onUp = () => {
              dragRef.current = null;
              document.removeEventListener('mousemove', onMove);
              document.removeEventListener('mouseup', onUp);
            };
            document.addEventListener('mousemove', onMove);
            document.addEventListener('mouseup', onUp);
          }}
          style={{
            width: '4px',
            flexShrink: 0,
            cursor: 'ew-resize',
            background: 'transparent',
            transition: 'background 100ms ease',
            zIndex: 1,
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-accent)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
        />
      )}

      {(!isMobile || selectedMessageId) && (() => {
          const msgList = searchResults ?? messages;
          const curIdx = msgList.findIndex((m) => m.id === selectedMessageId);
          const prevId = curIdx > 0 ? msgList[curIdx - 1].id : null;
          const nextId = curIdx !== -1 && curIdx < msgList.length - 1 ? msgList[curIdx + 1].id : null;
          return (
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
          onPrev={prevId ? () => handleSelectMessage(prevId) : undefined}
          onNext={nextId ? () => handleSelectMessage(nextId) : undefined}
          onQuickReply={selectedMessage ? async (body) => {
            await sendMessage({
              to: [{ address: selectedMessage.from_addr, name: selectedMessage.from_name || undefined }],
              subject: `Re: ${selectedMessage.subject || ''}`,
              text_body: body,
              intent: 'reply',
              source_message_id: selectedMessage.id,
            });
            addToast('답장을 전송했습니다');
          } : undefined}
          onRestore={activeFolderSystemType === 'trash' && selectedMessageId ? () => handleRestore(selectedMessageId) : undefined}
          onComposeToAddress={(address) => setComposeContext({ intent: 'new', to: address })}
        />
          );
        })()}

      </div>{/* end layout wrapper */}

      {composeContext && (
        <ComposeModal
          intent={composeContext.intent}
          sourceMessage={composeContext.source}
          draftMessage={composeContext.draft}
          initialTo={composeContext.to}
          userEmail={userEmail}
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
                  setPendingCompose({ intent: 'reply', messageId: contextMenu.id });
                },
              },
              {
                label: '전달',
                onClick: () => {
                  handleSelectMessage(contextMenu.id);
                  setPendingCompose({ intent: 'forward', messageId: contextMenu.id });
                },
              },
              {
                label: ctxMsg?.starred ? '별표 해제' : '별표 추가',
                onClick: () => ctxMsg && handleStar(contextMenu.id, !ctxMsg.starred),
              },
              ctxMsg?.read
                ? {
                    label: '읽지 않음으로',
                    onClick: () => {
                      setMessages((prev) =>
                        prev.map((m) => (m.id === contextMenu.id ? { ...m, read: false } : m))
                      );
                      adjustUnread(activeFolderId, 1);
                      markRead(contextMenu.id, false).catch(() => {});
                    },
                  }
                : {
                    label: '읽음으로',
                    onClick: () => {
                      setMessages((prev) =>
                        prev.map((m) => (m.id === contextMenu.id ? { ...m, read: true } : m))
                      );
                      adjustUnread(activeFolderId, -1);
                      markRead(contextMenu.id, true).catch(() => {});
                    },
                  },
              {
                label: '삭제',
                danger: true,
                onClick: () => handleDeleteById(contextMenu.id),
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
        {!isMobile && (
          <button
            aria-label={readingPanePosition === 'right' ? '읽기 창 아래로' : '읽기 창 오른쪽으로'}
            title={readingPanePosition === 'right' ? '읽기 창 아래로' : '읽기 창 오른쪽으로'}
            onClick={() => setReadingPanePosition((p) => p === 'right' ? 'bottom' : 'right')}
            style={{ fontSize: '14px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >{readingPanePosition === 'right' ? '⬇' : '➡'}</button>
        )}
        <AccentPicker />
        <LocaleSelector />
        <ThemeToggle inline />
      </div>
    </div>
  );
}
