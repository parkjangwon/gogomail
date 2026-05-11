'use client';

import { useState, useEffect, useRef, ReactNode } from 'react';
import { Folder } from '@/lib/api';
import {
  PencilSquareIcon,
  ChevronRightIcon,
  ChevronDownIcon,
  ChevronDoubleLeftIcon,
  InboxIcon,
  StarIcon,
  PaperClipIcon,
  PaperAirplaneIcon,
  ArchiveBoxIcon,
  TrashIcon,
  NoSymbolIcon,
  PencilIcon,
  ArrowTopRightOnSquareIcon,
  XMarkIcon,
  CheckIcon,
  Cog6ToothIcon,
} from '@heroicons/react/24/outline';
import { SettingsModal } from '@/components/SettingsModal';


export const VIRTUAL_ALL = '__all__';
export const VIRTUAL_STARRED = '__starred__';
export const VIRTUAL_ATTACHMENTS = '__attachments__';

const VIRTUAL_NAV: { id: string; label: string; icon: ReactNode }[] = [
  { id: VIRTUAL_ALL, label: '모든 편지함', icon: <InboxIcon style={{ width: '16px', height: '16px', flexShrink: 0 }} /> },
  { id: VIRTUAL_STARRED, label: '중요 편지함', icon: <StarIcon style={{ width: '16px', height: '16px', flexShrink: 0 }} /> },
  { id: VIRTUAL_ATTACHMENTS, label: '첨부 편지함', icon: <PaperClipIcon style={{ width: '16px', height: '16px', flexShrink: 0 }} /> },
];

const SYSTEM_FOLDER_META: { systemType: string; label: string }[] = [
  { systemType: 'inbox', label: '받은 편지함' },
  { systemType: 'sent', label: '보낸 편지함' },
  { systemType: 'drafts', label: '임시 보관함' },
  { systemType: 'spam', label: '스팸 편지함' },
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
  subject?: string;
  since?: string;
  until?: string;
  has_attachment?: boolean;
}

const SYSTEM_FOLDER_ICONS: Record<string, ReactNode> = {
  inbox: <InboxIcon style={{ width: '16px', height: '16px' }} />,
  sent: <PaperAirplaneIcon style={{ width: '16px', height: '16px' }} />,
  drafts: <PencilSquareIcon style={{ width: '16px', height: '16px' }} />,
  trash: <TrashIcon style={{ width: '16px', height: '16px' }} />,
  spam: <NoSymbolIcon style={{ width: '16px', height: '16px' }} />,
  junk: <NoSymbolIcon style={{ width: '16px', height: '16px' }} />,
  archive: <ArchiveBoxIcon style={{ width: '16px', height: '16px' }} />,
};

interface SidebarProps {
  folders: Folder[];
  activeFolderId: string;
  onSelectFolder: (id: string) => void;
  onCompose: () => void;
  onComposeInNewWindow?: () => void;
  userName?: string;
  userEmailAddress?: string;
  onLogout?: () => void;
  isMobile?: boolean;
  isOpen?: boolean;
  onClose?: () => void;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
  onDropMessage?: (messageId: string, folderId: string) => void;
  onCreateFolder?: (name: string) => void;
  onRenameFolder?: (id: string, name: string) => void;
  onDeleteFolder?: (id: string) => void;
  footerExtra?: React.ReactNode;
  menuExtra?: React.ReactNode;
  width?: number;
}

export function Sidebar({
  folders,
  activeFolderId,
  onSelectFolder,
  onCompose,
  userName = '사용자',
  userEmailAddress,
  onLogout,
  isMobile,
  isOpen,
  onClose,
  collapsed = false,
  onToggleCollapse,
  onDropMessage,
  onCreateFolder,
  onRenameFolder,
  onDeleteFolder,
  onComposeInNewWindow,
  footerExtra,
  menuExtra,
  width,
}: SidebarProps) {
  const [dragOverFolderId, setDragOverFolderId] = useState<string | null>(null);
  const [newFolderInput, setNewFolderInput] = useState('');
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [renamingFolderId, setRenamingFolderId] = useState<string | null>(null);
  const [renamingValue, setRenamingValue] = useState('');
  const [hoveredFolderId, setHoveredFolderId] = useState<string | null>(null);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [headerHovered, setHeaderHovered] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showUserMenu) return;
    function onClick(e: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) {
        setShowUserMenu(false);
      }
    }
    document.addEventListener('mousedown', onClick);
    return () => document.removeEventListener('mousedown', onClick);
  }, [showUserMenu]);

  const systemFoldersByType = new Map(folders.map((f) => {
    const key = f.system_type === 'junk' ? 'spam' : (f.system_type ?? '');
    return [key, f];
  }));
  // Exclude all system-typed folders (including junk/archive not shown in nav) from custom section
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
        width: collapsed ? '48px' : `${width ?? 220}px`,
        minWidth: collapsed ? '48px' : `${width ?? 220}px`,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
        overflowY: 'auto',
        overflowX: 'hidden',
        transition: 'width 200ms ease, min-width 200ms ease',
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
      {collapsed && !isMobile ? (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', height: '100%', padding: '8px 0', gap: '2px' }}>
          {/* Expand button */}
          {onToggleCollapse && (
            <button
              aria-label="사이드바 확장"
              onClick={onToggleCollapse}
              title="사이드바 확장"
              style={{ width: '36px', height: '36px', borderRadius: '6px', border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '14px', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: '4px' }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            ><ChevronRightIcon style={{ width: '16px', height: '16px' }} /></button>
          )}
          {/* Compose */}
          <button
            aria-label="편지 쓰기"
            onClick={onCompose}
            title="편지 쓰기"
            style={{ width: '36px', height: '36px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', cursor: 'pointer', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
          ><PencilSquareIcon style={{ width: '16px', height: '16px' }} /></button>
          {onComposeInNewWindow && (
            <button
              aria-label="새창으로 쓰기"
              onClick={onComposeInNewWindow}
              title="새창으로 쓰기"
              style={{ width: '36px', height: '36px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: '8px' }}
            ><ArrowTopRightOnSquareIcon style={{ width: '16px', height: '16px' }} /></button>
          )}
          {/* System folders */}
          {SYSTEM_FOLDER_META.map((sf) => {
            const serverFolder = systemFoldersByType.get(sf.systemType);
            const unread = serverFolder?.unread ?? 0;
            const folderId = serverFolder?.id ?? sf.systemType;
            const isActive = activeFolderId === folderId;
            const icon = SYSTEM_FOLDER_ICONS[sf.systemType] ?? <InboxIcon style={{ width: '16px', height: '16px' }} />;
            return (
              <button
                key={sf.systemType}
                onClick={() => onSelectFolder(folderId)}
                title={sf.label}
                aria-label={`${sf.label}${unread > 0 ? ` (읽지 않음 ${unread})` : ''}`}
                style={{ position: 'relative', width: '36px', height: '36px', borderRadius: '6px', border: dragOverFolderId === folderId ? '2px solid var(--color-accent)' : 'none', background: isActive ? 'var(--color-bg-tertiary)' : 'transparent', cursor: 'pointer', fontSize: '18px', display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'border 80ms ease' }}
                onMouseEnter={(e) => { if (!isActive) (e.currentTarget).style.background = 'var(--color-bg-overlay)'; }}
                onMouseLeave={(e) => { if (!isActive) (e.currentTarget).style.background = 'transparent'; }}
                onDragOver={(e) => { if (onDropMessage) { e.preventDefault(); setDragOverFolderId(folderId); } }}
                onDragLeave={() => setDragOverFolderId(null)}
                onDrop={(e) => { e.preventDefault(); setDragOverFolderId(null); const id = e.dataTransfer.getData('text/plain'); if (id && onDropMessage) onDropMessage(id, folderId); }}
              >
                {icon}
                {unread > 0 && (
                  <span style={{ position: 'absolute', top: '2px', right: '2px', width: '14px', height: '14px', borderRadius: '50%', background: 'var(--color-accent)', color: '#fff', fontSize: '9px', fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center', lineHeight: 1 }}>
                    {unread > 9 ? '9+' : unread}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      ) : (
      <>
      {/* Account header — Notion Mail style */}
      <div ref={userMenuRef} style={{ position: 'relative' }}>
        <div style={{ padding: '10px 10px 8px', display: 'flex', alignItems: 'center', gap: '6px' }} onMouseEnter={() => setHeaderHovered(true)} onMouseLeave={() => setHeaderHovered(false)}>
          {isMobile && onClose && (
            <button aria-label="메뉴 닫기" onClick={onClose}
              style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-secondary)', padding: '0 2px', lineHeight: 1, flexShrink: 0, display: 'inline-flex' }}>
              <XMarkIcon style={{ width: '18px', height: '18px' }} />
            </button>
          )}
          {/* Avatar + name/email — click to open account menu */}
          <button
            aria-label="계정 메뉴"
            aria-expanded={showUserMenu}
            onClick={() => setShowUserMenu((v) => !v)}
            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '3px 4px', borderRadius: '6px', display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: 0, textAlign: 'left' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-overlay)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            <div aria-hidden="true" style={{
              width: '28px', height: '28px', borderRadius: '6px',
              background: 'var(--color-accent)', color: '#fff',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: '12px', fontWeight: 700, flexShrink: 0,
            }}>
              {getInitials(userName)}
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '3px' }}>
                <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {userName !== userEmailAddress ? userName : userName.split('@')[0]}
                </span>
                <ChevronDownIcon style={{ width: '12px', height: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0, opacity: headerHovered ? 1 : 0, transition: 'opacity 150ms' }} />
              </div>
              {userEmailAddress && (
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {userEmailAddress}
                </div>
              )}
            </div>
          </button>
          {/* Collapse button */}
          {!isMobile && onToggleCollapse && (
            <button
              aria-label="사이드바 접기"
              onClick={onToggleCollapse}
              title="사이드바 접기 ([)"
              style={{ width: '28px', height: '28px', borderRadius: '6px', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, opacity: headerHovered ? 1 : 0, pointerEvents: (headerHovered ? 'auto' : 'none') as React.CSSProperties['pointerEvents'], transition: 'opacity 150ms' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
            ><ChevronDoubleLeftIcon style={{ width: '15px', height: '15px' }} /></button>
          )}
          {/* Compose button */}
          <button
            aria-label="편지 쓰기"
            onClick={onCompose}
            title="편지 쓰기 (c)"
            style={{ width: '28px', height: '28px', borderRadius: '6px', border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
          ><PencilSquareIcon style={{ width: '15px', height: '15px' }} /></button>
        </div>

        {/* User menu dropdown */}
        {showUserMenu && (
          <div
            role="menu"
            style={{
              position: 'absolute', top: '100%', left: '8px', right: '8px',
              background: 'var(--color-bg-primary)',
              border: '1px solid var(--color-border-default)',
              borderRadius: '10px',
              boxShadow: '0 8px 24px rgba(0,0,0,0.12)',
              zIndex: 400,
              overflow: 'hidden',
            }}
          >
            {/* Profile section */}
            <div style={{ padding: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--color-border-subtle)' }}>
              <div style={{
                width: '48px', height: '48px', borderRadius: '50%',
                background: 'var(--color-accent)', color: '#fff',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '18px', fontWeight: 700,
              }}>
                {getInitials(userName)}
              </div>
              <div style={{ textAlign: 'center', minWidth: 0, width: '100%' }}>
                <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--color-text-primary)' }}>
                  {userName !== userEmailAddress ? userName : userName.split('@')[0]}
                </div>
                {userEmailAddress && (
                  <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {userEmailAddress}
                  </div>
                )}
              </div>
            </div>
            {/* Session info */}
            <div style={{ padding: '10px 14px', borderBottom: '1px solid var(--color-border-subtle)' }}>
              {(() => {
                let loginAtStr = '';
                let expiresStr = '';
                try {
                  const lat = localStorage.getItem('webmail_login_at');
                  if (lat) {
                    loginAtStr = new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(lat));
                  }
                  const exp = localStorage.getItem('webmail_token_expires_at');
                  if (exp) {
                    expiresStr = new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(exp));
                  }
                } catch { /* */ }
                return (
                  <>
                    {loginAtStr && (
                      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: 'var(--color-text-secondary)', marginBottom: '4px' }}>
                        <span style={{ color: 'var(--color-text-tertiary)' }}>최근 로그인</span>
                        <span>{loginAtStr}</span>
                      </div>
                    )}
                    {expiresStr && (
                      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: 'var(--color-text-secondary)' }}>
                        <span style={{ color: 'var(--color-text-tertiary)' }}>세션 만료</span>
                        <span>{expiresStr}</span>
                      </div>
                    )}
                  </>
                );
              })()}
            </div>
            {/* Extra settings (theme, locale, etc.) */}
            {menuExtra && (
              <div style={{ padding: '8px 14px', borderBottom: '1px solid var(--color-border-subtle)' }}>
                {menuExtra}
              </div>
            )}
            {/* Settings */}
            <button
              role="menuitem"
              onClick={() => { setShowUserMenu(false); setShowSettings(true); }}
              style={{
                width: '100%', padding: '10px 14px', border: 'none',
                background: 'transparent', color: 'var(--color-text-primary)',
                fontSize: '13px', fontWeight: 500, cursor: 'pointer', textAlign: 'left',
                display: 'flex', alignItems: 'center', gap: '8px',
                borderTop: '1px solid var(--color-border-subtle)',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >
              <Cog6ToothIcon style={{ width: '15px', height: '15px', color: 'var(--color-text-secondary)' }} />
              설정
            </button>
            {/* Logout */}
            {onLogout && (
              <button
                role="menuitem"
                onClick={() => { setShowUserMenu(false); onLogout(); }}
                style={{
                  width: '100%', padding: '10px 14px', border: 'none',
                  background: 'transparent', color: 'var(--color-destructive)',
                  fontSize: '13px', fontWeight: 500, cursor: 'pointer', textAlign: 'left',
                }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >
                로그아웃
              </button>
            )}
          </div>
        )}
      </div>

      {/* Nav */}
      <nav style={{ flex: 1, padding: '8px 0 8px' }}>

        {/* Virtual folders */}
        {VIRTUAL_NAV.map((vf) => {
          const isActive = activeFolderId === vf.id;
          return (
            <button
              key={vf.id}
              onClick={() => onSelectFolder(vf.id)}
              aria-current={isActive ? 'page' : undefined}
              style={{ width: 'calc(100% - 8px)', display: 'flex', alignItems: 'center', gap: '8px', padding: '7px 16px', border: '1px solid transparent', background: isActive ? 'var(--color-bg-tertiary)' : 'transparent', color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)', fontSize: '14px', fontWeight: isActive ? 500 : 400, cursor: 'pointer', borderRadius: '4px', marginInline: '4px' } as React.CSSProperties}
              onMouseEnter={(e) => { if (!isActive) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-overlay)'; }}
              onMouseLeave={(e) => { if (!isActive) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
            >
              <span style={{ fontSize: '14px', lineHeight: 1 }}>{vf.icon}</span>
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{vf.label}</span>
            </button>
          );
        })}

        {/* Divider */}
        <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '6px 16px' }} />

        {SYSTEM_FOLDER_META.map((sf) => {
          const serverFolder = systemFoldersByType.get(sf.systemType);
          if (!serverFolder) return null;
          const unread = serverFolder.unread ?? 0;
          const folderId = serverFolder.id;
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
                border: dragOverFolderId === folderId ? '1px solid var(--color-accent)' : '1px solid transparent',
                background: dragOverFolderId === folderId ? 'var(--color-accent-subtle)' : isActive ? 'var(--color-bg-tertiary)' : 'transparent',
                color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                fontSize: '14px',
                fontWeight: isActive ? 500 : 400,
                cursor: 'pointer',
                transition: 'background 100ms ease, border 80ms ease',
                borderRadius: '4px',
                marginInline: '4px',
              } as React.CSSProperties}
              onMouseEnter={(e) => {
                if (!isActive) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-overlay)';
                }
              }}
              onMouseLeave={(e) => {
                if (!isActive && dragOverFolderId !== folderId) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }
              }}
              onDragOver={(e) => { if (onDropMessage) { e.preventDefault(); setDragOverFolderId(folderId); } }}
              onDragLeave={() => setDragOverFolderId(null)}
              onDrop={(e) => { e.preventDefault(); setDragOverFolderId(null); const id = e.dataTransfer.getData('text/plain'); if (id && onDropMessage) onDropMessage(id, folderId); }}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: 0, overflow: 'hidden' }}>
                <span style={{ flexShrink: 0, display: 'inline-flex', opacity: 0.6 }}>{SYSTEM_FOLDER_ICONS[sf.systemType]}</span>
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{sf.label}</span>
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

        {/* 개인 메일함 section */}
        {folders.filter((f) => !systemFolderIds.has(f.id)).length > 0 || onCreateFolder ? (
          <div style={{ padding: '12px 16px 4px', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span>개인 메일함</span>
          </div>
        ) : null}

        {/* Extra server folders not in system list */}
        {folders
          .filter((f) => !systemFolderIds.has(f.id))
          .map((f) => {
            const isActive = activeFolderId === f.id;
            const badge = formatBadge(f.unread);
            const isRenaming = renamingFolderId === f.id;
            const isHovered = hoveredFolderId === f.id;
            return (
              <div
                key={f.id}
                style={{ position: 'relative', marginInline: '4px' }}
                onMouseEnter={() => setHoveredFolderId(f.id)}
                onMouseLeave={() => setHoveredFolderId(null)}
              >
                {isRenaming ? (
                  <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '4px 8px' }}>
                    <input
                      autoFocus
                      value={renamingValue}
                      onChange={(e) => setRenamingValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && renamingValue.trim()) { onRenameFolder?.(f.id, renamingValue.trim()); setRenamingFolderId(null); }
                        if (e.key === 'Escape') setRenamingFolderId(null);
                      }}
                      style={{ flex: 1, fontSize: '13px', padding: '3px 6px', border: '1px solid var(--color-accent)', borderRadius: '4px', outline: 'none', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }}
                    />
                    <button onClick={() => { if (renamingValue.trim()) { onRenameFolder?.(f.id, renamingValue.trim()); setRenamingFolderId(null); } }} style={{ padding: '3px 6px', border: 'none', background: 'var(--color-accent)', color: '#fff', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}><CheckIcon style={{ width: '12px', height: '12px' }} /></button>
                    <button onClick={() => setRenamingFolderId(null)} style={{ padding: '3px 6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}><XMarkIcon style={{ width: '12px', height: '12px' }} /></button>
                  </div>
                ) : (
                  <button
                    onClick={() => onSelectFolder(f.id)}
                    aria-current={isActive ? 'page' : undefined}
                    style={{
                      width: '100%',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '7px 16px',
                      border: dragOverFolderId === f.id ? '1px solid var(--color-accent)' : '1px solid transparent',
                      background: dragOverFolderId === f.id ? 'var(--color-accent-subtle)' : isActive ? 'var(--color-bg-tertiary)' : 'transparent',
                      color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                      fontSize: '14px',
                      fontWeight: isActive ? 500 : 400,
                      cursor: 'pointer',
                      borderRadius: '4px',
                      transition: 'background 80ms ease, border 80ms ease',
                    } as React.CSSProperties}
                    onDragOver={(e) => { if (onDropMessage) { e.preventDefault(); setDragOverFolderId(f.id); } }}
                    onDragLeave={() => setDragOverFolderId(null)}
                    onDrop={(e) => { e.preventDefault(); setDragOverFolderId(null); const id = e.dataTransfer.getData('text/plain'); if (id && onDropMessage) onDropMessage(id, f.id); }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>{f.name}</span>
                    {isHovered && (onRenameFolder || onDeleteFolder) ? (
                      <span style={{ display: 'flex', gap: '2px', flexShrink: 0, marginInlineStart: '4px' }}>
                        {onRenameFolder && <span onClick={(e) => { e.stopPropagation(); setRenamingValue(f.name); setRenamingFolderId(f.id); }} style={{ padding: '2px 4px', borderRadius: '3px', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', alignItems: 'center' }} title="이름 변경"><PencilIcon style={{ width: '12px', height: '12px' }} /></span>}
                        {onDeleteFolder && <span onClick={(e) => { e.stopPropagation(); if (window.confirm(`"${f.name}" 폴더를 삭제하시겠습니까?`)) onDeleteFolder(f.id); }} style={{ padding: '2px 4px', borderRadius: '3px', cursor: 'pointer', color: 'var(--color-destructive)', display: 'inline-flex', alignItems: 'center' }} title="삭제"><TrashIcon style={{ width: '12px', height: '12px' }} /></span>}
                      </span>
                    ) : badge ? (
                      <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', background: 'var(--color-bg-tertiary)', borderRadius: '10px', padding: '1px 6px', flexShrink: 0, marginInlineStart: '8px' }}>{badge}</span>
                    ) : null}
                  </button>
                )}
              </div>
            );
          })}

        {/* Create new folder */}
        {onCreateFolder && (
          <div style={{ marginInline: '4px' }}>
            {showNewFolder ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '4px 8px' }}>
                <input
                  autoFocus
                  value={newFolderInput}
                  onChange={(e) => setNewFolderInput(e.target.value)}
                  placeholder="폴더 이름"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && newFolderInput.trim()) { onCreateFolder(newFolderInput.trim()); setNewFolderInput(''); setShowNewFolder(false); }
                    if (e.key === 'Escape') { setShowNewFolder(false); setNewFolderInput(''); }
                  }}
                  style={{ flex: 1, fontSize: '13px', padding: '3px 6px', border: '1px solid var(--color-accent)', borderRadius: '4px', outline: 'none', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }}
                />
                <button onClick={() => { if (newFolderInput.trim()) { onCreateFolder(newFolderInput.trim()); setNewFolderInput(''); setShowNewFolder(false); } }} style={{ padding: '3px 6px', border: 'none', background: 'var(--color-accent)', color: '#fff', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}><CheckIcon style={{ width: '12px', height: '12px' }} /></button>
                <button onClick={() => { setShowNewFolder(false); setNewFolderInput(''); }} style={{ padding: '3px 6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}><XMarkIcon style={{ width: '12px', height: '12px' }} /></button>
              </div>
            ) : (
              <button
                onClick={() => setShowNewFolder(true)}
                style={{ width: '100%', textAlign: 'left', padding: '5px 16px', border: '1px solid transparent', background: 'transparent', color: 'var(--color-text-tertiary)', fontSize: '13px', cursor: 'pointer', borderRadius: '4px', display: 'flex', alignItems: 'center', gap: '6px' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
              >
                <span>+</span> 폴더 추가
              </button>
            )}
          </div>
        )}
      </nav>

      {footerExtra && (
        <div style={{ padding: '8px 12px', borderTop: '1px solid var(--color-border-subtle)' }}>
          {footerExtra}
        </div>
      )}
      </>
      )}
    </aside>
    {showSettings && (
      <SettingsModal onClose={() => setShowSettings(false)} userEmail={userEmailAddress} />
    )}
    </>
  );
}
