'use client';

import { useTranslations } from 'next-intl';
import { createCalendarModalStyles } from './calendarModalStyles';

export type TodoModalProps = {
  show: boolean;
  todoDraft: string;
  todoDueDate: string;
  onDraftChange: (value: string) => void;
  onDueDateChange: (value: string) => void;
  onSubmit: () => Promise<void> | void;
  onClose: () => void;
  canSubmit: boolean;
};

export function TodoCreateModal({
  show,
  todoDraft,
  todoDueDate,
  onDraftChange,
  onDueDateChange,
  onSubmit,
  onClose,
  canSubmit,
}: TodoModalProps) {
  const t = useTranslations('calendarFull.todo');
  const tc = useTranslations('calendarFull.common');
  const M = createCalendarModalStyles();
  if (!show) return null;

  const closeAndReset = () => {
    onDraftChange('');
    onDueDateChange('');
    onClose();
  };

  return (
    <div style={M.overlay} onClick={onClose}>
      <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>{t('addTitle')}</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>{t('titleLabel')}</label>
            <input
              autoFocus
              type="text"
              placeholder={t('titlePlaceholder')}
              value={todoDraft}
              onChange={(e) => onDraftChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && canSubmit) {
                  void onSubmit();
                  onClose();
                }
                if (e.key === 'Escape') onClose();
              }}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>{t('dueLabel')}</label>
            <input
              type="date"
              value={todoDueDate}
              onChange={(e) => onDueDateChange(e.target.value)}
              style={M.input}
            />
          </div>
        </div>
        <div style={M.footer}>
          <button
            onClick={closeAndReset}
            style={M.cancelBtn}
          >
            {tc('cancel')}
          </button>
          <button onClick={() => { void onSubmit(); onClose(); }} disabled={!canSubmit} style={M.primaryBtn(!canSubmit)}>
            {t('addButton')}
          </button>
        </div>
      </div>
    </div>
  );
}
