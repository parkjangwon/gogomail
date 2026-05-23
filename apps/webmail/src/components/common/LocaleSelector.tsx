'use client';

import { useState, useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';

const LOCALES = [
  { code: 'ko', label: '한국어' },
  { code: 'en', label: 'English' },
  { code: 'ja', label: '日本語' },
  { code: 'zh-CN', label: '中文(简体)' },
];

function getStoredLocale(): string {
  if (typeof document === 'undefined') return 'ko';
  const match = document.cookie.match(/(?:^|;\s*)webmail_locale=([^;]+)/);
  return match?.[1] ?? 'ko';
}

export function LocaleSelector() {
  const t = useTranslations('localeSelector');
  const [current, setCurrent] = useState('ko');
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setCurrent(getStoredLocale());
  }, []);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  function select(code: string) {
    document.cookie = `webmail_locale=${code}; path=/; max-age=31536000; SameSite=Lax`;
    setCurrent(code);
    setOpen(false);
    window.location.reload();
  }

  const currentLabel = LOCALES.find((l) => l.code === current)?.label ?? 'KO';

  return (
    <div ref={ref} style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen((o) => !o)}
        aria-label={t('aria')}
        aria-expanded={open}
        style={{
          height: '34px',
          padding: '0 10px',
          borderRadius: '8px',
          border: '1px solid var(--color-border-subtle)',
          background: 'var(--color-bg-secondary)',
          color: 'var(--color-text-secondary)',
          fontSize: '13px',
          fontWeight: 500,
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          transition: 'background 120ms ease, border-color 120ms ease',
          whiteSpace: 'nowrap',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = 'var(--color-bg-tertiary)';
          e.currentTarget.style.borderColor = 'var(--color-border-default)';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = 'var(--color-bg-secondary)';
          e.currentTarget.style.borderColor = 'var(--color-border-subtle)';
        }}
      >
        {currentLabel}
        <span aria-hidden="true" style={{ fontSize: '10px', opacity: 0.6 }}>▾</span>
      </button>

      {open && (
        <div
          role="listbox"
          aria-label={t('aria')}
          style={{
            position: 'absolute',
            top: 'calc(100% + 6px)',
            right: 0,
            minWidth: '130px',
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '8px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
            zIndex: 200,
            overflow: 'hidden',
          }}
        >
          {LOCALES.map((l) => (
            <button
              key={l.code}
              role="option"
              aria-selected={l.code === current}
              onClick={() => select(l.code)}
              style={{
                display: 'block',
                width: '100%',
                padding: '9px 14px',
                border: 'none',
                background: l.code === current ? 'var(--color-accent-subtle)' : 'transparent',
                color: l.code === current ? 'var(--color-accent)' : 'var(--color-text-primary)',
                fontSize: '13px',
                fontWeight: l.code === current ? 500 : 400,
                textAlign: 'left',
                cursor: 'pointer',
                transition: 'background 80ms ease',
              }}
              onMouseEnter={(e) => {
                if (l.code !== current)
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                if (l.code !== current)
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
              }}
            >
              {l.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
