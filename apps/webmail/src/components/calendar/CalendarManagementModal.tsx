'use client';

import { useTranslations } from 'next-intl';
import { Calendar } from '@/lib/api';
import { createCalendarModalStyles } from './calendarModalStyles';

export type CalendarManagementModalProps = {
  show: boolean;
  editingCal: Calendar | null;
  calName: string;
  calDesc: string;
  calColor: string;
  calError: string;
  calSaving: boolean;
  colors: readonly string[];
  onClose: () => void;
  onDelete: () => Promise<void> | void;
  onSave: () => Promise<void> | void;
  onNameChange: (value: string) => void;
  onDescChange: (value: string) => void;
  onColorChange: (value: string) => void;
  onColorQuickSelect: (value: string) => void;
};

export function CalendarManagementModal({
  show,
  editingCal,
  calName,
  calDesc,
  calColor,
  calError,
  calSaving,
  colors,
  onClose,
  onDelete,
  onSave,
  onNameChange,
  onDescChange,
  onColorChange,
  onColorQuickSelect,
}: CalendarManagementModalProps) {
  const t = useTranslations('calendarFull.management');
  const tc = useTranslations('calendarFull.common');
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={() => { if (!calSaving) onClose(); }}>
      <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>{editingCal ? t('editTitle') : t('newTitle')}</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>{t('nameLabel')}</label>
            <input
              autoFocus
              placeholder={t('namePlaceholder')}
              value={calName}
              onChange={(e) => onNameChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>{t('descLabel')}</label>
            <input
              placeholder={t('descPlaceholder')}
              value={calDesc}
              onChange={(e) => onDescChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>{t('colorLabel')}</label>
            <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', alignItems: 'center' }}>
              {colors.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => onColorQuickSelect(c)}
                  style={{
                    width: '24px',
                    height: '24px',
                    borderRadius: '50%',
                    background: c,
                    border: calColor === c ? '3px solid var(--color-text-primary)' : '2.5px solid transparent',
                    cursor: 'pointer',
                    padding: 0,
                    boxShadow: calColor === c ? `0 0 0 1.5px ${c}` : 'none',
                    transition: 'border 100ms',
                  }}
                />
              ))}
              <input
                type="color"
                value={calColor}
                onChange={(e) => onColorChange(e.target.value)}
                title={t('colorPickerTitle')}
                style={{
                  width: '24px',
                  height: '24px',
                  borderRadius: '50%',
                  border: '1px solid var(--color-border-default)',
                  cursor: 'pointer',
                  padding: 0,
                  background: 'none',
                }}
              />
            </div>
          </div>
          {calError && <div style={M.error}>{calError}</div>}
        </div>
        <div style={M.footerSplit}>
          {editingCal
            ? <button onClick={onDelete} disabled={calSaving} style={M.dangerBtn}>{t('deleteButton')}</button>
            : <span />}
          <div style={{ display: 'flex', gap: '8px' }}>
            <button onClick={onClose} disabled={calSaving} style={M.cancelBtn}>{tc('cancel')}</button>
            <button onClick={onSave} disabled={calSaving || !calName.trim()} style={M.primaryBtn(calSaving || !calName.trim())}>
              {calSaving ? t('savingButton') : t('saveButton')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
