'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { Calendar, CalendarObject, listCalendars, listCalendarObjects, createCalendarEvent, updateCalendarEvent, createCalendar, updateCalendar, deleteCalendar, createCalendarTodo, setTodoStatus, deleteCalendarObject, CalendarSubscription, listCalendarSubscriptions, addCalendarSubscription, deleteCalendarSubscription, fetchSubscriptionICS } from '@/lib/api';
import { formatDate, formatMonthYear, formatWeekRange } from '@/lib/calendar/dateUtils';
import { ParsedEvent, ParsedTodo, parseEvents, parseTodos } from '@/lib/calendar/eventParser';
import { CalendarSidebar } from './calendar/CalendarSidebar';
import { CalendarToolbar } from './calendar/CalendarToolbar';
import { QuickCreatePopover } from './calendar/QuickCreatePopover';
import { EventPopover } from './calendar/EventPopover';
import { parseSubscriptionEvents } from '@/lib/calendar/subscriptionParser';
import { CalendarManagementModal, EventCreateModal, EventEditModal, SubscriptionAddModal, TodoCreateModal } from './calendar/CalendarModals';
import { MonthView } from './calendar/MonthView';
import { WeekView } from './calendar/WeekView';
import { DayView } from './calendar/DayView';

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

  // Event edit modal
  const [showEditModal, setShowEditModal] = useState(false);
  const [editingEvent, setEditingEvent] = useState<import('@/lib/calendar/eventParser').ParsedEvent | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editStart, setEditStart] = useState('');
  const [editEnd, setEditEnd] = useState('');
  const [editAllDay, setEditAllDay] = useState(false);
  const [editLocation, setEditLocation] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [editCalId, setEditCalId] = useState('');
  const [editSaving, setEditSaving] = useState(false);
  const [editError, setEditError] = useState('');
  const [editRrule, setEditRrule] = useState<'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'>('NONE');
  const [editRruleInterval, setEditRruleInterval] = useState(1);
  const [editRruleEnd, setEditRruleEnd] = useState<'never' | 'count' | 'until'>('never');
  const [editRruleCount, setEditRruleCount] = useState(10);
  const [editRruleUntil, setEditRruleUntil] = useState('');
  const [editRruleDays, setEditRruleDays] = useState<number[]>([]);
  const [editScope, setEditScope] = useState<'this' | 'all'>('this');

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

  const handleCreateAllDayToggle = (checked: boolean) => {
    setCreateAllDay(checked);
    if (checked) {
      setCreateStart(createStart.split('T')[0] || toLocalDate(new Date()));
      setCreateEnd(createEnd.split('T')[0] || toLocalDate(new Date()));
    } else {
      setCreateStart((createStart.includes('T') ? createStart : createStart + 'T09:00'));
      setCreateEnd((createEnd.includes('T') ? createEnd : createEnd + 'T10:00'));
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

  const openEditModal = useCallback((ev: import('@/lib/calendar/eventParser').ParsedEvent) => {
    setEditingEvent(ev);
    setEditTitle(ev.summary === '(제목 없음)' ? '' : ev.summary);
    setEditLocation(ev.location ?? '');
    setEditDesc(ev.description ?? '');
    setEditAllDay(ev.allDay);
    setEditCalId(ev.calendarId);
    setEditError('');
    setEditScope('all');
    // Parse rrule from ICS
    const icsRaw = ev.obj.ICS;
    let rruleStr = '';
    try {
      let text = '';
      try { text = atob(icsRaw); } catch { text = icsRaw; }
      text = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');
      const m = text.match(/(?:^|\n)RRULE:([^\n]*)/im);
      rruleStr = m ? m[1].trim() : '';
    } catch { /* ignore */ }
    if (rruleStr) {
      const freqM = rruleStr.match(/FREQ=([A-Z]+)/);
      const freq = (freqM?.[1] ?? 'NONE') as 'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';
      setEditRrule(['DAILY','WEEKLY','MONTHLY','YEARLY'].includes(freq) ? freq : 'NONE');
      const intM = rruleStr.match(/INTERVAL=(\d+)/);
      setEditRruleInterval(intM ? parseInt(intM[1], 10) : 1);
      const countM = rruleStr.match(/COUNT=(\d+)/);
      const untilM = rruleStr.match(/UNTIL=([^;]+)/);
      if (countM) { setEditRruleEnd('count'); setEditRruleCount(parseInt(countM[1], 10)); setEditRruleUntil(''); }
      else if (untilM) {
        setEditRruleEnd('until');
        const u = untilM[1].replace(/[TZ]/g, '');
        setEditRruleUntil(`${u.slice(0,4)}-${u.slice(4,6)}-${u.slice(6,8)}`);
        setEditRruleCount(10);
      } else { setEditRruleEnd('never'); setEditRruleCount(10); setEditRruleUntil(''); }
      const bydayM = rruleStr.match(/BYDAY=([^;]+)/);
      if (bydayM) {
        const names = ['SU','MO','TU','WE','TH','FR','SA'];
        setEditRruleDays(bydayM[1].split(',').map((d) => names.indexOf(d.trim())).filter((i) => i >= 0));
      } else { setEditRruleDays([]); }
    } else {
      setEditRrule('NONE'); setEditRruleInterval(1); setEditRruleEnd('never');
      setEditRruleCount(10); setEditRruleUntil(''); setEditRruleDays([]);
    }
    setEditStart(ev.allDay ? toLocalDate(ev.start) : toLocalDT(ev.start));
    setEditEnd(ev.allDay ? toLocalDate(ev.end) : toLocalDT(ev.end));
    setShowEditModal(true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const buildEditRrule = (): string | undefined => {
    if (editRrule === 'NONE') return undefined;
    const parts: string[] = [`FREQ=${editRrule}`];
    if (editRruleInterval > 1) parts.push(`INTERVAL=${editRruleInterval}`);
    if (editRrule === 'WEEKLY' && editRruleDays.length > 0) {
      const names = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'];
      parts.push(`BYDAY=${editRruleDays.map((d) => names[d]).join(',')}`);
    }
    if (editRruleEnd === 'count') parts.push(`COUNT=${editRruleCount}`);
    else if (editRruleEnd === 'until' && editRruleUntil) {
      const u = new Date(editRruleUntil + 'T23:59:59Z');
      const p = (n: number) => String(n).padStart(2, '0');
      parts.push(`UNTIL=${u.getUTCFullYear()}${p(u.getUTCMonth()+1)}${p(u.getUTCDate())}T235959Z`);
    }
    return parts.join(';');
  };

  const handleEditSubmit = useCallback(async () => {
    if (!editingEvent || !editTitle.trim()) { setEditError('제목을 입력하세요'); return; }
    const startDate = new Date(editAllDay ? editStart + 'T00:00:00' : editStart);
    const endDate = new Date(editAllDay ? editEnd + 'T00:00:00' : editEnd);
    if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) { setEditError('날짜를 확인하세요'); return; }
    if (endDate <= startDate) { setEditError('종료 시간이 시작 시간보다 늦어야 합니다'); return; }
    setEditSaving(true); setEditError('');
    try {
      const uid = editingEvent.obj.UID;
      const objectName = editingEvent.obj.ObjectName;
      if (editRrule !== 'NONE' && editScope === 'this') {
        setEditError('개별 반복 일정 수정은 아직 지원되지 않습니다. 모든 반복 이벤트로 저장하세요.');
        return;
      }
      const sourceCalId = editingEvent.calendarId;
      const targetCalId = editCalId || sourceCalId;
      await updateCalendarEvent(targetCalId, objectName, uid, {
        title: editTitle.trim(),
        start: startDate,
        end: endDate,
        allDay: editAllDay,
        location: editLocation.trim() || undefined,
        description: editDesc.trim() || undefined,
        rrule: buildEditRrule(),
      });
      if (targetCalId !== sourceCalId) {
        await deleteCalendarObject(sourceCalId, objectName);
      }
      setShowEditModal(false);
      setEditingEvent(null);
      await reloadObjects();
    } catch (e) {
      setEditError(e instanceof Error ? e.message : '수정 실패');
    } finally {
      setEditSaving(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingEvent, editTitle, editAllDay, editStart, editEnd, editLocation, editDesc, editCalId, editRrule, editRruleInterval, editRruleEnd, editRruleCount, editRruleUntil, editRruleDays, editScope, reloadObjects]);

  const handleDeleteEvent = useCallback(async (ev: import('@/lib/calendar/eventParser').ParsedEvent) => {
    if (!window.confirm(`"${ev.summary}" 일정을 삭제하시겠습니까?`)) return;
    setPopover(null);
    try {
      await deleteCalendarObject(ev.calendarId, ev.obj.ObjectName);
      await reloadObjects();
    } catch { /* ignore */ }
  }, [reloadObjects]);

  const handleDayClick = (d: Date) => {
    setCurrentDate(d);
    setView('day');
  };

  const recurrenceLabel = { DAILY: '일', WEEKLY: '주', MONTHLY: '개월', YEARLY: '년', NONE: '' }[createRrule];
  const canSubmitCreateEvent = Boolean(createTitle.trim());

  return (
    <div style={{ display: 'flex', flex: 1, overflow: 'hidden', background: 'var(--color-bg-primary)' }}>
      <CalendarSidebar
        currentDate={currentDate}
        today={today}
        onDaySelect={handleDayClick}
        loading={loading}
        calendars={calendars}
        selectedCalIds={selectedCalIds}
        calHoverId={calHoverId}
        setCalHoverId={setCalHoverId}
        todoTogglingId={todoTogglingId}
        todoDeleteId={todoDeleteId}
        todoHoverId={todoHoverId}
        setTodoHoverId={setTodoHoverId}
        todos={todos}
        onToggleCalendar={toggleCalendar}
        onOpenCalendarModal={openCalModal}
        onToggleTodo={handleToggleTodo}
        onDeleteTodo={handleDeleteTodo}
        todoFocused={todoFocused}
        todoDraft={todoDraft}
        todoDueDate={todoDueDate}
        setTodoDraft={setTodoDraft}
        setTodoDueDate={setTodoDueDate}
        onShowTodo={() => setShowTodoModal(true)}
        onCreateTodo={handleCreateTodo}
        onCancelTodoInline={() => {
          setTodoDraft('');
          setTodoDueDate('');
          setTodoFocused(false);
        }}
        setTodoFocused={setTodoFocused}
        showAddMenu={showAddMenu}
        setShowAddMenu={setShowAddMenu}
        openCreateModal={openCreateModal}
        openCalendarModal={() => openCalModal(null)}
        openSubscriptionModal={() => {
          setSubError('');
          setShowSubModal(true);
        }}
        subscriptions={subscriptions}
        selectedSubIds={selectedSubIds}
        subHoverId={subHoverId}
        setSubHoverId={setSubHoverId}
        onToggleSubscription={toggleSubscription}
        onDeleteSubscription={handleDeleteSubscription}
      />

      <SubscriptionAddModal
        show={showSubModal}
        subError={subError}
        subUrl={subUrl}
        subName={subName}
        subColor={subColor}
        subSaving={subSaving}
        onClose={() => setShowSubModal(false)}
        onSubmit={handleAddSubscription}
        onUrlChange={setSubUrl}
        onNameChange={setSubName}
        onColorChange={setSubColor}
      />

      <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
        <CalendarToolbar
          title={title}
          view={view}
          onGoToday={goToday}
          onNavigate={navigate}
          onSetView={setView}
        />

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
          onEdit={openEditModal}
          onDelete={handleDeleteEvent}
        />
      )}

      <CalendarManagementModal
        show={showCalModal}
        editingCal={editingCal}
        calName={calName}
        calDesc={calDesc}
        calColor={calColor}
        calError={calError}
        calSaving={calSaving}
        colors={CAL_COLORS}
        onClose={() => setShowCalModal(false)}
        onDelete={handleCalDelete}
        onSave={handleCalSave}
        onNameChange={setCalName}
        onDescChange={setCalDesc}
        onColorChange={setCalColor}
        onColorQuickSelect={setCalColor}
      />

      <EventCreateModal
        show={showCreateModal}
        calendars={calendars}
        createTitle={createTitle}
        createStart={createStart}
        createEnd={createEnd}
        createAllDay={createAllDay}
        createLocation={createLocation}
        createDesc={createDesc}
        createCalId={createCalId}
        createError={createError}
        createSaving={createSaving}
        createRrule={createRrule}
        createRruleInterval={createRruleInterval}
        createRruleEnd={createRruleEnd}
        createRruleCount={createRruleCount}
        createRruleUntil={createRruleUntil}
        createRruleDays={createRruleDays}
        canSubmit={canSubmitCreateEvent}
        showCalSelect={calendars.length > 1}
        dayLabels={['일', '월', '화', '수', '목', '금', '토']}
        ruleIntervalLabel={recurrenceLabel}
        onClose={() => setShowCreateModal(false)}
        onSubmit={handleCreateSubmit}
        onTitleChange={setCreateTitle}
        onStartChange={setCreateStart}
        onEndChange={setCreateEnd}
        onAllDayToggle={handleCreateAllDayToggle}
        onLocationChange={setCreateLocation}
        onDescChange={setCreateDesc}
        onCalIdChange={setCreateCalId}
        onRruleChange={(r) => { setCreateRrule(r); setCreateRruleDays([]); }}
        onRruleIntervalChange={setCreateRruleInterval}
        onRruleEndChange={setCreateRruleEnd}
        onRruleCountChange={setCreateRruleCount}
        onRruleUntilChange={setCreateRruleUntil}
        onToggleRruleDay={(day) => setCreateRruleDays((prev) => prev.includes(day) ? prev.filter((x) => x !== day) : [...prev, day])}
      />

      <EventEditModal
        show={showEditModal}
        calendars={calendars}
        createTitle={editTitle}
        createStart={editStart}
        createEnd={editEnd}
        createAllDay={editAllDay}
        createLocation={editLocation}
        createDesc={editDesc}
        createCalId={editCalId}
        createError={editError}
        createSaving={editSaving}
        createRrule={editRrule}
        createRruleInterval={editRruleInterval}
        createRruleEnd={editRruleEnd}
        createRruleCount={editRruleCount}
        createRruleUntil={editRruleUntil}
        createRruleDays={editRruleDays}
        canSubmit={Boolean(editTitle.trim())}
        dayLabels={['일', '월', '화', '수', '목', '금', '토']}
        ruleIntervalLabel={{ DAILY: '일', WEEKLY: '주', MONTHLY: '개월', YEARLY: '년', NONE: '' }[editRrule]}
        isRecurring={editRrule !== 'NONE'}
        editScope={editScope}
        onEditScopeChange={setEditScope}
        onClose={() => { setShowEditModal(false); setEditingEvent(null); }}
        onSubmit={handleEditSubmit}
        onTitleChange={setEditTitle}
        onStartChange={setEditStart}
        onEndChange={setEditEnd}
        onAllDayToggle={(checked) => {
          setEditAllDay(checked);
          if (checked) {
            setEditStart(editStart.split('T')[0] || toLocalDate(new Date()));
            setEditEnd(editEnd.split('T')[0] || toLocalDate(new Date()));
          } else {
            setEditStart(editStart.includes('T') ? editStart : editStart + 'T09:00');
            setEditEnd(editEnd.includes('T') ? editEnd : editEnd + 'T10:00');
          }
        }}
        onLocationChange={setEditLocation}
        onDescChange={setEditDesc}
        onCalIdChange={setEditCalId}
        onRruleChange={(r) => { setEditRrule(r); setEditRruleDays([]); }}
        onRruleIntervalChange={setEditRruleInterval}
        onRruleEndChange={setEditRruleEnd}
        onRruleCountChange={setEditRruleCount}
        onRruleUntilChange={setEditRruleUntil}
        onToggleRruleDay={(day) => setEditRruleDays((prev) => prev.includes(day) ? prev.filter((x) => x !== day) : [...prev, day])}
      />

      <TodoCreateModal
        show={showTodoModal}
        todoDraft={todoDraft}
        todoDueDate={todoDueDate}
        onDraftChange={setTodoDraft}
        onDueDateChange={setTodoDueDate}
        onSubmit={handleCreateTodo}
        onClose={() => setShowTodoModal(false)}
        canSubmit={Boolean(todoDraft.trim())}
      />
    </div>
  );
}
