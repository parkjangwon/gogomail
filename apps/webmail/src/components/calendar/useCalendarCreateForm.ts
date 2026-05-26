import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Calendar, createCalendarEvent } from '@/lib/api';

export type RruleFreq = 'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';
export type RruleEnd = 'never' | 'count' | 'until';

interface UseCalendarCreateFormParams {
  calendars: Calendar[];
  onCreated: () => Promise<void>;
}

const pad2 = (n: number) => String(n).padStart(2, '0');
const toLocalDT = (d: Date) =>
  `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
const toLocalDate = (d: Date) =>
  `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;

export function useCalendarCreateForm({ calendars, onCreated }: UseCalendarCreateFormParams) {
  const t = useTranslations('calendarFull');

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
  const [createRrule, setCreateRrule] = useState<RruleFreq>('NONE');
  const [createRruleInterval, setCreateRruleInterval] = useState(1);
  const [createRruleEnd, setCreateRruleEnd] = useState<RruleEnd>('never');
  const [createRruleCount, setCreateRruleCount] = useState(10);
  const [createRruleUntil, setCreateRruleUntil] = useState('');
  const [createRruleDays, setCreateRruleDays] = useState<number[]>([]);

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

  const openCreateModal = (baseDate?: Date, currentDate?: Date) => {
    const base = baseDate ?? currentDate ?? new Date();
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

  const closeCreateModal = () => setShowCreateModal(false);

  const handleCreateSubmit = async () => {
    if (!createTitle.trim()) { setCreateError(t('event.titleRequired')); return; }
    if (!createCalId) { setCreateError(t('event.calRequired')); return; }
    const startDate = new Date(createAllDay ? createStart + 'T00:00:00' : createStart);
    const endDate = new Date(createAllDay ? createEnd + 'T00:00:00' : createEnd);
    if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) { setCreateError(t('event.invalidDate')); return; }
    if (endDate <= startDate) { setCreateError(t('event.endBeforeStart')); return; }
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
      await onCreated();
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : t('event.saveFailed'));
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

  return {
    showCreateModal,
    createTitle, setCreateTitle,
    createStart, setCreateStart,
    createEnd, setCreateEnd,
    createAllDay, setCreateAllDay,
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
    openCreateModal,
    closeCreateModal,
    handleCreateSubmit,
    handleCreateAllDayToggle,
  };
}
