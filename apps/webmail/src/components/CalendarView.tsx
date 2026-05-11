'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { Calendar, CalendarObject, listCalendars, listCalendarObjects, parseICS, icalDateToDate, createCalendarEvent } from '@/lib/api';

// ── helpers ──────────────────────────────────────────────────────────────────

function startOfWeek(d: Date): Date {
  const copy = new Date(d);
  const day = copy.getDay(); // 0=Sun
  const diff = day === 0 ? -6 : 1 - day; // Mon-based
  copy.setDate(copy.getDate() + diff);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

function isSameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

function addDays(d: Date, n: number): Date {
  const c = new Date(d);
  c.setDate(c.getDate() + n);
  return c;
}

function formatDate(d: Date): string {
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월 ${d.getDate()}일`;
}

function formatMonthYear(d: Date): string {
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월`;
}

function formatWeekRange(d: Date): string {
  const mon = startOfWeek(d);
  const sun = addDays(mon, 6);
  if (mon.getMonth() === sun.getMonth()) {
    return `${mon.getFullYear()}년 ${mon.getMonth() + 1}월 ${mon.getDate()}일 – ${sun.getDate()}일`;
  }
  return `${mon.getFullYear()}년 ${mon.getMonth() + 1}월 ${mon.getDate()}일 – ${sun.getMonth() + 1}월 ${sun.getDate()}일`;
}

function formatHour(h: number): string {
  return `${String(h).padStart(2, '0')}:00`;
}

function formatTime(d: Date): string {
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

// ── parsed event ─────────────────────────────────────────────────────────────

interface ParsedEvent {
  obj: CalendarObject;
  summary: string;
  description: string;
  location: string;
  start: Date;
  end: Date;
  allDay: boolean;
  calendarId: string;
  color: string;
}

function parseEvents(objects: CalendarObject[], calendars: Calendar[]): ParsedEvent[] {
  const calMap = new Map(calendars.map((c) => [c.ID, c]));
  const events: ParsedEvent[] = [];
  for (const obj of objects) {
    if (!obj.ICS) continue;
    const ics = parseICS(obj.ICS);
    const start = icalDateToDate(ics.dtstart);
    if (!start) continue;
    const endRaw = icalDateToDate(ics.dtend);
    // For all-day events, dtend is exclusive — subtract 1ms to stay on the same day
    const end = endRaw
      ? ics.allDay ? new Date(endRaw.getTime() - 1) : endRaw
      : new Date(start.getTime() + 60 * 60 * 1000);
    const cal = calMap.get(obj.CalendarID);
    events.push({
      obj,
      summary: ics.summary || obj.UID || '(제목 없음)',
      description: ics.description,
      location: ics.location,
      start,
      end,
      allDay: ics.allDay,
      calendarId: obj.CalendarID,
      color: cal?.Color || 'var(--color-accent)',
    });
  }
  return events;
}

// ── EventPopover ─────────────────────────────────────────────────────────────

interface EventPopoverProps {
  event: ParsedEvent;
  anchorRect: DOMRect;
  onClose: () => void;
}

function EventPopover({ event, anchorRect, onClose }: EventPopoverProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  const style: React.CSSProperties = {
    position: 'fixed',
    zIndex: 300,
    background: 'var(--color-bg-primary)',
    border: '1px solid var(--color-border-default)',
    borderRadius: '8px',
    boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
    padding: '16px',
    minWidth: '220px',
    maxWidth: '320px',
    top: Math.min(anchorRect.bottom + 6, window.innerHeight - 220),
    left: Math.min(anchorRect.left, window.innerWidth - 340),
  };

  return (
    <div ref={ref} style={style}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: 0 }}>
          <div style={{ width: '10px', height: '10px', borderRadius: '50%', background: event.color, flexShrink: 0 }} />
          <span style={{ fontWeight: 600, fontSize: '14px', color: 'var(--color-text-primary)', wordBreak: 'break-word' }}>
            {event.summary}
          </span>
        </div>
        <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '16px', lineHeight: 1, padding: '0 2px', flexShrink: 0 }}>×</button>
      </div>
      <div style={{ marginTop: '10px', fontSize: '13px', color: 'var(--color-text-secondary)', display: 'flex', flexDirection: 'column', gap: '6px' }}>
        {event.allDay ? (
          <div>{formatDate(event.start)}</div>
        ) : (
          <div>{formatDate(event.start)} {formatTime(event.start)} – {formatTime(event.end)}</div>
        )}
        {event.location && <div>📍 {event.location}</div>}
        {event.description && (
          <div style={{ borderTop: '1px solid var(--color-border-subtle)', paddingTop: '8px', marginTop: '2px', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
            {event.description}
          </div>
        )}
      </div>
    </div>
  );
}

// ── MonthView ────────────────────────────────────────────────────────────────

interface MonthViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  today: Date;
  onDayClick: (d: Date) => void;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
}

function MonthView({ currentDate, events, today, onDayClick, onEventClick }: MonthViewProps) {
  const month = currentDate.getMonth();
  const firstDay = startOfMonth(currentDate);
  // Grid starts on Monday
  const gridStart = startOfWeek(firstDay);
  // Total cells: enough to cover the month (up to 6 weeks)
  const totalCells = 42;
  const days: Date[] = [];
  for (let i = 0; i < totalCells; i++) {
    days.push(addDays(gridStart, i));
  }
  // Trim trailing weeks if not needed (stop at 5 weeks if last cell is far into next month)
  const needed = days.findLastIndex((d) => d.getMonth() === month || d <= firstDay) + 1;
  const cellCount = Math.ceil(Math.max(needed, 28) / 7) * 7;
  const visibleDays = days.slice(0, cellCount);

  const weekDays = ['월', '화', '수', '목', '금', '토', '일'];
  const isWeekend = (d: Date) => d.getDay() === 0 || d.getDay() === 6;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
      {/* Day header row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
        {weekDays.map((wd, i) => (
          <div
            key={wd}
            style={{
              padding: '6px 8px',
              textAlign: 'center',
              fontSize: '12px',
              fontWeight: 600,
              color: i >= 5 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)',
              background: i >= 5 ? 'var(--color-bg-secondary)' : undefined,
              borderRight: i < 6 ? '1px solid var(--color-border-subtle)' : undefined,
            }}
          >
            {wd}
          </div>
        ))}
      </div>
      {/* Day cells */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', flex: 1, overflow: 'auto' }}>
        {visibleDays.map((day, idx) => {
          const isCurrentMonth = day.getMonth() === month;
          const isToday = isSameDay(day, today);
          const weekend = isWeekend(day);
          const dayEvents = events.filter((ev) => {
            const s = new Date(ev.start); s.setHours(0, 0, 0, 0);
            const e = new Date(ev.end); e.setHours(23, 59, 59, 999);
            const d = new Date(day); d.setHours(12, 0, 0, 0);
            return d >= s && d <= e;
          });

          return (
            <div
              key={idx}
              onClick={() => onDayClick(day)}
              style={{
                borderRight: (idx % 7) < 6 ? '1px solid var(--color-border-subtle)' : undefined,
                borderBottom: '1px solid var(--color-border-subtle)',
                padding: '4px',
                minHeight: '90px',
                background: weekend ? 'var(--color-bg-secondary)' : 'var(--color-bg-primary)',
                cursor: 'pointer',
                overflow: 'hidden',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px' }}>
                <span
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '24px',
                    height: '24px',
                    borderRadius: '50%',
                    fontSize: '12px',
                    fontWeight: isToday ? 700 : 400,
                    color: isToday
                      ? '#fff'
                      : isCurrentMonth
                      ? 'var(--color-text-primary)'
                      : 'var(--color-text-tertiary)',
                    background: isToday ? 'var(--color-accent)' : undefined,
                  }}
                >
                  {day.getDate()}
                </span>
              </div>
              {dayEvents.slice(0, 3).map((ev) => (
                <div
                  key={ev.obj.ID}
                  onClick={(e) => { e.stopPropagation(); onEventClick(ev, e.currentTarget.getBoundingClientRect()); }}
                  title={ev.summary}
                  style={{
                    background: ev.color,
                    color: '#fff',
                    fontSize: '11px',
                    padding: '1px 4px',
                    borderRadius: '3px',
                    marginBottom: '2px',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    cursor: 'pointer',
                  }}
                >
                  {!ev.allDay && <span style={{ marginRight: '3px', opacity: 0.85 }}>{formatTime(ev.start)}</span>}
                  {ev.summary}
                </div>
              ))}
              {dayEvents.length > 3 && (
                <div style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', paddingLeft: '2px' }}>
                  +{dayEvents.length - 3}개 더
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── WeekView ─────────────────────────────────────────────────────────────────

interface WeekViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  today: Date;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
}

const HOUR_HEIGHT = 48;
const HOURS = Array.from({ length: 24 }, (_, i) => i);

function WeekView({ currentDate, events, today, onEventClick }: WeekViewProps) {
  const mon = startOfWeek(currentDate);
  const days = Array.from({ length: 7 }, (_, i) => addDays(mon, i));
  const weekDayLabels = ['월', '화', '수', '목', '금', '토', '일'];
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

// ── DayView ──────────────────────────────────────────────────────────────────

interface DayViewProps {
  currentDate: Date;
  events: ParsedEvent[];
  today: Date;
  onEventClick: (e: ParsedEvent, rect: DOMRect) => void;
}

function DayView({ currentDate, events, today, onEventClick }: DayViewProps) {
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
          {['일', '월', '화', '수', '목', '금', '토'][currentDate.getDay()]}요일
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

// ── CalendarView (main) ───────────────────────────────────────────────────────

export function CalendarView() {
  const [view, setView] = useState<'month' | 'week' | 'day'>('month');
  const [currentDate, setCurrentDate] = useState<Date>(() => {
    const d = new Date(); d.setHours(0, 0, 0, 0); return d;
  });
  const today = useRef<Date>((() => { const d = new Date(); d.setHours(0, 0, 0, 0); return d; })()).current;

  const [calendars, setCalendars] = useState<Calendar[]>([]);
  const [objects, setObjects] = useState<CalendarObject[]>([]);
  const [selectedCalIds, setSelectedCalIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);

  const [popover, setPopover] = useState<{ event: ParsedEvent; rect: DOMRect } | null>(null);

  // Event creation form
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createTitle, setCreateTitle] = useState('');
  const [createStart, setCreateStart] = useState('');
  const [createEnd, setCreateEnd] = useState('');
  const [createAllDay, setCreateAllDay] = useState(false);
  const [createLocation, setCreateLocation] = useState('');
  const [createDesc, setCreateDesc] = useState('');
  const [createCalId, setCreateCalId] = useState('');
  const [createSaving, setCreateSaving] = useState(false);
  const [createError, setCreateError] = useState('');

  // Load calendars on mount
  useEffect(() => {
    let cancelled = false;
    listCalendars().then((cals) => {
      if (cancelled) return;
      setCalendars(cals);
      setSelectedCalIds(new Set(cals.map((c) => c.ID)));
    });
    return () => { cancelled = true; };
  }, []);

  // Load objects when calendars change
  useEffect(() => {
    if (calendars.length === 0) { setLoading(false); return; }
    let cancelled = false;
    setLoading(true);
    Promise.all(calendars.map((c) => listCalendarObjects(c.ID)))
      .then((results) => {
        if (cancelled) return;
        setObjects(results.flat());
        setLoading(false);
      })
      .catch(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [calendars]);

  // Derived: parse + filter events
  const allEvents = parseEvents(objects, calendars);
  const events = allEvents.filter((ev) => selectedCalIds.has(ev.calendarId));

  // Navigation
  const navigate = useCallback((delta: number) => {
    setCurrentDate((d) => {
      const c = new Date(d);
      if (view === 'month') { c.setMonth(c.getMonth() + delta); c.setDate(1); }
      else if (view === 'week') c.setDate(c.getDate() + delta * 7);
      else c.setDate(c.getDate() + delta);
      return c;
    });
  }, [view]);

  const goToday = useCallback(() => {
    const d = new Date(); d.setHours(0, 0, 0, 0); setCurrentDate(d);
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;
      if (popover || showCreateModal) {
        if (e.key === 'Escape') { setPopover(null); setShowCreateModal(false); }
        return;
      }
      switch (e.key) {
        case 'd': setView('day'); break;
        case 'w': setView('week'); break;
        case 'm': setView('month'); break;
        case 't': goToday(); break;
        case 'ArrowLeft': navigate(-1); break;
        case 'ArrowRight': navigate(1); break;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [navigate, goToday, popover, showCreateModal]);

  // Title
  let title = '';
  if (view === 'month') title = formatMonthYear(currentDate);
  else if (view === 'week') title = formatWeekRange(currentDate);
  else title = formatDate(currentDate);

  const pad2Local = (n: number) => String(n).padStart(2, '0');
  const toLocalDT = (d: Date) =>
    `${d.getFullYear()}-${pad2Local(d.getMonth() + 1)}-${pad2Local(d.getDate())}T${pad2Local(d.getHours())}:${pad2Local(d.getMinutes())}`;
  const toLocalDate = (d: Date) =>
    `${d.getFullYear()}-${pad2Local(d.getMonth() + 1)}-${pad2Local(d.getDate())}`;

  const openCreateModal = (baseDate?: Date) => {
    const base = baseDate ?? currentDate;
    const start = new Date(base); start.setHours(9, 0, 0, 0);
    const end = new Date(base); end.setHours(10, 0, 0, 0);
    setCreateTitle(''); setCreateLocation(''); setCreateDesc(''); setCreateError('');
    setCreateAllDay(false);
    setCreateStart(toLocalDT(start));
    setCreateEnd(toLocalDT(end));
    setCreateCalId(calendars[0]?.ID ?? '');
    setShowCreateModal(true);
  };

  const handleCreateSubmit = async () => {
    if (!createTitle.trim()) { setCreateError('제목을 입력하세요'); return; }
    if (!createCalId) { setCreateError('캘린더를 선택하세요'); return; }
    const startDate = new Date(createAllDay ? createStart + 'T00:00:00' : createStart);
    const endDate = new Date(createAllDay ? createEnd + 'T00:00:00' : createEnd);
    if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) { setCreateError('날짜를 확인하세요'); return; }
    if (endDate <= startDate) { setCreateError('종료 시간이 시작 시간보다 늦어야 합니다'); return; }
    setCreateSaving(true); setCreateError('');
    try {
      await createCalendarEvent(createCalId, {
        title: createTitle.trim(),
        start: startDate, end: endDate, allDay: createAllDay,
        location: createLocation.trim() || undefined,
        description: createDesc.trim() || undefined,
      });
      setShowCreateModal(false);
      // Reload calendar objects
      const allObjects: CalendarObject[] = [];
      await Promise.all(calendars.map(async (cal) => {
        const objs = await import('@/lib/api').then(m => m.listCalendarObjects(cal.ID ?? ''));
        allObjects.push(...objs);
      }));
      setObjects(allObjects);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : '저장 실패');
    } finally {
      setCreateSaving(false);
    }
  };

  const toggleCalendar = (id: string) => {
    setSelectedCalIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const handleEventClick = (ev: ParsedEvent, rect: DOMRect) => {
    setPopover({ event: ev, rect });
  };

  const handleDayClick = (d: Date) => {
    setCurrentDate(d);
    setView('day');
  };

  return (
    <div style={{ display: 'flex', flex: 1, overflow: 'hidden', background: 'var(--color-bg-primary)' }}>
      {/* Left sidebar */}
      <div
        style={{
          width: '180px',
          flexShrink: 0,
          borderRight: '1px solid var(--color-border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          padding: '12px 8px',
          gap: '4px',
          overflowY: 'auto',
          background: 'var(--color-bg-secondary)',
        }}
      >
        <button
          onClick={() => openCreateModal(currentDate)}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            padding: '7px 10px',
            borderRadius: '6px',
            border: '1px dashed var(--color-border-default)',
            background: 'none',
            color: 'var(--color-text-secondary)',
            fontSize: '12px',
            cursor: 'pointer',
            marginBottom: '8px',
            width: '100%',
          }}
        >
          <span style={{ fontSize: '16px', lineHeight: 1 }}>+</span> 새 캘린더
        </button>

        <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '4px 6px 2px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
          내 캘린더
        </div>

        {loading && calendars.length === 0 && (
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '6px' }}>로딩 중...</div>
        )}

        {calendars.length === 0 && !loading && (
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '6px' }}>캘린더 없음</div>
        )}

        {calendars.map((cal) => {
          const checked = selectedCalIds.has(cal.ID);
          return (
            <label
              key={cal.ID}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                padding: '5px 6px',
                borderRadius: '5px',
                cursor: 'pointer',
                fontSize: '13px',
                color: 'var(--color-text-primary)',
              }}
            >
              <span
                onClick={() => toggleCalendar(cal.ID)}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: '14px',
                  height: '14px',
                  borderRadius: '3px',
                  border: `2px solid ${cal.Color || 'var(--color-accent)'}`,
                  background: checked ? (cal.Color || 'var(--color-accent)') : 'transparent',
                  cursor: 'pointer',
                  flexShrink: 0,
                }}
              >
                {checked && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
              </span>
              <span
                onClick={() => toggleCalendar(cal.ID)}
                style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}
                title={cal.Name}
              >
                {cal.Name}
              </span>
            </label>
          );
        })}
      </div>

      {/* Main calendar area */}
      <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
        {/* Toolbar */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            padding: '8px 12px',
            borderBottom: '1px solid var(--color-border-subtle)',
            gap: '8px',
            flexShrink: 0,
            background: 'var(--color-bg-primary)',
          }}
        >
          {/* Nav buttons */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <button
              onClick={() => navigate(-1)}
              aria-label="이전"
              style={{
                padding: '5px 8px',
                borderRadius: '5px',
                border: '1px solid var(--color-border-default)',
                background: 'none',
                color: 'var(--color-text-primary)',
                cursor: 'pointer',
                fontSize: '14px',
                lineHeight: 1,
              }}
            >
              ‹
            </button>
            <button
              onClick={goToday}
              style={{
                padding: '5px 10px',
                borderRadius: '5px',
                border: '1px solid var(--color-border-default)',
                background: 'none',
                color: 'var(--color-text-primary)',
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: 500,
              }}
            >
              오늘
            </button>
            <button
              onClick={() => navigate(1)}
              aria-label="다음"
              style={{
                padding: '5px 8px',
                borderRadius: '5px',
                border: '1px solid var(--color-border-default)',
                background: 'none',
                color: 'var(--color-text-primary)',
                cursor: 'pointer',
                fontSize: '14px',
                lineHeight: 1,
              }}
            >
              ›
            </button>
          </div>

          {/* Title */}
          <div style={{ flex: 1, fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', paddingLeft: '4px' }}>
            {title}
          </div>

          {/* + new event */}
          <button
            onClick={() => openCreateModal(currentDate)}
            style={{
              padding: '5px 12px',
              borderRadius: '5px',
              border: 'none',
              background: 'var(--color-accent)',
              color: '#fff',
              cursor: 'pointer',
              fontSize: '12px',
              fontWeight: 500,
            }}
          >
            + 새 일정
          </button>

          {/* View toggle */}
          <div style={{ display: 'flex', borderRadius: '6px', border: '1px solid var(--color-border-default)', overflow: 'hidden' }}>
            {(['day', 'week', 'month'] as const).map((v) => {
              const labels = { day: '일', week: '주', month: '월' };
              return (
                <button
                  key={v}
                  onClick={() => setView(v)}
                  style={{
                    padding: '5px 10px',
                    border: 'none',
                    borderRight: v !== 'month' ? '1px solid var(--color-border-default)' : 'none',
                    background: view === v ? 'var(--color-accent)' : 'none',
                    color: view === v ? '#fff' : 'var(--color-text-primary)',
                    cursor: 'pointer',
                    fontSize: '12px',
                    fontWeight: 500,
                  }}
                >
                  {labels[v]}
                </button>
              );
            })}
          </div>
        </div>

        {/* Calendar body */}
        {loading && objects.length === 0 ? (
          <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-tertiary)', fontSize: '14px' }}>
            로딩 중...
          </div>
        ) : view === 'month' ? (
          <MonthView
            currentDate={currentDate}
            events={events}
            today={today}
            onDayClick={handleDayClick}
            onEventClick={handleEventClick}
          />
        ) : view === 'week' ? (
          <WeekView
            currentDate={currentDate}
            events={events}
            today={today}
            onEventClick={handleEventClick}
          />
        ) : (
          <DayView
            currentDate={currentDate}
            events={events}
            today={today}
            onEventClick={handleEventClick}
          />
        )}
      </div>

      {/* Event popover */}
      {popover && (
        <EventPopover
          event={popover.event}
          anchorRect={popover.rect}
          onClose={() => setPopover(null)}
        />
      )}

      {/* Event creation modal */}
      {showCreateModal && (
        <div
          style={{ position: 'fixed', inset: 0, zIndex: 400, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.45)' }}
          onClick={() => !createSaving && setShowCreateModal(false)}
        >
          <div onClick={(e) => e.stopPropagation()} style={{ background: 'var(--color-bg-primary)', borderRadius: '12px', padding: '24px 28px', width: '440px', maxWidth: '95vw', boxShadow: '0 20px 60px rgba(0,0,0,0.28)', display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--color-text-primary)' }}>새 일정</div>

            {/* Title */}
            <input autoFocus type="text" placeholder="제목 (필수)" value={createTitle} onChange={(e) => setCreateTitle(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleCreateSubmit(); }}
              style={{ width: '100%', padding: '8px 10px', fontSize: '14px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' }} />

            {/* Calendar */}
            {calendars.length > 1 && (
              <select value={createCalId} onChange={(e) => setCreateCalId(e.target.value)}
                style={{ padding: '7px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                {calendars.map((c) => <option key={c.ID} value={c.ID ?? ''}>{c.Name ?? '(캘린더)'}</option>)}
              </select>
            )}

            {/* All day */}
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
              <input type="checkbox" checked={createAllDay} onChange={(e) => {
                const allDay = e.target.checked;
                setCreateAllDay(allDay);
                if (allDay) {
                  setCreateStart(createStart.split('T')[0] || toLocalDate(new Date()));
                  setCreateEnd(createEnd.split('T')[0] || toLocalDate(new Date()));
                } else {
                  setCreateStart((createStart.includes('T') ? createStart : createStart + 'T09:00'));
                  setCreateEnd((createEnd.includes('T') ? createEnd : createEnd + 'T10:00'));
                }
              }} />
              하루 종일
            </label>

            {/* Start / End */}
            <div style={{ display: 'flex', gap: '10px' }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '4px' }}>시작</div>
                <input type={createAllDay ? 'date' : 'datetime-local'} value={createStart} onChange={(e) => setCreateStart(e.target.value)}
                  style={{ width: '100%', padding: '7px 8px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '4px' }}>종료</div>
                <input type={createAllDay ? 'date' : 'datetime-local'} value={createEnd} onChange={(e) => setCreateEnd(e.target.value)}
                  style={{ width: '100%', padding: '7px 8px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }} />
              </div>
            </div>

            {/* Location */}
            <input type="text" placeholder="장소 (선택)" value={createLocation} onChange={(e) => setCreateLocation(e.target.value)}
              style={{ width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' }} />

            {/* Description */}
            <textarea placeholder="메모 (선택)" value={createDesc} onChange={(e) => setCreateDesc(e.target.value)} rows={2}
              style={{ width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', resize: 'none', boxSizing: 'border-box', fontFamily: 'inherit' }} />

            {/* Error */}
            {createError && <div style={{ fontSize: '12px', color: '#e53e3e' }}>{createError}</div>}

            {/* Actions */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '4px' }}>
              <button onClick={() => setShowCreateModal(false)} disabled={createSaving}
                style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'none', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>
                취소
              </button>
              <button onClick={handleCreateSubmit} disabled={createSaving}
                style={{ padding: '8px 20px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: createSaving ? 'wait' : 'pointer', opacity: createSaving ? 0.7 : 1 }}>
                {createSaving ? '저장 중...' : '저장'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
