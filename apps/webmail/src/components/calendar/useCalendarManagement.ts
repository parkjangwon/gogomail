'use client';
import { useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import { createCalendar, updateCalendar, deleteCalendar } from '@/lib/api';
import type { Calendar, CalendarObject } from '@/lib/api';

interface UseCalendarManagementParams {
  calendars: Calendar[];
  setCalendars: Dispatch<SetStateAction<Calendar[]>>;
  setObjects: Dispatch<SetStateAction<CalendarObject[]>>;
  setSelectedCalIds: Dispatch<SetStateAction<Set<string>>>;
  t: (key: string, values?: Record<string, any>) => string;
}

export interface UseCalendarManagementReturn {
  showCalModal: boolean;
  setShowCalModal: Dispatch<SetStateAction<boolean>>;
  editingCal: Calendar | null;
  calName: string;
  setCalName: Dispatch<SetStateAction<string>>;
  calColor: string;
  setCalColor: Dispatch<SetStateAction<string>>;
  calDesc: string;
  setCalDesc: Dispatch<SetStateAction<string>>;
  calSaving: boolean;
  calError: string;
  calHoverId: string | null;
  setCalHoverId: Dispatch<SetStateAction<string | null>>;
  CAL_COLORS: string[];
  openCalModal: (cal: Calendar | null) => void;
  handleCalSave: () => Promise<void>;
  handleCalDelete: () => Promise<void>;
}

export function useCalendarManagement({
  calendars,
  setCalendars,
  setObjects,
  setSelectedCalIds,
  t,
}: UseCalendarManagementParams): UseCalendarManagementReturn {
  const [showCalModal, setShowCalModal] = useState(false);
  const [editingCal, setEditingCal] = useState<Calendar | null>(null);
  const [calName, setCalName] = useState('');
  const [calColor, setCalColor] = useState('#2F6EE0');
  const [calDesc, setCalDesc] = useState('');
  const [calSaving, setCalSaving] = useState(false);
  const [calError, setCalError] = useState('');
  const [calHoverId, setCalHoverId] = useState<string | null>(null);

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

  return {
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
  };
}
