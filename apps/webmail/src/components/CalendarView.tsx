'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { formatDate, formatMonthYear, formatWeekRange } from '@/lib/calendar/dateUtils';
import { ParsedEvent, parseEvents, parseTodos } from '@/lib/calendar/eventParser';
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
import { useCalendarManagement } from './calendar/useCalendarManagement';
import { useCalendarTodos } from './calendar/useCalendarTodos';
import { useCalendarSubscriptionForm } from './calendar/useCalendarSubscriptionForm';

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

  const {
    showCalModal, setShowCalModal,
    editingCal,
    calName, setCalName,
    calColor, setCalColor,
    calDesc, setCalDesc,
    calSaving, calError,
    calHoverId, setCalHoverId,
    CAL_COLORS,
    openCalModal,
    handleCalSave,
    handleCalDelete,
  } = useCalendarManagement({ calendars, setCalendars, setObjects, setSelectedCalIds, t });

  const {
    todoDraft, setTodoDraft,
    todoFocused, setTodoFocused,
    todoDueDate, setTodoDueDate,
    todoTogglingId,
    todoDeleteId,
    todoHoverId, setTodoHoverId,
    quickCreate, setQuickCreate,
    showTodoModal, setShowTodoModal,
    handleToggleTodo,
    handleDeleteTodo,
    handleCreateTodo,
    handleCellClick,
    handleQuickSaveTodo,
  } = useCalendarTodos({ calendars, refresh });

  const [showAddMenu, setShowAddMenu] = useState(false);

  const {
    subHoverId, setSubHoverId,
    showSubModal, setShowSubModal,
    subUrl, setSubUrl,
    subName, setSubName,
    subColor, setSubColor,
    subSaving, subError, setSubError,
    handleAddSubscriptionForm,
  } = useCalendarSubscriptionForm({ handleAddSubscription, t });

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

  const handleQuickSaveEvent = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    const { createCalendarEvent } = await import('@/lib/api');
    await createCalendarEvent(calendars[0].ID, { title, start: day, end: day, allDay: true });
    await refresh();
  }, [calendars, refresh]);

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
