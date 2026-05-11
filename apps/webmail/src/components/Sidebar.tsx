'use client';

import { useState, useEffect, useRef } from 'react';
import { Folder } from '@/lib/api';

const RECENT_SEARCHES_KEY = 'webmail_recent_searches';
const MAX_RECENT = 5;

function loadRecentSearches(): string[] {
  try {
    return JSON.parse(localStorage.getItem(RECENT_SEARCHES_KEY) ?? '[]') as string[];
  } catch {
    return [];
  }
}

function saveRecentSearch(query: string): string[] {
  const trimmed = query.trim();
  if (!trimmed) return loadRecentSearches();
  const prev = loadRecentSearches().filter((q) => q !== trimmed);
  const next = [trimmed, ...prev].slice(0, MAX_RECENT);
  localStorage.setItem(RECENT_SEARCHES_KEY, JSON.stringify(next));
  return next;
}

const SYSTEM_FOLDER_META: { systemType: string; label: string }[] = [
  { systemType: 'inbox', label: '수신함' },
  { systemType: 'sent', label: '보낸 편지함' },
  { systemType: 'drafts', label: '임시 보관함' },
  { systemType: 'trash', label: '휴지통' },
];

function formatBadge(count: number): string {
  if (count <= 0) return '';
  if (count > 99) return '99+';
  return String(count);
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

export interface AdvancedFilters {
  from?: string;
  since?: string;
  until?: string;
  has_attachment?: boolean;
}

interface SidebarProps {
  folders: Folder[];
  activeFolderId: string;
  onSelectFolder: (id: string) => void;
  onCompose: () => void;
  onSearch?: (q: string) => void;
  searchQuery?: string;
  advancedFilters?: AdvancedFilters;
  onAdvancedFilterChange?: (filters: AdvancedFilters) => void;
  userName?: string;
  onLogout?: () => void;
  isMobile?: boolean;
  isOpen?: boolean;
  onClose?: () => void;
}

export function Sidebar({
  folders,
  activeFolderId,
  onSelectFolder,
  onCompose,
  onSearch,
  searchQuery = '',
  advancedFilters = {},
  onAdvancedFilterChange,
  userName = '사용자',
  onLogout,
  isMobile,
  isOpen,
  onClose,
}: SidebarProps) {
  const showAdvanced = searchQuery.trim().length > 0;
  const [recentSearches, setRecentSearches] = useState<string[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const hideTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setRecentSearches(loadRecentSearches());
  }, []);
  const systemFoldersByType = new Map(folders.map((f) => [f.system_type ?? '', f]));
  const systemFolderIds = new Set(folders.filter((f) => f.system_type).map((f) => f.id));

  const asideStyle: React.CSSProperties = isMobile
    ? {
        position: 'fixed',
        top: 0,
        left: 0,
        height: '100%',
        width: '260px',
        zIndex: 300,
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
        overflowY: 'auto',
        overflowX: 'hidden',
        transform: isOpen ? 'translateX(0)' : 'translateX(-100%)',
        transition: 'transform 200ms ease',
      }
    : {
        width: '220px',
        minWidth: '220px',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
        overflowY: 'auto',
        overflowX: 'hidden',
      };

  return (
    <>
      {isMobile && isOpen && (
        <div
          aria-hidden="true"
          onClick={onClose}
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.4)',
            zIndex: 299,
          }}
        />
      )}
    <aside
      aria-label="메일 탐색"
      style={asideStyle}
    >
      {/* Account header */}
      <div
        style={{
          padding: '16px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          borderBottom: '1px solid var(--color-border-subtle)',
        }}
      >
        {isMobile && onClose && (
          <button
            aria-label="메뉴 닫기"
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-secondary)', fontSize: '18px', padding: '0 4px 0 0', lineHeight: 1 }}
          >×</button>
        )}
        <div
          aria-hidden="true"
          style={{
            width: '32px',
            height: '32px',
            borderRadius: '50%',
            background: 'var(--color-accent)',
            color: '#fff',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            fontWeight: 600,
            flexShrink: 0,
          }}
        >
          {getInitials(userName)}
        </div>
        <span
          style={{
            fontSize: '14px',
            fontWeight: 500,
            color: 'var(--color-text-primary)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {userName}
        </span>
      </div>

      {/* Search */}
      <div style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
        <div style={{ padding: '12px 16px 8px', position: 'relative' }}>
          <input
            type="search"
            placeholder="검색..."
            aria-label="메일 검색"
            value={searchQuery}
            onChange={(e) => {
              onSearch?.(e.target.value);
              if (e.target.value.trim()) setShowSuggestions(false);
              else setShowSuggestions(true);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && searchQuery.trim()) {
                const next = saveRecentSearch(searchQuery);
                setRecentSearches(next);
                setShowSuggestions(false);
              }
            }}
            onFocus={(e) => {
              e.target.style.borderColor = 'var(--color-accent)';
              if (hideTimeout.current) clearTimeout(hideTimeout.current);
              if (!searchQuery.trim()) setShowSuggestions(true);
            }}
            onBlur={(e) => {
              e.target.style.borderColor = 'var(--color-border-default)';
              hideTimeout.current = setTimeout(() => setShowSuggestions(false), 150);
            }}
            style={{
              width: '100%',
              padding: '7px 10px',
              borderRadius: '6px',
              border: '1px solid var(--color-border-default)',
              background: 'var(--color-bg-primary)',
              color: 'var(--color-text-primary)',
              fontSize: '13px',
              outline: 'none',
              boxSizing: 'border-box',
            }}
          />
          {showSuggestions && !searchQuery.trim() && recentSearches.length > 0 && (
            <div
              role="listbox"
              aria-label="최근 검색"
              style={{
                position: 'absolute',
                top: '100%',
                left: '16px',
                right: '16px',
                background: 'var(--color-bg-primary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '6px',
                boxShadow: '0 4px 12px rgba(0,0,0,0.12)',
                zIndex: 350,
                overflow: 'hidden',
                marginTop: '2px',
              }}
            >
              <div style={{ padding: '6px 10px 4px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, letterSpacing: '0.05em', textTransform: 'uppercase' }}>
                최근 검색
              </div>
              {recentSearches.map((q) => (
                <button
                  key={q}
                  role="option"
                  aria-selected={false}
                  onMouseDown={() => {
                    onSearch?.(q);
                    const next = saveRecentSearch(q);
                    setRecentSearches(next);
                    setShowSuggestions(false);
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    width: '100%',
                    textAlign: 'left',
                    padding: '7px 10px',
                    border: 'none',
                    background: 'transparent',
                    color: 'var(--color-text-primary)',
                    fontSize: '13px',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  <span style={{ color: 'var(--color-text-tertiary)', fontSize: '12px' }}>↺</span>
                  {q}
                </button>
              ))}
            </div>
          )}
        </div>

        {showAdvanced && onAdvancedFilterChange && (
          <div style={{ padding: '0 16px 10px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: '2px' }}>필터</div>
            {/* From */}
            <input
              type="text"
              placeholder="보낸 사람"
              aria-label="보낸 사람 필터"
              value={advancedFilters.from ?? ''}
              onChange={(e) => onAdvancedFilterChange({ ...advancedFilters, from: e.target.value || undefined })}
              style={{ padding: '5px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '12px', outline: 'none', width: '100%', boxSizing: 'border-box' }}
            />
            {/* Date range */}
            <div style={{ display: 'flex', gap: '4px' }}>
              <input
                type="date"
                aria-label="시작 날짜"
                value={advancedFilters.since ?? ''}
                onChange={(e) => onAdvancedFilterChange({ ...advancedFilters, since: e.target.value || undefined })}
                style={{ flex: 1, padding: '5px 4px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '11px', outline: 'none', minWidth: 0 }}
              />
              <input
                type="date"
                aria-label="종료 날짜"
                value={advancedFilters.until ?? ''}
                onChange={(e) => onAdvancedFilterChange({ ...advancedFilters, until: e.target.value || undefined })}
                style={{ flex: 1, padding: '5px 4px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '11px', outline: 'none', minWidth: 0 }}
              />
            </div>
            {/* Has attachment */}
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={advancedFilters.has_attachment ?? false}
                onChange={(e) => onAdvancedFilterChange({ ...advancedFilters, has_attachment: e.target.checked || undefined })}
              />
              첨부파일 있음
            </label>
          </div>
        )}
      </div>

      {/* Nav */}
      <nav style={{ flex: 1, padding: '8px 0' }}>
        <div
          style={{
            padding: '12px 16px 4px',
            fontSize: '11px',
            fontWeight: 600,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            color: 'var(--color-text-tertiary)',
          }}
        >
          메일함
        </div>

        {SYSTEM_FOLDER_META.map((sf) => {
          const serverFolder = systemFoldersByType.get(sf.systemType);
          const unread = serverFolder?.unread ?? 0;
          const folderId = serverFolder?.id ?? sf.systemType;
          const isActive = activeFolderId === folderId;
          const badge = formatBadge(unread);

          return (
            <button
              key={sf.systemType}
              onClick={() => onSelectFolder(folderId)}
              aria-current={isActive ? 'page' : undefined}
              style={{
                width: 'calc(100% - 8px)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '7px 16px',
                border: 'none',
                background: isActive ? 'var(--color-bg-tertiary)' : 'transparent',
                color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                fontSize: '14px',
                fontWeight: isActive ? 500 : 400,
                cursor: 'pointer',
                transition: 'background 100ms ease',
                borderRadius: '4px',
                marginInline: '4px',
              } as React.CSSProperties}
              onMouseEnter={(e) => {
                if (!isActive) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-overlay)';
                }
              }}
              onMouseLeave={(e) => {
                if (!isActive) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }
              }}
            >
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {sf.label}
              </span>
              {badge && (
                <span
                  aria-label={`읽지 않은 메일 ${badge}개`}
                  style={{
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--color-text-secondary)',
                    background: 'var(--color-bg-tertiary)',
                    borderRadius: '10px',
                    padding: '1px 6px',
                    flexShrink: 0,
                    marginInlineStart: '8px',
                  }}
                >
                  {badge}
                </span>
              )}
            </button>
          );
        })}

        {/* Extra server folders not in system list */}
        {folders
          .filter((f) => !systemFolderIds.has(f.id))
          .map((f) => {
            const isActive = activeFolderId === f.id;
            const badge = formatBadge(f.unread);
            return (
              <button
                key={f.id}
                onClick={() => onSelectFolder(f.id)}
                aria-current={isActive ? 'page' : undefined}
                style={{
                  width: 'calc(100% - 8px)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '7px 16px',
                  border: 'none',
                  background: isActive ? 'var(--color-bg-tertiary)' : 'transparent',
                  color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  fontSize: '14px',
                  fontWeight: isActive ? 500 : 400,
                  cursor: 'pointer',
                  borderRadius: '4px',
                  marginInline: '4px',
                } as React.CSSProperties}
              >
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {f.name}
                </span>
                {badge && (
                  <span
                    style={{
                      fontSize: '12px',
                      color: 'var(--color-text-secondary)',
                      background: 'var(--color-bg-tertiary)',
                      borderRadius: '10px',
                      padding: '1px 6px',
                      flexShrink: 0,
                      marginInlineStart: '8px',
                    }}
                  >
                    {badge}
                  </span>
                )}
              </button>
            );
          })}
      </nav>

      {/* Compose button + logout */}
      <div style={{ padding: '12px 16px', borderTop: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {onLogout && (
          <button
            onClick={onLogout}
            style={{
              width: '100%',
              padding: '7px 16px',
              borderRadius: '6px',
              border: '1px solid var(--color-border-default)',
              background: 'transparent',
              color: 'var(--color-text-secondary)',
              fontSize: '13px',
              cursor: 'pointer',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            로그아웃
          </button>
        )}
        <button
          onClick={onCompose}
          style={{
            width: '100%',
            padding: '9px 16px',
            borderRadius: '6px',
            border: 'none',
            background: 'var(--color-accent)',
            color: '#fff',
            fontSize: '14px',
            fontWeight: 500,
            cursor: 'pointer',
            transition: 'background 100ms ease',
          }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent-hover)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-accent)';
          }}
        >
          편지 쓰기
        </button>
      </div>
    </aside>
    </>
  );
}
