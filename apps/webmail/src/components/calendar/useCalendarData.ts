import { useState, useEffect, useCallback } from 'react';
import { Calendar, CalendarObject, CalendarSubscription, listCalendars, listCalendarObjects, listCalendarSubscriptions, addCalendarSubscription, deleteCalendarSubscription, fetchSubscriptionICS } from '@/lib/api';

export function useCalendarData() {
  const [calendars, setCalendars] = useState<Calendar[]>([]);
  const [objects, setObjects] = useState<CalendarObject[]>([]);
  const [selectedCalIds, setSelectedCalIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);

  const [subscriptions, setSubscriptions] = useState<CalendarSubscription[]>([]);
  const [selectedSubIds, setSelectedSubIds] = useState<Set<string>>(new Set());
  const [subICSCache, setSubICSCache] = useState<Map<string, string>>(new Map());

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

  const reloadObjects = useCallback(async () => {
    if (calendars.length === 0) return;
    const results = await Promise.all(calendars.map((c) => listCalendarObjects(c.ID)));
    setObjects(results.flat());
  }, [calendars]);

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

  const handleAddSubscription = async (url: string, name: string, color: string): Promise<CalendarSubscription> => {
    const sub = await addCalendarSubscription(url, name || url, color);
    setSubscriptions((prev) => [...prev, sub]);
    setSelectedSubIds((prev) => new Set(prev).add(sub.id));
    return sub;
  };

  const handleDeleteSubscription = async (id: string) => {
    try {
      await deleteCalendarSubscription(id);
      setSubscriptions((prev) => prev.filter((s) => s.id !== id));
      setSubICSCache((prev) => { const m = new Map(prev); m.delete(id); return m; });
      setSelectedSubIds((prev) => { const s = new Set(prev); s.delete(id); return s; });
    } catch { /* ignore */ }
  };

  return {
    calendars,
    setCalendars,
    objects,
    setObjects,
    selectedCalIds,
    setSelectedCalIds,
    loading,
    subscriptions,
    setSubscriptions,
    selectedSubIds,
    setSelectedSubIds,
    subICSCache,
    toggleCalendar,
    toggleSubscription,
    handleAddSubscription,
    handleDeleteSubscription,
    refresh: reloadObjects,
  };
}
