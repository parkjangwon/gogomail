'use client';

import { useTranslations } from 'next-intl';
import { createCalendarModalStyles } from './calendarModalStyles';
import { EventCreateModalProps } from './EventCreateModal';

export type EventEditModalProps = Omit<EventCreateModalProps, 'showCalSelect'> & {
  isRecurring?: boolean;
};

export function EventEditModal({
  show,
  calendars,
  createTitle,
  createStart,
  createEnd,
  createAllDay,
  createLocation,
  createDesc,
  createCalId,
  createError,
  createSaving,
  createRrule,
  createRruleInterval,
  createRruleEnd,
  createRruleCount,
  createRruleUntil,
  createRruleDays,
  canSubmit,
  dayLabels,
  ruleIntervalLabel,
  isRecurring,
  onClose,
  onSubmit,
  onTitleChange,
  onStartChange,
  onEndChange,
  onAllDayToggle,
  onLocationChange,
  onDescChange,
  onCalIdChange,
  onRruleChange,
  onRruleIntervalChange,
  onRruleEndChange,
  onRruleCountChange,
  onRruleUntilChange,
  onToggleRruleDay,
}: EventEditModalProps) {
  const t = useTranslations('calendarFull.event');
  const tc = useTranslations('calendarFull.common');
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={() => { if (!createSaving) onClose(); }}>
      <div style={M.card('460px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>{t('editTitle')}</span></div>
        <div style={M.body}>
          {isRecurring && (
            <div role="note" style={{ padding: '8px 10px', borderRadius: '8px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', color: 'var(--color-text-secondary)', fontSize: '12px', lineHeight: 1.5 }}>
              <div style={{ fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: '2px' }}>
                {t('recurringNote')}
              </div>
              {t('recurringDesc')}
            </div>
          )}

          <div>
            <label style={M.label}>{t('titleLabel')}</label>
            <input
              autoFocus
              type="text"
              placeholder={t('titlePlaceholder')}
              value={createTitle}
              onChange={(e) => onTitleChange(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') onSubmit(); }}
              style={M.input}
            />
          </div>

          {calendars.length > 1 && (
            <div>
              <label style={M.label}>{t('calendarLabel')}</label>
              <select value={createCalId} onChange={(e) => onCalIdChange(e.target.value)} style={M.select}>
                {calendars.map((c) => <option key={c.ID} value={c.ID ?? ''}>{c.Name ?? t('defaultCalName')}</option>)}
              </select>
            </div>
          )}

          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
            <input type="checkbox" checked={createAllDay} onChange={(e) => onAllDayToggle(e.target.checked)} />
            {t('allDay')}
          </label>

          <div style={{ display: 'flex', gap: '10px' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <label style={M.label}>{t('startLabel')}</label>
              <input
                type={createAllDay ? 'date' : 'datetime-local'}
                value={createStart}
                onChange={(e) => onStartChange(e.target.value)}
                style={{ ...M.input, minWidth: 0 }}
              />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <label style={M.label}>{t('endLabel')}</label>
              <input
                type={createAllDay ? 'date' : 'datetime-local'}
                value={createEnd}
                onChange={(e) => onEndChange(e.target.value)}
                style={{ ...M.input, minWidth: 0 }}
              />
            </div>
          </div>

          <div>
            <label style={M.label}>{t('locationLabel')}</label>
            <input
              type="text"
              placeholder={t('locationPlaceholder')}
              value={createLocation}
              onChange={(e) => onLocationChange(e.target.value)}
              style={M.input}
            />
          </div>

          <div>
            <label style={M.label}>{t('memoLabel')}</label>
            <textarea
              placeholder={t('memoPlaceholder')}
              value={createDesc}
              onChange={(e) => onDescChange(e.target.value)}
              rows={2}
              style={{ ...M.input, resize: 'none', fontFamily: 'inherit' }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '10px', borderRadius: '8px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', width: '36px', flexShrink: 0 }}>{t('repeatLabel')}</span>
              <select value={createRrule} onChange={(e) => onRruleChange(e.target.value as EventCreateModalProps['createRrule'])} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                <option value="NONE">{t('recurNone')}</option>
                <option value="DAILY">{t('recurDaily')}</option>
                <option value="WEEKLY">{t('recurWeekly')}</option>
                <option value="MONTHLY">{t('recurMonthly')}</option>
                <option value="YEARLY">{t('recurYearly')}</option>
              </select>
              {createRrule !== 'NONE' && (
                <>
                  <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('repeatEvery')}</span>
                  <input
                    type="number"
                    min={1}
                    max={99}
                    value={createRruleInterval}
                    onChange={(e) => onRruleIntervalChange(Math.max(1, Number(e.target.value)))}
                    style={{ width: '44px', padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' }}
                  />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{ruleIntervalLabel}</span>
                </>
              )}
            </div>
            {createRrule === 'WEEKLY' && (
              <div style={{ display: 'flex', gap: '4px', paddingLeft: '44px' }}>
                {dayLabels.map((d, i) => (
                  <button
                    key={i}
                    type="button"
                    onClick={() => onToggleRruleDay(i)}
                    style={{
                      width: '26px',
                      height: '26px',
                      borderRadius: '50%',
                      border: '1px solid var(--color-border-default)',
                      background: createRruleDays.includes(i) ? 'var(--color-accent)' : 'transparent',
                      color: createRruleDays.includes(i) ? '#fff' : 'var(--color-text-secondary)',
                      fontSize: '11px',
                      cursor: 'pointer',
                      padding: 0,
                      fontWeight: 500,
                    }}
                  >
                    {d}
                  </button>
                ))}
              </div>
            )}
            {createRrule !== 'NONE' && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', paddingLeft: '44px' }}>
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{t('endLabel2')}</span>
                <select value={createRruleEnd} onChange={(e) => onRruleEndChange(e.target.value as EventCreateModalProps['createRruleEnd'])} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                  <option value="never">{t('endNever')}</option>
                  <option value="count">{t('endCount')}</option>
                  <option value="until">{t('endUntil')}</option>
                </select>
                {createRruleEnd === 'count' && (
                  <>
                    <input
                      type="number"
                      min={1}
                      max={999}
                      value={createRruleCount}
                      onChange={(e) => onRruleCountChange(Math.max(1, Number(e.target.value)))}
                      style={{ width: '52px', padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' }}
                    />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('endCountSuffix')}</span>
                  </>
                )}
                {createRruleEnd === 'until' && (
                  <input
                    type="date"
                    value={createRruleUntil}
                    onChange={(e) => onRruleUntilChange(e.target.value)}
                    style={{ padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }}
                  />
                )}
              </div>
            )}
          </div>

          {createError && <div style={M.error}>{createError}</div>}
        </div>
        <div style={M.footer}>
          <button onClick={onClose} disabled={createSaving} style={M.cancelBtn}>{tc('cancel')}</button>
          <button onClick={onSubmit} disabled={createSaving || !canSubmit} style={M.primaryBtn(createSaving || !canSubmit)}>
            {createSaving ? t('savingButton') : t('editButton')}
          </button>
        </div>
      </div>
    </div>
  );
}
