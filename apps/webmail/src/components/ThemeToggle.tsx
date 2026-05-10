'use client';

import { useEffect, useState } from 'react';

type Theme = 'light' | 'dark';

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>('dark');

  useEffect(() => {
    const stored = localStorage.getItem('webmail_theme') as Theme | null;
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const resolved: Theme = stored ?? (prefersDark ? 'dark' : 'light');
    setTheme(resolved);
  }, []);

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    localStorage.setItem('webmail_theme', next);
    document.documentElement.setAttribute('data-theme', next);
  }

  const isDark = theme === 'dark';

  return (
    <button
      onClick={toggle}
      aria-label={isDark ? '라이트 모드로 전환' : '다크 모드로 전환'}
      title={isDark ? '라이트 모드' : '다크 모드'}
      style={{
        position: 'fixed',
        top: '14px',
        right: '16px',
        zIndex: 50,
        width: '34px',
        height: '34px',
        borderRadius: '8px',
        border: '1px solid var(--color-border-subtle)',
        background: 'var(--color-bg-secondary)',
        color: 'var(--color-text-secondary)',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: '16px',
        transition: 'background 120ms ease, border-color 120ms ease, transform 80ms ease',
        flexShrink: 0,
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = 'var(--color-bg-tertiary)';
        e.currentTarget.style.borderColor = 'var(--color-border-default)';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = 'var(--color-bg-secondary)';
        e.currentTarget.style.borderColor = 'var(--color-border-subtle)';
      }}
      onMouseDown={(e) => {
        e.currentTarget.style.transform = 'scale(0.92)';
      }}
      onMouseUp={(e) => {
        e.currentTarget.style.transform = 'scale(1)';
      }}
    >
      {isDark ? '☀️' : '🌙'}
    </button>
  );
}
