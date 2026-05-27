'use client';
import { useState, useCallback } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import {
  createCalendarTodo,
  setTodoStatus,
  deleteCalendarObject,
} from '@/lib/api';
import type { Calendar } from '@/lib/api';
import type { ParsedTodo } from '@/lib/calendar/eventParser';

interface UseCalendarTodosParams {
  calendars: Calendar[];
  refresh: () => Promise<void>;
}

export interface UseCalendarTodosReturn {
  todoDraft: string;
  setTodoDraft: Dispatch<SetStateAction<string>>;
  todoFocused: boolean;
  setTodoFocused: Dispatch<SetStateAction<boolean>>;
  todoDueDate: string;
  setTodoDueDate: Dispatch<SetStateAction<string>>;
  todoTogglingId: string | null;
  todoDeleteId: string | null;
  todoHoverId: string | null;
  setTodoHoverId: Dispatch<SetStateAction<string | null>>;
  quickCreate: { day: Date; rect: DOMRect } | null;
  setQuickCreate: Dispatch<SetStateAction<{ day: Date; rect: DOMRect } | null>>;
  showTodoModal: boolean;
  setShowTodoModal: Dispatch<SetStateAction<boolean>>;
  handleToggleTodo: (todo: ParsedTodo) => Promise<void>;
  handleDeleteTodo: (todo: ParsedTodo) => Promise<void>;
  handleCreateTodo: () => Promise<void>;
  handleCellClick: (day: Date, rect: DOMRect) => void;
  handleQuickSaveTodo: (title: string, day: Date) => Promise<void>;
}

export function useCalendarTodos({
  calendars,
  refresh,
}: UseCalendarTodosParams): UseCalendarTodosReturn {
  const [todoDraft, setTodoDraft] = useState('');
  const [todoFocused, setTodoFocused] = useState(false);
  const [todoDueDate, setTodoDueDate] = useState('');
  const [todoTogglingId, setTodoTogglingId] = useState<string | null>(null);
  const [todoDeleteId, setTodoDeleteId] = useState<string | null>(null);
  const [todoHoverId, setTodoHoverId] = useState<string | null>(null);
  const [quickCreate, setQuickCreate] = useState<{ day: Date; rect: DOMRect } | null>(null);
  const [showTodoModal, setShowTodoModal] = useState(false);

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

  const handleCellClick = useCallback((day: Date, rect: DOMRect) => {
    setQuickCreate({ day, rect });
  }, []);

  const handleQuickSaveTodo = useCallback(async (title: string, day: Date) => {
    if (calendars.length === 0) return;
    await createCalendarTodo({ title, due: day, calendarId: calendars[0].ID });
    await refresh();
  }, [calendars, refresh]);

  return {
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
  };
}
