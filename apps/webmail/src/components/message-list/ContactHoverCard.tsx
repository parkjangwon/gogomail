'use client';

import { avatarColor } from './messageListTypes';

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
  const initials = (name || addr).charAt(0).toUpperCase();
  const color = avatarColor(name || addr);
  const cardW = 224;
  const clampedX = Math.min(x, (typeof window !== 'undefined' ? window.innerWidth : 1200) - cardW - 16);

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
      <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '10px', paddingBottom: '10px', borderBottom: '1px solid var(--color-border-subtle)' }}>
        받은 편지함에 {count}개 메일
      </div>
      {onComposeTo && (
        <button type="button" onClick={() => { onComposeTo(addr, name); onClose(); }}
          style={{ width: '100%', padding: '6px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          + 새 메일 작성
        </button>
      )}
    </div>
  );
}

