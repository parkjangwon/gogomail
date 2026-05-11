'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { searchMessages, MessageSummary, Folder } from '@/lib/api';
import {
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
} from '@heroicons/react/24/outline';
import { ReactNode } from 'react';

interface SpotlightItem {
  type: 'action' | 'mail' | 'contact' | 'folder';
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
  onOpenSettings: () => void;
  onSearch: (q: string) => void;
}

const SYSTEM_ICONS: Record<string, ReactNode> = {
  inbox: <InboxIcon style={{ width: 16, height: 16 }} />,
  sent: <PaperAirplaneIcon style={{ width: 16, height: 16 }} />,
  drafts: <PencilSquareIcon style={{ width: 16, height: 16 }} />,
  trash: <TrashIcon style={{ width: 16, height: 16 }} />,
  spam: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
  archive: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
};

function sectionLabel(type: SpotlightItem['type']): string {
  switch (type) {
    case 'action': return '빠른 실행';
    case 'folder': return '폴더';
    case 'mail': return '메일';
    case 'contact': return '연락처';
  }
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return '방금';
  if (m < 60) return `${m}분 전`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}시간 전`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}일 전`;
  return new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric' }).format(new Date(iso));
}

export function SpotlightSearch({
  onClose,
  folders,
  onSelectFolder,
  onCompose,
  onSelectMessage,
  onOpenSettings,
  onSearch,
}: SpotlightSearchProps) {
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<SpotlightItem[]>([]);
  const [selIdx, setSelIdx] = useState(0);
  const [searching, setSearching] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Build quick actions
  const buildQuickActions = useCallback((): SpotlightItem[] => {
    const systemFolderItems: SpotlightItem[] = [];
    for (const f of folders) {
      const icon = f.system_type ? (SYSTEM_ICONS[f.system_type] ?? <FolderIcon style={{ width: 16, height: 16 }} />) : <FolderIcon style={{ width: 16, height: 16 }} />;
      const label = f.system_type === 'inbox' ? '받은 편지함' : f.system_type === 'sent' ? '보낸 편지함' : f.system_type === 'drafts' ? '임시 보관함' : f.system_type === 'trash' ? '휴지통' : f.system_type === 'spam' ? '스팸 편지함' : f.name;
      systemFolderItems.push({ type: 'folder', id: f.id, title: label, subtitle: f.unread ? `읽지 않음 ${f.unread}` : undefined, icon, onSelect: () => { onSelectFolder(f.id); onClose(); } });
    }
    return [
      { type: 'action', id: 'compose', title: '새 메일 작성', subtitle: 'C', icon: <PencilSquareIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onCompose(); onClose(); } },
      { type: 'action', id: 'starred', title: '별표 메일', icon: <StarIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__starred__'); onClose(); } },
      { type: 'action', id: 'unread', title: '읽지 않은 메일', icon: <EnvelopeIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__unread__'); onClose(); } },
      { type: 'action', id: 'attach', title: '첨부파일 메일', icon: <PaperClipIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onSelectFolder('__attachments__'); onClose(); } },
      { type: 'action', id: 'settings', title: '설정 열기', subtitle: ',', icon: <Cog6ToothIcon style={{ width: 16, height: 16 }} />, onSelect: () => { onOpenSettings(); onClose(); } },
      ...systemFolderItems,
    ];
  }, [folders, onSelectFolder, onCompose, onOpenSettings, onClose]);

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
          onSelect: () => { onCompose(); onClose(); /* will prefill via onSearch */ },
        }));
    } catch { return []; }
  }, [onCompose, onClose]);

  const recentSearchKey = 'webmail_recent_searches';
  const recentSearches: string[] = (() => {
    try { return JSON.parse(localStorage.getItem(recentSearchKey) ?? '[]').slice(0, 4) as string[]; } catch { return []; }
  })();

  // Update items based on query
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const q = query.trim();

    if (!q) {
      const quickActions = buildQuickActions();
      const contacts = buildContactItems('').slice(0, 3);
      setItems([...quickActions, ...contacts]);
      setSelIdx(0);
      return;
    }

    // Immediate: filter actions + contacts
    const ql = q.toLowerCase();
    const actions = buildQuickActions().filter((a) =>
      a.title.toLowerCase().includes(ql) || (a.subtitle ?? '').toLowerCase().includes(ql)
    );
    const contacts = buildContactItems(ql);
    setItems([...actions, ...contacts]);
    setSelIdx(0);

    // Debounced: search mail
    setSearching(true);
    debounceRef.current = setTimeout(async () => {
      try {
        const res = await searchMessages({ q, limit: 8 });
        const mailItems: SpotlightItem[] = (res.messages ?? []).map((m: MessageSummary) => ({
          type: 'mail' as const,
          id: m.id,
          title: m.subject || '(제목 없음)',
          subtitle: m.from_name || m.from_addr,
          badge: relativeTime(m.received_at),
          icon: <EnvelopeIcon style={{ width: 16, height: 16, opacity: m.read ? 0.5 : 1 }} />,
          onSelect: () => { onSelectMessage(m.id); onClose(); },
        }));
        // "전체 검색" action at the end
        const searchAll: SpotlightItem = {
          type: 'action',
          id: '__search_all__',
          title: `"${q}" 전체 검색`,
          icon: <MagnifyingGlassIcon style={{ width: 16, height: 16 }} />,
          onSelect: () => { onSearch(q); onClose(); },
        };
        setItems([...actions, ...contacts, ...mailItems, searchAll]);
        setSelIdx(0);
      } catch { /* */ }
      setSearching(false);
    }, 200);

    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [query, buildQuickActions, buildContactItems, onSelectMessage, onSearch, onClose]);

  // Keyboard navigation
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setSelIdx((i) => Math.min(i + 1, items.length - 1)); }
      if (e.key === 'ArrowUp') { e.preventDefault(); setSelIdx((i) => Math.max(i - 1, 0)); }
      if (e.key === 'Enter') { e.preventDefault(); items[selIdx]?.onSelect(); }
      if (e.key === 'Escape') { e.preventDefault(); onClose(); }
      if (e.key === 'Tab') { e.preventDefault(); setSelIdx((i) => (e.shiftKey ? Math.max(i - 1, 0) : Math.min(i + 1, items.length - 1))); }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [items, selIdx, onClose]);

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
  for (const item of items) {
    if (!seen.has(item.type)) {
      seen.add(item.type);
      grouped.push({ label: sectionLabel(item.type), items: [] });
    }
    grouped[grouped.length - 1].items.push({ ...item, idx: globalIdx++ });
  }

  return (
    <div
      aria-modal="true"
      role="dialog"
      aria-label="통합 검색"
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
        {/* Search input */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '14px 18px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          {searching
            ? <ArrowRightIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0, animation: 'spin 600ms linear infinite' }} />
            : <MagnifyingGlassIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
          }
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="메일, 연락처, 폴더, 명령 검색..."
            aria-label="통합 검색 입력"
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

        {/* Recent searches (shown only when empty + no query) */}
        {!query && recentSearches.length > 0 && (
          <div style={{ padding: '8px 12px 0' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '4px 6px', letterSpacing: '0.05em', textTransform: 'uppercase' }}>최근 검색</div>
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
          {items.length === 0 && query && !searching && (
            <div style={{ padding: '32px', textAlign: 'center', fontSize: '14px', color: 'var(--color-text-tertiary)' }}>
              결과가 없습니다
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
          <span><kbd style={kbdStyle}>↑↓</kbd> 이동</span>
          <span><kbd style={kbdStyle}>↵</kbd> 선택</span>
          <span><kbd style={kbdStyle}>Esc</kbd> 닫기</span>
          <span style={{ marginLeft: 'auto' }}>GoGoMail 통합 검색</span>
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
