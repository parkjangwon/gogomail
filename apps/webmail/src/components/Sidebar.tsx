'use client';

import { Folder } from '@/lib/api';

const SYSTEM_FOLDERS = [
  { id: 'inbox', label: '수신함' },
  { id: 'sent', label: '보낸 편지함' },
  { id: 'drafts', label: '임시 보관함' },
  { id: 'trash', label: '휴지통' },
  { id: 'all', label: '전체 메일' },
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

interface SidebarProps {
  folders: Folder[];
  activeFolderId: string;
  onSelectFolder: (id: string) => void;
  onCompose: () => void;
  userName?: string;
}

export function Sidebar({
  folders,
  activeFolderId,
  onSelectFolder,
  onCompose,
  userName = '사용자',
}: SidebarProps) {
  // Merge server folders with system folder labels
  const folderMap = new Map(folders.map((f) => [f.id, f]));

  return (
    <aside
      aria-label="메일 탐색"
      style={{
        width: '220px',
        minWidth: '220px',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
        overflowY: 'auto',
        overflowX: 'hidden',
      }}
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
      <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-border-subtle)' }}>
        <input
          type="search"
          placeholder="검색..."
          aria-label="메일 검색"
          style={{
            width: '100%',
            padding: '7px 10px',
            borderRadius: '6px',
            border: '1px solid var(--color-border-default)',
            background: 'var(--color-bg-primary)',
            color: 'var(--color-text-primary)',
            fontSize: '13px',
            outline: 'none',
          }}
          onFocus={(e) => {
            e.target.style.borderColor = 'var(--color-accent)';
          }}
          onBlur={(e) => {
            e.target.style.borderColor = 'var(--color-border-default)';
          }}
        />
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

        {SYSTEM_FOLDERS.map((sf) => {
          const serverFolder = folderMap.get(sf.id);
          const unread = serverFolder?.unread_count ?? 0;
          const isActive = activeFolderId === sf.id;
          const badge = formatBadge(unread);

          return (
            <button
              key={sf.id}
              onClick={() => onSelectFolder(sf.id)}
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
          .filter((f) => !SYSTEM_FOLDERS.find((sf) => sf.id === f.id))
          .map((f) => {
            const isActive = activeFolderId === f.id;
            const badge = formatBadge(f.unread_count);
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

      {/* Compose button */}
      <div style={{ padding: '12px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
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
  );
}
