// Calendar date utility functions
// Extracted from CalendarView.tsx to enable reuse and testing

export function startOfWeek(d: Date): Date {
  const copy = new Date(d);
  const day = copy.getDay(); // 0=Sun
  const diff = day === 0 ? -6 : 1 - day; // Mon-based
  copy.setDate(copy.getDate() + diff);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

export function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

export function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function addDays(d: Date, n: number): Date {
  const c = new Date(d);
  c.setDate(c.getDate() + n);
  return c;
}

type TFn = (key: string, values?: Record<string, unknown>) => string;

export function formatDate(d: Date, t?: TFn): string {
  if (t) {
    return t('misc.calendarDate.ymdFormat', { year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate() });
  }
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월 ${d.getDate()}일`;
}

export function formatMonthYear(d: Date, t?: TFn): string {
  if (t) {
    return t('misc.calendarDate.ymFormat', { year: d.getFullYear(), month: d.getMonth() + 1 });
  }
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월`;
}

export function formatWeekRange(d: Date, t?: TFn): string {
  const mon = startOfWeek(d);
  const sun = addDays(mon, 6);
  if (mon.getMonth() === sun.getMonth()) {
    if (t) {
      return t('misc.calendarDate.weekSame', {
        year: mon.getFullYear(), month: mon.getMonth() + 1,
        startDay: mon.getDate(), endDay: sun.getDate(),
      });
    }
    return `${mon.getFullYear()}년 ${mon.getMonth() + 1}월 ${mon.getDate()}일 – ${sun.getDate()}일`;
  }
  if (t) {
    return t('misc.calendarDate.weekDifferent', {
      year: mon.getFullYear(),
      startMonth: mon.getMonth() + 1, startDay: mon.getDate(),
      endMonth: sun.getMonth() + 1, endDay: sun.getDate(),
    });
  }
  return `${mon.getFullYear()}년 ${mon.getMonth() + 1}월 ${mon.getDate()}일 – ${sun.getMonth() + 1}월 ${sun.getDate()}일`;
}

export function formatHour(h: number): string {
  return `${String(h).padStart(2, '0')}:00`;
}

export function formatTime(d: Date): string {
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}
