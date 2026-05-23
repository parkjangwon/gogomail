'use client';
import { useTranslations } from 'next-intl';

type CalendarViewType = 'month' | 'week' | 'day';

interface CalendarToolbarProps {
  title: string;
  view: CalendarViewType;
  onGoToday: () => void;
  onNavigate: (delta: number) => void;
  onSetView: (view: CalendarViewType) => void;
}

const viewButtons: CalendarViewType[] = ['day', 'week', 'month'];

export function CalendarToolbar({
  title,
  view,
  onGoToday,
  onNavigate,
  onSetView,
}: CalendarToolbarProps) {
  const t = useTranslations('calendar');
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        padding: '8px 12px',
        borderBottom: '1px solid var(--color-border-subtle)',
        gap: '8px',
        flexShrink: 0,
        background: 'var(--color-bg-primary)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
        <button
          onClick={onGoToday}
          style={{
            padding: '5px 12px',
            borderRadius: '5px',
            border: '1px solid var(--color-border-default)',
            background: 'none',
            color: 'var(--color-text-primary)',
            cursor: 'pointer',
            fontSize: '12px',
            fontWeight: 500,
          }}
        >
          {t('today')}
        </button>
        <button
          onClick={() => onNavigate(-1)}
          aria-label={t('prev')}
          style={{
            padding: '5px 8px',
            borderRadius: '5px',
            border: '1px solid var(--color-border-default)',
            background: 'none',
            color: 'var(--color-text-primary)',
            cursor: 'pointer',
            fontSize: '14px',
            lineHeight: 1,
          }}
        >
          ‹
        </button>
        <button
          onClick={() => onNavigate(1)}
          aria-label={t('next')}
          style={{
            padding: '5px 8px',
            borderRadius: '5px',
            border: '1px solid var(--color-border-default)',
            background: 'none',
            color: 'var(--color-text-primary)',
            cursor: 'pointer',
            fontSize: '14px',
            lineHeight: 1,
          }}
        >
          ›
        </button>
      </div>

      <div style={{ flex: 1, fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', paddingLeft: '4px' }}>
        {title}
      </div>

      <div style={{ display: 'flex', borderRadius: '6px', border: '1px solid var(--color-border-default)', overflow: 'hidden' }}>
        {viewButtons.map((v) => {
          const labels = { day: t('viewDay'), week: t('viewWeek'), month: t('viewMonth') };
          return (
            <button
              key={v}
              onClick={() => onSetView(v)}
              style={{
                padding: '5px 10px',
                border: 'none',
                borderRight: v !== 'month' ? '1px solid var(--color-border-default)' : 'none',
                background: view === v ? 'var(--color-accent)' : 'none',
                color: view === v ? '#fff' : 'var(--color-text-primary)',
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: 500,
              }}
            >
              {labels[v]}
            </button>
          );
        })}
      </div>
    </div>
  );
}

