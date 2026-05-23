'use client';

import { useRef, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { isSameDay, formatHour, formatTime } from '@/lib/calendar/dateUtils';
import { ParsedEvent } from '@/lib/calendar/eventParser';

export interface DayViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  today: Date;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
}

const HOUR_HEIGHT = 48;
const HOURS = Array.from({ length: 24 }, (_, i) => i);

export function DayView({ currentDate, events, today, onEventClick }: DayViewProps) {
  const t = useTranslations('calendar');
  const isToday = isSameDay(currentDate, today);
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = 7 * HOUR_HEIGHT;
    }
  }, []);

  const dayEvents = events.filter((ev) => {
    if (ev.allDay) return isSameDay(ev.start, currentDate);
    return isSameDay(ev.start, currentDate) || isSameDay(ev.end, currentDate);
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ borderBottom: '1px solid var(--color-border-subtle)', padding: '8px 16px', display: 'flex', alignItems: 'center', gap: '12px', flexShrink: 0 }}>
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: '36px',
            height: '36px',
            borderRadius: '50%',
            fontSize: '18px',
            fontWeight: 700,
            color: isToday ? '#fff' : 'var(--color-text-primary)',
            background: isToday ? 'var(--color-accent)' : undefined,
          }}
        >
          {currentDate.getDate()}
        </div>
        <div style={{ fontSize: '14px', color: 'var(--color-text-secondary)' }}>
          {t('dayWeekday', { weekday: [t('wkSun'), t('wkMon'), t('wkTue'), t('wkWed'), t('wkThu'), t('wkFri'), t('wkSat')][currentDate.getDay()] })}
        </div>
      </div>
      {/* Grid */}
      <div ref={scrollRef} style={{ flex: 1, overflow: 'auto', position: 'relative' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '48px 1fr', position: 'relative' }}>
          {/* Hour labels */}
          <div>
            {HOURS.map((h) => (
              <div
                key={h}
                style={{
                  height: `${HOUR_HEIGHT}px`,
                  borderBottom: '1px solid var(--color-border-subtle)',
                  borderRight: '1px solid var(--color-border-subtle)',
                  display: 'flex',
                  alignItems: 'flex-start',
                  justifyContent: 'flex-end',
                  paddingRight: '6px',
                  paddingTop: '2px',
                  fontSize: '10px',
                  color: 'var(--color-text-tertiary)',
                  boxSizing: 'border-box',
                }}
              >
                {h > 0 ? formatHour(h) : ''}
              </div>
            ))}
          </div>
          {/* Event column */}
          <div style={{ position: 'relative', background: 'var(--color-bg-primary)' }}>
            {HOURS.map((h) => (
              <div
                key={h}
                style={{
                  height: `${HOUR_HEIGHT}px`,
                  borderBottom: '1px solid var(--color-border-subtle)',
                  boxSizing: 'border-box',
                }}
              />
            ))}
            {dayEvents.filter((ev) => !ev.allDay).map((ev) => {
              const startH = ev.start.getHours() + ev.start.getMinutes() / 60;
              const endH = ev.end.getHours() + ev.end.getMinutes() / 60;
              const duration = Math.max(endH - startH, 0.25);
              return (
                <div
                  key={ev.obj.ID}
                  onClick={(e) => { e.stopPropagation(); onEventClick(ev, e.currentTarget.getBoundingClientRect()); }}
                  style={{
                    position: 'absolute',
                    top: `${startH * HOUR_HEIGHT}px`,
                    left: '4px',
                    right: '4px',
                    height: `${duration * HOUR_HEIGHT - 2}px`,
                    background: ev.color,
                    color: '#fff',
                    borderRadius: '4px',
                    padding: '4px 8px',
                    fontSize: '13px',
                    overflow: 'hidden',
                    cursor: 'pointer',
                    zIndex: 1,
                  }}
                >
                  <div style={{ fontWeight: 600 }}>{ev.summary}</div>
                  <div style={{ fontSize: '11px', opacity: 0.9 }}>{formatTime(ev.start)} – {formatTime(ev.end)}</div>
                  {ev.location && <div style={{ fontSize: '11px', opacity: 0.8 }}>{ev.location}</div>}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
