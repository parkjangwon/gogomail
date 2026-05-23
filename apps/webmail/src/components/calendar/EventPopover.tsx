'use client';

import { useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { ParsedEvent } from '@/lib/calendar/eventParser';
import { formatDate, formatTime } from '@/lib/calendar/dateUtils';

export interface EventPopoverProps {
  event: ParsedEvent;
  anchorRect: DOMRect;
  onClose: () => void;
  onEdit: (event: ParsedEvent) => void;
  onDelete: (event: ParsedEvent) => void;
}

export function EventPopover({ event, anchorRect, onClose, onEdit, onDelete }: EventPopoverProps) {
  const t = useTranslations('calendar');
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  const style: React.CSSProperties = {
    position: 'fixed',
    zIndex: 300,
    background: 'var(--color-bg-primary)',
    border: '1px solid var(--color-border-default)',
    borderRadius: '8px',
    boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
    padding: '16px',
    minWidth: '220px',
    maxWidth: '320px',
    top: Math.min(anchorRect.bottom + 6, window.innerHeight - 260),
    left: Math.min(anchorRect.left, window.innerWidth - 340),
  };

  const btnBase: React.CSSProperties = {
    padding: '5px 10px',
    fontSize: '12px',
    borderRadius: '6px',
    cursor: 'pointer',
    fontWeight: 500,
    border: '1px solid var(--color-border-default)',
    background: 'none',
  };

  return (
    <div ref={ref} style={style}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: 0 }}>
          <div style={{ width: '10px', height: '10px', borderRadius: '50%', background: event.color, flexShrink: 0 }} />
          <span style={{ fontWeight: 600, fontSize: '14px', color: 'var(--color-text-primary)', wordBreak: 'break-word' }}>
            {event.summary}
          </span>
        </div>
        <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '16px', lineHeight: 1, padding: '0 2px', flexShrink: 0 }}>×</button>
      </div>
      <div style={{ marginTop: '10px', fontSize: '13px', color: 'var(--color-text-secondary)', display: 'flex', flexDirection: 'column', gap: '6px' }}>
        {event.allDay ? (
          <div>{formatDate(event.start)}</div>
        ) : (
          <div>{formatDate(event.start)} {formatTime(event.start)} – {formatTime(event.end)}</div>
        )}
        {event.location && <div>📍 {event.location}</div>}
        {event.description && (
          <div style={{ borderTop: '1px solid var(--color-border-subtle)', paddingTop: '8px', marginTop: '2px', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
            {event.description}
          </div>
        )}
      </div>
      <div style={{ display: 'flex', gap: '6px', marginTop: '12px', paddingTop: '10px', borderTop: '1px solid var(--color-border-subtle)' }}>
        <button
          style={{ ...btnBase, color: 'var(--color-text-primary)' }}
          onClick={() => { onClose(); onEdit(event); }}
        >
          {t('edit')}
        </button>
        <button
          style={{ ...btnBase, color: 'var(--color-destructive)', borderColor: 'var(--color-destructive)' }}
          onClick={() => { onDelete(event); }}
        >
          {t('delete')}
        </button>
      </div>
    </div>
  );
}
