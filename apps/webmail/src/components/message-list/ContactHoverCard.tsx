'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { avatarColor } from './messageListTypes';
import { getDirectoryProfile, type DirectoryProfile } from '@/lib/api';

type ContactHoverCardProps = {
  name: string;
  addr: string;
  count: number;
  x: number;
  y: number;
  onClose: () => void;
  onComposeTo?: (addr: string, name: string) => void;
};

export function ContactHoverCard({ name, addr, count, x, y, onClose, onComposeTo }: ContactHoverCardProps) {
  const t = useTranslations('contactHover');
  const initials = (name || addr).charAt(0).toUpperCase();
  const color = avatarColor(addr);
  const cardW = 240;
  const clampedX = Math.min(x, (typeof window !== 'undefined' ? window.innerWidth : 1200) - cardW - 16);

  const [profile, setProfile] = useState<DirectoryProfile | null>(null);

  useEffect(() => {
    let cancelled = false;
    getDirectoryProfile(addr).then((p) => {
      if (!cancelled && p?.found) setProfile(p);
    });
    return () => { cancelled = true; };
  }, [addr]);

  const displayTitle = profile?.title || '';
  const displayOrg = profile?.org_unit_name || '';

  return (
    <div onMouseEnter={() => { /* keep */ }} onMouseLeave={onClose}
      style={{ position: 'fixed', top: y, left: clampedX, zIndex: 900, width: `${cardW}px`, background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '12px', boxShadow: '0 8px 32px rgba(0,0,0,0.2)', padding: '14px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
        <div style={{ width: '40px', height: '40px', borderRadius: '50%', background: color, color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '16px', fontWeight: 700, flexShrink: 0 }}>
          {initials}
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name || addr}</div>
          <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{addr}</div>
        </div>
      </div>

      {(displayTitle || displayOrg) && (
        <div style={{ marginBottom: '8px', paddingBottom: '8px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          {displayTitle && (
            <div style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-primary)', marginBottom: displayOrg ? '2px' : 0 }}>
              {displayTitle}
            </div>
          )}
          {displayOrg && (
            <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <svg width="11" height="11" viewBox="0 0 20 20" fill="currentColor" style={{ flexShrink: 0, opacity: 0.6 }}>
                <path d="M2 11a1 1 0 011-1h2a1 1 0 011 1v5a1 1 0 01-1 1H3a1 1 0 01-1-1v-5zm6-4a1 1 0 011-1h2a1 1 0 011 1v9a1 1 0 01-1 1H9a1 1 0 01-1-1V7zm6-3a1 1 0 011-1h2a1 1 0 011 1v12a1 1 0 01-1 1h-2a1 1 0 01-1-1V4z" />
              </svg>
              {displayOrg}
            </div>
          )}
        </div>
      )}

      <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '10px', paddingBottom: '10px', borderBottom: '1px solid var(--color-border-subtle)' }}>
        {t('inboxCount', { count })}
      </div>
      {onComposeTo && (
        <button type="button" onClick={() => { onComposeTo(addr, name); onClose(); }}
          style={{ width: '100%', padding: '6px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          {t('newMail')}
        </button>
      )}
    </div>
  );
}
