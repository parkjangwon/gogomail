'use client';

import { useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { startOfMonth, startOfWeek, isSameDay, addDays } from '@/lib/calendar/dateUtils';
import { ParsedEvent, ParsedTodo } from '@/lib/calendar/eventParser';

export interface MonthViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  todos: ParsedTodo[];
  today: Date;
  onDayClick: (d: Date) => void;
  onCellClick: (d: Date, rect: DOMRect) => void;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
  onTodoToggle: (t: ParsedTodo) => void;
}

const toDateKey = (d: Date) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;

export function MonthView({ currentDate, events, todos, today, onDayClick, onCellClick, onEventClick, onTodoToggle }: MonthViewProps) {
  const t = useTranslations('calendar');
  const month = currentDate.getMonth();
  const firstDay = startOfMonth(currentDate);
  const gridStart = startOfWeek(firstDay);
  const days: Date[] = [];
  for (let i = 0; i < 42; i++) days.push(addDays(gridStart, i));
  const needed = days.findLastIndex((d) => d.getMonth() === month || d <= firstDay) + 1;
  const cellCount = Math.ceil(Math.max(needed, 28) / 7) * 7;
  const visibleDays = days.slice(0, cellCount);
  const weekDays = [t('wkMon'), t('wkTue'), t('wkWed'), t('wkThu'), t('wkFri'), t('wkSat'), t('wkSun')];

  // Pre-bucket events/todos by day key to avoid O(days × events) per render.
  const eventsByDay = useMemo(() => {
    const map = new Map<string, ParsedEvent[]>();
    for (const ev of events) {
      const s = new Date(ev.start); s.setHours(0, 0, 0, 0);
      const e = new Date(ev.end); e.setHours(23, 59, 59, 999);
      for (const day of visibleDays) {
        const d = new Date(day); d.setHours(12, 0, 0, 0);
        if (d >= s && d <= e) {
          const key = toDateKey(day);
          const bucket = map.get(key) ?? [];
          bucket.push(ev);
          map.set(key, bucket);
        }
      }
    }
    return map;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [events, visibleDays.length, month]);

  const todosByDay = useMemo(() => {
    const map = new Map<string, ParsedTodo[]>();
    for (const todo of todos) {
      if (!todo.dueDate) continue;
      const key = toDateKey(todo.dueDate);
      const bucket = map.get(key) ?? [];
      bucket.push(todo);
      map.set(key, bucket);
    }
    return map;
  }, [todos]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
      {/* Day header row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
        {weekDays.map((wd) => (
          <div key={wd} style={{ padding: '8px 0', textAlign: 'center', fontSize: '11px', fontWeight: 500, color: 'var(--color-text-tertiary)', letterSpacing: '0.04em' }}>
            {wd}
          </div>
        ))}
      </div>
      {/* Day cells */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', flex: 1, overflow: 'auto' }}>
        {visibleDays.map((day, idx) => {
          const isCurrentMonth = day.getMonth() === month;
          const isToday = isSameDay(day, today);
          const dayKey = toDateKey(day);
          const dayEvents = eventsByDay.get(dayKey) ?? [];
          const dayTodos = todosByDay.get(dayKey) ?? [];
          const maxItems = 3;
          const overflow = dayEvents.length + dayTodos.length - maxItems;

          return (
            <div
              key={idx}
              onClick={(e) => onCellClick(day, e.currentTarget.getBoundingClientRect())}
              style={{
                borderRight: (idx % 7) < 6 ? '1px solid var(--color-border-subtle)' : undefined,
                borderBottom: '1px solid var(--color-border-subtle)',
                padding: '4px 6px 6px',
                minHeight: '120px',
                background: isCurrentMonth ? 'var(--color-bg-primary)' : 'var(--color-bg-secondary)',
                cursor: 'pointer',
                overflow: 'hidden',
              }}
            >
              {/* Day number — click to navigate to day view */}
              <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '2px', paddingTop: '3px' }}>
                <span
                  onClick={(e) => { e.stopPropagation(); onDayClick(day); }}
                  style={{
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    width: '28px', height: '28px', borderRadius: '50%',
                    fontSize: '13px', fontWeight: isToday ? 700 : 400,
                    color: isToday ? '#fff' : isCurrentMonth ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)',
                    background: isToday ? 'var(--color-accent)' : undefined,
                    cursor: 'pointer',
                    transition: 'background 150ms',
                  }}
                >
                  {day.getDate()}
                </span>
              </div>
              {/* Events: all-day = colored pill, timed = dot + title */}
              {dayEvents.slice(0, maxItems).map((ev) => (
                <div
                  key={ev.obj.ID}
                  onClick={(e) => { e.stopPropagation(); onEventClick(ev, e.currentTarget.getBoundingClientRect()); }}
                  title={ev.summary}
                  style={{
                    display: 'flex', alignItems: 'center', gap: '4px',
                    fontSize: '11px', marginBottom: '2px', overflow: 'hidden', cursor: 'pointer',
                    padding: ev.allDay ? '2px 5px' : '1px 3px',
                    borderRadius: ev.allDay ? '3px' : '2px',
                    background: ev.allDay ? ev.color : 'transparent',
                    color: ev.allDay ? '#fff' : 'var(--color-text-primary)',
                    fontWeight: ev.allDay ? 500 : 400,
                  }}
                >
                  {!ev.allDay && (
                    <span style={{ display: 'inline-block', width: '7px', height: '7px', borderRadius: '50%', background: ev.color, flexShrink: 0 }} />
                  )}
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{ev.summary}</span>
                </div>
              ))}
              {/* Todos */}
              {dayTodos.slice(0, Math.max(0, maxItems - dayEvents.length)).map((todo) => (
                <div
                  key={todo.obj.ID}
                  onClick={(e) => { e.stopPropagation(); onTodoToggle(todo); }}
                  title={todo.summary}
                  style={{
                    display: 'flex', alignItems: 'center', gap: '3px',
                    fontSize: '11px', padding: '1px 3px', marginBottom: '1px', cursor: 'pointer',
                    color: todo.completed ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)',
                    textDecoration: todo.completed ? 'line-through' : 'none',
                  }}
                >
                  <span style={{ color: todo.color, flexShrink: 0, fontSize: '12px', lineHeight: 1 }}>{todo.completed ? '☑' : '☐'}</span>
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{todo.summary}</span>
                </div>
              ))}
              {overflow > 0 && (
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', paddingLeft: '2px', fontWeight: 500 }}>
                  {t('moreEvents', { count: overflow })}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
