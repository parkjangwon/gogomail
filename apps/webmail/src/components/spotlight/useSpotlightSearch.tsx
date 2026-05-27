'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import {
  autocompleteContacts,
  Calendar,
  CalendarObject,
  ContactObject,
  DriveNode,
  Folder,
  listAddressBooks,
  listCalendarObjects,
  listCalendars,
  listContacts,
  listDriveNodes,
  MessageSummary,
  parseVCard,
  searchMessages,
} from '@/lib/api';
import { parseEvents, parseTodos } from '@/lib/calendar/eventParser';
import { loadLocalEmailTemplates } from '@/lib/emailTemplates';
import { NAV_ITEMS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { useNotifications } from '@/lib/notifications/store';
import {
  CalendarDaysIcon,
  CheckCircleIcon,
  MagnifyingGlassIcon,
  PencilSquareIcon,
  StarIcon,
  FolderIcon,
  Cog6ToothIcon,
  UserCircleIcon,
  EnvelopeIcon,
  PaperClipIcon,
  DocumentTextIcon,
  DocumentIcon,
  BellIcon,
} from '@heroicons/react/24/outline';
import { SpotlightItem, SYSTEM_ICONS, SCOPES, SpotlightT, relativeTime, formatDriveSize } from './spotlightHelpers';

export interface SpotlightSearchProps {
  onClose: () => void;
  folders: Folder[];
  onSelectFolder: (id: string) => void;
  onCompose: () => void;
  onSelectMessage: (id: string, folderId?: string) => void;
  onOpenCalendar: () => void;
  onOpenDrive: () => void;
  onOpenSettings: (sectionId?: SectionId) => void;
  onOpenNotifications?: () => void;
  onSearch: (q: string) => void;
  onComposeToAddress?: (email: string) => void;
  movingMessageId?: string;
  onMoveMessage?: (folderId: string) => void;
  onComposeWithTemplate?: (t: { name: string; subject: string; body: string }) => void;
}

export function useSpotlightSearch(props: SpotlightSearchProps) {
  const {
    onClose,
    folders,
    onSelectFolder,
    onCompose,
    onSelectMessage,
    onOpenCalendar,
    onOpenDrive,
    onOpenSettings,
    onOpenNotifications,
    onSearch,
    onComposeToAddress,
    movingMessageId,
    onMoveMessage,
    onComposeWithTemplate,
  } = props;

  const t = useTranslations('spotlight');
  const tSettings = useTranslations('settingsView');
  const tNotif = useTranslations('notifications');
  const { notifications } = useNotifications();
  const isMoveMode = !!movingMessageId;
  const [query, setQuery] = useState('');
  const [scope, setScope] = useState<'all' | 'mail' | 'contacts' | 'calendar' | 'drive' | 'folders' | 'commands' | 'settings' | 'notifications'>('all');
  const [items, setItems] = useState<SpotlightItem[]>([]);
  const [selIdx, setSelIdx] = useState(0);
  const [searching, setSearching] = useState(false);
  const [activeOperators, setActiveOperators] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const contactCacheRef = useRef<SpotlightItem[] | null>(null);
  const calendarCacheRef = useRef<SpotlightItem[] | null>(null);
  const driveCacheRef = useRef<SpotlightItem[] | null>(null);

  // Build quick actions
  const buildQuickActions = useCallback((): SpotlightItem[] => {
    const systemFolderItems: SpotlightItem[] = [];
    for (const f of folders) {
      if (f.system_type === 'drafts' || f.system_type === 'spam') continue; // skip for move
      const icon = f.system_type ? (SYSTEM_ICONS[f.system_type] ?? <FolderIcon style={{ width: 16, height: 16 }} />) : <FolderIcon style={{ width: 16, height: 16 }} />;
      const label = f.system_type === 'inbox' ? t('folder.inbox') : f.system_type === 'sent' ? t('folder.sent') : f.system_type === 'drafts' ? t('folder.drafts') : f.system_type === 'trash' ? t('folder.trash') : f.system_type === 'spam' ? t('folder.spam') : f.name;
      const onSelect = isMoveMode && onMoveMessage
        ? () => { onMoveMessage(f.id); onClose(); }
        : () => { onSelectFolder(f.id); onClose(); };
      systemFolderItems.push({ type: 'folder', id: f.id, title: label, subtitle: f.unread ? t('folder.unreadSubtitle', { count: f.unread }) : undefined, icon, onSelect });
    }
    if (isMoveMode) return systemFolderItems;
    return [
      { type: 'action', id: 'compose', title: t('action.compose'), subtitle: 'S', icon: <PencilSquareIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onCompose(); onClose(); } },
      { type: 'action', id: 'starred', title: t('action.starred'), icon: <StarIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__starred__'); onClose(); } },
      { type: 'action', id: 'unread', title: t('action.unread'), icon: <EnvelopeIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__unread__'); onClose(); } },
      { type: 'action', id: 'attach', title: t('action.attachments'), icon: <PaperClipIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__attachments__'); onClose(); } },
      { type: 'action', id: 'settings', title: t('action.settings'), subtitle: ',', icon: <Cog6ToothIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onOpenSettings(); onClose(); } },
      ...systemFolderItems,
    ];
  }, [folders, onSelectFolder, onCompose, onOpenSettings, onClose, isMoveMode, onMoveMessage, t]);

  // Build contact items from localStorage
  const buildContactItems = useCallback((q: string): SpotlightItem[] => {
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      return Object.entries(contacts)
        .filter(([email, name]) => !q || email.includes(q) || name.toLowerCase().includes(q.toLowerCase()))
        .slice(0, 5)
        .map(([email, name]) => ({
          type: 'contact' as const,
          id: email,
          title: name || email,
          subtitle: email,
          icon: <UserCircleIcon style={{ width: 16, height: 16 }} />,
          onSelect: () => { onComposeToAddress?.(email); onClose(); },
        }));
    } catch { return []; }
  }, [onComposeToAddress, onClose]);

  const buildRemoteContactItems = useCallback(async (q: string): Promise<SpotlightItem[]> => {
    const ql = q.toLowerCase();
    const suggestions = q ? await autocompleteContacts(q, 12) : [];
    if (!contactCacheRef.current) {
      const books = await listAddressBooks();
      const contactRows = await Promise.all(books.map(async (book) => {
        const contacts = await listContacts(book.ID);
        return contacts.map((contact: ContactObject) => ({ bookName: book.Name, contact }));
      }));
      contactCacheRef.current = contactRows.flat().map(({ bookName, contact }) => {
        const parsed = parseVCard(contact.VCard);
        const email = parsed.email || '';
        const title = parsed.fn || email || contact.ObjectName;
        const subtitle = [email, parsed.org, bookName].filter(Boolean).join(' · ');
        return {
          type: 'contact' as const,
          id: `contact-${contact.ID}`,
          title,
          subtitle,
          icon: <UserCircleIcon style={{ width: 16, height: 16 }} />,
          onSelect: () => { if (email) onComposeToAddress?.(email); onClose(); },
        };
      });
    }
    const fromSuggestions = suggestions.map((suggestion) => ({
      type: 'contact' as const,
      id: `contact-suggestion-${suggestion.email}`,
      title: suggestion.display_name || suggestion.email,
      subtitle: [suggestion.email, suggestion.organization].filter(Boolean).join(' · '),
      icon: <UserCircleIcon style={{ width: 16, height: 16 }} />,
      onSelect: () => { onComposeToAddress?.(suggestion.email); onClose(); },
    }));
    const seen = new Set<string>();
    return [...fromSuggestions, ...contactCacheRef.current]
      .filter((item) => !ql || `${item.title} ${item.subtitle ?? ''}`.toLowerCase().includes(ql))
      .filter((item) => {
        const key = (item.subtitle || item.title).toLowerCase();
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      })
      .slice(0, 8);
  }, [onClose, onComposeToAddress]);

  const buildCalendarItems = useCallback(async (q: string): Promise<SpotlightItem[]> => {
    const ql = q.toLowerCase();
    if (!calendarCacheRef.current) {
      const calendars = await listCalendars();
      const objects = (await Promise.all(calendars.map((calendar: Calendar) => listCalendarObjects(calendar.ID)))).flat() as CalendarObject[];
      const events = parseEvents(objects, calendars).map((event) => ({
        type: 'calendar' as const,
        id: `calendar-event-${event.obj.ID}`,
        title: event.summary,
        subtitle: [
          event.allDay
            ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(event.start)
            : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(event.start),
          event.location,
        ].filter(Boolean).join(' · '),
        badge: t('calendar.event'),
        icon: <CalendarDaysIcon style={{ width: 16, height: 16 }} />,
        onSelect: () => { onOpenCalendar(); onClose(); },
      }));
      const todos = parseTodos(objects, calendars).map((todo) => ({
        type: 'calendar' as const,
        id: `calendar-todo-${todo.obj.ID}`,
        title: todo.summary,
        subtitle: [todo.dueDate ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(todo.dueDate) : '', todo.description].filter(Boolean).join(' · '),
        badge: todo.completed ? t('calendar.completed') : t('calendar.todo'),
        icon: <CheckCircleIcon style={{ width: 16, height: 16 }} />,
        onSelect: () => { onOpenCalendar(); onClose(); },
      }));
      calendarCacheRef.current = [...events, ...todos];
    }
    return calendarCacheRef.current
      .filter((item: SpotlightItem) => !ql || `${item.title} ${item.subtitle ?? ''}`.toLowerCase().includes(ql))
      .slice(0, 8);
  }, [onClose, onOpenCalendar, t]);

  const buildDriveItems = useCallback(async (q: string): Promise<SpotlightItem[]> => {
    const ql = q.toLowerCase();
    if (!driveCacheRef.current) {
      const collected: DriveNode[] = [];
      const queue: Array<string | undefined> = [undefined];
      const visited = new Set<string>();
      while (queue.length > 0 && collected.length < 120) {
        const parentId = queue.shift();
        const key = parentId ?? '__root__';
        if (visited.has(key)) continue;
        visited.add(key);
        const nodes = await listDriveNodes(parentId);
        collected.push(...nodes);
        for (const node of nodes) {
          if (node.node_type === 'folder') queue.push(node.id);
          if (queue.length + collected.length >= 160) break;
        }
      }
      driveCacheRef.current = collected.map((node) => ({
        type: 'drive' as const,
        id: `drive-${node.id}`,
        title: node.name,
        subtitle: node.node_type === 'folder' ? t('drive.folder') : node.mime_type || t('drive.file'),
        badge: node.node_type === 'file' && node.size ? formatDriveSize(node.size) : undefined,
        icon: node.node_type === 'folder' ? <FolderIcon style={{ width: 16, height: 16 }} /> : <DocumentIcon style={{ width: 16, height: 16 }} />,
        onSelect: () => { onOpenDrive(); onClose(); },
      }));
    }
    return driveCacheRef.current
      .filter((item: SpotlightItem) => !ql || `${item.title} ${item.subtitle ?? ''}`.toLowerCase().includes(ql))
      .slice(0, 8);
  }, [onClose, onOpenDrive, t]);

  // Parse Gmail-style operators from a query string
  function parseQuery(raw: string): { params: Record<string, string | boolean>; freeText: string; operators: string[] } {
    const operatorRe = /\b(from|to|subject):(\S+)|\bhas:attachment\b/gi;
    const params: Record<string, string | boolean> = {};
    const operators: string[] = [];
    const freeText = raw.replace(operatorRe, (match, key, val) => {
      if (match.toLowerCase() === 'has:attachment') {
        params.has_attachment = true;
        operators.push('has:attachment');
      } else {
        params[key.toLowerCase()] = val;
        operators.push(`${key.toLowerCase()}:${val}`);
      }
      return '';
    }).replace(/\s+/g, ' ').trim();
    return { params, freeText, operators };
  }

  const buildTemplateItems = useCallback((q: string): SpotlightItem[] => {
    if (!onComposeWithTemplate) return [];
    try {
      const templates = loadLocalEmailTemplates();
      return templates
        .filter((tpl) => !q || tpl.name.toLowerCase().includes(q.toLowerCase()) || tpl.subject.toLowerCase().includes(q.toLowerCase()))
        .slice(0, 5)
        .map((tpl) => ({
          type: 'template' as const,
          id: `tpl-${tpl.name}`,
          title: tpl.name,
          subtitle: tpl.subject || t('noSubject'),
          icon: <DocumentTextIcon style={{ width: 16, height: 16 }} />,
          onSelect: () => { onComposeWithTemplate(tpl); onClose(); },
        }));
    } catch { return []; }
  }, [onComposeWithTemplate, onClose, t]);

  const buildSettingsItems = useCallback((q: string): SpotlightItem[] => {
    const ql = q.toLowerCase();
    return NAV_ITEMS
      .filter((item) => !ql || tSettings(item.labelKey as Parameters<typeof tSettings>[0]).toLowerCase().includes(ql))
      .map((item) => ({
        type: 'settings' as const,
        id: `settings-${item.id}`,
        title: tSettings(item.labelKey as Parameters<typeof tSettings>[0]),
        subtitle: t('action.settings'),
        icon: item.icon,
        onSelect: () => { onOpenSettings(item.id); onClose(); },
      }));
  }, [tSettings, t, onOpenSettings, onClose]);

  const buildNotificationItems = useCallback((q: string): SpotlightItem[] => {
    const ql = q.toLowerCase();
    return notifications
      .filter((n) => !ql || `${n.title} ${n.body ?? ''}`.toLowerCase().includes(ql))
      .slice(0, 8)
      .map((n) => ({
        type: 'notification' as const,
        id: `notif-${n.id}`,
        title: n.title,
        subtitle: n.body,
        badge: n.read ? undefined : tNotif('unread'),
        icon: <BellIcon style={{ width: 16, height: 16 }} />,
        onSelect: () => { onOpenNotifications?.(); onClose(); },
      }));
  }, [notifications, tNotif, onOpenNotifications, onClose]);

  const recentSearchKey = 'webmail_recent_searches';
  const [recentSearches, setRecentSearches] = useState<string[]>(() => {
    try { return JSON.parse(localStorage.getItem(recentSearchKey) ?? '[]').slice(0, 4) as string[]; } catch { return []; }
  });
  const clearRecentSearch = useCallback((q: string) => {
    const next = recentSearches.filter((x: string) => x !== q);
    localStorage.setItem(recentSearchKey, JSON.stringify(next));
    setRecentSearches(next);
  }, [recentSearches, recentSearchKey]);

  // Do NOT reset scope when query changes — user explicitly selected it with ←/→ or click.

  // Update items based on query
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const q = query.trim();

    if (!q) {
      setActiveOperators([]);
      const quickActions = buildQuickActions();
      const contacts = isMoveMode ? [] : buildContactItems('').slice(0, 3);
      const templates = isMoveMode ? [] : buildTemplateItems('').slice(0, 3);
      const notificationItems = isMoveMode ? [] : buildNotificationItems('').slice(0, 3);
      setItems([...quickActions, ...contacts, ...templates, ...notificationItems]);
      setSelIdx(0);
      return;
    }

    // Immediate: filter actions + local contacts + templates + settings + notifications
    const ql = q.toLowerCase();
    const actions = buildQuickActions().filter((a: SpotlightItem) =>
      a.title.toLowerCase().includes(ql) || (a.subtitle ?? '').toLowerCase().includes(ql)
    );
    const localContacts = isMoveMode ? [] : buildContactItems(ql);
    const templates = isMoveMode ? [] : buildTemplateItems(ql);
    const settingsItems = isMoveMode ? [] : buildSettingsItems(ql).slice(0, 5);
    const notificationItems = isMoveMode ? [] : buildNotificationItems(ql).slice(0, 5);
    setItems([...actions, ...localContacts, ...templates, ...settingsItems, ...notificationItems]);
    setSelIdx(0);

    if (isMoveMode) return;

    // Parse operators
    const { params: opParams, freeText, operators } = parseQuery(q);
    setActiveOperators(operators);

    // Debounced: search mail, contacts, calendar, and drive with operator support
    setSearching(true);
    debounceRef.current = setTimeout(async () => {
      try {
        const searchParams: Record<string, string | boolean | number> = { limit: 8 };
        if (freeText) searchParams.q = freeText;
        if (opParams.from) searchParams.from = opParams.from as string;
        if (opParams.to) searchParams.to = opParams.to as string;
        if (opParams.subject) searchParams.subject = opParams.subject as string;
        if (opParams.has_attachment) searchParams.has_attachment = true;
        const [res, remoteContacts, calendarItems, driveItems] = await Promise.all([
          searchMessages(searchParams as Parameters<typeof searchMessages>[0]),
          buildRemoteContactItems(q),
          buildCalendarItems(q),
          buildDriveItems(q),
        ]);
        const mailItems: SpotlightItem[] = (res.messages ?? []).map((m: MessageSummary) => ({
          type: 'mail' as const,
          id: m.id,
          title: m.subject || t('noSubject'),
          subtitle: m.from_name || m.from_addr,
          badge: relativeTime(t as SpotlightT, m.received_at),
          icon: <EnvelopeIcon style={{ width: 16, height: 16, opacity: m.read ? 0.5 : 1 }} />,
          onSelect: () => { onSelectMessage(m.id); onClose(); },
        }));
        // "search all" action at the end
        const searchAll: SpotlightItem = {
          type: 'action',
          id: '__search_all__',
          title: t('searchAll', { q }),
          icon: <MagnifyingGlassIcon style={{ width: 16, height: 16 }} />,
          onSelect: () => { onSearch(q); onClose(); },
        };
        setItems([...actions, ...remoteContacts, ...templates, ...settingsItems, ...notificationItems, ...mailItems, ...calendarItems, ...driveItems, searchAll]);
        setSelIdx(0);
      } catch { /* */ }
      setSearching(false);
    }, 200);

    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [query, buildQuickActions, buildContactItems, buildTemplateItems, buildSettingsItems, buildNotificationItems, buildRemoteContactItems, buildCalendarItems, buildDriveItems, onSelectMessage, onSearch, onClose, isMoveMode, t]);

  // Apply scope filter to items
  const scopeTypeMap: Record<typeof scope, SpotlightItem['type'][] | null> = {
    all: null,
    mail: ['mail'],
    contacts: ['contact'],
    calendar: ['calendar'],
    drive: ['drive'],
    folders: ['folder'],
    commands: ['action', 'template'],
    settings: ['settings'],
    notifications: ['notification'],
  };
  const allowedTypes = scopeTypeMap[scope];
  const visibleItems = allowedTypes ? items.filter((i: SpotlightItem) => allowedTypes.includes(i.type)) : items;

  // Keyboard navigation
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setSelIdx((i: number) => Math.min(i + 1, visibleItems.length - 1)); }
      if (e.key === 'ArrowUp') { e.preventDefault(); setSelIdx((i: number) => Math.max(i - 1, 0)); }
      if (e.key === 'Enter') { e.preventDefault(); visibleItems[selIdx]?.onSelect(); }
      if (e.key === 'Escape') { e.preventDefault(); onClose(); }
      if (e.key === 'Tab') { e.preventDefault(); setSelIdx((i: number) => (e.shiftKey ? Math.max(i - 1, 0) : Math.min(i + 1, visibleItems.length - 1))); }
      // Left/right arrows cycle scope filter chips — only when query is empty so
      // normal cursor movement in the text input is not interrupted.
      if (!isMoveMode && !query && e.key === 'ArrowLeft') {
        e.preventDefault();
        setScope((s: typeof scope) => { const i = SCOPES.indexOf(s); return SCOPES[(i - 1 + SCOPES.length) % SCOPES.length]; });
      }
      if (!isMoveMode && !query && e.key === 'ArrowRight') {
        e.preventDefault();
        setScope((s: typeof scope) => { const i = SCOPES.indexOf(s); return SCOPES[(i + 1) % SCOPES.length]; });
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [visibleItems, selIdx, onClose, isMoveMode, query]);

  // Scroll selected item into view
  useEffect(() => {
    if (!listRef.current) return;
    const el = listRef.current.querySelector<HTMLElement>(`[data-idx="${selIdx}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [selIdx]);

  useEffect(() => { inputRef.current?.focus(); }, []);

  return {
    t,
    tSettings,
    query,
    setQuery,
    scope,
    setScope,
    items,
    selIdx,
    setSelIdx,
    searching,
    activeOperators,
    inputRef,
    listRef,
    recentSearches,
    clearRecentSearch,
    isMoveMode,
    visibleItems,
  };
}
