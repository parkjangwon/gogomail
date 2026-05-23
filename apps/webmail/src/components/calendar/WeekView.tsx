'use client';

import { useRef, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { startOfWeek, isSameDay, addDays, formatHour, formatTime } from '@/lib/calendar/dateUtils';
import { ParsedEvent } from '@/lib/calendar/eventParser';

export interface WeekViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  today: Date;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
}

const HOUR_HEIGHT = 48;
const HOURS = Array.from({ length: 24 }, (_, i) => i);

export function WeekView({ currentDate, events, today, onEventClick }: WeekViewProps) {
  const t = useTranslations('calendar');
  const mon = startOfWeek(currentDate);
  const days = Array.from({ length: 7 }, (_, i) => addDays(mon, i));
  const weekDayLabels = [t('wkMon'), t('wkTue'), t('wkWed'), t('wkThu'), t('wkFri'), t('wkSat'), t('wkSun')];
  const isWeekend = (d: Date) => d.getDay() === 0 || d.getDay() === 6;

  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = 7 * HOUR_HEIGHT; // scroll to 7am
    }
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
      {/* Header row with day names */}
      <div style={{ display: 'grid', gridTemplateColumns: '48px repeat(7, 1fr)', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
        <div style={{ borderRight: '1px solid var(--color-border-subtle)' }} />
        {days.map((day, i) => {
          const isToday = isSameDay(day, today);
          const weekend = isWeekend(day);
          return (
            <div
              key={i}
              style={{
                padding: '6px 4px',
                textAlign: 'center',
                borderRight: i < 6 ? '1px solid var(--color-border-subtle)' : undefined,
                background: weekend ? 'var(--color-bg-secondary)' : undefined,
              }}
            >
              <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)' }}>{weekDayLabels[i]}</div>
              <div
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: '28px',
                  height: '28px',
                  borderRadius: '50%',
                  margin: '0 auto',
                  fontSize: '14px',
                  fontWeight: isToday ? 700 : 400,
                  color: isToday ? '#fff' : 'var(--color-text-primary)',
                  background: isToday ? 'var(--color-accent)' : undefined,
                }}
              >
                {day.getDate()}
              </div>
            </div>
          );
        })}
      </div>
      {/* Time grid */}
      <div ref={scrollRef} style={{ flex: 1, overflow: 'auto', position: 'relative' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '48px repeat(7, 1fr)', position: 'relative' }}>
          {/* Hour labels */}
          <div style={{ gridColumn: 1, gridRow: '1 / -1' }}>
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
          {/* Day columns */}
          {days.map((day, di) => {
            const weekend = isWeekend(day);
            const dayEvents = events.filter((ev) => {
              if (ev.allDay) return false;
              return isSameDay(ev.start, day) || isSameDay(ev.end, day);
            });
            return (
              <div
                key={di}
                style={{
                  gridColumn: di + 2,
                  position: 'relative',
                  background: weekend ? 'var(--color-bg-secondary)' : 'var(--color-bg-primary)',
                  borderRight: di < 6 ? '1px solid var(--color-border-subtle)' : undefined,
                }}
              >
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
                {dayEvents.map((ev) => {
                  const startH = ev.start.getHours() + ev.start.getMinutes() / 60;
                  const endH = ev.end.getHours() + ev.end.getMinutes() / 60;
                  const duration = Math.max(endH - startH, 0.25);
                  return (
                    <div
                      key={ev.obj.ID}
                      onClick={(e) => { e.stopPropagation(); onEventClick(ev, e.currentTarget.getBoundingClientRect()); }}
                      title={ev.summary}
                      style={{
                        position: 'absolute',
                        top: `${startH * HOUR_HEIGHT}px`,
                        left: '2px',
                        right: '2px',
                        height: `${duration * HOUR_HEIGHT - 2}px`,
                        background: ev.color,
                        color: '#fff',
                        borderRadius: '3px',
                        padding: '2px 4px',
                        fontSize: '11px',
                        overflow: 'hidden',
                        cursor: 'pointer',
                        zIndex: 1,
                      }}
                    >
                      <div style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{ev.summary}</div>
                      <div style={{ opacity: 0.85, fontSize: '10px' }}>{formatTime(ev.start)} – {formatTime(ev.end)}</div>
                    </div>
                  );
                })}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
