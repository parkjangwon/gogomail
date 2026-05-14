'use client';

import type { ICSEvent } from './readingPaneTypes';

interface CalendarInvitesProps {
  events: ICSEvent[];
  onAddToCalendar: (event: ICSEvent) => void;
  addingCalendarId: string | null;
  calendarAdded: string | null;
}

export function CalendarInvites({
  events,
  onAddToCalendar,
  addingCalendarId,
  calendarAdded,
}: CalendarInvitesProps) {
  return (
    <div style={{ marginBottom: '16px', maxWidth: '680px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
      {events.map((event) => {
        const fmtDt = (value: string) => {
          try {
            const clean = value.replace('Z', '');
            const d = value.length === 8
              ? new Date(`${value.slice(0, 4)}-${value.slice(4, 6)}-${value.slice(6, 8)}`)
              : new Date(`${clean.slice(0, 4)}-${clean.slice(4, 6)}-${clean.slice(6, 8)}T${clean.slice(9, 11)}:${clean.slice(11, 13)}:${clean.slice(13, 15)}`);
            return new Intl.DateTimeFormat('ko-KR', {
              dateStyle: 'medium',
              timeStyle: value.length === 8 ? undefined : 'short',
              hour12: false,
            }).format(d);
          } catch {
            return value;
          }
        };
        const added = calendarAdded === event.dtstart;
        const adding = addingCalendarId === event.dtstart;
        return (
          <div key={event.dtstart} style={{ display: 'flex', alignItems: 'flex-start', gap: '12px', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
            <div style={{ flexShrink: 0, width: '40px', height: '40px', borderRadius: '8px', background: 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px' }}>📅</div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 600, fontSize: '14px', color: 'var(--color-text-primary)', marginBottom: '3px' }}>{event.summary}</div>
              <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{fmtDt(event.dtstart)}{event.dtend ? ` ~ ${fmtDt(event.dtend)}` : ''}</div>
              {event.location && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>📍 {event.location}</div>}
            </div>
            <button
              onClick={() => onAddToCalendar(event)}
              disabled={adding || added}
              style={{ flexShrink: 0, padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: added ? 'var(--color-accent-subtle)' : 'transparent', color: added ? 'var(--color-accent)' : 'var(--color-text-primary)', fontSize: '12px', cursor: adding || added ? 'default' : 'pointer', fontWeight: 500, whiteSpace: 'nowrap' }}
            >
              {adding ? '추가 중...' : added ? '✓ 추가됨' : '캘린더에 추가'}
            </button>
          </div>
        );
      })}
    </div>
  );
}
