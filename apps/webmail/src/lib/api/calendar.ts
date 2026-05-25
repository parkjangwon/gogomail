import { calendarUID } from '../stableId';
import { request } from './http';

export interface Calendar {
  ID: string;
  UserID: string;
  Name: string;
  Color: string;
  Description: string;
  SyncToken: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CalendarObject {
  ID: string;
  UserID: string;
  CalendarID: string;
  ObjectName: string;
  UID: string;
  Component: string;
  ETag: string;
  Size: number;
  ICS: string; // base64-encoded iCalendar bytes
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CalendarSubscription {
  id: string;
  name: string;
  url: string;
  color: string;
}

export interface CreateCalendarEventRequest {
  title: string;
  start: Date;
  end: Date;
  allDay: boolean;
  location?: string;
  description?: string;
  rrule?: string;
}

export interface ParsedVTODOFields {
  summary: string;
  description: string;
  due: string;
  status: string;
}

export interface CreateTodoRequest {
  title: string;
  due?: Date;
  calendarId: string;
}

/** Decode a base64 string as UTF-8 text (handles multi-byte characters). */
function base64ToUTF8(b64: string): string {
  try {
    const binStr = atob(b64);
    const bytes = new Uint8Array(binStr.length);
    for (let i = 0; i < binStr.length; i++) {
      bytes[i] = binStr.charCodeAt(i);
    }
    return new TextDecoder('utf-8').decode(bytes);
  } catch {
    return b64;
  }
}

/** Parse key iCal fields from base64-encoded ICS data. */
export function parseICS(base64ICS: string): {
  summary: string;
  description: string;
  location: string;
  dtstart: string;
  dtend: string;
  allDay: boolean;
} {
  let text = '';
  try { text = base64ToUTF8(base64ICS); } catch { text = base64ICS; }

  // Unfold long lines (RFC 5545 line folding: CRLF + whitespace)
  text = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');

  const get = (prop: string): string => {
    const m = text.match(new RegExp(`(?:^|\\n)${prop}(?:;[^\\n:]*)?:([^\\n]*)`, 'im'));
    return m ? m[1].trim() : '';
  };

  const dtstart = get('DTSTART');
  const dtend = get('DTEND');
  // All-day events use DATE format (8 digits, no T)
  const allDay = /^\d{8}$/.test(dtstart);

  return {
    summary: get('SUMMARY'),
    description: get('DESCRIPTION'),
    location: get('LOCATION'),
    dtstart,
    dtend,
    allDay,
  };
}

/** Convert iCal date/datetime string to JS Date. */
export function icalDateToDate(dtStr: string): Date | null {
  if (!dtStr) return null;
  // DATE format: YYYYMMDD
  if (/^\d{8}$/.test(dtStr)) {
    const y = parseInt(dtStr.slice(0, 4), 10);
    const mo = parseInt(dtStr.slice(4, 6), 10) - 1;
    const d = parseInt(dtStr.slice(6, 8), 10);
    return new Date(y, mo, d);
  }
  // DATETIME format: YYYYMMDDTHHmmss[Z]
  const m = dtStr.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z?)$/);
  if (m) {
    const [, y, mo, d, h, min, s, z] = m;
    if (z === 'Z') {
      return new Date(Date.UTC(+y, +mo - 1, +d, +h, +min, +s));
    }
    return new Date(+y, +mo - 1, +d, +h, +min, +s);
  }
  return null;
}

export function parseVTODOICS(base64ICS: string): ParsedVTODOFields {
  let text = '';
  try { text = base64ToUTF8(base64ICS); } catch { text = base64ICS; }
  text = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');
  const get = (prop: string): string => {
    const m = text.match(new RegExp(`(?:^|\\n)${prop}(?:;[^\\n:]*)?:([^\\n]*)`, 'im'));
    return m ? m[1].trim() : '';
  };
  return {
    summary: get('SUMMARY'),
    description: get('DESCRIPTION'),
    due: get('DUE'),
    status: get('STATUS') || 'NEEDS-ACTION',
  };
}

export async function listCalendars(): Promise<Calendar[]> {
  try {
    const data = await request<{ calendars?: Calendar[] }>('calendars');
    return data.calendars ?? [];
  } catch { return []; }
}

export async function listCalendarObjects(calendarId: string): Promise<CalendarObject[]> {
  try {
    const data = await request<{ objects?: CalendarObject[] }>(`calendars/${encodeURIComponent(calendarId)}/objects`);
    return data.objects ?? [];
  } catch { return []; }
}

export async function createCalendar(name: string, color: string, description = ''): Promise<Calendar> {
  const data = await request<{ calendar: Calendar }>('calendars', {
    method: 'POST',
    body: JSON.stringify({ name, color, description }),
  });
  return data.calendar;
}

export async function updateCalendar(id: string, patch: { name?: string; color?: string; description?: string }): Promise<void> {
  await request<unknown>(`calendars/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function deleteCalendar(id: string): Promise<void> {
  await request<unknown>(`calendars/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

function pad2(n: number): string { return String(n).padStart(2, '0'); }
function toICSDate(d: Date): string {
  return `${d.getUTCFullYear()}${pad2(d.getUTCMonth() + 1)}${pad2(d.getUTCDate())}T${pad2(d.getUTCHours())}${pad2(d.getUTCMinutes())}${pad2(d.getUTCSeconds())}Z`;
}
function toICSAllDay(d: Date): string {
  return `${d.getFullYear()}${pad2(d.getMonth() + 1)}${pad2(d.getDate())}`;
}
function icsEscape(s: string): string { return s.replace(/\\/g, '\\\\').replace(/;/g, '\\;').replace(/,/g, '\\,').replace(/\n/g, '\\n'); }

export async function createCalendarEvent(calendarId: string, req: CreateCalendarEventRequest): Promise<void> {
  const uid = calendarUID();
  const objectName = `${uid}.ics`;
  const lines: string[] = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VEVENT',
    `UID:${uid}`,
    `SUMMARY:${icsEscape(req.title)}`,
  ];
  if (req.allDay) {
    lines.push(`DTSTART;VALUE=DATE:${toICSAllDay(req.start)}`);
    const endDate = new Date(req.end);
    endDate.setDate(endDate.getDate() + 1);
    lines.push(`DTEND;VALUE=DATE:${toICSAllDay(endDate)}`);
  } else {
    lines.push(`DTSTART:${toICSDate(req.start)}`);
    lines.push(`DTEND:${toICSDate(req.end)}`);
  }
  if (req.location) lines.push(`LOCATION:${icsEscape(req.location)}`);
  if (req.description) lines.push(`DESCRIPTION:${icsEscape(req.description)}`);
  if (req.rrule) lines.push(`RRULE:${req.rrule}`);
  lines.push('END:VEVENT', 'END:VCALENDAR');
  const ics = lines.join('\r\n');
  await request<unknown>(`calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(objectName)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/calendar' },
    body: ics,
  });
}

export async function updateCalendarEvent(calendarId: string, objectName: string, uid: string, req: CreateCalendarEventRequest): Promise<void> {
  const lines: string[] = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VEVENT',
    `UID:${uid}`,
    `SUMMARY:${icsEscape(req.title)}`,
  ];
  if (req.allDay) {
    lines.push(`DTSTART;VALUE=DATE:${toICSAllDay(req.start)}`);
    const endDate = new Date(req.end);
    endDate.setDate(endDate.getDate() + 1);
    lines.push(`DTEND;VALUE=DATE:${toICSAllDay(endDate)}`);
  } else {
    lines.push(`DTSTART:${toICSDate(req.start)}`);
    lines.push(`DTEND:${toICSDate(req.end)}`);
  }
  if (req.location) lines.push(`LOCATION:${icsEscape(req.location)}`);
  if (req.description) lines.push(`DESCRIPTION:${icsEscape(req.description)}`);
  if (req.rrule) lines.push(`RRULE:${req.rrule}`);
  lines.push('END:VEVENT', 'END:VCALENDAR');
  await request<unknown>(`calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(objectName)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/calendar' },
    body: lines.join('\r\n'),
  });
}

export async function createCalendarTodo(req: CreateTodoRequest): Promise<void> {
  const uid = calendarUID();
  const objectName = `${uid}.ics`;
  const lines: string[] = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VTODO',
    `UID:${uid}`,
    `SUMMARY:${icsEscape(req.title)}`,
    'STATUS:NEEDS-ACTION',
  ];
  if (req.due) lines.push(`DUE;VALUE=DATE:${toICSAllDay(req.due)}`);
  lines.push('END:VTODO', 'END:VCALENDAR');
  await request<unknown>(
    `calendars/${encodeURIComponent(req.calendarId)}/objects/${encodeURIComponent(objectName)}`,
    { method: 'PUT', headers: { 'Content-Type': 'text/calendar' }, body: lines.join('\r\n') },
  );
}

export async function setTodoStatus(calendarId: string, obj: CalendarObject, completed: boolean): Promise<void> {
  const f = parseVTODOICS(obj.ICS);
  const lines: string[] = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VTODO',
    `UID:${obj.UID}`,
    `SUMMARY:${icsEscape(f.summary)}`,
    `STATUS:${completed ? 'COMPLETED' : 'NEEDS-ACTION'}`,
  ];
  if (f.due) lines.push(`DUE:${f.due}`);
  if (f.description) lines.push(`DESCRIPTION:${icsEscape(f.description)}`);
  lines.push('END:VTODO', 'END:VCALENDAR');
  await request<unknown>(
    `calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(obj.ObjectName)}`,
    { method: 'PUT', headers: { 'Content-Type': 'text/calendar' }, body: lines.join('\r\n') },
  );
}

export async function deleteCalendarObject(calendarId: string, objectName: string): Promise<void> {
  await request<unknown>(
    `calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(objectName)}`,
    { method: 'DELETE' },
  );
}

export async function listCalendarSubscriptions(): Promise<CalendarSubscription[]> {
  try {
    const data = await request<{ subscriptions?: CalendarSubscription[] }>('calendar-subscriptions');
    return data.subscriptions ?? [];
  } catch { return []; }
}

export async function addCalendarSubscription(
  url: string, name: string, color: string,
): Promise<CalendarSubscription> {
  const data = await request<{ subscription: CalendarSubscription }>('calendar-subscriptions', {
    method: 'POST',
    body: JSON.stringify({ url, name, color }),
  });
  return data.subscription;
}

export async function deleteCalendarSubscription(id: string): Promise<void> {
  await request<unknown>(`calendar-subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function fetchSubscriptionICS(id: string): Promise<string> {
  const res = await fetch(`/api/mail/calendar-subscriptions/${encodeURIComponent(id)}/events`);
  if (!res.ok) throw new Error(`Failed to fetch subscription events: ${res.status}`);
  return res.text();
}
