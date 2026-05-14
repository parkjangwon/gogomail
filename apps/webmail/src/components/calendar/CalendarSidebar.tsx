import { CalendarDaysIcon, CheckIcon, FolderPlusIcon, LinkIcon } from '@heroicons/react/24/outline';
import { Calendar, CalendarSubscription, CalendarObject } from '@/lib/api';
import { ParsedTodo } from '@/lib/calendar/eventParser';
import { MiniCalendar } from './MiniCalendar';

interface CalendarSidebarProps {
  currentDate: Date;
  today: Date;
  onDaySelect: (day: Date) => void;
  loading: boolean;
  calendars: Calendar[];
  selectedCalIds: Set<string>;
  calHoverId: string | null;
  setCalHoverId: (id: string | null) => void;
  todoTogglingId: string | null;
  todoDeleteId: string | null;
  todoHoverId: string | null;
  setTodoHoverId: (id: string | null) => void;
  todos: ParsedTodo[];
  onToggleCalendar: (id: string) => void;
  onOpenCalendarModal: (calendar: Calendar | null) => void;
  onToggleTodo: (todo: ParsedTodo) => void;
  onDeleteTodo: (todo: ParsedTodo) => void;

  todoFocused: boolean;
  todoDraft: string;
  todoDueDate: string;
  setTodoDraft: (value: string) => void;
  setTodoDueDate: (value: string) => void;
  onShowTodo: () => void;
  onCreateTodo: () => void;
  onCancelTodoInline: () => void;
  setTodoFocused: (value: boolean) => void;

  showAddMenu: boolean;
  setShowAddMenu: (value: boolean | ((value: boolean) => boolean)) => void;
  openCreateModal: (baseDate?: Date) => void;
  openCalendarModal: () => void;
  openSubscriptionModal: () => void;

  subscriptions: CalendarSubscription[];
  selectedSubIds: Set<string>;
  subHoverId: string | null;
  setSubHoverId: (id: string | null) => void;
  onToggleSubscription: (id: string) => void;
  onDeleteSubscription: (id: string) => void;
}

export function CalendarSidebar({
  currentDate,
  today,
  onDaySelect,
  loading,
  calendars,
  selectedCalIds,
  calHoverId,
  setCalHoverId,
  todoTogglingId,
  todoDeleteId,
  todoHoverId,
  setTodoHoverId,
  todos,
  onToggleCalendar,
  onOpenCalendarModal,
  onToggleTodo,
  onDeleteTodo,
  todoFocused,
  todoDraft,
  todoDueDate,
  setTodoDraft,
  setTodoDueDate,
  onShowTodo,
  onCreateTodo,
  onCancelTodoInline,
  setTodoFocused,
  showAddMenu,
  setShowAddMenu,
  openCreateModal,
  openCalendarModal,
  openSubscriptionModal,
  subscriptions,
  selectedSubIds,
  subHoverId,
  setSubHoverId,
  onToggleSubscription,
  onDeleteSubscription,
}: CalendarSidebarProps) {
  const createMenuItems = [
    {
      Icon: CalendarDaysIcon,
      label: '일정',
      action: () => {
        setShowAddMenu(false);
        openCreateModal(currentDate);
      },
    },
    {
      Icon: CheckIcon,
      label: '할 일',
      action: () => {
        setShowAddMenu(false);
        onShowTodo();
      },
    },
    {
      Icon: FolderPlusIcon,
      label: '새 캘린더',
      action: () => {
        setShowAddMenu(false);
        openCalendarModal();
      },
    },
    {
      Icon: LinkIcon,
      label: '캘린더 구독',
      action: () => {
        setShowAddMenu(false);
        openSubscriptionModal();
      },
    },
  ];

  return (
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
      <MiniCalendar
        selectedDate={currentDate}
        today={today}
        onDateSelect={onDaySelect}
      />

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
              {createMenuItems.map(({ Icon, label, action }) => (
                <button
                  key={label}
                  onClick={action}
                  style={{
                    width: '100%', display: 'flex', alignItems: 'center', gap: '10px',
                    padding: '9px 14px', border: 'none', background: 'none',
                    color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer',
                    textAlign: 'left',
                  }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)';
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'none';
                  }}
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
                onClick={() => onToggleCalendar(cal.ID)}
                style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '14px', height: '14px', borderRadius: '3px', border: `2px solid ${cal.Color || 'var(--color-accent)'}`, background: checked ? (cal.Color || 'var(--color-accent)') : 'transparent', cursor: 'pointer', flexShrink: 0 }}
              >
                {checked && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
              </span>
              <span onClick={() => onToggleCalendar(cal.ID)} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, fontSize: '13px', color: 'var(--color-text-primary)' }} title={cal.Name}>
                {cal.Name}
              </span>
              {hovered && (
                <button onClick={(e) => { e.stopPropagation(); onOpenCalendarModal(cal); }} style={{ padding: '2px 4px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', fontSize: '12px', lineHeight: 1, borderRadius: '3px', flexShrink: 0 }} title="편집">
                  ···
                </button>
              )}
            </div>
          );
        })}

        <div style={{ marginTop: '8px' }}>
          <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '6px 6px 4px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            할일
          </div>

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
                <button
                  onClick={() => onToggleTodo(todo)}
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
                  <button onClick={() => onDeleteTodo(todo)} disabled={isDeleting}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '14px', color: 'var(--color-text-tertiary)', padding: '0 2px', flexShrink: 0, lineHeight: 1, opacity: isDeleting ? 0.5 : 1 }}>×</button>
                )}
              </div>
            );
          })}

          {todoFocused ? (
            <div style={{ marginTop: '6px', border: '1px solid var(--color-border-default)', borderRadius: '8px', background: 'var(--color-bg-primary)', boxShadow: '0 2px 8px rgba(0,0,0,0.08)', overflow: 'hidden' }}>
              <input
                autoFocus
                type="text"
                placeholder="새 할 일"
                value={todoDraft}
                onChange={(e) => setTodoDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onCreateTodo();
                  if (e.key === 'Escape') onCancelTodoInline();
                }}
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
                <button onClick={onCancelTodoInline}
                  style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'none', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', fontWeight: 500 }}>
                  취소
                </button>
                <button onClick={onCreateTodo} disabled={!todoDraft.trim()}
                  style={{ padding: '5px 14px', borderRadius: '6px', border: 'none', background: todoDraft.trim() ? 'var(--color-accent)' : 'var(--color-bg-tertiary)', color: todoDraft.trim() ? '#fff' : 'var(--color-text-tertiary)', fontSize: '12px', cursor: todoDraft.trim() ? 'pointer' : 'default', fontWeight: 500 }}>
                  저장
                </button>
              </div>
            </div>
          ) : null}
        </div>

        <div style={{ marginTop: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 6px 2px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              다른 캘린더
            </div>
            <button
              onClick={openSubscriptionModal}
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
                  onClick={() => onToggleSubscription(sub.id)}
                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '14px', height: '14px', borderRadius: '3px', border: `2px solid ${sub.color}`, background: checked ? sub.color : 'transparent', cursor: 'pointer', flexShrink: 0 }}
                >
                  {checked && <span style={{ color: '#fff', fontSize: '9px', lineHeight: 1, fontWeight: 700 }}>✓</span>}
                </span>
                <span onClick={() => onToggleSubscription(sub.id)} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, fontSize: '13px', color: 'var(--color-text-primary)' }} title={sub.name}>
                  {sub.name}
                </span>
                {hovered && (
                  <button onClick={() => onDeleteSubscription(sub.id)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '14px', padding: '0 2px', flexShrink: 0, lineHeight: 1 }} title="구독 취소">×</button>
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
  );
}

