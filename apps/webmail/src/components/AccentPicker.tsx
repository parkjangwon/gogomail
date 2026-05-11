'use client';

import { useState, useEffect } from 'react';

interface AccentPreset {
  name: string;
  swatch: string;
  light: { accent: string; hover: string; subtle: string };
  dark: { accent: string; hover: string; subtle: string };
}

const PRESETS: AccentPreset[] = [
  { name: '블루', swatch: '#2F6EE0', light: { accent: '#2F6EE0', hover: '#2560C8', subtle: '#EBF1FD' }, dark: { accent: '#5B8EF0', hover: '#6B9AF4', subtle: '#1E2B45' } },
  { name: '틸', swatch: '#0D9488', light: { accent: '#0D9488', hover: '#0F766E', subtle: '#CCFBF1' }, dark: { accent: '#14B8A6', hover: '#2DD4BF', subtle: '#1A3D38' } },
  { name: '보라', swatch: '#7C3AED', light: { accent: '#7C3AED', hover: '#6D28D9', subtle: '#EDE9FE' }, dark: { accent: '#A78BFA', hover: '#C4B5FD', subtle: '#2E1B4E' } },
  { name: '주황', swatch: '#EA580C', light: { accent: '#EA580C', hover: '#C2410C', subtle: '#FFEDD5' }, dark: { accent: '#FB923C', hover: '#FDBA74', subtle: '#451A03' } },
  { name: '핑크', swatch: '#DB2777', light: { accent: '#DB2777', hover: '#BE185D', subtle: '#FCE7F3' }, dark: { accent: '#F472B6', hover: '#FBCFE8', subtle: '#4A1032' } },
];

const STORAGE_KEY = 'webmail_accent';

function applyAccent(preset: AccentPreset) {
  const id = 'webmail-accent-override';
  let el = document.getElementById(id) as HTMLStyleElement | null;
  if (!el) {
    el = document.createElement('style');
    el.id = id;
    document.head.appendChild(el);
  }
  el.textContent = `
    :root {
      --color-accent: ${preset.light.accent};
      --color-accent-hover: ${preset.light.hover};
      --color-accent-subtle: ${preset.light.subtle};
    }
    [data-theme="dark"] {
      --color-accent: ${preset.dark.accent};
      --color-accent-hover: ${preset.dark.hover};
      --color-accent-subtle: ${preset.dark.subtle};
    }
  `;
}

export function AccentPicker() {
  const [open, setOpen] = useState(false);
  const [activeIdx, setActiveIdx] = useState(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      return saved ? Math.max(0, PRESETS.findIndex((p) => p.swatch === saved)) : 0;
    } catch { return 0; }
  });

  useEffect(() => {
    applyAccent(PRESETS[activeIdx]);
  }, [activeIdx]);

  function select(idx: number) {
    setActiveIdx(idx);
    try { localStorage.setItem(STORAGE_KEY, PRESETS[idx].swatch); } catch { /* */ }
    setOpen(false);
  }

  return (
    <div style={{ position: 'relative' }}>
      <button
        aria-label="색상 테마 선택"
        title="색상 테마"
        onClick={() => setOpen((v) => !v)}
        style={{
          width: '22px',
          height: '22px',
          borderRadius: '50%',
          border: '2px solid var(--color-border-default)',
          background: PRESETS[activeIdx].swatch,
          cursor: 'pointer',
          padding: 0,
          boxShadow: open ? '0 0 0 2px var(--color-accent)' : 'none',
          transition: 'box-shadow 100ms ease',
        }}
      />
      {open && (
        <>
          <div
            aria-hidden="true"
            style={{ position: 'fixed', inset: 0, zIndex: 49 }}
            onClick={() => setOpen(false)}
          />
          <div style={{
            position: 'absolute',
            top: '130%',
            right: 0,
            zIndex: 50,
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '8px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
            padding: '10px 12px',
            display: 'flex',
            flexDirection: 'column',
            gap: '8px',
            minWidth: '120px',
          }}>
            <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 500, marginBottom: '2px' }}>색상 선택</span>
            <div style={{ display: 'flex', gap: '8px' }}>
              {PRESETS.map((p, i) => (
                <button
                  key={p.name}
                  aria-label={p.name}
                  title={p.name}
                  onClick={() => select(i)}
                  style={{
                    width: '22px',
                    height: '22px',
                    borderRadius: '50%',
                    border: i === activeIdx ? '2px solid var(--color-text-primary)' : '2px solid transparent',
                    background: p.swatch,
                    cursor: 'pointer',
                    padding: 0,
                    outline: 'none',
                    boxShadow: i === activeIdx ? '0 0 0 1px var(--color-bg-primary) inset' : 'none',
                    transition: 'border 80ms ease',
                  }}
                />
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
