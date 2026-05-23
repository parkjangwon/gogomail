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
import {
  CalendarDaysIcon,
  CheckCircleIcon,
  MagnifyingGlassIcon,
  PencilSquareIcon,
  InboxIcon,
  StarIcon,
  FolderIcon,
  Cog6ToothIcon,
  UserCircleIcon,
  EnvelopeIcon,
  PaperClipIcon,
  ArrowRightIcon,
  ClockIcon,
  TrashIcon,
  PaperAirplaneIcon,
  ArchiveBoxIcon,
  DocumentTextIcon,
  DocumentIcon,
} from '@heroicons/react/24/outline';
import { ReactNode } from 'react';

interface SpotlightItem {
  type: 'action' | 'mail' | 'contact' | 'calendar' | 'drive' | 'folder' | 'template';
  id: string;
  title: string;
  subtitle?: string;
  badge?: string;
  icon: ReactNode;
  onSelect: () => void;
}

interface SpotlightSearchProps {
  onClose: () => void;
  folders: Folder[];
  onSelectFolder: (id: string) => void;
  onCompose: () => void;
  onSelectMessage: (id: string, folderId?: string) => void;
  onOpenCalendar: () => void;
  onOpenDrive: () => void;
  onOpenSettings: () => void;
  onSearch: (q: string) => void;
  onComposeToAddress?: (email: string) => void;
  movingMessageId?: string;
  onMoveMessage?: (folderId: string) => void;
  onComposeWithTemplate?: (t: { name: string; subject: string; body: string }) => void;
}

const SYSTEM_ICONS: Record<string, ReactNode> = {
  inbox: <InboxIcon style={{ width: 16, height: 16 }} />,
  sent: <PaperAirplaneIcon style={{ width: 16, height: 16 }} />,
  drafts: <PencilSquareIcon style={{ width: 16, height: 16 }} />,
  trash: <TrashIcon style={{ width: 16, height: 16 }} />,
  spam: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
  archive: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
};

type SpotlightT = ReturnType<typeof useTranslations>;

function sectionLabel(t: SpotlightT, type: SpotlightItem['type']): string {
  switch (type) {
    case 'action': return t('section.action');
    case 'folder': return t('section.folder');
    case 'mail': return t('section.mail');
    case 'contact': return t('section.contact');
    case 'calendar': return t('section.calendar');
    case 'drive': return t('section.drive');
    case 'template': return t('section.template');
  }
}

function relativeTime(t: SpotlightT, iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return t('time.justNow');
  if (m < 60) return t('time.minutesAgo', { n: m });
  const h = Math.floor(m / 60);
  if (h < 24) return t('time.hoursAgo', { n: h });
  const d = Math.floor(h / 24);
  if (d < 7) return t('time.daysAgo', { n: d });
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(new Date(iso));
}

function formatDriveSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '';
  if (bytes < 1024) return `${bytes}B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = bytes / 1024;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value >= 10 ? Math.round(value) : value.toFixed(1)}${units[index]}`;
}

export function SpotlightSearch({
  onClose,
  folders,
  onSelectFolder,
  onCompose,
  onSelectMessage,
  onOpenCalendar,
  onOpenDrive,
  onOpenSettings,
  onSearch,
  onComposeToAddress,
  movingMessageId,
  onMoveMessage,
  onComposeWithTemplate,
}: SpotlightSearchProps) {
  const t = useTranslations('spotlight');
  const isMoveMode = !!movingMessageId;
  const [query, setQuery] = useState('');
  const [scope, setScope] = useState<'all' | 'mail' | 'contacts' | 'calendar' | 'drive' | 'folders' | 'commands'>('all');
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
      .filter((item) => !ql || `${item.title} ${item.subtitle ?? ''}`.toLowerCase().includes(ql))
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
      .filter((item) => !ql || `${item.title} ${item.subtitle ?? ''}`.toLowerCase().includes(ql))
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

  const recentSearchKey = 'webmail_recent_searches';
  const recentSearches: string[] = (() => {
    try { return JSON.parse(localStorage.getItem(recentSearchKey) ?? '[]').slice(0, 4) as string[]; } catch { return []; }
  })();

  // Reset scope when query changes
  useEffect(() => { setScope('all'); }, [query]);

  // Update items based on query
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const q = query.trim();

    if (!q) {
      setActiveOperators([]);
      const quickActions = buildQuickActions();
      const contacts = isMoveMode ? [] : buildContactItems('').slice(0, 3);
      const templates = isMoveMode ? [] : buildTemplateItems('').slice(0, 3);
      setItems([...quickActions, ...contacts, ...templates]);
      setSelIdx(0);
      return;
    }

    // Immediate: filter actions + local contacts + templates
    const ql = q.toLowerCase();
    const actions = buildQuickActions().filter((a) =>
      a.title.toLowerCase().includes(ql) || (a.subtitle ?? '').toLowerCase().includes(ql)
    );
    const localContacts = isMoveMode ? [] : buildContactItems(ql);
    const templates = isMoveMode ? [] : buildTemplateItems(ql);
    setItems([...actions, ...localContacts, ...templates]);
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
          badge: relativeTime(t, m.received_at),
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
        setItems([...actions, ...remoteContacts, ...templates, ...mailItems, ...calendarItems, ...driveItems, searchAll]);
        setSelIdx(0);
      } catch { /* */ }
      setSearching(false);
    }, 200);

    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [query, buildQuickActions, buildContactItems, buildTemplateItems, buildRemoteContactItems, buildCalendarItems, buildDriveItems, onSelectMessage, onSearch, onClose, isMoveMode, t]);

  // Apply scope filter to items
  const scopeTypeMap: Record<typeof scope, SpotlightItem['type'][] | null> = {
    all: null,
    mail: ['mail'],
    contacts: ['contact'],
    calendar: ['calendar'],
    drive: ['drive'],
    folders: ['folder'],
    commands: ['action', 'template'],
  };
  const allowedTypes = scopeTypeMap[scope];
  const visibleItems = allowedTypes ? items.filter((i) => allowedTypes.includes(i.type)) : items;

  // Keyboard navigation
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setSelIdx((i) => Math.min(i + 1, visibleItems.length - 1)); }
      if (e.key === 'ArrowUp') { e.preventDefault(); setSelIdx((i) => Math.max(i - 1, 0)); }
      if (e.key === 'Enter') { e.preventDefault(); visibleItems[selIdx]?.onSelect(); }
      if (e.key === 'Escape') { e.preventDefault(); onClose(); }
      if (e.key === 'Tab') { e.preventDefault(); setSelIdx((i) => (e.shiftKey ? Math.max(i - 1, 0) : Math.min(i + 1, visibleItems.length - 1))); }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [visibleItems, selIdx, onClose]);

  // Scroll selected item into view
  useEffect(() => {
    if (!listRef.current) return;
    const el = listRef.current.querySelector<HTMLElement>(`[data-idx="${selIdx}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [selIdx]);

  useEffect(() => { inputRef.current?.focus(); }, []);

  // Group items by type for section labels
  const grouped: { label: string; items: (SpotlightItem & { idx: number })[] }[] = [];
  let globalIdx = 0;
  const seen = new Set<string>();
  for (const item of visibleItems) {
    if (!seen.has(item.type)) {
      seen.add(item.type);
      grouped.push({ label: sectionLabel(t, item.type), items: [] });
    }
    grouped[grouped.length - 1].items.push({ ...item, idx: globalIdx++ });
  }

  return (
    <div
      aria-modal="true"
      role="dialog"
      aria-label={t('dialogLabel')}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      style={{
        position: 'fixed', inset: 0, zIndex: 900,
        background: 'rgba(0,0,0,0.45)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '12vh',
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: '600px',
          margin: '0 16px',
          background: 'var(--color-bg-primary)',
          borderRadius: '14px',
          boxShadow: '0 24px 80px rgba(0,0,0,0.3)',
          overflow: 'hidden',
          border: '1px solid var(--color-border-default)',
          animation: 'spotlightIn 120ms cubic-bezier(0.16,1,0.3,1)',
        }}
      >
        {/* Move mode badge */}
        {isMoveMode && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 18px 0', borderBottom: 'none' }}>
            <ArrowRightIcon style={{ width: 13, height: 13, color: 'var(--color-accent)' }} />
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-accent)' }}>{t('moveBadge')}</span>
          </div>
        )}

        {/* Search input */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: isMoveMode ? '8px 18px 14px' : '14px 18px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          {searching
            ? <ArrowRightIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0, animation: 'spin 600ms linear infinite' }} />
            : <MagnifyingGlassIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
          }
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={isMoveMode ? t('placeholderMove') : t('placeholderSearch')}
            aria-label={isMoveMode ? t('ariaMove') : t('ariaSearch')}
            style={{
              flex: 1,
              border: 'none',
              outline: 'none',
              background: 'transparent',
              fontSize: '16px',
              color: 'var(--color-text-primary)',
              fontFamily: 'inherit',
            }}
          />
          <kbd style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '4px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', border: '1px solid var(--color-border-default)', flexShrink: 0 }}>Esc</kbd>
        </div>

        {/* Scope filter chips */}
        {!isMoveMode && (
          <div style={{ display: 'flex', gap: '6px', padding: '6px 16px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, flexWrap: 'wrap' }}>
            {(['all', 'mail', 'contacts', 'calendar', 'drive', 'folders', 'commands'] as const).map((s) => {
              const labels: Record<typeof s, string> = { all: t('scope.all'), mail: t('scope.mail'), contacts: t('scope.contacts'), calendar: t('scope.calendar'), drive: t('scope.drive'), folders: t('scope.folders'), commands: t('scope.commands') };
              return (
                <button key={s} type="button" onClick={() => setScope(s)}
                  style={{ padding: '3px 10px', borderRadius: '12px', border: 'none', cursor: 'pointer', fontSize: '12px', fontWeight: 500,
                    background: scope === s ? 'var(--color-accent)' : 'var(--color-bg-tertiary)',
                    color: scope === s ? '#fff' : 'var(--color-text-secondary)',
                  }}>
                  {labels[s]}
                </button>
              );
            })}
          </div>
        )}

        {/* Active operator chips */}
        {activeOperators.length > 0 && !isMoveMode && (
          <div style={{ display: 'flex', gap: '6px', padding: '4px 18px', flexWrap: 'wrap' }}>
            {activeOperators.map((op) => (
              <span key={op} style={{ display: 'inline-flex', alignItems: 'center', gap: '3px', fontSize: '11px', fontWeight: 600, color: 'var(--color-accent)', background: 'var(--color-accent-subtle)', borderRadius: '4px', padding: '2px 7px', letterSpacing: '0.02em' }}>
                {op}
              </span>
            ))}
          </div>
        )}

        {/* Recent searches (shown only when empty + no query, not in move mode) */}
        {!query && recentSearches.length > 0 && !isMoveMode && (
          <div style={{ padding: '8px 12px 0' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '4px 6px', letterSpacing: '0.05em', textTransform: 'uppercase' }}>{t('recentSearches')}</div>
            {recentSearches.map((q) => (
              <button
                key={q}
                onMouseDown={() => setQuery(q)}
                style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '6px 6px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', borderRadius: '6px', textAlign: 'left' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >
                <ClockIcon style={{ width: 13, height: 13, color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
                {q}
              </button>
            ))}
          </div>
        )}

        {/* Results */}
        <div ref={listRef} style={{ maxHeight: '420px', overflowY: 'auto', padding: '8px 12px 12px' }}>
          {visibleItems.length === 0 && query && !searching && (
            <div style={{ padding: '32px', textAlign: 'center', fontSize: '14px', color: 'var(--color-text-tertiary)' }}>
              {t('noResults')}
            </div>
          )}
          {grouped.map((group) => (
            <div key={group.label}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '8px 6px 4px', letterSpacing: '0.05em', textTransform: 'uppercase' }}>
                {group.label}
              </div>
              {group.items.map((item) => {
                const isSel = item.idx === selIdx;
                return (
                  <button
                    key={item.id}
                    data-idx={item.idx}
                    onMouseEnter={() => setSelIdx(item.idx)}
                    onMouseDown={(e) => { e.preventDefault(); item.onSelect(); }}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '10px',
                      width: '100%',
                      padding: '8px 10px',
                      border: 'none',
                      borderRadius: '8px',
                      background: isSel ? 'var(--color-accent)' : 'transparent',
                      color: isSel ? '#fff' : 'var(--color-text-primary)',
                      cursor: 'pointer',
                      textAlign: 'left',
                      transition: 'background 80ms ease',
                    }}
                  >
                    <span style={{ flexShrink: 0, opacity: isSel ? 1 : 0.7, display: 'inline-flex' }}>
                      {item.icon}
                    </span>
                    <span style={{ flex: 1, minWidth: 0 }}>
                      <span style={{ fontSize: '14px', fontWeight: 500, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {item.title}
                      </span>
                      {item.subtitle && (
                        <span style={{ fontSize: '12px', opacity: isSel ? 0.8 : 0.6, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {item.subtitle}
                        </span>
                      )}
                    </span>
                    {item.badge && (
                      <span style={{ fontSize: '11px', opacity: isSel ? 0.8 : 0.5, flexShrink: 0, whiteSpace: 'nowrap' }}>
                        {item.badge}
                      </span>
                    )}
                    {item.type === 'action' && item.subtitle && item.subtitle.length <= 3 && (
                      <kbd style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '4px', background: isSel ? 'rgba(255,255,255,0.2)' : 'var(--color-bg-tertiary)', border: `1px solid ${isSel ? 'rgba(255,255,255,0.2)' : 'var(--color-border-default)'}`, color: isSel ? '#fff' : 'var(--color-text-tertiary)', flexShrink: 0 }}>
                        {item.subtitle}
                      </kbd>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
        </div>

        {/* Footer hint */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px 18px', borderTop: '1px solid var(--color-border-subtle)', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
          <span><kbd style={kbdStyle}>↑↓</kbd> {t('footer.navigate')}</span>
          <span><kbd style={kbdStyle}>↵</kbd> {t('footer.select')}</span>
          <span><kbd style={kbdStyle}>Esc</kbd> {t('footer.close')}</span>
          <span style={{ marginLeft: 'auto' }}>{t('footer.brand')}</span>
        </div>
      </div>

      <style>{`
        @keyframes spotlightIn {
          from { opacity: 0; transform: scale(0.96) translateY(-8px); }
          to   { opacity: 1; transform: scale(1) translateY(0); }
        }
        @keyframes spin {
          from { transform: rotate(0deg); }
          to   { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

const kbdStyle: React.CSSProperties = {
  display: 'inline-block',
  padding: '1px 5px',
  borderRadius: '4px',
  background: 'var(--color-bg-tertiary)',
  border: '1px solid var(--color-border-default)',
  fontSize: '10px',
  fontFamily: 'inherit',
  color: 'var(--color-text-secondary)',
  marginRight: '3px',
};
