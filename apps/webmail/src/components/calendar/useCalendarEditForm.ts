import { useState, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { Calendar, updateCalendarEvent, deleteCalendarObject } from '@/lib/api';
import { ParsedEvent } from '@/lib/calendar/eventParser';

export type RruleFreq = 'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';
export type RruleEnd = 'never' | 'count' | 'until';

interface UseCalendarEditFormParams {
  calendars: Calendar[];
  onUpdated: () => Promise<void>;
}

const pad2 = (n: number) => String(n).padStart(2, '0');
const toLocalDT = (d: Date) =>
  `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
const toLocalDate = (d: Date) =>
  `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;

export function useCalendarEditForm({ onUpdated }: UseCalendarEditFormParams) {
  const t = useTranslations('calendarFull');

  const [showEditModal, setShowEditModal] = useState(false);
  const [editingEvent, setEditingEvent] = useState<ParsedEvent | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editStart, setEditStart] = useState('');
  const [editEnd, setEditEnd] = useState('');
  const [editAllDay, setEditAllDay] = useState(false);
  const [editLocation, setEditLocation] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [editCalId, setEditCalId] = useState('');
  const [editSaving, setEditSaving] = useState(false);
  const [editError, setEditError] = useState('');

  const [editRrule, setEditRrule] = useState<RruleFreq>('NONE');
  const [editRruleInterval, setEditRruleInterval] = useState(1);
  const [editRruleEnd, setEditRruleEnd] = useState<RruleEnd>('never');
  const [editRruleCount, setEditRruleCount] = useState(10);
  const [editRruleUntil, setEditRruleUntil] = useState('');
  const [editRruleDays, setEditRruleDays] = useState<number[]>([]);

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

  const openEditModal = useCallback((ev: ParsedEvent) => {
    setEditingEvent(ev);
    setEditTitle(ev.summary === t('event.untitled') ? '' : ev.summary);
    setEditLocation(ev.location ?? '');
    setEditDesc(ev.description ?? '');
    setEditAllDay(ev.allDay);
    setEditCalId(ev.calendarId);
    setEditError('');
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
      const freq = (freqM?.[1] ?? 'NONE') as RruleFreq;
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

  const closeEditModal = () => {
    setShowEditModal(false);
    setEditingEvent(null);
  };

  const handleEditSubmit = useCallback(async () => {
    if (!editingEvent || !editTitle.trim()) { setEditError(t('event.titleRequired')); return; }
    const startDate = new Date(editAllDay ? editStart + 'T00:00:00' : editStart);
    const endDate = new Date(editAllDay ? editEnd + 'T00:00:00' : editEnd);
    if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) { setEditError(t('event.invalidDate')); return; }
    if (editAllDay ? endDate < startDate : endDate <= startDate) { setEditError(t('event.endBeforeStart')); return; }
    setEditSaving(true); setEditError('');
    try {
      const uid = editingEvent.obj.UID;
      const objectName = editingEvent.obj.ObjectName;
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
      await onUpdated();
    } catch (e) {
      setEditError(e instanceof Error ? e.message : t('event.editFailed'));
    } finally {
      setEditSaving(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingEvent, editTitle, editAllDay, editStart, editEnd, editLocation, editDesc, editCalId, editRrule, editRruleInterval, editRruleEnd, editRruleCount, editRruleUntil, editRruleDays, onUpdated]);

  const handleDeleteEvent = useCallback(async (ev: ParsedEvent) => {
    if (!window.confirm(t('event.confirmDelete', { summary: ev.summary }))) return;
    try {
      await deleteCalendarObject(ev.calendarId, ev.obj.ObjectName);
      await onUpdated();
    } catch { /* ignore */ }
  }, [onUpdated, t]);

  const handleEditAllDayToggle = (checked: boolean) => {
    setEditAllDay(checked);
    if (checked) {
      setEditStart(editStart.split('T')[0] || toLocalDate(new Date()));
      setEditEnd(editEnd.split('T')[0] || toLocalDate(new Date()));
    } else {
      setEditStart(editStart.includes('T') ? editStart : editStart + 'T09:00');
      setEditEnd(editEnd.includes('T') ? editEnd : editEnd + 'T10:00');
    }
  };

  return {
    showEditModal,
    editingEvent,
    editTitle, setEditTitle,
    editStart, setEditStart,
    editEnd, setEditEnd,
    editAllDay, setEditAllDay,
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
    handleDeleteEvent,
    handleEditAllDayToggle,
  };
}
