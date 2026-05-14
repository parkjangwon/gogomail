'use client';

import { useState, useEffect } from 'react';
import { startOfMonth, startOfWeek, isSameDay, addDays } from '@/lib/calendar/dateUtils';

export interface MiniCalendarProps {
  selectedDate: Date;
  today: Date;
  onDateSelect: (d: Date) => void;
}

export function MiniCalendar({ selectedDate, today, onDateSelect }: MiniCalendarProps) {
  const [viewMonth, setViewMonth] = useState<Date>(() => {
    const d = new Date(selectedDate);
    d.setDate(1);
    d.setHours(0, 0, 0, 0);
    return d;
  });

  useEffect(() => {
    setViewMonth((prev) => {
      if (
        prev.getFullYear() === selectedDate.getFullYear() &&
        prev.getMonth() === selectedDate.getMonth()
      ) return prev;
      const d = new Date(selectedDate);
      d.setDate(1);
      d.setHours(0, 0, 0, 0);
      return d;
    });
  }, [selectedDate]);

  const month = viewMonth.getMonth();
  const firstDay = startOfMonth(viewMonth);
  const gridStart = startOfWeek(firstDay);
  const days: Date[] = [];
  for (let i = 0; i < 42; i++) days.push(addDays(gridStart, i));
  const needed = days.findLastIndex((d) => d.getMonth() === month || d <= firstDay) + 1;
  const cellCount = Math.ceil(Math.max(needed, 28) / 7) * 7;
  const visibleDays = days.slice(0, cellCount);
  const weekDays = ['월', '화', '수', '목', '금', '토', '일'];

  return (
    <div style={{ padding: '10px 8px 6px', userSelect: 'none' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
        <button
          onClick={() => setViewMonth((d) => { const c = new Date(d); c.setMonth(c.getMonth() - 1); return c; })}
          style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '16px', padding: '2px 6px', borderRadius: '4px', lineHeight: 1 }}
          aria-label="이전 달"
        >‹</button>
        <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)' }}>
          {viewMonth.getFullYear()}년 {viewMonth.getMonth() + 1}월
        </span>
        <button
          onClick={() => setViewMonth((d) => { const c = new Date(d); c.setMonth(c.getMonth() + 1); return c; })}
          style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '16px', padding: '2px 6px', borderRadius: '4px', lineHeight: 1 }}
          aria-label="다음 달"
        >›</button>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', marginBottom: '2px' }}>
        {weekDays.map((wd) => (
          <div key={wd} style={{ textAlign: 'center', fontSize: '10px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '2px 0' }}>
            {wd}
          </div>
        ))}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)' }}>
        {visibleDays.map((day, idx) => {
          const isCurrentMonth = day.getMonth() === month;
          const isToday = isSameDay(day, today);
          const isSelected = isSameDay(day, selectedDate) && !isToday;
          return (
            <button
              key={idx}
              onClick={() => onDateSelect(day)}
              style={{
                background: isToday ? 'var(--color-accent)' : isSelected ? 'var(--color-bg-tertiary)' : 'none',
                border: 'none',
                borderRadius: '50%',
                width: '26px',
                height: '26px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '11px',
                fontWeight: isToday ? 700 : 400,
                color: isToday ? '#fff' : isCurrentMonth ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)',
                cursor: 'pointer',
                margin: '1px auto',
                padding: 0,
                opacity: !isCurrentMonth ? 0.45 : 1,
              }}
            >
              {day.getDate()}
            </button>
          );
        })}
      </div>
    </div>
  );
}
