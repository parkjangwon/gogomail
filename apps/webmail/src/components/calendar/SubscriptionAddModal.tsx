'use client';

import { useTranslations } from 'next-intl';
import { createCalendarModalStyles } from './calendarModalStyles';

export type SubscriptionModalProps = {
  show: boolean;
  subError: string;
  subUrl: string;
  subName: string;
  subColor: string;
  subSaving: boolean;
  onClose: () => void;
  onSubmit: () => Promise<void> | void;
  onUrlChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onColorChange: (value: string) => void;
};

export function SubscriptionAddModal({
  show,
  subError,
  subUrl,
  subName,
  subColor,
  subSaving,
  onClose,
  onSubmit,
  onUrlChange,
  onNameChange,
  onColorChange,
}: SubscriptionModalProps) {
  const t = useTranslations('calendarFull.subscription');
  const tc = useTranslations('calendarFull.common');
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={onClose}>
      <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>{t('addTitle')}</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>{t('urlLabel')}</label>
            <input
              autoFocus
              type="url"
              placeholder="https://calendar.google.com/calendar/ical/..."
              value={subUrl}
              onChange={(e) => onUrlChange(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && onSubmit()}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>{t('nameLabel')}</label>
            <input
              type="text"
              placeholder={t('namePlaceholder')}
              value={subName}
              onChange={(e) => onNameChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <label style={M.label}>{t('colorLabel')}</label>
            <input
              type="color"
              value={subColor}
              onChange={(e) => onColorChange(e.target.value)}
              style={{
                width: '32px',
                height: '32px',
                border: 'none',
                borderRadius: '50%',
                cursor: 'pointer',
                padding: 0,
                background: 'none',
              }}
            />
          </div>
          {subError && <div style={M.error}>{subError}</div>}
        </div>
        <div style={M.footer}>
          <button onClick={onClose} style={M.cancelBtn}>{tc('cancel')}</button>
          <button onClick={onSubmit} disabled={subSaving || !subUrl.trim()} style={M.primaryBtn(!subUrl.trim() || subSaving)}>
            {subSaving ? t('addingButton') : t('addButton')}
          </button>
        </div>
      </div>
    </div>
  );
}
