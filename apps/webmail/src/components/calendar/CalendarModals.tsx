'use client';

import { Calendar } from '@/lib/api';

type SubscriptionModalProps = {
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

type CalendarManagementModalProps = {
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

type EventCreateModalProps = {
  show: boolean;
  calendars: Calendar[];
  createTitle: string;
  createStart: string;
  createEnd: string;
  createAllDay: boolean;
  createLocation: string;
  createDesc: string;
  createCalId: string;
  createError: string;
  createSaving: boolean;
  createRrule: 'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';
  createRruleInterval: number;
  createRruleEnd: 'never' | 'count' | 'until';
  createRruleCount: number;
  createRruleUntil: string;
  createRruleDays: number[];
  canSubmit: boolean;
  showCalSelect: boolean;
  dayLabels: string[];
  ruleIntervalLabel: string;
  onClose: () => void;
  onSubmit: () => Promise<void> | void;
  onTitleChange: (value: string) => void;
  onStartChange: (value: string) => void;
  onEndChange: (value: string) => void;
  onAllDayToggle: (checked: boolean) => void;
  onLocationChange: (value: string) => void;
  onDescChange: (value: string) => void;
  onCalIdChange: (value: string) => void;
  onRruleChange: (value: EventCreateModalProps['createRrule']) => void;
  onRruleIntervalChange: (value: number) => void;
  onRruleEndChange: (value: EventCreateModalProps['createRruleEnd']) => void;
  onRruleCountChange: (value: number) => void;
  onRruleUntilChange: (value: string) => void;
  onToggleRruleDay: (day: number) => void;
};

type TodoModalProps = {
  show: boolean;
  todoDraft: string;
  todoDueDate: string;
  onDraftChange: (value: string) => void;
  onDueDateChange: (value: string) => void;
  onSubmit: () => Promise<void> | void;
  onClose: () => void;
  canSubmit: boolean;
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
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={onClose}>
      <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>캘린더 구독 추가</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>ICS/iCal URL *</label>
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
            <label style={M.label}>이름 (선택)</label>
            <input
              type="text"
              placeholder="캘린더 이름"
              value={subName}
              onChange={(e) => onNameChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <label style={M.label}>색상</label>
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
          <button onClick={onClose} style={M.cancelBtn}>취소</button>
          <button onClick={onSubmit} disabled={subSaving || !subUrl.trim()} style={M.primaryBtn(!subUrl.trim() || subSaving)}>
            {subSaving ? '추가 중...' : '구독 추가'}
          </button>
        </div>
      </div>
    </div>
  );
}

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
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={() => { if (!calSaving) onClose(); }}>
      <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>{editingCal ? '캘린더 편집' : '새 캘린더'}</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>캘린더 이름 *</label>
            <input
              autoFocus
              placeholder="내 캘린더"
              value={calName}
              onChange={(e) => onNameChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>설명 (선택)</label>
            <input
              placeholder="설명 추가"
              value={calDesc}
              onChange={(e) => onDescChange(e.target.value)}
              style={M.input}
            />
          </div>
          <div>
            <label style={M.label}>색상</label>
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
                title="직접 선택"
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
            ? <button onClick={onDelete} disabled={calSaving} style={M.dangerBtn}>삭제</button>
            : <span />}
          <div style={{ display: 'flex', gap: '8px' }}>
            <button onClick={onClose} disabled={calSaving} style={M.cancelBtn}>취소</button>
            <button onClick={onSave} disabled={calSaving || !calName.trim()} style={M.primaryBtn(calSaving || !calName.trim())}>
              {calSaving ? '저장 중...' : '저장'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export function EventCreateModal({
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
  showCalSelect,
  dayLabels,
  ruleIntervalLabel,
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
}: EventCreateModalProps) {
  const M = createCalendarModalStyles();
  if (!show) return null;

  return (
    <div style={M.overlay} onClick={() => { if (!createSaving) onClose(); }}>
      <div style={M.card('460px')} onClick={(e) => e.stopPropagation()}>
        <div style={M.header}><span style={M.title}>새 일정</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>제목 *</label>
            <input
              autoFocus
              type="text"
              placeholder="일정 제목"
              value={createTitle}
              onChange={(e) => onTitleChange(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') onSubmit(); }}
              style={M.input}
            />
          </div>

          {showCalSelect && (
            <div>
              <label style={M.label}>캘린더</label>
              <select value={createCalId} onChange={(e) => onCalIdChange(e.target.value)} style={M.select}>
                {calendars.map((c) => <option key={c.ID} value={c.ID ?? ''}>{c.Name ?? '(캘린더)'}</option>)}
              </select>
            </div>
          )}

          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
            <input type="checkbox" checked={createAllDay} onChange={(e) => onAllDayToggle(e.target.checked)} />
            하루 종일
          </label>

          <div style={{ display: 'flex', gap: '10px' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <label style={M.label}>시작</label>
              <input
                type={createAllDay ? 'date' : 'datetime-local'}
                value={createStart}
                onChange={(e) => onStartChange(e.target.value)}
                style={{ ...M.input, minWidth: 0 }}
              />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <label style={M.label}>종료</label>
              <input
                type={createAllDay ? 'date' : 'datetime-local'}
                value={createEnd}
                onChange={(e) => onEndChange(e.target.value)}
                style={{ ...M.input, minWidth: 0 }}
              />
            </div>
          </div>

          <div>
            <label style={M.label}>장소 (선택)</label>
            <input
              type="text"
              placeholder="장소 추가"
              value={createLocation}
              onChange={(e) => onLocationChange(e.target.value)}
              style={M.input}
            />
          </div>

          <div>
            <label style={M.label}>메모 (선택)</label>
            <textarea
              placeholder="메모 추가"
              value={createDesc}
              onChange={(e) => onDescChange(e.target.value)}
              rows={2}
              style={{ ...M.input, resize: 'none', fontFamily: 'inherit' }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '10px', borderRadius: '8px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', width: '36px', flexShrink: 0 }}>반복</span>
              <select value={createRrule} onChange={(e) => onRruleChange(e.target.value as EventCreateModalProps['createRrule'])} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                <option value="NONE">없음</option>
                <option value="DAILY">매일</option>
                <option value="WEEKLY">매주</option>
                <option value="MONTHLY">매월</option>
                <option value="YEARLY">매년</option>
              </select>
              {createRrule !== 'NONE' && (
                <>
                  <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>마다</span>
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
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>종료</span>
                <select value={createRruleEnd} onChange={(e) => onRruleEndChange(e.target.value as EventCreateModalProps['createRruleEnd'])} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                  <option value="never">계속 반복</option>
                  <option value="count">횟수 지정</option>
                  <option value="until">날짜 지정</option>
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
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>회</span>
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
          <button onClick={onClose} disabled={createSaving} style={M.cancelBtn}>취소</button>
          <button onClick={onSubmit} disabled={createSaving || !canSubmit} style={M.primaryBtn(createSaving || !canSubmit)}>
            {createSaving ? '저장 중...' : '저장'}
          </button>
        </div>
      </div>
    </div>
  );
}

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
        <div style={M.header}><span style={M.title}>할 일 추가</span></div>
        <div style={M.body}>
          <div>
            <label style={M.label}>제목 *</label>
            <input
              autoFocus
              type="text"
              placeholder="할 일 제목"
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
            <label style={M.label}>마감일 (선택)</label>
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
            취소
          </button>
          <button onClick={() => { void onSubmit(); onClose(); }} disabled={!canSubmit} style={M.primaryBtn(!canSubmit)}>
            추가
          </button>
        </div>
      </div>
    </div>
  );
}

function createCalendarModalStyles() {
  return {
    overlay: { position: 'fixed' as const, inset: 0, zIndex: 400, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.4)' },
    card: (w: string) => ({ background: 'var(--color-bg-primary)', borderRadius: '14px', width: w, maxWidth: 'calc(100vw - 32px)', boxShadow: '0 24px 64px rgba(0,0,0,0.22)', display: 'flex', flexDirection: 'column' as const, overflow: 'hidden' }),
    header: { padding: '20px 24px 16px', borderBottom: '1px solid var(--color-border-subtle)' },
    title: { fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' },
    body: { padding: '20px 24px', display: 'flex', flexDirection: 'column' as const, gap: '14px' },
    footer: { padding: '16px 24px 20px', borderTop: '1px solid var(--color-border-subtle)', display: 'flex', justifyContent: 'flex-end', gap: '8px' },
    footerSplit: { padding: '16px 24px 20px', borderTop: '1px solid var(--color-border-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' },
    label: { fontSize: '12px', color: 'var(--color-text-secondary)', display: 'block' as const, marginBottom: '4px' },
    input: { width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' as const },
    select: { width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', cursor: 'pointer' },
    error: { fontSize: '12px', color: '#e53e3e' },
    cancelBtn: { padding: '8px 16px', borderRadius: '7px', border: '1px solid var(--color-border-default)', background: 'none', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 },
    primaryBtn: (disabled: boolean) => ({ padding: '8px 20px', borderRadius: '7px', border: 'none', background: disabled ? 'var(--color-bg-tertiary)' : 'var(--color-accent)', color: disabled ? 'var(--color-text-tertiary)' : '#fff', fontSize: '13px', fontWeight: 600 as const, cursor: disabled ? 'default' as const : 'pointer' as const }),
    dangerBtn: { padding: '8px 14px', borderRadius: '7px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 },
  };
}
