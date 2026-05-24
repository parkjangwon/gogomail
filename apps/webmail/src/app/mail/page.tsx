'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { deleteMessage, restoreMessage, bulkRestoreMessages, createFolder, renameFolder, deleteFolder, starMessage, markRead, moveMessage, bulkMarkRead, searchMessages, getMessages, sendMessage, listThreads, listThreadMessages, getNotificationPreferences, setNotificationPreferences, UIComposeIntent, MessageAddress, MessageDetail, MessageSummary, ThreadSummary, type ThreadNotificationOverride } from '@/lib/api';
import { AdvancedFilters, VIRTUAL_ALL, VIRTUAL_STARRED, VIRTUAL_ATTACHMENTS, VIRTUAL_UNREAD, VIRTUAL_SNOOZED, VIRTUAL_PINNED, VIRTUAL_IMPORTANT, VIRTUAL_TASKS } from '@/components/Sidebar';
import { useMailList, type RefreshIntervalSeconds } from '@/hooks/useMailList';
import { useMessage } from '@/hooks/useMessage';
import { useIsMobile } from '@/hooks/useIsMobile';
import { useIsOnline } from '@/hooks/useIsOnline';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ReadingPane } from '@/components/ReadingPane';
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
import { loadFilterRules } from '@/components/settings/settingsConfig';
import { SpotlightSearch } from '@/components/SpotlightSearch';
import { MFASetupPromptModal } from '@/components/MFASetupPromptModal';
import {
  buildThreadMessages,
  getEmptyFolderLabel,
  getNextMessageId,
  getVisibleMailMessages,
  patchThreadsForMessages,
  parseSearchOperators,
  shouldHideMessageAfterSnooze,
} from '@/lib/mail/mailPageUtils';
import { focusNavGroup } from '@/lib/navKeyboard';
import { stableId } from '@/lib/stableId';
import { useNotifications } from '@/lib/notifications/store';

const WEBMAIL_ACTIVE_APP_KEY = 'webmail_active_app';
const NOTIFICATION_FOLDER_OVERRIDES_KEY = 'webmail_notification_folder_overrides';
const NOTIFICATION_THREAD_OVERRIDES_KEY = 'webmail_notification_thread_overrides';
const BADGE_COUNT_MODE_KEY = 'webmail_badge_count_mode';
const REFRESH_INTERVAL_KEY = 'webmail_refresh_interval';
type BadgeCountMode = 'unread' | 'all' | 'none';
type NavigatorWithBadging = Navigator & {
  setAppBadge?: (contents?: number) => Promise<void>;
  clearAppBadge?: () => Promise<void>;
};

function isAppId(value: string | null): value is AppId {
  return value === 'mail' || value === 'calendar' || value === 'contacts' || value === 'drive' || value === 'settings';
}

function getInitialActiveApp(): AppId {
  if (typeof window === 'undefined') return 'mail';
  try {
    const urlApp = new URLSearchParams(window.location.search).get('app');
    if (isAppId(urlApp)) return urlApp;
    const stored = window.localStorage.getItem(WEBMAIL_ACTIVE_APP_KEY);
    if (isAppId(stored)) return stored;
  } catch {
    // ignore
  }
  return 'mail';
}

function folderNotificationsEnabled(folderId: string): boolean {
  if (!folderId || typeof window === 'undefined') return true;
  try {
    const raw = window.localStorage.getItem(NOTIFICATION_FOLDER_OVERRIDES_KEY);
    if (!raw) return true;
    const overrides = JSON.parse(raw) as Record<string, { enabled?: boolean }>;
    return overrides[folderId]?.enabled !== false;
  } catch {
    return true;
  }
}

function threadNotificationsEnabled(threadId: string | undefined, messageId: string): boolean {
  if (typeof window === 'undefined') return true;
  const key = threadId || messageId;
  if (!key) return true;
  try {
    const raw = window.localStorage.getItem(NOTIFICATION_THREAD_OVERRIDES_KEY);
    if (!raw) return true;
    const overrides = JSON.parse(raw) as Record<string, { enabled?: boolean }>;
    return overrides[key]?.enabled !== false;
  } catch {
    return true;
  }
}

function readBadgeCountMode(): BadgeCountMode {
  if (typeof window === 'undefined') return 'unread';
  try {
    const value = window.localStorage.getItem(BADGE_COUNT_MODE_KEY);
    return value === 'all' || value === 'none' ? value : 'unread';
  } catch {
    return 'unread';
  }
}

function readRefreshIntervalSeconds(): RefreshIntervalSeconds {
  if (typeof window === 'undefined') return 30;
  try {
    const value = Number(window.localStorage.getItem(REFRESH_INTERVAL_KEY) ?? 30);
    return value === 60 || value === 300 ? value : 30;
  } catch {
    return 30;
  }
}

function getFocusedNavGroup(): string | null {
  if (typeof document === 'undefined') return null;
  const active = document.activeElement as HTMLElement | null;
  return active?.closest<HTMLElement>('[data-nav-group]')?.dataset.navGroup ?? null;
}

function getMailNavGroups(readingPaneOpen: boolean): string[] {
  return readingPaneOpen ? ['sidebar-nav', 'message-list', 'reading-pane'] : ['sidebar-nav', 'message-list'];
}

function moveMailPanelFocus(direction: 'prev' | 'next', readingPaneOpen: boolean): boolean {
  const groups = getMailNavGroups(readingPaneOpen);
  const currentGroup = getFocusedNavGroup();
  const currentIndex = currentGroup ? groups.indexOf(currentGroup) : -1;
  const fallbackIndex = direction === 'next' ? 0 : groups.length - 1;
  const nextIndex = currentIndex === -1
    ? fallbackIndex
    : Math.max(0, Math.min(groups.length - 1, currentIndex + (direction === 'next' ? 1 : -1)));
  return !!focusNavGroup(groups[nextIndex]);
}

export default function MailPage() {
  const router = useRouter();
  const t = useTranslations();
  const tNotif = useTranslations('notifications');
  const { push: pushNotification } = useNotifications();

  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [userEmail, setUserEmail] = useState('');
  type ComposeContext = { intent: UIComposeIntent; source?: MessageDetail; draft?: MessageDetail; to?: string; initialSubject?: string; initialBody?: string };
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
  const [pinnedIds, setPinnedIds] = useState<Set<string>>(() => {
    try { return new Set<string>(JSON.parse(localStorage.getItem('webmail_pinned') ?? '[]') as string[]); } catch { return new Set(); }
  });
  const handlePin = useCallback((id: string) => {
    setPinnedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      try { localStorage.setItem('webmail_pinned', JSON.stringify([...next])); } catch { /* */ }
      return next;
    });
  }, []);
  const [importantIds, setImportantIds] = useState<Set<string>>(() => {
    try { return new Set<string>(JSON.parse(localStorage.getItem('webmail_important') ?? '[]') as string[]); } catch { return new Set(); }
  });
  const handleImportant = useCallback((id: string) => {
    setImportantIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      try { localStorage.setItem('webmail_important', JSON.stringify([...next])); } catch { /* */ }
      return next;
    });
  }, []);

  const setLabel = useCallback((id: string, color: string | null) => {
    setMessageLabels((prev) => {
      const next = { ...prev };
      if (color) next[id] = color; else delete next[id];
      try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
  }, []);

  const [activeApp, setActiveApp] = useState<AppId>(getInitialActiveApp);
  const [badgeCountMode, setBadgeCountMode] = useState<BadgeCountMode>(readBadgeCountMode);
  const [refreshIntervalSeconds, setRefreshIntervalSeconds] = useState<RefreshIntervalSeconds>(readRefreshIntervalSeconds);
  const [threadNotificationOverrides, setThreadNotificationOverrides] = useState<Record<string, ThreadNotificationOverride>>({});
  const [settingsInitialSection, setSettingsInitialSection] = useState<SectionId | undefined>(undefined);
  const [showSpotlight, setShowSpotlight] = useState(false);
  const [spotlightMoveId, setSpotlightMoveId] = useState<string | null>(null);

  useEffect(() => {
    try {
      localStorage.setItem(WEBMAIL_ACTIVE_APP_KEY, activeApp);
    } catch {
      // ignore
    }

    try {
      const url = new URL(window.location.href);
      if (activeApp === 'mail') url.searchParams.delete('app');
      else url.searchParams.set('app', activeApp);
      const nextUrl = `${url.pathname}${url.search}${url.hash}`;
      const currentUrl = `${window.location.pathname}${window.location.search}${window.location.hash}`;
      if (nextUrl !== currentUrl) window.history.replaceState(window.history.state, '', nextUrl);
    } catch {
      // ignore
    }
  }, [activeApp]);

  const [wmSettings, setWmSettings] = useState<{ showPreview: boolean; externalImages: string }>(() => {
    try {
      const s = JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>;
      return { showPreview: s.showPreview !== false, externalImages: (s.externalImages as string) ?? 'ask' };
    } catch { return { showPreview: true, externalImages: 'ask' }; }
  });
  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key !== 'webmail_settings') return;
      try {
        const s = JSON.parse(e.newValue ?? '{}') as Record<string, unknown>;
        setWmSettings({ showPreview: s.showPreview !== false, externalImages: (s.externalImages as string) ?? 'ask' });
      } catch { /* */ }
    }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const [pendingCompose, setPendingCompose] = useState<{ intent: 'reply' | 'forward'; messageId: string } | null>(null);

  const threadViewEnabled = true; // thread view always on (toggle removed)
  const [threads, setThreads] = useState<ThreadSummary[]>([]);

  const isMobile = useIsMobile();
  const gPrefixRef = useRef(false);
  const isOnline = useIsOnline();

  const pendingDeletesRef = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const addToast = useCallback((message: string, type: ToastItem['type'] = 'success', options?: { duration?: number; action?: ToastItem['action'] }) => {
    const id = stableId('toast');
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
    addToast(color ? t('misc.mailPage.labelAdded', { count: ids.length }) : t('misc.mailPage.labelRemoved', { count: ids.length }), 'info');
  }, [addToast]);

  const { folders, messages, setMessages, foldersLoading, messagesLoading, setMessagesLoading, hasMore, loadingMore, loadMore, adjustUnread, refresh, refreshing } =
    useMailList(activeFolderId, refreshIntervalSeconds);
  const [threadMessages, setThreadMessages] = useState<MessageSummary[]>([]);

  const patchVisibleMessages = useCallback((ids: string[], patch: Partial<MessageSummary>) => {
    const idSet = new Set(ids);
    const applyPatch = (items: MessageSummary[]) => items.map((m) => (idSet.has(m.id) ? { ...m, ...patch } : m));
    setMessages(applyPatch);
    setSearchResults((prev) => (prev ? applyPatch(prev) : prev));
    setThreadMessages(applyPatch);
    setThreads((prev) => patchThreadsForMessages(prev, ids, patch));
  }, [setMessages]);
  // Remove messages from all visible sources (messages, searchResults, threadMessages) so
  // delete/archive/move actions take immediate effect even when the user is in search mode.
  const removeVisibleMessages = useCallback((ids: string[]) => {
    const idSet = new Set(ids);
    const filterFn = (prev: MessageSummary[]) => prev.filter((m) => !idSet.has(m.id));
    setMessages(filterFn);
    setSearchResults((prev) => (prev ? filterFn(prev) : prev));
    setThreadMessages(filterFn);
  }, [setMessages]);
  const findVisibleMessage = useCallback((id: string) => (
    messages.find((m) => m.id === id) ??
    searchResults?.find((m) => m.id === id) ??
    threadMessages.find((m) => m.id === id) ??
    buildThreadMessages(threads).find((m) => m.id === id)
  ), [messages, searchResults, threadMessages, threads]);
  const countUnreadVisible = useCallback((ids: string[]) => (
    ids.reduce((count, id) => count + (findVisibleMessage(id)?.read === false ? 1 : 0), 0)
  ), [findVisibleMessage]);

  // Set default folder to inbox UUID once folders are loaded, and recover from stale saved IDs.
  useEffect(() => {
    if (folders.length === 0 || activeFolderId.startsWith('__')) return;
    const inbox = folders.find((f) => f.system_type === 'inbox') ?? folders[0];
    if (!activeFolderId || !folders.some((f) => f.id === activeFolderId)) {
      if (inbox) setActiveFolderId(inbox.id);
    }
  }, [folders, activeFolderId]);

  // Virtual folder message loading.
  // __starred__, __unread__, __attachments__: use the messages API directly with
  // server-side filters (starred/read/has_attachment) so we never miss messages
  // due to a small page limit.
  // __pinned__, __important__, __snoozed__, __tasks__: stored in localStorage so
  // we have to fetch a broad set and filter client-side; 500 covers typical usage.
  useEffect(() => {
    if (!activeFolderId.startsWith('__') || activeFolderId === VIRTUAL_ALL) return;
    let cancelled = false;
    setMessagesLoading(true);

    const load = async (): Promise<MessageSummary[]> => {
      // Backend caps list requests at 200; exceeding it returns 400.
      const MAX = 200;
      if (activeFolderId === VIRTUAL_STARRED) {
        const data = await getMessages('', '', MAX, { starred: true });
        return data.messages ?? [];
      }
      if (activeFolderId === VIRTUAL_UNREAD) {
        const data = await getMessages('', '', MAX, { read: false });
        return data.messages ?? [];
      }
      if (activeFolderId === VIRTUAL_ATTACHMENTS) {
        const data = await getMessages('', '', MAX, { has_attachment: true });
        return data.messages ?? [];
      }
      // localStorage-based virtual folders: fetch a broad recent pool and filter.
      const data = await searchMessages({ limit: MAX });
      let msgs = data.messages ?? [];
      if (activeFolderId === VIRTUAL_SNOOZED) {
        try {
          const snoozed: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
          const now = Date.now();
          msgs = msgs.filter((m) => snoozed[m.id] && new Date(snoozed[m.id]).getTime() > now);
        } catch { /* ignore */ }
      } else if (activeFolderId === VIRTUAL_PINNED) {
        try {
          const pinned: string[] = JSON.parse(localStorage.getItem('webmail_pinned') ?? '[]');
          const pinnedSet = new Set(pinned);
          msgs = msgs.filter((m) => pinnedSet.has(m.id));
        } catch { /* ignore */ }
      } else if (activeFolderId === VIRTUAL_IMPORTANT) {
        try {
          const important: string[] = JSON.parse(localStorage.getItem('webmail_important') ?? '[]');
          const importantSet = new Set(important);
          msgs = msgs.filter((m) => importantSet.has(m.id));
        } catch { /* ignore */ }
      } else if (activeFolderId === VIRTUAL_TASKS) {
        try {
          const tasks: { messageId: string }[] = JSON.parse(localStorage.getItem('webmail_tasks') ?? '[]');
          const taskIds = new Set(tasks.map((t) => t.messageId));
          msgs = msgs.filter((m) => taskIds.has(m.id));
        } catch { /* ignore */ }
      }
      return msgs;
    };

    load()
      .then((msgs) => { if (!cancelled) { setMessages(msgs); setMessagesLoading(false); } })
      .catch(() => { if (!cancelled) setMessagesLoading(false); });

    return () => { cancelled = true; };
  }, [activeFolderId, setMessages, setMessagesLoading]);

  // Thread view: fetch threads when enabled and folder changes
  useEffect(() => {
    if (!threadViewEnabled || !activeFolderId || activeFolderId.startsWith('__')) {
      setThreads([]);
      return;
    }
    let cancelled = false;
    listThreads({ folder_id: activeFolderId, limit: 50 })
      .then((r) => { if (!cancelled) setThreads(r.threads ?? []); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [threadViewEnabled, activeFolderId]);

  const { message: selectedMessage, loading: messageLoading } =
    useMessage(selectedMessageId);

  // selectedMessageSummary: the MessageSummary row that was clicked (may carry thread_id)
  const selectedMessageSummary = (threadViewEnabled && threads.length > 0)
    ? threads.find((t) => (t.latest_message_id || t.id) === selectedMessageId) ?? null
    : null;
  const selectedThreadId = selectedMessageSummary?.id ?? null;
  const selectedNotificationThreadId = selectedThreadId ?? selectedMessage?.thread_id ?? selectedMessage?.id ?? '';
  const selectedThreadMuted = !!selectedNotificationThreadId && threadNotificationOverrides[selectedNotificationThreadId]?.enabled === false;

  useEffect(() => {
    // If viewing a thread from thread-view mode, fetch via thread API
    if (selectedThreadId) {
      let cancelled = false;
      listThreadMessages(selectedThreadId)
        .then((msgs) => {
          if (cancelled) return;
          const sorted = [...msgs].sort(
            (a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
          );
          setThreadMessages(sorted);
        })
        .catch(() => { if (!cancelled) setThreadMessages([]); });
      return () => { cancelled = true; };
    }
    // Fallback: subject-based grouping for normal message view
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
  }, [selectedThreadId, selectedMessage?.id, selectedMessage?.subject]);

  useEffect(() => {
    const onBadgeModeChange = (event: StorageEvent) => {
      if (event.key === BADGE_COUNT_MODE_KEY) setBadgeCountMode(readBadgeCountMode());
    };
    window.addEventListener('storage', onBadgeModeChange);
    return () => window.removeEventListener('storage', onBadgeModeChange);
  }, []);

  useEffect(() => {
    const onRefreshIntervalChange = (event: StorageEvent) => {
      if (event.key === REFRESH_INTERVAL_KEY) setRefreshIntervalSeconds(readRefreshIntervalSeconds());
    };
    window.addEventListener('storage', onRefreshIntervalChange);
    return () => window.removeEventListener('storage', onRefreshIntervalChange);
  }, []);

  // Update document title + favicon badge according to the selected badge mode.
  useEffect(() => {
    const totalUnread = folders.reduce((sum, f) => sum + (f.unread ?? 0), 0);
    const totalMessages = folders.reduce((sum, f) => sum + (f.total ?? 0), 0);
    const badgeCount = badgeCountMode === 'none' ? 0 : badgeCountMode === 'all' ? totalMessages : totalUnread;
    document.title = badgeCount > 0 ? `GoGoMail (${badgeCount})` : 'GoGoMail';
    const badging = navigator as NavigatorWithBadging;
    if (badgeCount > 0 && typeof badging.setAppBadge === 'function') {
      void badging.setAppBadge(badgeCount).catch(() => {});
    } else if (badgeCount === 0 && typeof badging.clearAppBadge === 'function') {
      void badging.clearAppBadge().catch(() => {});
    }

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
      if (badgeCount > 0) {
        const label = badgeCount > 99 ? '99+' : String(badgeCount);
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
  }, [folders, badgeCountMode]);

  const [mustChangePassword, setMustChangePassword] = useState(false);
  const [sessionWarning, setSessionWarning] = useState<string | null>(null);

  const DEV_USER_ID = process.env.NODE_ENV !== 'production' ? (process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID || '') : '';
  const DEV_SKIP_LOGIN = process.env.NODE_ENV !== 'production' && process.env.NEXT_PUBLIC_GOGOMAIL_DEV_SKIP_LOGIN === 'true';

  // Check auth on mount, load email
  useEffect(() => {
    const authenticated = localStorage.getItem('webmail_authenticated');
    if (!authenticated && !(DEV_SKIP_LOGIN && DEV_USER_ID)) { router.push('/login'); return; }
    let email = localStorage.getItem('webmail_email') ?? '';
    if (!email && DEV_SKIP_LOGIN && DEV_USER_ID.includes('@')) email = DEV_USER_ID;
    setUserEmail(email);
    if (localStorage.getItem('webmail_must_change_password') === '1') {
      setMustChangePassword(true);
    }
  }, [router, DEV_SKIP_LOGIN, DEV_USER_ID]);

  // Session expiry warning: check every 60s, warn when < 10 min left
  useEffect(() => {
    function check() {
      const expiresAt = localStorage.getItem('webmail_token_expires_at');
      if (!expiresAt) { setSessionWarning(null); return; }
      const msLeft = new Date(expiresAt).getTime() - Date.now();
      if (msLeft <= 0) { setSessionWarning(t('misc.mailPage.sessionExpired')); return; }
      const minsLeft = Math.floor(msLeft / 60000);
      if (minsLeft < 10) setSessionWarning(t('misc.mailPage.sessionExpiresIn', { mins: minsLeft }));
      else setSessionWarning(null);
    }
    check();
    const id = setInterval(check, 60000);
    return () => clearInterval(id);
  }, []);

  const handleLogout = useCallback(() => {
    fetch('/api/auth/logout', { method: 'POST' }).catch(() => {});
    localStorage.removeItem('webmail_authenticated');
    localStorage.removeItem('webmail_email');
    localStorage.removeItem('webmail_must_change_password');
    localStorage.removeItem('webmail_token_expires_at');
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

  // Mark selected message as read locally + server (delay controlled by readMark setting)
  useEffect(() => {
    if (!selectedMessageId || activeFolderSystemType === 'drafts') return;
    let readMark: string;
    try { readMark = (JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as { readMark?: string }).readMark ?? 'instant'; } catch { readMark = 'instant'; }
    if (readMark === 'manual') return;
    const delay = readMark === '2s' ? 2000 : 0;
    let cancelled = false;
    const timer = setTimeout(() => {
      if (cancelled) return;
      const msg = findVisibleMessage(selectedMessageId);
      if (!msg || msg.read) return;
      patchVisibleMessages([selectedMessageId], { read: true });
      adjustUnread(activeFolderId, -1);
      markRead(selectedMessageId, true).catch(() => {});
    }, delay);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [selectedMessageId, findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId, activeFolderSystemType]);

  const handleMarkUnread = useCallback(async () => {
    if (!selectedMessageId) return;
    patchVisibleMessages([selectedMessageId], { read: false });
    adjustUnread(activeFolderId, 1);
    addToast(t('misc.mailPage.markedUnread'), 'info');
    markRead(selectedMessageId, false).catch(() => {
      patchVisibleMessages([selectedMessageId], { read: true });
      adjustUnread(activeFolderId, -1);
    });
  }, [selectedMessageId, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleMarkRead = useCallback(async () => {
    if (!selectedMessageId) return;
    const msg = findVisibleMessage(selectedMessageId);
    if (msg?.read) return;
    patchVisibleMessages([selectedMessageId], { read: true });
    adjustUnread(activeFolderId, -1);
    addToast(t('misc.mailPage.markedRead'), 'info');
    markRead(selectedMessageId, true).catch(() => {
      patchVisibleMessages([selectedMessageId], { read: false });
      adjustUnread(activeFolderId, 1);
    });
  }, [selectedMessageId, findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleToggleReadMessage = useCallback((id: string, read: boolean) => {
    const prev = findVisibleMessage(id);
    if (!prev || prev.read === read) return;
    patchVisibleMessages([id], { read });
    adjustUnread(activeFolderId, read ? -1 : 1);
    markRead(id, read).catch(() => {
      patchVisibleMessages([id], { read: !read });
      adjustUnread(activeFolderId, read ? 1 : -1);
    });
  }, [findVisibleMessage, patchVisibleMessages, adjustUnread, activeFolderId]);

  const runSearch = useCallback(async (q: string, filters: AdvancedFilters) => {
    if (!q.trim() && !filters.from && !filters.to && !filters.subject && !filters.since && !filters.until && !filters.has_attachment) {
      setSearchResults(null);
      return;
    }
    setSearchLoading(true);
    try {
      const res = await searchMessages({
        q: q.trim() || undefined,
        from: filters.from || undefined,
        to: filters.to || undefined,
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

  const getNextId = useCallback((id: string): string | null => getNextMessageId(messages, id), [messages]);

  const handleDeleteById = useCallback((id: string) => {
    const msgToDelete = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (msgToDelete && !msgToDelete.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    if (selectedMessageId === id) setSelectedMessageId(nextId);

    const timer = setTimeout(() => {
      pendingDeletesRef.current.delete(id);
      deleteMessage(id).catch(() => {});
    }, 5000);
    pendingDeletesRef.current.set(id, timer);

    addToast(t('misc.mailPage.deleted'), 'info', {
      duration: 5000,
      action: {
        label: t('misc.mailPage.undo'),
        onClick: () => {
          const timer = pendingDeletesRef.current.get(id);
          if (timer) { clearTimeout(timer); pendingDeletesRef.current.delete(id); }
          if (msgToDelete) {
            setMessages((prev) => [msgToDelete, ...prev]);
            setSearchResults((prev) => (prev ? [msgToDelete, ...prev] : prev));
          }
        },
      },
    });
  }, [messages, searchResults, selectedMessageId, removeVisibleMessages, setMessages, addToast]);

  const handleDelete = useCallback(() => {
    if (!selectedMessageId) return;
    handleDeleteById(selectedMessageId);
  }, [selectedMessageId, handleDeleteById]);

  const handleBulkDelete = useCallback(async (ids: string[]) => {
    const unreadDeleteCount = countUnreadVisible(ids);
    if (unreadDeleteCount > 0) adjustUnread(activeFolderId, -unreadDeleteCount);
    removeVisibleMessages(ids);
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    const results = await Promise.allSettled(ids.map((id) => deleteMessage(id)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed > 0) {
      addToast(t('misc.mailPage.bulkDeleteMixed', { ok: ids.length - failed, failed }), 'error');
    } else {
      addToast(t('misc.mailPage.bulkDeleted', { count: ids.length }));
    }
  }, [selectedMessageId, countUnreadVisible, adjustUnread, activeFolderId, removeVisibleMessages, addToast]);

  const handleRestore = useCallback(async (id: string) => {
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    if (selectedMessageId === id) setSelectedMessageId(nextId);
    try { await restoreMessage(id); addToast(t('misc.mailPage.restored')); }
    catch { addToast(t('misc.mailPage.restoreFailed'), 'error'); }
  }, [selectedMessageId, getNextId, removeVisibleMessages, addToast]);

  const handleBulkRestore = useCallback(async (ids: string[]) => {
    removeVisibleMessages(ids);
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    try { await bulkRestoreMessages(ids); addToast(t('misc.mailPage.bulkRestored', { count: ids.length })); }
    catch { addToast(t('misc.mailPage.restoreFailed'), 'error'); }
  }, [selectedMessageId, removeVisibleMessages, addToast]);

  // Restore archived messages back to inbox (archive uses move, not the trash restore API)
  const handleRestoreFromArchive = useCallback((id: string) => {
    const inboxFolder = folders.find((f) => f.system_type === 'inbox');
    if (!inboxFolder) return;
    const msg = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    if (selectedMessageId === id) setSelectedMessageId(nextId);
    addToast(t('misc.mailPage.restored'), 'info');
    void moveMessage(id, inboxFolder.id).catch(() => {
      if (msg) {
        setMessages((prev) => [msg, ...prev]);
        setSearchResults((prev) => (prev ? [msg, ...prev] : prev));
      }
      addToast(t('misc.mailPage.restoreFailed'), 'error');
    });
  }, [folders, messages, searchResults, selectedMessageId, getNextId, removeVisibleMessages, setMessages, addToast]);

  const handleBulkRestoreFromArchive = useCallback(async (ids: string[]) => {
    const inboxFolder = folders.find((f) => f.system_type === 'inbox');
    if (!inboxFolder) return;
    removeVisibleMessages(ids);
    if (ids.includes(selectedMessageId ?? '')) setSelectedMessageId(null);
    await Promise.allSettled(ids.map((id) => moveMessage(id, inboxFolder.id)));
    addToast(t('misc.mailPage.bulkRestored', { count: ids.length }));
  }, [folders, selectedMessageId, removeVisibleMessages, addToast]);

  const handleBulkMarkRead = useCallback(async (ids: string[]) => {
    const unreadCount = countUnreadVisible(ids);
    patchVisibleMessages(ids, { read: true });
    if (unreadCount > 0) adjustUnread(activeFolderId, -unreadCount);
    try {
      await bulkMarkRead(ids, true);
      addToast(t('misc.mailPage.bulkMarkedRead', { count: ids.length }), 'info');
    } catch {
      patchVisibleMessages(ids, { read: false });
      if (unreadCount > 0) adjustUnread(activeFolderId, unreadCount);
      addToast(t('misc.mailPage.markReadFailed'), 'error');
    }
  }, [messages, countUnreadVisible, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleBulkStar = useCallback(async (ids: string[], starred: boolean) => {
    patchVisibleMessages(ids, { starred });
    await Promise.allSettled(ids.map((id) => starMessage(id, starred)));
    addToast(starred ? t('misc.mailPage.starAdded', { count: ids.length }) : t('misc.mailPage.starRemoved', { count: ids.length }), 'info');
  }, [patchVisibleMessages, addToast]);

  const handleMarkAllRead = useCallback(async () => {
    const unreadIds = messages.filter((m) => !m.read).map((m) => m.id);
    if (unreadIds.length === 0) return;
    patchVisibleMessages(unreadIds, { read: true });
    adjustUnread(activeFolderId, -unreadIds.length);
    try {
      await bulkMarkRead(unreadIds, true);
      addToast(t('misc.mailPage.bulkMarkedRead', { count: unreadIds.length }), 'info');
    } catch {
      patchVisibleMessages(unreadIds, { read: false });
      adjustUnread(activeFolderId, unreadIds.length);
      addToast(t('misc.mailPage.markReadFailed'), 'error');
    }
  }, [messages, patchVisibleMessages, adjustUnread, activeFolderId, addToast]);

  const handleArchiveById = useCallback((id: string) => {
    const archiveFolder = folders.find((f) => f.system_type === 'archive');
    if (!archiveFolder) return;
    const msgToArchive = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (msgToArchive && !msgToArchive.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    if (selectedMessageId === id) setSelectedMessageId(nextId);
    addToast(t('misc.mailPage.archived'), 'info', {
      action: {
        label: t('misc.mailPage.undo'),
        onClick: () => {
          if (msgToArchive) {
            setMessages((prev) => [msgToArchive, ...prev]);
            setSearchResults((prev) => (prev ? [msgToArchive, ...prev] : prev));
            if (!msgToArchive.read) adjustUnread(activeFolderId, 1);
          }
        },
      },
    });
    void moveMessage(id, archiveFolder.id).catch(() => {
      if (msgToArchive) {
        setMessages((prev) => [msgToArchive, ...prev]);
        setSearchResults((prev) => (prev ? [msgToArchive, ...prev] : prev));
      }
    });
  }, [folders, getNextId, removeVisibleMessages, setMessages, selectedMessageId, messages, searchResults, addToast, adjustUnread, activeFolderId]);

  const handleArchive = useCallback(() => {
    if (!selectedMessageId) return;
    handleArchiveById(selectedMessageId);
  }, [selectedMessageId, handleArchiveById]);

  const handleSpam = useCallback(() => {
    if (!selectedMessageId) return;
    const spamFolder = folders.find((f) => f.system_type === 'spam' || f.system_type === 'junk');
    if (!spamFolder) return;
    const id = selectedMessageId;
    const spamMsg = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (spamMsg && !spamMsg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    setSelectedMessageId(nextId);
    addToast(t('misc.mailPage.movedToSpam'), 'info', {
      action: {
        label: t('misc.mailPage.undo'),
        onClick: () => {
          if (spamMsg) {
            setMessages((prev) => [spamMsg, ...prev]);
            setSearchResults((prev) => (prev ? [spamMsg, ...prev] : prev));
            if (!spamMsg.read) adjustUnread(activeFolderId, 1);
          }
        },
      },
    });
    void moveMessage(id, spamFolder.id).catch(() => {
      if (spamMsg) {
        setMessages((prev) => [spamMsg, ...prev]);
        setSearchResults((prev) => (prev ? [spamMsg, ...prev] : prev));
      }
      addToast(t('misc.mailPage.moveFailed'), 'error');
    });
  }, [selectedMessageId, folders, getNextId, removeVisibleMessages, setMessages, addToast, messages, searchResults, adjustUnread, activeFolderId]);

  const handleNotSpam = useCallback(() => {
    if (!selectedMessageId) return;
    const inboxFolder = folders.find((f) => f.system_type === 'inbox');
    if (!inboxFolder) return;
    const id = selectedMessageId;
    const notSpamMsg = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (notSpamMsg && !notSpamMsg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    void moveMessage(id, inboxFolder.id).then(() => {
      removeVisibleMessages([id]);
      setSelectedMessageId(nextId);
      addToast(t('misc.mailPage.movedToInbox'), 'info');
    }).catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
  }, [selectedMessageId, folders, getNextId, removeVisibleMessages, messages, searchResults, adjustUnread, activeFolderId, addToast]);

  const handleMove = useCallback(async (folderId: string) => {
    if (!selectedMessageId) return;
    const id = selectedMessageId;
    const msg = messages.find((m) => m.id === id) ?? searchResults?.find((m) => m.id === id);
    if (msg && !msg.read) adjustUnread(activeFolderId, -1);
    const nextId = getNextId(id);
    removeVisibleMessages([id]);
    setSelectedMessageId(nextId);
    moveMessage(id, folderId)
      .then(() => addToast(t('misc.mailPage.moved')))
      .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
  }, [selectedMessageId, getNextId, removeVisibleMessages, messages, searchResults, adjustUnread, activeFolderId, addToast]);

  const handleStar = useCallback(async (id: string, starred: boolean) => {
    patchVisibleMessages([id], { starred });
    starMessage(id, starred).catch(() => {
      patchVisibleMessages([id], { starred: !starred });
    });
  }, [patchVisibleMessages]);

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
    if (selectedMessageId) {
      setSelectedMessageId(null);
      return true;
    }
    return false;
  }, [composeContext, showSpotlight, contextMenu, showShortcuts, mobileSidebarOpen, selectedMessageId]);


  // Persist last-selected message per folder

  // Keyboard shortcuts (skip when typing in input/textarea/contenteditable)
  useEffect(() => {
    // Korean QWERTY → Latin normalization (allows shortcuts to work in Korean IME mode)
    const KO: Record<string, string> = {
      'ㄷ':'e','ㄱ':'r','ㅅ':'t','ㅛ':'y','ㅕ':'u','ㅑ':'i','ㅐ':'o','ㅔ':'p',
      'ㅁ':'a','ㄴ':'s','ㅇ':'d','ㄹ':'f','ㅎ':'g','ㅗ':'h','ㅓ':'j','ㅏ':'k','ㅣ':'l',
      'ㅋ':'z','ㅌ':'x','ㅊ':'c','ㅍ':'v','ㅠ':'b','ㅜ':'n','ㅡ':'m',
      'ㅂ':'q','ㅈ':'w',
    };
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        if (handleGlobalEscape()) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
        }
        return;
      }

      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      const key = KO[e.key] ?? e.key;
      const list = searchResults ?? messages;
      const currentIdx = list.findIndex((m) => m.id === selectedMessageId);
      const isMailApp = activeApp === 'mail';

      // g+key two-key folder/app navigation
      if (gPrefixRef.current) {
        gPrefixRef.current = false;
        if (key === 'u') {
          e.preventDefault();
          const firstUnread = list.find((m) => !m.read);
          if (firstUnread) setSelectedMessageId(firstUnread.id);
          return;
        }
        const virtualFolderMap: Record<string, string> = { w: VIRTUAL_TASKS, x: VIRTUAL_IMPORTANT };
        if (virtualFolderMap[key]) { e.preventDefault(); handleSelectFolder(virtualFolderMap[key]); return; }
        const systemTypeMap: Record<string, string> = { i: 'inbox', s: 'sent', t: 'trash', a: 'archive', p: 'spam' };
        const target = systemTypeMap[key];
        if (target) {
          const folder = folders.find((f) => f.system_type === target);
          if (folder) { e.preventDefault(); handleSelectFolder(folder.id); return; }
        }
        const appSwitchMap: Record<string, AppId> = { m: 'mail', c: 'calendar', a: 'contacts', k: 'contacts', d: 'drive', v: 'drive', ',': 'settings' };
        const appTarget = appSwitchMap[key];
        if (appTarget) { e.preventDefault(); setActiveApp(appTarget); return; }
      }

      if (key === 'g' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        gPrefixRef.current = true;
        setTimeout(() => { gPrefixRef.current = false; }, 1000);
        return;
      }

      if (!isMailApp) {
        switch (key) {
          case '?':
            setShowShortcuts((v) => !v);
            return;
          case '[':
            setSidebarCollapsed((v) => !v);
            return;
          case 'k':
            if (e.ctrlKey || e.metaKey) {
              e.preventDefault();
              setShowSpotlight(true);
            }
            return;
          default:
            return;
        }
      }

      switch (key) {
        case 'ArrowRight':
          if (moveMailPanelFocus('next', !!selectedMessageId && !isMobile)) e.preventDefault();
          return;
        case 'ArrowLeft':
          if (moveMailPanelFocus('prev', !!selectedMessageId && !isMobile)) e.preventDefault();
          return;
        case 'j': {
          const next = list[currentIdx + 1];
          if (next) setSelectedMessageId(next.id);
          break;
        }
        case 'k': {
          if (e.ctrlKey || e.metaKey) {
            e.preventDefault();
            setShowSpotlight(true);
          } else {
            const prev = list[currentIdx - 1];
            if (prev) setSelectedMessageId(prev.id);
          }
          break;
        }
        case 's':
          if (!e.ctrlKey && !e.metaKey && !e.altKey && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'new' });
          }
          break;
        case 'n': {
          // Next unread message
          const nextUnread = list.slice(currentIdx + 1).find((m) => !m.read);
          if (nextUnread) setSelectedMessageId(nextUnread.id);
          break;
        }
        case 'N': {
          // Prev unread message (Shift+n)
          const prevUnread = list.slice(0, currentIdx).reverse().find((m) => !m.read);
          if (prevUnread) setSelectedMessageId(prevUnread.id);
          break;
        }
        case 'u':
          if (selectedMessageId && !composeContext) handleMarkUnread();
          break;
        case 'm':
          if (selectedMessageId && !composeContext) void handleMarkRead();
          break;
        case 'M': // Shift+m
          if (selectedMessageId && !composeContext) void handleMarkUnread();
          break;
        case '!':
          if (selectedMessageId && !composeContext) handleSpam();
          break;
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
        case 'p': {
          if (selectedMessageId && !composeContext) handlePin(selectedMessageId);
          break;
        }
        case 'i': {
          if (selectedMessageId && !composeContext) {
            handleImportant(selectedMessageId);
            addToast(importantIds.has(selectedMessageId) ? t('misc.mailPage.importantUnmarked') : t('misc.mailPage.importantMarked'), 'info');
          }
          break;
        }
        case 't': {
          if (selectedMessageId && !composeContext && selectedMessage) {
            try {
              const tasks: unknown[] = JSON.parse(localStorage.getItem('webmail_tasks') ?? '[]');
              tasks.unshift({ id: crypto.randomUUID(), subject: selectedMessage.subject || t('misc.mailPage.noSubject'), from: selectedMessage.from_addr, messageId: selectedMessageId, done: false, createdAt: new Date().toISOString() });
              localStorage.setItem('webmail_tasks', JSON.stringify(tasks));
              addToast(t('misc.mailPage.taskAdded', { subject: selectedMessage.subject || t('misc.mailPage.noSubject') }), 'success');
            } catch { /* */ }
          }
          break;
        }
        case '#':
        case 'Delete':
          if (selectedMessageId && !composeContext) handleDelete();
          break;
        case 'o': {
          if (selectedMessageId && !composeContext) {
            if (!selectedMessageId) {
              const first = list[0];
              if (first) setSelectedMessageId(first.id);
            }
          } else if (!selectedMessageId && !composeContext) {
            const first = list[0];
            if (first) setSelectedMessageId(first.id);
          }
          break;
        }
        case 'v': {
          if (selectedMessageId && !composeContext) {
            e.preventDefault();
            setShowSpotlight(true);
            setSpotlightMoveId(selectedMessageId);
          }
          break;
        }
        case '?':
          setShowShortcuts((v) => !v);
          break;
        case '[':
          if (!composeContext) setSidebarCollapsed((v) => !v);
          break;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [messages, searchResults, selectedMessageId, selectedMessage, composeContext, openCompose, showShortcuts, handleDelete, handleArchive, handleSpam, handleMarkRead, handleMarkUnread, handleStar, getNextId, folders, messageLabels, setLabel, activeFolderSystemType, setActiveApp, showSpotlight, handleMove, handlePin, activeApp, isMobile, handleGlobalEscape]);

  const refreshRef = useRef(refresh);
  useEffect(() => { refreshRef.current = refresh; }, [refresh]);
  useEffect(() => {
    const id = setInterval(() => {
      if (document.visibilityState === 'visible') refreshRef.current();
    }, refreshIntervalSeconds * 1000);
    return () => clearInterval(id);
  }, [refreshIntervalSeconds]);

  // Register the service worker only when notifications were already allowed.
  // The permission prompt stays in Settings so entering webmail never surprises users.
  useEffect(() => {
    if (typeof Notification === 'undefined') return;
    const doSetup = async () => {
      if (Notification.permission === 'granted' && 'serviceWorker' in navigator && 'PushManager' in window) {
        try {
          await navigator.serviceWorker.register('/sw.js');
          // VAPID push subscription is handled in Settings when user explicitly enables notifications
        } catch {
          // ignore SW registration failure
        }
      }
    };
    doSetup().catch(() => {});
  }, []);

  useEffect(() => {
    getNotificationPreferences()
      .then((prefs) => {
        try {
          const threadOverrides = prefs.thread_overrides ?? {};
          setThreadNotificationOverrides(threadOverrides);
          window.localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(prefs.folder_overrides ?? {}));
          window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(threadOverrides));
          window.localStorage.setItem('webmail_dnd', prefs.global_dnd_enabled ? '1' : '0');
          const firstRange = prefs.global_dnd_schedule?.time_ranges?.[0];
          if (firstRange?.start) window.localStorage.setItem('webmail_dnd_start', firstRange.start);
          if (firstRange?.end) window.localStorage.setItem('webmail_dnd_end', firstRange.end);
        } catch {
          // local notification policy cache is best-effort
        }
      })
      .catch(() => {});
  }, []);

  const handleToggleThreadMute = useCallback(async () => {
    if (!selectedNotificationThreadId) return;
    const nextMuted = !selectedThreadMuted;
    const previous = threadNotificationOverrides;
    const next = { ...previous };
    if (nextMuted) {
      next[selectedNotificationThreadId] = { enabled: false };
    } else {
      delete next[selectedNotificationThreadId];
    }
    setThreadNotificationOverrides(next);
    try {
      window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(next));
      const base = await getNotificationPreferences();
      const saved = await setNotificationPreferences({
        ...base,
        thread_overrides: next,
      });
      const savedThreads = saved.thread_overrides ?? next;
      setThreadNotificationOverrides(savedThreads);
      window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(savedThreads));
    } catch {
      setThreadNotificationOverrides(previous);
      try {
        window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(previous));
      } catch {
        // local notification policy cache is best-effort
      }
    }
  }, [selectedNotificationThreadId, selectedThreadMuted, threadNotificationOverrides]);

  // Detect new unread messages after refresh and notify
  const seenMsgIdsRef = useRef<Set<string> | null>(null);
  useEffect(() => {
    if (messages.length === 0) return;
    if (seenMsgIdsRef.current === null) {
      seenMsgIdsRef.current = new Set(messages.map((m) => m.id));
      return;
    }
    const newUnread = messages.filter((m) =>
      !m.read &&
      !seenMsgIdsRef.current!.has(m.id) &&
      folderNotificationsEnabled(m.folder_id) &&
      threadNotificationsEnabled(m.thread_id, m.id)
    );
    messages.forEach((m) => seenMsgIdsRef.current!.add(m.id));
    // In-app notification center push is independent of OS-level permission/DnD.
    // Browser mirroring is centralized in the notification store so user toggles,
    // quiet hours, and click handling stay consistent across event sources.
    if (newUnread.length > 0) {
      for (const m of newUnread) {
        const sender = m.from_name || m.from_addr || '';
        let detail: 'sender' | 'subject' | 'preview' = 'subject';
        try {
          const stored = localStorage.getItem('webmail_notif_detail');
          if (stored === 'sender' || stored === 'subject' || stored === 'preview') detail = stored;
        } catch {
          // keep default detail
        }
        const body = detail === 'sender'
          ? undefined
          : ((detail === 'preview' ? m.preview : m.subject) || t('misc.mailPage.noSubject')).slice(0, 120);
        pushNotification({
          id: `mail_received_${m.id}`,
          category: 'mail_received',
          severity: 'info',
          title: tNotif('mailReceived', { sender }),
          body,
          actionUrl: `/mail/${m.id}`,
          metadata: { messageId: m.id },
        });
      }
    }
  }, [messages, pushNotification, tNotif]);

  // Reset seen IDs when folder changes (avoid false notifications on folder switch)
  useEffect(() => { seenMsgIdsRef.current = null; }, [activeFolderId]);

  // Apply client-side filter rules to newly loaded messages
  useEffect(() => {
    if (messages.length === 0) return;
    const rules = loadFilterRules().filter((r) => r.enabled);
    if (rules.length === 0) return;

    const labelUpdates: Record<string, string> = {};
    const markReadIds: string[] = [];
    const markUnreadIds: string[] = [];
    const markStarredIds: string[] = [];
    const trashIds: string[] = [];

    for (const msg of messages) {
      for (const rule of rules) {
        const condResults = rule.conditions.map((cond) => {
          if (cond.field === 'has_attachment') return !!(msg as MessageSummary & { has_attachment?: boolean }).has_attachment;
          if (cond.field === 'is_unread') return !msg.read;
          if (cond.field === 'size_larger') return ((msg as MessageSummary & { size?: number }).size ?? 0) > Number(cond.value);
          if (cond.field === 'size_smaller') return ((msg as MessageSummary & { size?: number }).size ?? Infinity) < Number(cond.value);
          const haystack = ((): string => {
            switch (cond.field) {
              case 'from': return (msg.from_addr + ' ' + (msg.from_name ?? '')).toLowerCase();
              case 'to': return ((msg as MessageSummary & { to?: string }).to ?? '').toLowerCase();
              case 'cc': return ((msg as MessageSummary & { cc?: string }).cc ?? '').toLowerCase();
              case 'subject': return (msg.subject ?? '').toLowerCase();
              case 'body': return (msg.preview ?? '').toLowerCase();
              default: return '';
            }
          })();
          const needle = cond.value.toLowerCase();
          switch (cond.matchType) {
            case 'contains': return haystack.includes(needle);
            case 'not_contains': return !haystack.includes(needle);
            case 'equals': return haystack.trim() === needle;
            case 'starts_with': return haystack.startsWith(needle);
            case 'ends_with': return haystack.endsWith(needle);
            case 'regex': try { return new RegExp(cond.value, 'i').test(haystack); } catch { return false; }
            default: return false;
          }
        });
        const matches = rule.logic === 'and' ? condResults.every(Boolean) : condResults.some(Boolean);
        if (!matches) continue;

        const a = rule.action;
        if (a.labelColor && !labelUpdates[msg.id]) labelUpdates[msg.id] = a.labelColor;
        if (a.markRead && !msg.read) markReadIds.push(msg.id);
        if (a.markUnread && msg.read) markUnreadIds.push(msg.id);
        if (a.markStarred && !msg.starred) markStarredIds.push(msg.id);
        if (a.deleteMsg) trashIds.push(msg.id);
        if (rule.stopProcessing) break;
      }
    }

    if (Object.keys(labelUpdates).length > 0) {
      setMessageLabels((prev) => {
        const next = { ...prev };
        let changed = false;
        for (const [id, color] of Object.entries(labelUpdates)) {
          if (!next[id]) { next[id] = color; changed = true; }
        }
        if (changed) { try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ } return next; }
        return prev;
      });
    }
    if (markReadIds.length > 0) {
      setMessages((prev) => prev.map((m) => markReadIds.includes(m.id) ? { ...m, read: true } : m));
      markReadIds.forEach((id) => markRead(id, true).catch(() => {}));
    }
    if (markUnreadIds.length > 0) {
      setMessages((prev) => prev.map((m) => markUnreadIds.includes(m.id) ? { ...m, read: false } : m));
      markUnreadIds.forEach((id) => markRead(id, false).catch(() => {}));
    }
    if (markStarredIds.length > 0) {
      setMessages((prev) => prev.map((m) => markStarredIds.includes(m.id) ? { ...m, starred: true } : m));
      markStarredIds.forEach((id) => starMessage(id, true).catch(() => {}));
    }
    if (trashIds.length > 0) {
      const trashFolder = folders.find((f) => f.system_type === 'trash');
      if (trashFolder) {
        setMessages((prev) => prev.filter((m) => !trashIds.includes(m.id)));
        trashIds.forEach((id) => moveMessage(id, trashFolder.id).catch(() => {}));
      }
    }
  }, [messages, folders]);

  // Snooze: hide message until a future time, then resurface it
  const handleSnooze = useCallback((id: string, until: Date) => {
    try {
      const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
      stored[id] = until.toISOString();
      localStorage.setItem('webmail_snoozed', JSON.stringify(stored));
    } catch { /* ignore */ }
    if (shouldHideMessageAfterSnooze(activeFolderId)) {
      setMessages((prev) => prev.filter((m) => m.id !== id));
      if (selectedMessageId === id) setSelectedMessageId(null);
    }
    addToast(t('misc.mailPage.snoozeNotifyAt', { time: until.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' }) }), 'info', { duration: 4000 });
  }, [activeFolderId, selectedMessageId, setMessages, addToast]);

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
        addToast(t('misc.mailPage.snoozeReturned', { count: expired.length }), 'info');
        refresh();
      } catch { /* ignore */ }
    };
    const id = setInterval(check, 60_000);
    check();
    return () => clearInterval(id);
  }, [addToast, refresh]);

  // Check for overdue follow-up reminders on load and every 5 minutes
  useEffect(() => {
    const checkFollowUps = () => {
      try {
        type FollowUp = { remindAt: string; subject: string; to: string; createdAt: string };
        const followups: FollowUp[] = JSON.parse(localStorage.getItem('webmail_followups') ?? '[]');
        const now = Date.now();
        const overdue = followups.filter((f) => new Date(f.remindAt).getTime() <= now);
        if (overdue.length === 0) return;
        const remaining = followups.filter((f) => new Date(f.remindAt).getTime() > now);
        localStorage.setItem('webmail_followups', JSON.stringify(remaining));
        overdue.forEach((f) => {
          addToast(t('misc.mailPage.followUpReminder', { subject: f.subject || t('misc.mailPage.noSubject') }), 'info', { duration: 8000 });
        });
      } catch { /* ignore */ }
    };
    checkFollowUps();
    const id = setInterval(checkFollowUps, 5 * 60_000);
    return () => clearInterval(id);
  }, [addToast]);

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
          {t('misc.mailPage.mustChangePassword')}
          <button
            onClick={() => { localStorage.removeItem('webmail_must_change_password'); setMustChangePassword(false); }}
            style={{ marginLeft: '12px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >{t('misc.mailPage.close')}</button>
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
          >{t('misc.mailPage.loginAgain')}</button>
          <button
            onClick={() => setSessionWarning(null)}
            style={{ marginLeft: '8px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >{t('misc.mailPage.close')}</button>
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
          {t('misc.mailPage.offline')}
        </div>
      )}

      <AppIconBar activeApp={activeApp} onChangeApp={setActiveApp} mailUnread={folders.reduce((s, f) => s + (f.unread ?? 0), 0)} />

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
            onDropMessage={(messageId, folderId) => {
              setMessages((prev) => prev.filter((m) => m.id !== messageId));
              if (selectedMessageId === messageId) setSelectedMessageId(null);
              moveMessage(messageId, folderId)
                .then(() => addToast(t('misc.mailPage.moved')))
                .catch(() => addToast(t('misc.mailPage.moveFailed'), 'error'));
            }}
            onCreateFolder={async (name) => {
              try { await createFolder(name); refresh(); addToast(t('misc.mailPage.folderCreated', { name })); }
              catch { addToast(t('misc.mailPage.folderCreateFailed'), 'error'); }
            }}
            onRenameFolder={async (id, name) => {
              try { await renameFolder(id, name); refresh(); addToast(t('misc.mailPage.folderRenamed')); }
              catch { addToast(t('misc.mailPage.folderRenameFailed'), 'error'); }
            }}
            onDeleteFolder={async (id) => {
              try { await deleteFolder(id); if (activeFolderId === id) setActiveFolderId(''); refresh(); addToast(t('misc.mailPage.folderDeleted')); }
              catch { addToast(t('misc.mailPage.folderDeleteFailed'), 'error'); }
            }}
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
              onRefresh={refresh}
              refreshing={refreshing}
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

      <MFASetupPromptModal
        onGoToSettings={() => {
          setSettingsInitialSection('security');
          setActiveApp('settings');
        }}
      />

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
              aria-label={t('misc.mailPage.readingRegion')}
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
                onPrint={selectedMessage ? () => {
                  const msg = selectedMessage;
                  const w = window.open('', '_blank', 'width=780,height=900,menubar=yes,toolbar=yes');
                  if (!w) { window.print(); return; }
                  const date = new Intl.DateTimeFormat('ko-KR', { dateStyle: 'full', timeStyle: 'short', hour12: false }).format(new Date(msg.received_at));
                  const body = msg.html_body
                    ? `<div>${msg.html_body}</div>`
                    : (msg.text_body || '').split('\n').map((l) => `<p style="margin:0 0 4px">${l || '&nbsp;'}</p>`).join('');
                  const emailOf = (a: MessageAddress) => a.email || a.address || '';
                  const subjectStr = msg.subject || t('misc.mailPage.noSubject');
                  const fromLbl = t('mail.from');
                  const toLbl = t('mail.to');
                  const dateLbl = t('mail.date');
                  w.document.write(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>${subjectStr}</title><style>body{font-family:-apple-system,sans-serif;font-size:14px;color:#111;max-width:720px;margin:0 auto;padding:24px}h1{font-size:20px;margin:0 0 12px}table{border-collapse:collapse;margin-bottom:16px;font-size:13px}td{padding:3px 8px 3px 0;vertical-align:top}td:first-child{color:#555;white-space:nowrap;min-width:80px}hr{border:none;border-top:1px solid #ddd;margin:16px 0}@media print{body{padding:0}}</style></head><body><h1>${subjectStr}</h1><table><tr><td>${fromLbl}</td><td><b>${msg.from_name ? `${msg.from_name} &lt;${msg.from_addr}&gt;` : msg.from_addr}</b></td></tr><tr><td>${toLbl}</td><td>${(msg.to_addrs ?? []).map((a) => a.name ? `${a.name} &lt;${emailOf(a)}&gt;` : emailOf(a)).join(', ')}</td></tr><tr><td>${dateLbl}</td><td>${date}</td></tr></table><hr>${body}</body></html>`);
                  w.document.close();
                  w.onload = () => w.print();
                } : undefined}
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
                  addToast(t('misc.mailPage.replySent'));
                } : undefined}
                onRestore={selectedMessageId && (activeFolderSystemType === 'trash' || activeFolderSystemType === 'archive') ? () => activeFolderSystemType === 'archive' ? handleRestoreFromArchive(selectedMessageId) : handleRestore(selectedMessageId) : undefined}
                onComposeToAddress={(address) => openCompose({ intent: 'new', to: address })}
                onSnooze={activeFolderSystemType !== 'trash' ? handleSnooze : undefined}
                onOpenInWindow={selectedMessageId ? () => window.open(`/mail/${selectedMessageId}`, '_blank', 'width=900,height=700,menubar=no,toolbar=no') : undefined}
                onToggleRead={selectedMessageId ? () => { const m = findVisibleMessage(selectedMessageId); if (m?.read) handleMarkUnread(); else void handleMarkRead(); } : undefined}
                isRead={selectedMessageId ? findVisibleMessage(selectedMessageId)?.read : undefined}
                onStar={selectedMessageId ? () => { const m = findVisibleMessage(selectedMessageId); if (m) handleStar(m.id, !m.starred); } : undefined}
                isStarred={selectedMessageId ? findVisibleMessage(selectedMessageId)?.starred : undefined}
                onToggleThreadMute={selectedNotificationThreadId ? handleToggleThreadMute : undefined}
                isThreadMuted={selectedThreadMuted}
                threadMessages={threadMessages.length > 1 ? threadMessages : undefined}
                onSelectThread={handleSelectMessage}
                userEmail={userEmail || undefined}
                externalImages={wmSettings.externalImages}
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
          initialSubject={composeContext.initialSubject}
          initialBody={composeContext.initialBody}
          userEmail={userEmail}
          isMobile={isMobile}
          onClose={closeCompose}
          onArchiveSource={(composeContext.intent === 'reply' || composeContext.intent === 'reply_all') && composeContext.source
            ? () => handleArchiveById(composeContext.source!.id)
            : undefined}
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
                      const msg = messages.find((m) => m.id === contextMenu.id);
                      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
                      setMessages((prev) => prev.filter((m) => m.id !== contextMenu.id));
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
          onOpenSettings={() => { setActiveApp('settings'); setShowSpotlight(false); }}
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
