'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { getAllTimezones, POPULAR_TIMEZONES } from '@/lib/timezone';
import type { TzOption } from '@/lib/timezone';

interface TimezoneSelectProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

export function TimezoneSelect({ value, onChange, placeholder = '타임존 검색…' }: TimezoneSelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const allZones = useMemo(() => getAllTimezones(), []);

  const popularOptions = useMemo(() =>
    POPULAR_TIMEZONES
      .map((tz) => allZones.find((z) => z.value === tz))
      .filter((z): z is TzOption => !!z),
    [allZones],
  );

  const filteredOptions = useMemo(() => {
    if (!search.trim()) return null; // null = show popular
    const q = search.toLowerCase().replace(/\s/g, '_');
    return allZones.filter((z) =>
      z.value.toLowerCase().includes(q) || z.label.toLowerCase().includes(q),
    ).slice(0, 80);
  }, [search, allZones]);

  const displayOptions = filteredOptions ?? popularOptions;
  const selectedLabel = allZones.find((z) => z.value === value)?.label ?? value;

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
        setSearch('');
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  function handleOpen() {
    setOpen(true);
    setSearch('');
    setTimeout(() => inputRef.current?.focus(), 0);
  }

  function handleSelect(tz: string) {
    onChange(tz);
    setOpen(false);
    setSearch('');
  }

  return (
    <div ref={containerRef} style={{ position: 'relative', minWidth: '260px' }}>
      {/* Trigger button */}
      <button
        type="button"
        onClick={handleOpen}
        style={{
          width: '100%', textAlign: 'left',
          padding: '6px 10px', borderRadius: '6px',
          border: '1px solid var(--color-border-default)',
          background: 'var(--color-bg-secondary)',
          color: 'var(--color-text-primary)',
          fontSize: '13px', cursor: 'pointer',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px',
        }}
      >
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {selectedLabel}
        </span>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style={{ flexShrink: 0, opacity: 0.5 }}>
          <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>

      {/* Dropdown */}
      {open && (
        <div style={{
          position: 'absolute', top: 'calc(100% + 4px)', left: 0, right: 0,
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-default)',
          borderRadius: '8px',
          boxShadow: '0 8px 24px rgba(0,0,0,0.14)',
          zIndex: 500, overflow: 'hidden',
          minWidth: '300px',
        }}>
          {/* Search input */}
          <div style={{ padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)' }}>
            <input
              ref={inputRef}
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={placeholder}
              style={{
                width: '100%', padding: '5px 8px',
                border: '1px solid var(--color-border-default)',
                borderRadius: '5px',
                background: 'var(--color-bg-secondary)',
                color: 'var(--color-text-primary)',
                fontSize: '12px', outline: 'none',
              }}
            />
          </div>

          {/* Section label */}
          {!filteredOptions && (
            <div style={{ padding: '4px 12px 2px', fontSize: '10px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>
              주요 타임존
            </div>
          )}
          {filteredOptions && filteredOptions.length === 0 && (
            <div style={{ padding: '14px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)', textAlign: 'center' }}>
              검색 결과 없음
            </div>
          )}

          {/* Options list */}
          <div
            ref={listRef}
            style={{ maxHeight: '240px', overflowY: 'auto' }}
          >
            {displayOptions.map((tz) => {
              const isSelected = tz.value === value;
              return (
                <button
                  key={tz.value}
                  type="button"
                  onClick={() => handleSelect(tz.value)}
                  style={{
                    display: 'block', width: '100%', textAlign: 'left',
                    padding: '7px 12px', border: 'none', cursor: 'pointer',
                    background: isSelected ? 'var(--color-accent-subtle)' : 'transparent',
                    color: isSelected ? 'var(--color-accent)' : 'var(--color-text-primary)',
                    fontSize: '12px', fontWeight: isSelected ? 600 : 400,
                  }}
                  onMouseEnter={(e) => { if (!isSelected) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { if (!isSelected) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  {tz.label}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
