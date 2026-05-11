'use client';

import { useRef, useEffect } from 'react';
import { ClockIcon } from '@heroicons/react/24/outline';

export const SNOOZE_KEY = 'webmail_snoozed';

export function loadSnoozed(): Record<string, string> {
  try { return JSON.parse(localStorage.getItem(SNOOZE_KEY) ?? '{}') as Record<string, string>; } catch { return {}; }
}
export function snoozeMessage(id: string, until: Date) {
  try {
    const s = loadSnoozed();
    s[id] = until.toISOString();
    localStorage.setItem(SNOOZE_KEY, JSON.stringify(s));
  } catch { /* ignore */ }
}
export function unsnoozeMessage(id: string) {
  try {
    const s = loadSnoozed();
    delete s[id];
    localStorage.setItem(SNOOZE_KEY, JSON.stringify(s));
  } catch { /* ignore */ }
}
export function isCurrentlySnoozed(id: string, snoozed: Record<string, string>): boolean {
  const until = snoozed[id];
  if (!until) return false;
  return new Date(until) > new Date();
}

function buildPresets(): { label: string; sub: string; date: Date }[] {
  const now = new Date();
  const presets: { label: string; sub: string; date: Date }[] = [];

  // 1 hour from now
  const in1h = new Date(now.getTime() + 60 * 60 * 1000);
  presets.push({ label: '1시간 후', sub: formatTime(in1h), date: in1h });

  // Tonight 9pm
  const tonight = new Date(now);
  tonight.setHours(21, 0, 0, 0);
  if (tonight > now) presets.push({ label: '오늘 저녁', sub: '오후 9:00', date: tonight });

  // Tomorrow morning 8am
  const tomorrow = new Date(now);
  tomorrow.setDate(tomorrow.getDate() + 1);
  tomorrow.setHours(8, 0, 0, 0);
  presets.push({ label: '내일 아침', sub: formatDate(tomorrow) + ' 오전 8:00', date: tomorrow });

  // This weekend (next Saturday 9am)
  const weekend = new Date(now);
  const dow = weekend.getDay();
  const daysToSat = (6 - dow + 7) % 7 || 7;
  weekend.setDate(weekend.getDate() + daysToSat);
  weekend.setHours(9, 0, 0, 0);
  presets.push({ label: '이번 주말', sub: formatDate(weekend) + ' 오전 9:00', date: weekend });

  // Next Monday 8am
  const nextMon = new Date(now);
  const daysToMon = (8 - nextMon.getDay()) % 7 || 7;
  nextMon.setDate(nextMon.getDate() + daysToMon);
  nextMon.setHours(8, 0, 0, 0);
  presets.push({ label: '다음 주', sub: formatDate(nextMon) + ' 오전 8:00', date: nextMon });

  return presets;
}

function formatTime(d: Date): string {
  return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' });
}
function formatDate(d: Date): string {
  return d.toLocaleDateString('ko-KR', { month: 'long', day: 'numeric', weekday: 'short' });
}

interface SnoozePopoverProps {
  onSnooze: (until: Date) => void;
  onClose: () => void;
  align?: 'left' | 'right';
}

export function SnoozePopover({ onSnooze, onClose, align = 'right' }: SnoozePopoverProps) {
  const ref = useRef<HTMLDivElement>(null);
  const presets = buildPresets();

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onClose]);

  return (
    <div
      ref={ref}
      style={{
        position: 'absolute',
        top: '100%',
        [align === 'right' ? 'right' : 'left']: 0,
        marginTop: '4px',
        background: 'var(--color-bg-primary)',
        border: '1px solid var(--color-border-default)',
        borderRadius: '10px',
        boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
        zIndex: 600,
        minWidth: '220px',
        overflow: 'hidden',
      }}
    >
      <div style={{ padding: '8px 12px 4px', display: 'flex', alignItems: 'center', gap: '6px', borderBottom: '1px solid var(--color-border-subtle)' }}>
        <ClockIcon style={{ width: 13, height: 13, color: 'var(--color-text-tertiary)' }} />
        <span style={{ fontSize: '11px', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.07em', color: 'var(--color-text-tertiary)' }}>다시 알림</span>
      </div>
      {presets.map((p) => (
        <button
          key={p.label}
          onClick={() => { onSnooze(p.date); onClose(); }}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            width: '100%', padding: '9px 14px',
            border: 'none', background: 'transparent',
            color: 'var(--color-text-primary)', fontSize: '13px',
            cursor: 'pointer', textAlign: 'left', gap: '12px',
            transition: 'background 80ms ease',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <span style={{ fontWeight: 500 }}>{p.label}</span>
          <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>{p.sub}</span>
        </button>
      ))}
    </div>
  );
}
