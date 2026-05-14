'use client';

import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import {
  ChevronDownIcon,
  ChevronDoubleLeftIcon,
  PencilSquareIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

interface SidebarUserMenuProps {
  userName?: string;
  userEmailAddress?: string;
  avatarUrl: string;
  isMobile?: boolean;
  onClose?: () => void;
  onToggleCollapse?: () => void;
  onCompose: () => void;
  onLogout?: () => void;
  menuExtra?: ReactNode;
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

export function SidebarUserMenu({
  userName = '사용자',
  userEmailAddress,
  avatarUrl,
  isMobile,
  onClose,
  onToggleCollapse,
  onCompose,
  onLogout,
  menuExtra,
}: SidebarUserMenuProps) {
  const [showUserMenu, setShowUserMenu] = useState(false);
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

  return (
    <div ref={userMenuRef} style={{ position: 'relative' }}>
      <div style={{ padding: '10px 10px 8px', display: 'flex', alignItems: 'center', gap: '6px' }} onMouseEnter={() => setHeaderHovered(true)} onMouseLeave={() => setHeaderHovered(false)}>
        {isMobile && onClose && (
          <button aria-label="메뉴 닫기" onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-secondary)', padding: '0 2px', lineHeight: 1, flexShrink: 0, display: 'inline-flex' }}>
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        )}
        <button
          aria-label="계정 메뉴"
          aria-expanded={showUserMenu}
          onClick={() => setShowUserMenu((v) => !v)}
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '3px 4px', borderRadius: '6px', display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: 0, textAlign: 'left' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-overlay)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <div aria-hidden="true" style={{ width: '28px', height: '28px', borderRadius: '6px', background: avatarUrl ? 'transparent' : 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '12px', fontWeight: 700, flexShrink: 0, overflow: 'hidden' }}>
            {avatarUrl ? <img src={avatarUrl} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : getInitials(userName)}
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '3px' }}>
              <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {userName !== userEmailAddress ? userName : userName.split('@')[0]}
              </span>
              <ChevronDownIcon style={{ width: '12px', height: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0, opacity: headerHovered ? 1 : 0, transition: 'opacity 150ms' }} />
            </div>
            {userEmailAddress && (
              <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {userEmailAddress}
              </div>
            )}
          </div>
        </button>
        {!isMobile && onToggleCollapse && (
          <button
            aria-label="사이드바 접기"
            onClick={onToggleCollapse}
            title="사이드바 접기 ([)"
            style={{ width: '28px', height: '28px', borderRadius: '6px', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, opacity: headerHovered ? 1 : 0, pointerEvents: headerHovered ? 'auto' : 'none', transition: 'opacity 150ms' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
          ><ChevronDoubleLeftIcon style={{ width: '15px', height: '15px' }} /></button>
        )}
        <button
          aria-label="편지 쓰기"
          onClick={onCompose}
          title="편지 쓰기 (c)"
          style={{ width: '28px', height: '28px', borderRadius: '6px', border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
        ><PencilSquareIcon style={{ width: '15px', height: '15px' }} /></button>
      </div>

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
          <div style={{ padding: '16px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--color-border-subtle)' }}>
            <div style={{ width: '48px', height: '48px', borderRadius: '50%', background: avatarUrl ? 'transparent' : 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px', fontWeight: 700, overflow: 'hidden' }}>
              {avatarUrl ? <img src={avatarUrl} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : getInitials(userName)}
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
          {menuExtra && (
            <div style={{ padding: '8px 14px', borderBottom: '1px solid var(--color-border-subtle)' }}>
              {menuExtra}
            </div>
          )}
          {onLogout && (
            <button
              role="menuitem"
              onClick={() => { setShowUserMenu(false); onLogout(); }}
              style={{ width: '100%', padding: '10px 14px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', fontWeight: 500, cursor: 'pointer', textAlign: 'left' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
            >
              로그아웃
            </button>
          )}
        </div>
      )}
    </div>
  );
}
