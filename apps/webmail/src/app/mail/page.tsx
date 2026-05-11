'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { deleteMessage, restoreMessage, bulkRestoreMessages, createFolder, renameFolder, deleteFolder, starMessage, markRead, moveMessage, bulkMarkRead, searchMessages, sendMessage, ComposeIntent, MessageDetail, MessageSummary } from '@/lib/api';
import { AdvancedFilters, VIRTUAL_STARRED, VIRTUAL_ATTACHMENTS } from '@/components/Sidebar';
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
  type ComposeContext = { intent: ComposeIntent; source?: MessageDetail; draft?: MessageDetail; to?: string };
  const [composeContext, setComposeContext] = useState<ComposeContext | null>(null);
  const openCompose = useCallback((ctx: ComposeContext) => setComposeContext(ctx), []);
  const closeCompose = useCallback(() => setComposeContext(null), []);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [advancedFilters, setAdvancedFilters] = useState<AdvancedFilters>({});
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [showShortcuts, setShowShortcuts] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [sidebarWidth, setSidebarWidth] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_sidebar_width') ?? '220', 10) || 220; } catch { return 220; }
  });
  const [readingPaneWidth, setReadingPaneWidth] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_reading_pane_width') ?? '0', 10) || 0; } catch { return 0; }
  });
  const [contextMenu, setContextMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [swipeDeltaX, setSwipeDeltaX] = useState(0);
  const swipeTouchStartRef = useRef<number | null>(null);
  const [messageLabels, setMessageLabels] = useState<Record<string, string>>(() => {
    try { return JSON.parse(localStorage.getItem('webmail_labels') ?? '{}'); } catch { return {}; }
  });
  const setLabel = useCallback((id: string, color: string | null) => {
    setMessageLabels((prev) => {
      const next = { ...prev };
      if (color) next[id] = color; else delete next[id];
      try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
  }, []);

  const [pendingCompose, setPendingCompose] = useState<{ intent: 'reply' | 'forward'; messageId: string } | null>(null);
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

  const handleBulkLabel = useCallback((ids: string[], color: string | null) => {
    setMessageLabels((prev) => {
      const next = { ...prev };
      for (const id of ids) { if (color) next[id] = color; else delete next[id]; }
      try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
    addToast(color ? `${ids.length}개에 라벨을 지정했습니다` : `${ids.length}개의 라벨을 제거했습니다`, 'info');
  }, [addToast]);

  const { folders, messages, setMessages, foldersLoading, messagesLoading, hasMore, loadingMore, loadMore, adjustUnread, refresh, refreshing } =
    useMailList(activeFolderId);

  // Set default folder to inbox UUID once folders are loaded
  useEffect(() => {
    if (activeFolderId || folders.length === 0) return;
    const inbox = folders.find((f) => f.system_type === 'inbox') ?? folders[0];
    if (inbox) setActiveFolderId(inbox.id);
  }, [folders, activeFolderId]);

  // Virtual folder message loading
  useEffect(() => {
    if (!activeFolderId.startsWith('__')) return;
    let cancelled = false;
    const params = activeFolderId === VIRTUAL_ATTACHMENTS ? { has_attachment: true, limit: 100 } : { limit: 100 };
    searchMessages(params).then((res) => {
      if (cancelled) return;
      let msgs = res.messages ?? [];
      if (activeFolderId === VIRTUAL_STARRED) msgs = msgs.filter((m) => m.starred);
      setMessages(msgs);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [activeFolderId, setMessages]);

  const { message: selectedMessage, loading: messageLoading } =
    useMessage(selectedMessageId);

  const [threadMessages, setThreadMessages] = useState<MessageSummary[]>([]);
  useEffect(() => {
    if (!selectedMessage?.subject) { setThreadMessages([]); return; }
    const normalizedSubject = selectedMessage.subject.replace(/^(Re|Fwd?|Fw):\s*/gi, '').trim();
    if (!normalizedSubject) { setThreadMessages([]); return; }
    let cancelled = false;
    searchMessages({ subject: normalizedSubject, limit: 20 })
      .then((res) => {
        if (cancelled) return;
        const sorted = [...(res.messages ?? [])].sort(
          (a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
        );
        setThreadMessages(sorted);
      })
      .catch(() => { if (!cancelled) setThreadMessages([]); });
    return () => { cancelled = true; };
  }, [selectedMessage?.id, selectedMessage?.subject]);

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

  const DEV_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID || '';

  // Check auth on mount, load email
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) { router.push('/login'); return; }
    let email = localStorage.getItem('webmail_email') ?? '';
    if (!email && token === '__dev__' && DEV_USER_ID.includes('@')) email = DEV_USER_ID;
    setUserEmail(email);
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
    openCompose({ intent: 'new', draft: selectedMessage });
    setSelectedMessageId(null);
  }, [selectedMessage, activeFolderSystemType]);

  useEffect(() => {
    if (!pendingCompose || !selectedMessage || selectedMessage.id !== pendingCompose.messageId) return;
    openCompose({ intent: pendingCompose.intent, source: selectedMessage });
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

  const handleToggleReadMessage = useCallback((id: string, read: boolean) => {
    const prev = messages.find((m) => m.id === id);
    if (!prev || prev.read === read) return;
    setMessages((ms) => ms.map((m) => (m.id === id ? { ...m, read } : m)));
    adjustUnread(activeFolderId, read ? -1 : 1);
    markRead(id, read).catch(() => {
      setMessages((ms) => ms.map((m) => (m.id === id ? { ...m, read: !read } : m)));
      adjustUnread(activeFolderId, read ? 1 : -1);
    });
  }, [messages, setMessages, adjustUnread, activeFolderId]);

  const parseSearchOperators = useCallback((raw: string): { q: string; operators: AdvancedFilters } => {
    let q = raw;
    const operators: AdvancedFilters = {};
    q = q.replace(/\bfrom:(\S+)/gi, (_, val) => { operators.from = val; return ''; });
    q = q.replace(/\bsubject:(?:"([^"]+)"|(\S+))/gi, (_, quoted, plain) => { operators.subject = quoted ?? plain; return ''; });
    q = q.replace(/\bhas:attachment\b/gi, () => { operators.has_attachment = true; return ''; });
    q = q.replace(/\bbefore:(\S+)/gi, (_, val) => { operators.until = val; return ''; });
    q = q.replace(/\bafter:(\S+)/gi, (_, val) => { operators.since = val; return ''; });
    return { q: q.replace(/\s+/g, ' ').trim(), operators };
  }, []);

  const runSearch = useCallback(async (q: string, filters: AdvancedFilters) => {
    if (!q.trim() && !filters.from && !filters.subject && !filters.since && !filters.until && !filters.has_attachment) {
      setSearchResults(null);
      return;
    }
    setSearchLoading(true);
    try {
      const res = await searchMessages({
        q: q.trim() || undefined,
        from: filters.from || undefined,
        subject: filters.subject || undefined,
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
    const { q: plainQ, operators } = parseSearchOperators(q);
    const merged = { ...advancedFilters, ...operators };
    searchDebounceRef.current = setTimeout(() => runSearch(plainQ, merged), 300);
  }, [advancedFilters, runSearch, parseSearchOperators]);

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

  const getNextId = useCallback((id: string): string | null => {
    const idx = messages.findIndex((m) => m.id === id);
    return (messages[idx + 1] ?? messages[idx - 1])?.id ?? null;
  }, [messages]);

  const handleDeleteById = useCallback((id: string) => {
    const msgToDelete = messages.find((m) => m.id === id);
    if (msgToDelete && !msgToDelete.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    setMessages((prev) => prev.filter((m) => m.id !== id));
    if (selectedMessageId === id) setSelectedMessageId(nextId);

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
    const unreadDeleteCount = messages.filter((m) => ids.includes(m.id) && !m.read).length;
    if (unreadDeleteCount > 0) adjustUnread(activeFolderId, -unreadDeleteCount);
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
    const nextId = getNextId(id);
    setMessages((prev) => prev.filter((m) => m.id !== id));
    if (selectedMessageId === id) setSelectedMessageId(nextId);
    try { await restoreMessage(id); addToast('메일을 복구했습니다'); }
    catch { addToast('복구에 실패했습니다', 'error'); }
  }, [selectedMessageId, getNextId, setMessages, addToast]);

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

  const handleBulkStar = useCallback(async (ids: string[], starred: boolean) => {
    setMessages((prev) => prev.map((m) => ids.includes(m.id) ? { ...m, starred } : m));
    await Promise.allSettled(ids.map((id) => starMessage(id, starred)));
    addToast(starred ? `${ids.length}개에 별표를 추가했습니다` : `${ids.length}개의 별표를 제거했습니다`, 'info');
  }, [setMessages, addToast]);

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

  const handleArchiveById = useCallback((id: string) => {
    const archiveFolder = folders.find((f) => f.system_type === 'archive');
    if (!archiveFolder) return;
    const msgToArchive = messages.find((m) => m.id === id);
    if (msgToArchive && !msgToArchive.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    void moveMessage(id, archiveFolder.id).then(() => {
      setMessages((prev) => prev.filter((m) => m.id !== id));
      if (selectedMessageId === id) setSelectedMessageId(nextId);
    }).catch(() => {});
  }, [folders, getNextId, setMessages, selectedMessageId]);

  const handleArchive = useCallback(() => {
    if (!selectedMessageId) return;
    handleArchiveById(selectedMessageId);
  }, [selectedMessageId, handleArchiveById]);

  const handleSpam = useCallback(() => {
    if (!selectedMessageId) return;
    const spamFolder = folders.find((f) => f.system_type === 'spam' || f.system_type === 'junk');
    if (!spamFolder) return;
    const id = selectedMessageId;
    const spamMsg = messages.find((m) => m.id === id);
    if (spamMsg && !spamMsg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    void moveMessage(id, spamFolder.id).then(() => {
      setMessages((prev) => prev.filter((m) => m.id !== id));
      setSelectedMessageId(nextId);
      addToast('스팸으로 이동했습니다', 'info');
    }).catch(() => addToast('이동에 실패했습니다', 'error'));
  }, [selectedMessageId, folders, getNextId, setMessages, addToast]);

  const handleNotSpam = useCallback(() => {
    if (!selectedMessageId) return;
    const inboxFolder = folders.find((f) => f.system_type === 'inbox');
    if (!inboxFolder) return;
    const id = selectedMessageId;
    const notSpamMsg = messages.find((m) => m.id === id);
    if (notSpamMsg && !notSpamMsg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    void moveMessage(id, inboxFolder.id).then(() => {
      setMessages((prev) => prev.filter((m) => m.id !== id));
      setSelectedMessageId(nextId);
      addToast('받은 편지함으로 이동했습니다', 'info');
    }).catch(() => addToast('이동에 실패했습니다', 'error'));
  }, [selectedMessageId, folders, getNextId, setMessages, addToast]);

  const handleMove = useCallback(async (folderId: string) => {
    if (!selectedMessageId) return;
    const id = selectedMessageId;
    const msg = messages.find((m) => m.id === id);
    if (msg && !msg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    setMessages((prev) => prev.filter((m) => m.id !== id));
    setSelectedMessageId(nextId);
    moveMessage(id, folderId)
      .then(() => addToast('메일을 이동했습니다'))
      .catch(() => addToast('이동에 실패했습니다', 'error'));
  }, [selectedMessageId, getNextId, setMessages, addToast]);

  const handleStar = useCallback(async (id: string, starred: boolean) => {
    setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred } : m)));
    starMessage(id, starred).catch(() => {
      setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred: !starred } : m)));
    });
  }, [setMessages]);


  // Persist last-selected message per folder
  useEffect(() => {
    if (!selectedMessageId || !activeFolderId) return;
    try {
      const saved = JSON.parse(localStorage.getItem('webmail_last_selected') ?? '{}');
      saved[activeFolderId] = selectedMessageId;
      localStorage.setItem('webmail_last_selected', JSON.stringify(saved));
    } catch { /* */ }
  }, [selectedMessageId, activeFolderId]);

  // Restore last-selected message when folder loads
  useEffect(() => {
    if (selectedMessageId || !activeFolderId || messages.length === 0) return;
    try {
      const saved = JSON.parse(localStorage.getItem('webmail_last_selected') ?? '{}');
      const lastId = saved[activeFolderId] as string | undefined;
      if (lastId && messages.some((m) => m.id === lastId)) {
        setSelectedMessageId(lastId);
      }
    } catch { /* */ }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeFolderId, messages.length]);

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
        if (e.key === 'u') {
          e.preventDefault();
          const firstUnread = list.find((m) => !m.read);
          if (firstUnread) setSelectedMessageId(firstUnread.id);
          return;
        }
        const systemTypeMap: Record<string, string> = { i: 'inbox', s: 'sent', d: 'drafts', t: 'trash', a: 'archive', p: 'spam' };
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
          if (!composeContext) { e.preventDefault(); openCompose({ intent: 'new' }); }
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
            openCompose({ intent: 'reply', source: selectedMessage });
          }
          break;
        case 'a':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'reply_all', source: selectedMessage });
          }
          break;
        case 'f':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'forward', source: selectedMessage });
          }
          break;
        case 'e': {
          if (selectedMessageId && !composeContext) handleArchive();
          break;
        }
        case 'l': {
          if (selectedMessageId && !composeContext) {
            const colors = ['#ef4444','#f97316','#eab308','#22c55e','#3b82f6','#a855f7'];
            const current = messageLabels[selectedMessageId];
            const currentIdx = current ? colors.indexOf(current) : -1;
            if (currentIdx === colors.length - 1) setLabel(selectedMessageId, null);
            else setLabel(selectedMessageId, colors[currentIdx + 1]);
          }
          break;
        }
        case 'z': {
          if (selectedMessageId && !composeContext && activeFolderSystemType !== 'trash') {
            handleSnooze(selectedMessageId, new Date(Date.now() + 60 * 60 * 1000));
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
          else if (composeContext) closeCompose();
          else setSelectedMessageId(null);
          break;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [messages, searchResults, selectedMessageId, selectedMessage, composeContext, openCompose, closeCompose, showShortcuts, handleDelete, handleArchive, getNextId, folders, messageLabels, setLabel, activeFolderSystemType]);

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

  // Snooze: hide message until a future time, then resurface it
  const handleSnooze = useCallback((id: string, until: Date) => {
    try {
      const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
      stored[id] = until.toISOString();
      localStorage.setItem('webmail_snoozed', JSON.stringify(stored));
    } catch { /* ignore */ }
    setMessages((prev) => prev.filter((m) => m.id !== id));
    if (selectedMessageId === id) setSelectedMessageId(null);
    addToast(`스누즈: ${until.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' })}에 다시 알립니다`, 'info', { duration: 4000 });
  }, [selectedMessageId, setMessages, addToast]);

  // Check every 60s if any snoozed message should reappear
  useEffect(() => {
    const check = () => {
      try {
        const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
        const now = Date.now();
        const expired = Object.entries(stored).filter(([, ts]) => new Date(ts).getTime() <= now);
        if (expired.length === 0) return;
        const remaining = { ...stored };
        expired.forEach(([id]) => delete remaining[id]);
        localStorage.setItem('webmail_snoozed', JSON.stringify(remaining));
        // Only show toast — message reappears on next folder refresh
        addToast(`스누즈 알림: ${expired.length}개 메일이 돌아왔습니다`, 'info');
        refresh();
      } catch { /* ignore */ }
    };
    const id = setInterval(check, 60_000);
    return () => clearInterval(id);
  }, [addToast, refresh]);

  // Extract sender names from messages and store as contacts
  useEffect(() => {
    if (messages.length === 0) return;
    try {
      const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      let changed = false;
      messages.forEach((m) => {
        if (m.from_name && m.from_addr) {
          const key = m.from_addr.toLowerCase();
          if (!stored[key] || stored[key] !== m.from_name) {
            stored[key] = m.from_name;
            changed = true;
          }
        }
      });
      if (changed) localStorage.setItem('webmail_contacts', JSON.stringify(stored));
    } catch { /* ignore */ }
  }, [messages]);

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
        onCompose={() => { openCompose({ intent: 'new' }); setMobileSidebarOpen(false); }}
        onComposeInNewWindow={() => window.open('/compose', '_blank', 'width=620,height=720,menubar=no,toolbar=no,resizable=yes')}
        onSearch={handleSearch}
        searchQuery={searchQuery}
        advancedFilters={advancedFilters}
        onAdvancedFilterChange={handleFilterChange}
        userName={userEmail || '사용자'}
        userEmailAddress={userEmail || undefined}
        width={sidebarWidth}
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
        onCreateFolder={async (name) => {
          try { await createFolder(name); refresh(); addToast(`"${name}" 폴더를 만들었습니다`); }
          catch { addToast('폴더 생성에 실패했습니다', 'error'); }
        }}
        onRenameFolder={async (id, name) => {
          try { await renameFolder(id, name); refresh(); addToast('폴더 이름을 변경했습니다'); }
          catch { addToast('이름 변경에 실패했습니다', 'error'); }
        }}
        onDeleteFolder={async (id) => {
          try { await deleteFolder(id); if (activeFolderId === id) setActiveFolderId(''); refresh(); addToast('폴더를 삭제했습니다'); }
          catch { addToast('폴더 삭제에 실패했습니다', 'error'); }
        }}
        footerExtra={isMobile ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', paddingBottom: '4px' }}>
            <AccentPicker />
            <LocaleSelector />
            <ThemeToggle inline />
          </div>
        ) : undefined}
        menuExtra={
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <AccentPicker />
            <LocaleSelector />
            <ThemeToggle inline />
          </div>
        }
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

      <div style={{ flex: 1, display: 'flex', flexDirection: 'row', overflow: 'hidden', minWidth: 0 }}>

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
          searchQuery={searchResults !== null ? searchQuery : undefined}
          emptyFolderLabel={activeFolderSystemType === 'trash' ? '휴지통 비우기' : undefined}
          onEmptyFolder={activeFolderSystemType === 'trash' ? () => handleBulkDelete(messages.map((m) => m.id)) : undefined}
          onDeleteMessage={handleDeleteById}
          onArchiveMessage={activeFolderSystemType !== 'archive' && activeFolderSystemType !== 'trash' ? handleArchiveById : undefined}
          onToggleReadMessage={handleToggleReadMessage}
          onBulkRestore={activeFolderSystemType === 'trash' ? handleBulkRestore : undefined}
          onBulkLabel={handleBulkLabel}
          onBulkStar={handleBulkStar}
          messageLabels={messageLabels}
        />

      </div>{/* end layout wrapper */}

      {/* Slide-in reading pane overlay */}
      {(() => {
        const msgList = searchResults ?? messages;
        const curIdx = msgList.findIndex((m) => m.id === selectedMessageId);
        const prevId = curIdx > 0 ? msgList[curIdx - 1].id : null;
        const nextId = curIdx !== -1 && curIdx < msgList.length - 1 ? msgList[curIdx + 1].id : null;
        const panelOpen = !!selectedMessageId;
        return (
          <>
            {/* backdrop — semi-transparent, click closes panel */}
            <div
              aria-hidden="true"
              onClick={() => setSelectedMessageId(null)}
              style={{
                position: 'fixed', inset: 0, zIndex: 49,
                background: 'rgba(0,0,0,0.15)',
                opacity: panelOpen ? 1 : 0,
                pointerEvents: panelOpen ? 'auto' : 'none',
                transition: 'opacity 200ms ease',
              }}
            />
            <div
              role="region"
              aria-label="메일 읽기"
              onTouchStart={isMobile ? (e) => { swipeTouchStartRef.current = e.touches[0].clientX; } : undefined}
              onTouchMove={isMobile ? (e) => {
                if (swipeTouchStartRef.current === null) return;
                const delta = e.touches[0].clientX - swipeTouchStartRef.current;
                if (delta > 0) setSwipeDeltaX(delta);
              } : undefined}
              onTouchEnd={isMobile ? () => {
                if (swipeDeltaX > 80) setSelectedMessageId(null);
                setSwipeDeltaX(0);
                swipeTouchStartRef.current = null;
              } : undefined}
              style={{
                position: 'fixed',
                top: 0,
                right: 0,
                height: '100dvh',
                width: isMobile ? '100%' : readingPaneWidth > 0 ? `${readingPaneWidth}px` : 'min(720px, 55vw)',
                transform: panelOpen
                  ? (isMobile && swipeDeltaX > 0 ? `translateX(${swipeDeltaX}px)` : 'translateX(0)')
                  : 'translateX(100%)',
                transition: swipeDeltaX > 0 ? 'none' : 'transform 220ms cubic-bezier(0.4,0,0.2,1)',
                zIndex: 50,
                display: 'flex',
                flexDirection: 'column',
                background: 'var(--color-bg-primary)',
                borderLeft: isMobile ? 'none' : '1px solid var(--color-border-default)',
                boxShadow: panelOpen ? '-8px 0 32px rgba(0,0,0,0.12)' : 'none',
              }}
            >
              {/* Resize handle — left edge */}
              {!isMobile && panelOpen && (
                <div
                  aria-hidden="true"
                  style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: '5px', cursor: 'col-resize', zIndex: 10, transition: 'background 150ms ease' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-accent)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    const startX = e.clientX;
                    const startW = readingPaneWidth > 0 ? readingPaneWidth : Math.min(720, window.innerWidth * 0.55);
                    let lastW = startW;
                    const onMove = (ev: MouseEvent) => {
                      lastW = Math.min(window.innerWidth - 300, Math.max(380, startW - (ev.clientX - startX)));
                      setReadingPaneWidth(lastW);
                    };
                    const onUp = () => {
                      document.removeEventListener('mousemove', onMove);
                      document.removeEventListener('mouseup', onUp);
                      try { localStorage.setItem('webmail_reading_pane_width', String(lastW)); } catch { /* */ }
                    };
                    document.addEventListener('mousemove', onMove);
                    document.addEventListener('mouseup', onUp);
                  }}
                />
              )}
              <ReadingPane
                message={selectedMessage}
                folders={folders}
                onArchive={activeFolderSystemType !== 'archive' && activeFolderSystemType !== 'trash' && activeFolderSystemType !== 'spam' && activeFolderSystemType !== 'junk' ? handleArchive : undefined}
                onSpam={folders.some((f) => f.system_type === 'spam' || f.system_type === 'junk') && activeFolderSystemType !== 'spam' && activeFolderSystemType !== 'junk' && activeFolderSystemType !== 'trash' ? handleSpam : undefined}
                onNotSpam={activeFolderSystemType === 'spam' || activeFolderSystemType === 'junk' ? handleNotSpam : undefined}
                onDelete={handleDelete}
                onReply={() => selectedMessage && openCompose({ intent: 'reply', source: selectedMessage })}
                onReplyAll={() => selectedMessage && openCompose({ intent: 'reply_all', source: selectedMessage })}
                onForward={() => selectedMessage && openCompose({ intent: 'forward', source: selectedMessage })}
                onMove={handleMove}
                onPrint={() => window.print()}
                loading={messageLoading}
                onBack={() => setSelectedMessageId(null)}
                onPrev={prevId ? () => handleSelectMessage(prevId) : undefined}
                onNext={nextId ? () => handleSelectMessage(nextId) : undefined}
                messageIndex={curIdx >= 0 ? curIdx : undefined}
                messageTotal={curIdx >= 0 ? msgList.length : undefined}
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
                onComposeToAddress={(address) => openCompose({ intent: 'new', to: address })}
                onSnooze={activeFolderSystemType !== 'trash' ? handleSnooze : undefined}
                onOpenInWindow={selectedMessageId ? () => window.open(`/mail/${selectedMessageId}`, '_blank', 'width=900,height=700,menubar=no,toolbar=no') : undefined}
                threadMessages={threadMessages.length > 1 ? threadMessages : undefined}
                onSelectThread={handleSelectMessage}
                userEmail={userEmail || undefined}
              />
            </div>
          </>
        );
      })()}

      {composeContext && (
        <ComposeModal
          intent={composeContext.intent}
          sourceMessage={composeContext.source}
          draftMessage={composeContext.draft}
          initialTo={composeContext.to}
          userEmail={userEmail}
          isMobile={isMobile}
          onClose={closeCompose}
        />
      )}

      {/* Mobile FAB — compose button when sidebar is hidden */}
      {isMobile && !selectedMessageId && !composeContext && (

        <button
          aria-label="새 메일 작성"
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
              ...(['#ef4444','#f97316','#eab308','#22c55e','#3b82f6','#a855f7'] as const).map((color) => ({
                label: `${messageLabels[contextMenu.id] === color ? '✓ ' : ''}라벨 ${color === '#ef4444' ? '🔴' : color === '#f97316' ? '🟠' : color === '#eab308' ? '🟡' : color === '#22c55e' ? '🟢' : color === '#3b82f6' ? '🔵' : '🟣'}`,
                onClick: () => setLabel(contextMenu.id, messageLabels[contextMenu.id] === color ? null : color),
              })),
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

    </div>
  );
}
