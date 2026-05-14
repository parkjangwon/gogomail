'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { CalendarDaysIcon, CheckIcon, FolderPlusIcon, LinkIcon } from '@heroicons/react/24/outline';
import { Calendar, CalendarObject, listCalendars, listCalendarObjects, parseICS, icalDateToDate, createCalendarEvent, createCalendar, updateCalendar, deleteCalendar, createCalendarTodo, setTodoStatus, deleteCalendarObject, CalendarSubscription, listCalendarSubscriptions, addCalendarSubscription, deleteCalendarSubscription, fetchSubscriptionICS } from '@/lib/api';
import { formatDate, formatMonthYear, formatWeekRange } from '@/lib/calendar/dateUtils';
import { ParsedEvent, ParsedTodo, parseEvents, parseTodos } from '@/lib/calendar/eventParser';
import { MiniCalendar } from './calendar/MiniCalendar';
import { QuickCreatePopover } from './calendar/QuickCreatePopover';
import { EventPopover } from './calendar/EventPopover';
import { MonthView } from './calendar/MonthView';
import { WeekView } from './calendar/WeekView';
import { DayView } from './calendar/DayView';

// ── Subscription event parser ─────────────────────────────────────────────────

function parseSubscriptionEvents(rawICS: string, sub: CalendarSubscription): ParsedEvent[] {
  const events: ParsedEvent[] = [];
  // Split into VEVENT blocks
  const blocks = rawICS.split(/BEGIN:VEVENT/i).slice(1);
  for (const block of blocks) {
    const endIdx = block.search(/END:VEVENT/i);
    const eventBlock = 'BEGIN:VEVENT\n' + (endIdx >= 0 ? block.slice(0, endIdx) : block);
    const ics = parseICS(eventBlock); // parseICS handles raw text via its try/catch
    const start = icalDateToDate(ics.dtstart);
    if (!start) continue;
    const endRaw = icalDateToDate(ics.dtend);
    const end = endRaw
      ? ics.allDay ? new Date(endRaw.getTime() - 1) : endRaw
      : new Date(start.getTime() + 60 * 60 * 1000);
    events.push({
      obj: { ID: sub.id + '_' + (ics.summary || start.toISOString()), UserID: '', CalendarID: sub.id, ObjectName: '', UID: '', Component: 'VEVENT', ETag: '', Size: 0, ICS: '', CreatedAt: '', UpdatedAt: '' } as unknown as CalendarObject,
      summary: ics.summary || '(제목 없음)',
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

// ── MiniCalendar ─────────────────────────────────────────────────────────────

// ── QuickCreatePopover ───────────────────────────────────────────────────────



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
  // Recurrence
  const [createRrule, setCreateRrule] = useState<'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'>('NONE');
  const [createRruleInterval, setCreateRruleInterval] = useState(1);
  const [createRruleEnd, setCreateRruleEnd] = useState<'never' | 'count' | 'until'>('never');
  const [createRruleCount, setCreateRruleCount] = useState(10);
  const [createRruleUntil, setCreateRruleUntil] = useState('');
  const [createRruleDays, setCreateRruleDays] = useState<number[]>([]);

  // Calendar management modal
  const [showCalModal, setShowCalModal] = useState(false);
  const [editingCal, setEditingCal] = useState<Calendar | null>(null);
  const [calName, setCalName] = useState('');
  const [calColor, setCalColor] = useState('#2F6EE0');
  const [calDesc, setCalDesc] = useState('');
  const [calSaving, setCalSaving] = useState(false);
  const [calError, setCalError] = useState('');
  const [calHoverId, setCalHoverId] = useState<string | null>(null);

  // Todo state
  const [todoDraft, setTodoDraft] = useState('');
  const [todoFocused, setTodoFocused] = useState(false);
  const [todoDueDate, setTodoDueDate] = useState('');
  const [todoTogglingId, setTodoTogglingId] = useState<string | null>(null);
  const [todoDeleteId, setTodoDeleteId] = useState<string | null>(null);
  const [todoHoverId, setTodoHoverId] = useState<string | null>(null);
  const [quickCreate, setQuickCreate] = useState<{ day: Date; rect: DOMRect } | null>(null);

  const [showAddMenu, setShowAddMenu] = useState(false);
  const [showTodoModal, setShowTodoModal] = useState(false);

  // Subscription state
  const [subscriptions, setSubscriptions] = useState<CalendarSubscription[]>([]);
  const [selectedSubIds, setSelectedSubIds] = useState<Set<string>>(new Set());
  const [subICSCache, setSubICSCache] = useState<Map<string, string>>(new Map());
  const [subHoverId, setSubHoverId] = useState<string | null>(null);
  const [showSubModal, setShowSubModal] = useState(false);
  const [subUrl, setSubUrl] = useState('');
  const [subName, setSubName] = useState('');
  const [subColor, setSubColor] = useState('#4285f4');
  const [subSaving, setSubSaving] = useState(false);
  const [subError, setSubError] = useState('');

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

  // Load subscriptions on mount
  useEffect(() => {
    let cancelled = false;
    listCalendarSubscriptions().then((subs) => {
      if (cancelled) return;
      setSubscriptions(subs);
      setSelectedSubIds(new Set(subs.map((s) => s.id)));
    });
    return () => { cancelled = true; };
  }, []);

  // Fetch ICS for active subscriptions
  useEffect(() => {
    let cancelled = false;
    for (const sub of subscriptions) {
      if (!selectedSubIds.has(sub.id)) continue;
      if (subICSCache.has(sub.id)) continue;
      fetchSubscriptionICS(sub.id).then((ics) => {
        if (cancelled) return;
        setSubICSCache((prev) => new Map(prev).set(sub.id, ics));
      }).catch(() => {});
    }
    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subscriptions, selectedSubIds]);

  // Derived: parse + filter events and todos
  const allEvents = parseEvents(objects, calendars);
  const subEvents = subscriptions
    .filter((s) => selectedSubIds.has(s.id) && subICSCache.has(s.id))
    .flatMap((s) => parseSubscriptionEvents(subICSCache.get(s.id)!, s));
  const events = [...allEvents.filter((ev) => selectedCalIds.has(ev.calendarId)), ...subEvents];
  const allTodos = parseTodos(objects, calendars);
  const todos = allTodos.filter((t) => selectedCalIds.has(t.calendarId));

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

  const reloadObjects = useCallback(async () => {
    if (calendars.length === 0) return;
    const results = await Promise.all(calendars.map((c) => listCalendarObjects(c.ID)));
    setObjects(results.flat());
  }, [calendars]);

  const handleToggleTodo = useCallback(async (todo: ParsedTodo) => {
    setTodoTogglingId(todo.obj.ID);
    try {
      await setTodoStatus(todo.calendarId, todo.obj, !todo.completed);
      await reloadObjects();
    } finally {
      setTodoTogglingId(null);
    }
  }, [reloadObjects]);

  const handleDeleteTodo = useCallback(async (todo: ParsedTodo) => {
    setTodoDeleteId(todo.obj.ID);
    try {
      await deleteCalendarObject(todo.calendarId, todo.obj.ObjectName);
      await reloadObjects();
    } finally {
      setTodoDeleteId(null);
    }
  }, [reloadObjects]);

  const handleCreateTodo = useCallback(async () => {
    const title = todoDraft.trim();
    if (!title || calendars.length === 0) return;
    const due = todoDueDate ? new Date(todoDueDate + 'T00:00:00') : undefined;
    const calId = calendars[0].ID;
    try {
      await createCalendarTodo({ title, due, calendarId: calId });
      setTodoDraft('');
      setTodoDueDate('');
      setTodoFocused(false);
      await reloadObjects();
    } catch { /* ignore */ }
  }, [todoDraft, todoDueDate, calendars, reloadObjects]);

  const handleQuickSaveEvent = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    await createCalendarEvent(calendars[0].ID, { title, start: day, end: day, allDay: true });
    await reloadObjects();
  }, [calendars, reloadObjects]);

  const handleQuickSaveTodo = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    await createCalendarTodo({ title, due: day, calendarId: calendars[0].ID });
    await reloadObjects();
  }, [calendars, reloadObjects]);

  const handleCellClick = useCallback((day: Date, rect: DOMRect) => {
    setQuickCreate({ day, rect });
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;
      if (quickCreate || popover || showCreateModal || showCalModal) {
        if (e.key === 'Escape') { setQuickCreate(null); setPopover(null); setShowCreateModal(false); setShowCalModal(false); }
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

  const CAL_COLORS = ['#2F6EE0', '#ef4444', '#f97316', '#eab308', '#22c55e', '#8b5cf6', '#ec4899', '#14b8a6', '#6b7280'];

  const openCalModal = (cal: Calendar | null) => {
    setEditingCal(cal);
    setCalName(cal?.Name ?? '');
    setCalColor(cal?.Color ?? CAL_COLORS[0]);
    setCalDesc(cal?.Description ?? '');
    setCalError('');
    setShowCalModal(true);
  };

  const handleCalSave = async () => {
    if (!calName.trim()) { setCalError('캘린더 이름을 입력하세요'); return; }
    setCalSaving(true); setCalError('');
    try {
      if (editingCal) {
        await updateCalendar(editingCal.ID, { name: calName.trim(), color: calColor, description: calDesc.trim() });
        setCalendars((prev) => prev.map((c) => c.ID === editingCal.ID ? { ...c, Name: calName.trim(), Color: calColor, Description: calDesc.trim() } : c));
      } else {
        const newCal = await createCalendar(calName.trim(), calColor, calDesc.trim());
        setCalendars((prev) => [...prev, newCal]);
        setSelectedCalIds((prev) => new Set([...prev, newCal.ID]));
      }
      setShowCalModal(false);
    } catch (e) {
      setCalError(e instanceof Error ? e.message : '저장 실패');
    } finally {
      setCalSaving(false);
    }
  };

  const handleCalDelete = async () => {
    if (!editingCal) return;
    if (!window.confirm(`"${editingCal.Name}" 캘린더를 삭제하면 포함된 모든 일정도 삭제됩니다. 계속하시겠습니까?`)) return;
    setCalSaving(true);
    try {
      await deleteCalendar(editingCal.ID);
      setCalendars((prev) => prev.filter((c) => c.ID !== editingCal.ID));
      setObjects((prev) => prev.filter((o) => o.CalendarID !== editingCal.ID));
      setSelectedCalIds((prev) => { const next = new Set(prev); next.delete(editingCal.ID); return next; });
      setShowCalModal(false);
    } catch (e) {
      setCalError(e instanceof Error ? e.message : '삭제 실패');
    } finally {
      setCalSaving(false);
    }
  };

  const buildRrule = (): string | undefined => {
    if (createRrule === 'NONE') return undefined;
    const parts: string[] = [`FREQ=${createRrule}`];
    if (createRruleInterval > 1) parts.push(`INTERVAL=${createRruleInterval}`);
    if (createRrule === 'WEEKLY' && createRruleDays.length > 0) {
      const names = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'];
      parts.push(`BYDAY=${createRruleDays.map((d) => names[d]).join(',')}`);
    }
    if (createRruleEnd === 'count') parts.push(`COUNT=${createRruleCount}`);
    else if (createRruleEnd === 'until' && createRruleUntil) {
      const u = new Date(createRruleUntil + 'T23:59:59Z');
      const p = (n: number) => String(n).padStart(2, '0');
      parts.push(`UNTIL=${u.getUTCFullYear()}${p(u.getUTCMonth()+1)}${p(u.getUTCDate())}T235959Z`);
    }
    return parts.join(';');
  };

  const openCreateModal = (baseDate?: Date) => {
    const base = baseDate ?? currentDate;
    const start = new Date(base); start.setHours(9, 0, 0, 0);
    const end = new Date(base); end.setHours(10, 0, 0, 0);
    setCreateTitle(''); setCreateLocation(''); setCreateDesc(''); setCreateError('');
    setCreateAllDay(false);
    setCreateStart(toLocalDT(start));
    setCreateEnd(toLocalDT(end));
    setCreateCalId(calendars[0]?.ID ?? '');
    setCreateRrule('NONE'); setCreateRruleInterval(1);
    setCreateRruleEnd('never'); setCreateRruleCount(10);
    setCreateRruleUntil(''); setCreateRruleDays([]);
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
        rrule: buildRrule(),
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

  const toggleSubscription = (id: string) => {
    setSelectedSubIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const handleAddSubscription = async () => {
    const trimmed = subUrl.trim();
    if (!trimmed) return;
    setSubSaving(true);
    setSubError('');
    try {
      const sub = await addCalendarSubscription(trimmed, subName.trim() || trimmed, subColor);
      setSubscriptions((prev) => [...prev, sub]);
      setSelectedSubIds((prev) => new Set(prev).add(sub.id));
      setShowSubModal(false);
      setSubUrl('');
      setSubName('');
      setSubColor('#4285f4');
    } catch {
      setSubError('구독 추가에 실패했습니다.');
    } finally {
      setSubSaving(false);
    }
  };

  const handleDeleteSubscription = async (id: string) => {
    try {
      await deleteCalendarSubscription(id);
      setSubscriptions((prev) => prev.filter((s) => s.id !== id));
      setSubICSCache((prev) => { const m = new Map(prev); m.delete(id); return m; });
      setSelectedSubIds((prev) => { const s = new Set(prev); s.delete(id); return s; });
    } catch { /* ignore */ }
  };

  const handleEventClick = (ev: ParsedEvent, rect: DOMRect) => {
    setPopover({ event: ev, rect });
  };

  const handleDayClick = (d: Date) => {
    setCurrentDate(d);
    setView('day');
  };

  const M = {
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

  return (
    <div style={{ display: 'flex', flex: 1, overflow: 'hidden', background: 'var(--color-bg-primary)' }}>
      {/* Left sidebar */}
      <div
        style={{
          width: '240px',
          flexShrink: 0,
          borderRight: '1px solid var(--color-border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          overflowY: 'auto',
          background: 'var(--color-bg-secondary)',
        }}
      >
        {/* Mini monthly calendar */}
        <MiniCalendar
          selectedDate={currentDate}
          today={today}
          onDateSelect={handleDayClick}
        />

        {/* Unified create button (below mini calendar) */}
        <div style={{ padding: '0 10px 10px', position: 'relative' }}>
          <button
            onClick={() => setShowAddMenu((v) => !v)}
            style={{
              width: '100%',
              padding: '9px 16px',
              borderRadius: '8px',
              border: '1px solid var(--color-border-default)',
              background: 'var(--color-bg-primary)',
              color: 'var(--color-text-primary)',
              fontSize: '13px',
              fontWeight: 500,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
            }}
          >
            <span style={{ fontSize: '18px', lineHeight: 1, fontWeight: 300, color: 'var(--color-accent)' }}>+</span>
            <span>만들기</span>
            <span style={{ marginLeft: 'auto', fontSize: '10px', color: 'var(--color-text-tertiary)', transition: 'transform 150ms', transform: showAddMenu ? 'rotate(180deg)' : 'rotate(0deg)' }}>▾</span>
          </button>

          {showAddMenu && (
            <>
              <div style={{ position: 'fixed', inset: 0, zIndex: 199 }} onClick={() => setShowAddMenu(false)} />
              <div style={{
                position: 'absolute', top: 'calc(100% - 2px)', left: '10px', right: '10px',
                background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)',
                borderRadius: '8px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200,
                overflow: 'hidden', padding: '4px 0',
              }}>
                {[
                  { Icon: CalendarDaysIcon, label: '일정', action: () => { setShowAddMenu(false); openCreateModal(currentDate); } },
                  { Icon: CheckIcon, label: '할 일', action: () => { setShowAddMenu(false); setShowTodoModal(true); } },
                  { Icon: FolderPlusIcon, label: '새 캘린더', action: () => { setShowAddMenu(false); openCalModal(null); } },
                  { Icon: LinkIcon, label: '캘린더 구독', action: () => { setShowAddMenu(false); setShowSubModal(true); setSubError(''); } },
                ].map(({ Icon, label, action }) => (
                  <button
                    key={label}
                    onClick={action}
                    style={{
                      width: '100%', display: 'flex', alignItems: 'center', gap: '10px',
                      padding: '9px 14px', border: 'none', background: 'none',
                      color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer',
                      textAlign: 'left',
                    }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                  >
                    <Icon style={{ width: '16px', height: '16px', flexShrink: 0, color: 'var(--color-text-secondary)' }} />
                    {label}
                  </button>
                ))}
              </div>
            </>
          )}
        </div>

        <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '0 10px 10px' }} />

        {/* Calendar list */}
        <div style={{ padding: '0 8px', display: 'flex', flexDirection: 'column', gap: '2px', flex: 1 }}>
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
            const hovered = calHoverId === cal.ID;
            return (
              <div
                key={cal.ID}
                onMouseEnter={() => setCalHoverId(cal.ID)}
                onMouseLeave={() => setCalHoverId(null)}
                style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 6px', borderRadius: '5px', cursor: 'pointer', background: hovered ? 'var(--color-bg-tertiary)' : 'transparent' }}
              >
                <span
                  onClick={() => toggleCalendar(cal.ID)}
                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '14px', height: '14px', borderRadius: '3px', border: `2px solid ${cal.Color || 'var(--color-accent)'}`, background: checked ? (cal.Color || 'var(--color-accent)') : 'transparent', cursor: 'pointer', flexShrink: 0 }}
                >
                  {checked && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
                </span>
                <span onClick={() => toggleCalendar(cal.ID)} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, fontSize: '13px', color: 'var(--color-text-primary)' }} title={cal.Name}>
                  {cal.Name}
                </span>
                {hovered && (
                  <button onClick={(e) => { e.stopPropagation(); openCalModal(cal); }} style={{ padding: '2px 4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', fontSize: '12px', lineHeight: 1, borderRadius: '3px', flexShrink: 0 }} title="편집">
                    ···
                  </button>
                )}
              </div>
            );
          })}

          {/* Todo section */}
          <div style={{ marginTop: '8px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '6px 6px 4px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              할일
            </div>

            {/* Todo list items */}
            {todos.map((todo) => {
              const isHovered = todoHoverId === todo.obj.ID;
              const isToggling = todoTogglingId === todo.obj.ID;
              const isDeleting = todoDeleteId === todo.obj.ID;
              return (
                <div
                  key={todo.obj.ID}
                  onMouseEnter={() => setTodoHoverId(todo.obj.ID)}
                  onMouseLeave={() => setTodoHoverId(null)}
                  style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '4px 6px', borderRadius: '6px', background: isHovered ? 'var(--color-bg-tertiary)' : 'transparent' }}
                >
                  {/* Circle checkbox */}
                  <button
                    onClick={() => handleToggleTodo(todo)}
                    disabled={isToggling || isDeleting}
                    title={todo.completed ? '완료 취소' : '완료 표시'}
                    style={{
                      background: todo.completed ? 'var(--color-accent)' : 'transparent',
                      border: `1.5px solid ${todo.completed ? 'var(--color-accent)' : 'var(--color-text-tertiary)'}`,
                      borderRadius: '50%', width: '16px', height: '16px',
                      cursor: 'pointer', padding: 0, flexShrink: 0, marginTop: '2px',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      opacity: isToggling ? 0.5 : 1, transition: 'border-color 150ms, background 150ms',
                    }}
                  >
                    {todo.completed && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
                  </button>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '13px', color: todo.completed ? 'var(--color-text-tertiary)' : 'var(--color-text-primary)', textDecoration: todo.completed ? 'line-through' : 'none', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {todo.summary}
                    </div>
                    {todo.dueDate && (
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '1px' }}>
                        {todo.dueDate.getMonth() + 1}월 {todo.dueDate.getDate()}일
                      </div>
                    )}
                  </div>
                  {isHovered && (
                    <button onClick={() => handleDeleteTodo(todo)} disabled={isDeleting}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '14px', color: 'var(--color-text-tertiary)', padding: '0 2px', flexShrink: 0, lineHeight: 1, opacity: isDeleting ? 0.5 : 1 }}>×</button>
                  )}
                </div>
              );
            })}

            {/* Add todo — Google Tasks style */}
            {todoFocused ? (
              <div style={{ marginTop: '6px', border: '1px solid var(--color-border-default)', borderRadius: '8px', background: 'var(--color-bg-primary)', boxShadow: '0 2px 8px rgba(0,0,0,0.08)', overflow: 'hidden' }}>
                <input
                  autoFocus
                  type="text"
                  placeholder="새 할 일"
                  value={todoDraft}
                  onChange={(e) => setTodoDraft(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleCreateTodo(); if (e.key === 'Escape') { setTodoDraft(''); setTodoDueDate(''); setTodoFocused(false); } }}
                  style={{ width: '100%', border: 'none', outline: 'none', fontSize: '13px', color: 'var(--color-text-primary)', background: 'transparent', padding: '10px 12px 6px', boxSizing: 'border-box' }}
                />
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '2px 12px 6px' }}>
                  <span style={{ fontSize: '13px', color: 'var(--color-text-tertiary)' }}>📅</span>
                  <input
                    type="date"
                    value={todoDueDate}
                    onChange={(e) => setTodoDueDate(e.target.value)}
                    style={{ flex: 1, border: 'none', outline: 'none', fontSize: '12px', color: todoDueDate ? 'var(--color-text-secondary)' : 'var(--color-text-tertiary)', background: 'transparent', cursor: 'pointer' }}
                  />
                </div>
                <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '0 10px' }} />
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '6px', padding: '8px 10px' }}>
                  <button onClick={() => { setTodoDraft(''); setTodoDueDate(''); setTodoFocused(false); }}
                    style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'none', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', fontWeight: 500 }}>
                    취소
                  </button>
                  <button onClick={handleCreateTodo} disabled={!todoDraft.trim()}
                    style={{ padding: '5px 14px', borderRadius: '6px', border: 'none', background: todoDraft.trim() ? 'var(--color-accent)' : 'var(--color-bg-tertiary)', color: todoDraft.trim() ? '#fff' : 'var(--color-text-tertiary)', fontSize: '12px', cursor: todoDraft.trim() ? 'pointer' : 'default', fontWeight: 500 }}>
                    저장
                  </button>
                </div>
              </div>
            ) : null}
          </div>

          {/* 다른 캘린더 (Subscriptions) */}
          <div style={{ marginTop: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 6px 2px' }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                다른 캘린더
              </div>
              <button
                onClick={() => { setShowSubModal(true); setSubError(''); }}
                title="캘린더 구독 추가"
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '16px', lineHeight: 1, padding: '1px 4px', borderRadius: '4px' }}
              >+</button>
            </div>

            {subscriptions.map((sub) => {
              const checked = selectedSubIds.has(sub.id);
              const hovered = subHoverId === sub.id;
              return (
                <div
                  key={sub.id}
                  onMouseEnter={() => setSubHoverId(sub.id)}
                  onMouseLeave={() => setSubHoverId(null)}
                  style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 6px', borderRadius: '5px', cursor: 'pointer', background: hovered ? 'var(--color-bg-tertiary)' : 'transparent' }}
                >
                  <span
                    onClick={() => toggleSubscription(sub.id)}
                    style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '14px', height: '14px', borderRadius: '3px', border: `2px solid ${sub.color}`, background: checked ? sub.color : 'transparent', cursor: 'pointer', flexShrink: 0 }}
                  >
                    {checked && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
                  </span>
                  <span onClick={() => toggleSubscription(sub.id)} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, fontSize: '13px', color: 'var(--color-text-primary)' }} title={sub.name}>
                    {sub.name}
                  </span>
                  {hovered && (
                    <button onClick={() => handleDeleteSubscription(sub.id)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '14px', padding: '0 2px', flexShrink: 0, lineHeight: 1 }} title="구독 취소">×</button>
                  )}
                </div>
              );
            })}

            {subscriptions.length === 0 && (
              <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '4px 6px' }}>구독 캘린더 없음</div>
            )}
          </div>
        </div>
      </div>

      {/* Subscription modal */}
      {showSubModal && (
        <div style={M.overlay} onClick={() => setShowSubModal(false)}>
          <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
            <div style={M.header}><span style={M.title}>캘린더 구독 추가</span></div>
            <div style={M.body}>
              <div>
                <label style={M.label}>ICS/iCal URL *</label>
                <input autoFocus type="url" placeholder="https://calendar.google.com/calendar/ical/..." value={subUrl}
                  onChange={(e) => setSubUrl(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && handleAddSubscription()}
                  style={M.input} />
              </div>
              <div>
                <label style={M.label}>이름 (선택)</label>
                <input type="text" placeholder="캘린더 이름" value={subName} onChange={(e) => setSubName(e.target.value)} style={M.input} />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <label style={M.label}>색상</label>
                <input type="color" value={subColor} onChange={(e) => setSubColor(e.target.value)}
                  style={{ width: '32px', height: '32px', border: 'none', borderRadius: '50%', cursor: 'pointer', padding: 0, background: 'none' }} />
              </div>
              {subError && <div style={M.error}>{subError}</div>}
            </div>
            <div style={M.footer}>
              <button onClick={() => setShowSubModal(false)} style={M.cancelBtn}>취소</button>
              <button onClick={handleAddSubscription} disabled={subSaving || !subUrl.trim()} style={M.primaryBtn(!subUrl.trim() || subSaving)}>
                {subSaving ? '추가 중...' : '구독 추가'}
              </button>
            </div>
          </div>
        </div>
      )}

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
              onClick={goToday}
              style={{
                padding: '5px 12px',
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
            todos={todos}
            today={today}
            onDayClick={handleDayClick}
            onCellClick={handleCellClick}
            onEventClick={handleEventClick}
            onTodoToggle={handleToggleTodo}
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

      {/* Quick create popover */}
      {quickCreate && (
        <QuickCreatePopover
          day={quickCreate.day}
          anchorRect={quickCreate.rect}
          onClose={() => setQuickCreate(null)}
          onSaveEvent={handleQuickSaveEvent}
          onSaveTodo={handleQuickSaveTodo}
          onMoreOptions={(day, mode) => {
            if (mode === 'event') openCreateModal(day);
            else { setTodoDraft(''); setTodoDueDate(''); setTodoFocused(true); }
          }}
        />
      )}

      {/* Event popover */}
      {popover && (
        <EventPopover
          event={popover.event}
          anchorRect={popover.rect}
          onClose={() => setPopover(null)}
        />
      )}

      {/* Calendar management modal */}
      {showCalModal && (
        <div style={M.overlay} onClick={() => !calSaving && setShowCalModal(false)}>
          <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
            <div style={M.header}><span style={M.title}>{editingCal ? '캘린더 편집' : '새 캘린더'}</span></div>
            <div style={M.body}>
              <div>
                <label style={M.label}>캘린더 이름 *</label>
                <input autoFocus placeholder="내 캘린더" value={calName} onChange={(e) => setCalName(e.target.value)} style={M.input} />
              </div>
              <div>
                <label style={M.label}>설명 (선택)</label>
                <input placeholder="설명 추가" value={calDesc} onChange={(e) => setCalDesc(e.target.value)} style={M.input} />
              </div>
              <div>
                <label style={M.label}>색상</label>
                <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', alignItems: 'center' }}>
                  {CAL_COLORS.map((c) => (
                    <button key={c} type="button" onClick={() => setCalColor(c)} style={{ width: '24px', height: '24px', borderRadius: '50%', background: c, border: calColor === c ? '3px solid var(--color-text-primary)' : '2.5px solid transparent', cursor: 'pointer', padding: 0, boxShadow: calColor === c ? `0 0 0 1.5px ${c}` : 'none', transition: 'border 100ms' }} />
                  ))}
                  <input type="color" value={calColor} onChange={(e) => setCalColor(e.target.value)} title="직접 선택" style={{ width: '24px', height: '24px', borderRadius: '50%', border: '1px solid var(--color-border-default)', cursor: 'pointer', padding: 0, background: 'none' }} />
                </div>
              </div>
              {calError && <div style={M.error}>{calError}</div>}
            </div>
            <div style={M.footerSplit}>
              {editingCal
                ? <button onClick={handleCalDelete} disabled={calSaving} style={M.dangerBtn}>삭제</button>
                : <span />}
              <div style={{ display: 'flex', gap: '8px' }}>
                <button onClick={() => setShowCalModal(false)} disabled={calSaving} style={M.cancelBtn}>취소</button>
                <button onClick={handleCalSave} disabled={calSaving || !calName.trim()} style={M.primaryBtn(calSaving || !calName.trim())}>{calSaving ? '저장 중...' : '저장'}</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Event creation modal */}
      {showCreateModal && (
        <div style={M.overlay} onClick={() => !createSaving && setShowCreateModal(false)}>
          <div style={M.card('460px')} onClick={(e) => e.stopPropagation()}>
            <div style={M.header}><span style={M.title}>새 일정</span></div>
            <div style={M.body}>
              <div>
                <label style={M.label}>제목 *</label>
                <input autoFocus type="text" placeholder="일정 제목" value={createTitle} onChange={(e) => setCreateTitle(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleCreateSubmit(); }} style={M.input} />
              </div>

              {calendars.length > 1 && (
                <div>
                  <label style={M.label}>캘린더</label>
                  <select value={createCalId} onChange={(e) => setCreateCalId(e.target.value)} style={M.select}>
                    {calendars.map((c) => <option key={c.ID} value={c.ID ?? ''}>{c.Name ?? '(캘린더)'}</option>)}
                  </select>
                </div>
              )}

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

              <div style={{ display: 'flex', gap: '10px' }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <label style={M.label}>시작</label>
                  <input type={createAllDay ? 'date' : 'datetime-local'} value={createStart} onChange={(e) => setCreateStart(e.target.value)} style={{ ...M.input, minWidth: 0 }} />
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <label style={M.label}>종료</label>
                  <input type={createAllDay ? 'date' : 'datetime-local'} value={createEnd} onChange={(e) => setCreateEnd(e.target.value)} style={{ ...M.input, minWidth: 0 }} />
                </div>
              </div>

              <div>
                <label style={M.label}>장소 (선택)</label>
                <input type="text" placeholder="장소 추가" value={createLocation} onChange={(e) => setCreateLocation(e.target.value)} style={M.input} />
              </div>

              <div>
                <label style={M.label}>메모 (선택)</label>
                <textarea placeholder="메모 추가" value={createDesc} onChange={(e) => setCreateDesc(e.target.value)} rows={2}
                  style={{ ...M.input, resize: 'none', fontFamily: 'inherit' }} />
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '10px', borderRadius: '8px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', width: '36px', flexShrink: 0 }}>반복</span>
                  <select value={createRrule} onChange={(e) => { setCreateRrule(e.target.value as typeof createRrule); setCreateRruleDays([]); }} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                    <option value="NONE">없음</option>
                    <option value="DAILY">매일</option>
                    <option value="WEEKLY">매주</option>
                    <option value="MONTHLY">매월</option>
                    <option value="YEARLY">매년</option>
                  </select>
                  {createRrule !== 'NONE' && (
                    <>
                      <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>마다</span>
                      <input type="number" min={1} max={99} value={createRruleInterval} onChange={(e) => setCreateRruleInterval(Math.max(1, Number(e.target.value)))} style={{ width: '44px', padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' }} />
                      <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{{ DAILY: '일', WEEKLY: '주', MONTHLY: '개월', YEARLY: '년', NONE: '' }[createRrule]}</span>
                    </>
                  )}
                </div>
                {createRrule === 'WEEKLY' && (
                  <div style={{ display: 'flex', gap: '4px', paddingLeft: '44px' }}>
                    {['일','월','화','수','목','금','토'].map((d, i) => (
                      <button key={i} type="button" onClick={() => setCreateRruleDays((prev) => prev.includes(i) ? prev.filter((x) => x !== i) : [...prev, i])}
                        style={{ width: '26px', height: '26px', borderRadius: '50%', border: '1px solid var(--color-border-default)', background: createRruleDays.includes(i) ? 'var(--color-accent)' : 'transparent', color: createRruleDays.includes(i) ? '#fff' : 'var(--color-text-secondary)', fontSize: '11px', cursor: 'pointer', padding: 0, fontWeight: 500 }}
                      >{d}</button>
                    ))}
                  </div>
                )}
                {createRrule !== 'NONE' && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', paddingLeft: '44px' }}>
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>종료</span>
                    <select value={createRruleEnd} onChange={(e) => setCreateRruleEnd(e.target.value as typeof createRruleEnd)} style={{ padding: '4px 8px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', cursor: 'pointer' }}>
                      <option value="never">계속 반복</option>
                      <option value="count">횟수 지정</option>
                      <option value="until">날짜 지정</option>
                    </select>
                    {createRruleEnd === 'count' && (
                      <><input type="number" min={1} max={999} value={createRruleCount} onChange={(e) => setCreateRruleCount(Math.max(1, Number(e.target.value)))} style={{ width: '52px', padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' }} /><span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>회</span></>
                    )}
                    {createRruleEnd === 'until' && (
                      <input type="date" value={createRruleUntil} onChange={(e) => setCreateRruleUntil(e.target.value)} style={{ padding: '4px 6px', fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '5px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }} />
                    )}
                  </div>
                )}
              </div>

              {createError && <div style={M.error}>{createError}</div>}
            </div>
            <div style={M.footer}>
              <button onClick={() => setShowCreateModal(false)} disabled={createSaving} style={M.cancelBtn}>취소</button>
              <button onClick={handleCreateSubmit} disabled={createSaving || !createTitle.trim()} style={M.primaryBtn(createSaving || !createTitle.trim())}>
                {createSaving ? '저장 중...' : '저장'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Todo modal */}
      {showTodoModal && (
        <div style={M.overlay} onClick={() => setShowTodoModal(false)}>
          <div style={M.card('400px')} onClick={(e) => e.stopPropagation()}>
            <div style={M.header}><span style={M.title}>할 일 추가</span></div>
            <div style={M.body}>
              <div>
                <label style={M.label}>제목 *</label>
                <input autoFocus type="text" placeholder="할 일 제목" value={todoDraft}
                  onChange={(e) => setTodoDraft(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && todoDraft.trim()) { handleCreateTodo(); setShowTodoModal(false); } if (e.key === 'Escape') setShowTodoModal(false); }}
                  style={M.input} />
              </div>
              <div>
                <label style={M.label}>마감일 (선택)</label>
                <input type="date" value={todoDueDate} onChange={(e) => setTodoDueDate(e.target.value)} style={M.input} />
              </div>
            </div>
            <div style={M.footer}>
              <button onClick={() => { setTodoDraft(''); setTodoDueDate(''); setShowTodoModal(false); }} style={M.cancelBtn}>취소</button>
              <button onClick={() => { handleCreateTodo(); setShowTodoModal(false); }} disabled={!todoDraft.trim()} style={M.primaryBtn(!todoDraft.trim())}>추가</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
