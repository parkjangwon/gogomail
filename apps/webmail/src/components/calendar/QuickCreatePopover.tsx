'use client';

import { useState, useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';

export interface QuickCreatePopoverProps {
  day: Date;
  anchorRect: DOMRect;
  onClose: () => void;
  onSaveEvent: (title: string, day: Date) => Promise<void>;
  onSaveTodo: (title: string, day: Date) => Promise<void>;
  onMoreOptions: (day: Date, mode: 'event' | 'todo') => void;
}

export function QuickCreatePopover({ day, anchorRect, onClose, onSaveEvent, onSaveTodo, onMoreOptions }: QuickCreatePopoverProps) {
  const t = useTranslations();
  const [title, setTitle] = useState('');
  const [mode, setMode] = useState<'event' | 'todo'>('event');
  const [saving, setSaving] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  const handleSave = async () => {
    if (!title.trim() || saving) return;
    setSaving(true);
    try {
      if (mode === 'event') await onSaveEvent(title.trim(), day);
      else await onSaveTodo(title.trim(), day);
      onClose();
    } finally { setSaving(false); }
  };

  const dayLabels = (t.raw('misc.quickCreate.weekdays') as string[]) ?? ['일', '월', '화', '수', '목', '금', '토'];
  const dateStr = t('misc.quickCreate.dayFormat', {
    month: day.getMonth() + 1,
    day: day.getDate(),
    weekday: dayLabels[day.getDay()],
  });

  const top = Math.min(anchorRect.bottom + 4, window.innerHeight - 230);
  const left = Math.min(Math.max(anchorRect.left - 40, 8), window.innerWidth - 328);

  return (
    <div ref={ref} style={{
      position: 'fixed', zIndex: 350,
      background: 'var(--color-bg-primary)',
      border: '1px solid var(--color-border-default)',
      borderRadius: '12px',
      boxShadow: '0 8px 40px rgba(0,0,0,0.22)',
      padding: '16px 20px 18px',
      width: '310px', top, left,
    }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '6px' }}>
        <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '18px', color: 'var(--color-text-tertiary)', padding: '0 2px', lineHeight: 1 }}>×</button>
      </div>
      <input
        autoFocus
        type="text"
        placeholder={t('misc.quickCreate.placeholderTitle')}
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') onClose(); }}
        style={{
          width: '100%', border: 'none', borderBottom: '2px solid var(--color-accent)',
          outline: 'none', fontSize: '20px', fontWeight: 400,
          color: 'var(--color-text-primary)', background: 'transparent',
          padding: '2px 0 8px', marginBottom: '14px', boxSizing: 'border-box',
        }}
      />
      <div style={{ display: 'flex', borderBottom: '1px solid var(--color-border-subtle)', marginBottom: '14px' }}>
        {(['event', 'todo'] as const).map((m) => (
          <button key={m} onClick={() => setMode(m)} style={{
            padding: '6px 14px', fontSize: '13px', fontWeight: 500,
            border: 'none', background: 'none', cursor: 'pointer',
            color: mode === m ? 'var(--color-accent)' : 'var(--color-text-secondary)',
            borderBottom: mode === m ? '2px solid var(--color-accent)' : '2px solid transparent',
            marginBottom: '-1px',
          }}>
            {m === 'event' ? t('misc.quickCreate.tabEvent') : t('misc.quickCreate.tabTodo')}
          </button>
        ))}
      </div>
      <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <span>📅</span>
        <span>{dateStr}{mode === 'event' ? t('misc.quickCreate.allDay') : ''}</span>
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <button onClick={() => { onMoreOptions(day, mode); onClose(); }} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '13px', color: 'var(--color-text-secondary)', padding: '6px 0' }}>
          {t('misc.quickCreate.more')}
        </button>
        <button onClick={handleSave} disabled={!title.trim() || saving} style={{
          padding: '8px 20px', borderRadius: '6px', border: 'none',
          background: title.trim() && !saving ? 'var(--color-accent)' : 'var(--color-bg-tertiary)',
          color: title.trim() && !saving ? '#fff' : 'var(--color-text-tertiary)',
          fontSize: '13px', fontWeight: 500,
          cursor: title.trim() && !saving ? 'pointer' : 'default',
        }}>
          {saving ? t('misc.quickCreate.saving') : t('misc.quickCreate.save')}
        </button>
      </div>
    </div>
  );
}
