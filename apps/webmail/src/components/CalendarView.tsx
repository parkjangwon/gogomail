'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { createCalendar, updateCalendar, deleteCalendar, createCalendarTodo, setTodoStatus, deleteCalendarObject } from '@/lib/api';
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
import { useCalendarData } from './calendar/useCalendarData';
import { useCalendarCreateForm } from './calendar/useCalendarCreateForm';
import { useCalendarEditForm } from './calendar/useCalendarEditForm';

  // ── CalendarView (main) ───────────────────────────────────────────────────────

export function CalendarView() {
  const t = useTranslations('calendarFull');
  const [view, setView] = useState<'month' | 'week' | 'day'>('month');
  const [currentDate, setCurrentDate] = useState<Date>(() => {
    const d = new Date(); d.setHours(0, 0, 0, 0); return d;
  });
  const today = useRef<Date>((() => { const d = new Date(); d.setHours(0, 0, 0, 0); return d; })()).current;

  const [popover, setPopover] = useState<{ event: ParsedEvent; rect: DOMRect } | null>(null);

  const calData = useCalendarData();
  const { calendars, setCalendars, objects, setObjects, selectedCalIds, setSelectedCalIds, loading, subscriptions, selectedSubIds, subICSCache, toggleCalendar, toggleSubscription, handleAddSubscription, handleDeleteSubscription, refresh } = calData;

  const createForm = useCalendarCreateForm({ calendars, onCreated: refresh });
  const {
    showCreateModal,
    createTitle, setCreateTitle,
    createStart, setCreateStart,
    createEnd, setCreateEnd,
    createAllDay,
    createLocation, setCreateLocation,
    createDesc, setCreateDesc,
    createCalId, setCreateCalId,
    createSaving,
    createError,
    createRrule, setCreateRrule,
    createRruleInterval, setCreateRruleInterval,
    createRruleEnd, setCreateRruleEnd,
    createRruleCount, setCreateRruleCount,
    createRruleUntil, setCreateRruleUntil,
    createRruleDays, setCreateRruleDays,
    openCreateModal: openCreateModalBase,
    closeCreateModal,
    handleCreateSubmit,
    handleCreateAllDayToggle,
  } = createForm;

  const editForm = useCalendarEditForm({ calendars, onUpdated: refresh });
  const {
    showEditModal,
    editingEvent,
    editTitle, setEditTitle,
    editStart, setEditStart,
    editEnd, setEditEnd,
    editAllDay,
    editLocation, setEditLocation,
    editDesc, setEditDesc,
    editCalId, setEditCalId,
    editSaving,
    editError,
    editRrule, setEditRrule,
    editRruleInterval, setEditRruleInterval,
    editRruleEnd, setEditRruleEnd,
    editRruleCount, setEditRruleCount,
    editRruleUntil, setEditRruleUntil,
    editRruleDays, setEditRruleDays,
    openEditModal,
    closeEditModal,
    handleEditSubmit,
    handleDeleteEvent: handleDeleteEventBase,
    handleEditAllDayToggle,
  } = editForm;

  // Calendar management modal
  const [showCalModal, setShowCalModal] = useState(false);
  const [editingCal, setEditingCal] = useState<import('@/lib/api').Calendar | null>(null);
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
  const [subHoverId, setSubHoverId] = useState<string | null>(null);
  const [showSubModal, setShowSubModal] = useState(false);
  const [subUrl, setSubUrl] = useState('');
  const [subName, setSubName] = useState('');
  const [subColor, setSubColor] = useState('#4285f4');
  const [subSaving, setSubSaving] = useState(false);
  const [subError, setSubError] = useState('');

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

  const handleToggleTodo = useCallback(async (todo: ParsedTodo) => {
    setTodoTogglingId(todo.obj.ID);
    try {
      await setTodoStatus(todo.calendarId, todo.obj, !todo.completed);
      await refresh();
    } finally {
      setTodoTogglingId(null);
    }
  }, [refresh]);

  const handleDeleteTodo = useCallback(async (todo: ParsedTodo) => {
    setTodoDeleteId(todo.obj.ID);
    try {
      await deleteCalendarObject(todo.calendarId, todo.obj.ObjectName);
      await refresh();
    } finally {
      setTodoDeleteId(null);
    }
  }, [refresh]);

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
      await refresh();
    } catch { /* ignore */ }
  }, [todoDraft, todoDueDate, calendars, refresh]);

  const handleQuickSaveEvent = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    const { createCalendarEvent } = await import('@/lib/api');
    await createCalendarEvent(calendars[0].ID, { title, start: day, end: day, allDay: true });
    await refresh();
  }, [calendars, refresh]);

  const handleQuickSaveTodo = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    await createCalendarTodo({ title, due: day, calendarId: calendars[0].ID });
    await refresh();
  }, [calendars, refresh]);

  const handleCellClick = useCallback((day: Date, rect: DOMRect) => {
    setQuickCreate({ day, rect });
  }, []);

  // Wrap openCreateModal to pass currentDate
  const openCreateModal = useCallback((baseDate?: Date) => {
    openCreateModalBase(baseDate, currentDate);
  }, [openCreateModalBase, currentDate]);

  // Wrap handleDeleteEvent to also close popover
  const handleDeleteEvent = useCallback(async (ev: ParsedEvent) => {
    setPopover(null);
    await handleDeleteEventBase(ev);
  }, [handleDeleteEventBase]);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;
      if (quickCreate || popover || showCreateModal || showCalModal) {
        if (e.key === 'Escape') { setQuickCreate(null); setPopover(null); closeCreateModal(); setShowCalModal(false); }
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
  }, [navigate, goToday, popover, showCreateModal, closeCreateModal]);

  // Title
  let title = '';
  if (view === 'month') title = formatMonthYear(currentDate);
  else if (view === 'week') title = formatWeekRange(currentDate);
  else title = formatDate(currentDate);

  const CAL_COLORS = ['#2F6EE0', '#ef4444', '#f97316', '#eab308', '#22c55e', '#8b5cf6', '#ec4899', '#14b8a6', '#6b7280'];

  const openCalModal = (cal: import('@/lib/api').Calendar | null) => {
    setEditingCal(cal);
    setCalName(cal?.Name ?? '');
    setCalColor(cal?.Color ?? CAL_COLORS[0]);
    setCalDesc(cal?.Description ?? '');
    setCalError('');
    setShowCalModal(true);
  };

  const handleCalSave = async () => {
    if (!calName.trim()) { setCalError(t('management.nameRequired')); return; }
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
      setCalError(e instanceof Error ? e.message : t('management.saveFailed'));
    } finally {
      setCalSaving(false);
    }
  };

  const handleCalDelete = async () => {
    if (!editingCal) return;
    if (!window.confirm(t('management.confirmDelete', { name: editingCal.Name }))) return;
    setCalSaving(true);
    try {
      await deleteCalendar(editingCal.ID);
      setCalendars((prev) => prev.filter((c) => c.ID !== editingCal.ID));
      setObjects((prev) => prev.filter((o) => o.CalendarID !== editingCal.ID));
      setSelectedCalIds((prev) => { const next = new Set(prev); next.delete(editingCal.ID); return next; });
      setShowCalModal(false);
    } catch (e) {
      setCalError(e instanceof Error ? e.message : t('management.deleteFailed'));
    } finally {
      setCalSaving(false);
    }
  };

  const handleAddSubscriptionForm = async () => {
    const trimmed = subUrl.trim();
    if (!trimmed) return;
    setSubSaving(true);
    setSubError('');
    try {
      await handleAddSubscription(trimmed, subName.trim() || trimmed, subColor);
      setShowSubModal(false);
      setSubUrl('');
      setSubName('');
      setSubColor('#4285f4');
    } catch {
      setSubError(t('subscription.failed'));
    } finally {
      setSubSaving(false);
    }
  };

  const handleEventClick = (ev: ParsedEvent, rect: DOMRect) => {
    setPopover({ event: ev, rect });
  };

  const handleDayClick = (d: Date) => {
    setCurrentDate(d);
    setView('day');
  };

  const recurrenceLabel = createRrule === 'NONE' ? '' : t(`intervalLabel.${createRrule}`);
  const editRecurrenceLabel = editRrule === 'NONE' ? '' : t(`intervalLabel.${editRrule}`);
  const localizedDayLabels = [0, 1, 2, 3, 4, 5, 6].map((i) => t(`dayLabels.${i}`));
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
        onSubmit={handleAddSubscriptionForm}
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
            {t('view.loading')}
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
        dayLabels={localizedDayLabels}
        ruleIntervalLabel={recurrenceLabel}
        onClose={closeCreateModal}
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
        dayLabels={localizedDayLabels}
        ruleIntervalLabel={editRecurrenceLabel}
        isRecurring={editRrule !== 'NONE'}
        onClose={closeEditModal}
        onSubmit={handleEditSubmit}
        onTitleChange={setEditTitle}
        onStartChange={setEditStart}
        onEndChange={setEditEnd}
        onAllDayToggle={handleEditAllDayToggle}
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
