// Calendar event parsing logic
// Extracted from CalendarView.tsx to enable reuse

import { Calendar, CalendarObject, parseICS, icalDateToDate, parseVTODOICS } from '@/lib/api';

export interface ParsedEvent {
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

export interface ParsedTodo {
  obj: CalendarObject;
  summary: string;
  description: string;
  dueDate: Date | null;
  completedDate: Date | null;
  completed: boolean;
  calendarId: string;
  color: string;
}

type TFn = (key: string) => string;

export function parseEvents(objects: CalendarObject[], calendars: Calendar[], t?: TFn): ParsedEvent[] {
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
      ? ics.allDay
        ? new Date(endRaw.getTime() - 1)
        : endRaw
      : new Date(start.getTime() + 60 * 60 * 1000);

    const cal = calMap.get(obj.CalendarID);

    events.push({
      obj,
      summary: ics.summary || obj.UID || (t ? t('misc.calendarParser.noTitle') : '(제목 없음)'),
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

export function parseTodos(objects: CalendarObject[], calendars: Calendar[], t?: TFn): ParsedTodo[] {
  const calMap = new Map(calendars.map((c) => [c.ID, c]));
  const todos: ParsedTodo[] = [];

  for (const obj of objects) {
    if (!obj.ICS) continue;

    const ics = parseVTODOICS(obj.ICS);
    if (!ics) continue;

    const dueDate = ics.due ? icalDateToDate(ics.due) : null;
    const cal = calMap.get(obj.CalendarID);

    todos.push({
      obj,
      summary: ics.summary || obj.UID || (t ? t('misc.calendarParser.noTitle') : '(제목 없음)'),
      description: ics.description,
      dueDate,
      completedDate: null,
      completed: ics.status === 'COMPLETED',
      calendarId: obj.CalendarID,
      color: cal?.Color || 'var(--color-accent)',
    });
  }

  return todos;
}

export function filterEventsByDay(events: ParsedEvent[], day: Date): ParsedEvent[] {
  return events.filter((e) => {
    if (e.allDay) {
      const dayStart = new Date(day.getFullYear(), day.getMonth(), day.getDate());
      const dayEnd = new Date(dayStart.getTime() + 24 * 60 * 60 * 1000 - 1);
      return e.start <= dayEnd && e.end >= dayStart;
    }
    return (
      e.start.getFullYear() === day.getFullYear() &&
      e.start.getMonth() === day.getMonth() &&
      e.start.getDate() === day.getDate()
    );
  });
}

export function filterTodosByDay(todos: ParsedTodo[], day: Date): ParsedTodo[] {
  return todos.filter((t) => {
    if (!t.dueDate) return false;
    return (
      t.dueDate.getFullYear() === day.getFullYear() &&
      t.dueDate.getMonth() === day.getMonth() &&
      t.dueDate.getDate() === day.getDate()
    );
  });
}
