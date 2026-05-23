import { CalendarObject, CalendarSubscription, parseICS, icalDateToDate } from '@/lib/api';
import { ParsedEvent } from '@/lib/calendar/eventParser';

type TFn = (key: string) => string;

export function parseSubscriptionEvents(rawICS: string, sub: CalendarSubscription, t?: TFn): ParsedEvent[] {
  const events: ParsedEvent[] = [];
  const blocks = rawICS.split(/BEGIN:VEVENT/i).slice(1);

  for (const block of blocks) {
    const endIdx = block.search(/END:VEVENT/i);
    const eventBlock = 'BEGIN:VEVENT\n' + (endIdx >= 0 ? block.slice(0, endIdx) : block);
    const ics = parseICS(eventBlock);
    const start = icalDateToDate(ics.dtstart);
    if (!start) continue;

    const endRaw = icalDateToDate(ics.dtend);
    const end = endRaw
      ? ics.allDay ? new Date(endRaw.getTime() - 1) : endRaw
      : new Date(start.getTime() + 60 * 60 * 1000);

    events.push({
      obj: {
        ID: `${sub.id}_${ics.summary || start.toISOString()}`,
        UserID: '',
        CalendarID: sub.id,
        ObjectName: '',
        UID: '',
        Component: 'VEVENT',
        ETag: '',
        Size: 0,
        ICS: '',
        CreatedAt: '',
        UpdatedAt: '',
      } as unknown as CalendarObject,
      summary: ics.summary || (t ? t('misc.calendarParser.noTitle') : '(제목 없음)'),
      description: ics.description,
      location: ics.location,
      start,
      end,
      allDay: ics.allDay,
      calendarId: sub.id,
      color: sub.color,
    });
  }

  return events;
}

