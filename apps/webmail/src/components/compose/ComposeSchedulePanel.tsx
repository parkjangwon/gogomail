'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';

interface ComposeSchedulePanelProps {
  open: boolean;
  scheduledAt: string;
  setScheduledAt: Dispatch<SetStateAction<string>>;
  setShowSchedule: Dispatch<SetStateAction<boolean>>;
  scheduleMinDateTime: string;
}

export function ComposeSchedulePanel({ open, scheduledAt, setScheduledAt, setShowSchedule, scheduleMinDateTime }: ComposeSchedulePanelProps) {
  const t = useTranslations('composeFull');

  if (!open && !scheduledAt) return null;

  if (!open && scheduledAt) {
    return (
      <button
        type="button"
        onClick={() => setScheduledAt('')}
        style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
      >{t('unschedule')}</button>
    );
  }

  return (
    <>
      <input
        type="datetime-local"
        value={scheduledAt}
        onChange={(e) => setScheduledAt(e.target.value)}
        min={scheduleMinDateTime}
        aria-label={t('scheduleAria')}
        aria-describedby="compose-schedule-help"
        style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none' }}
      />
      <span id="compose-schedule-help" style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>{t('scheduleHelp')}</span>
      <button
        type="button"
        onClick={() => { setScheduledAt(''); setShowSchedule(false); }}
        style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
      >{t('unschedule')}</button>
    </>
  );
}
