'use client';

import { useState, useEffect } from 'react';
import { XMarkIcon } from '@heroicons/react/24/outline';

interface WebmailSettings {
  readMark: 'instant' | '2s' | 'manual';
  listDensity: 'default' | 'compact';
  defaultSort: 'newest' | 'oldest';
  quoteOnReply: boolean;
  signature: string;
  theme: 'light' | 'dark' | 'system';
  notifications: boolean;
  accentColor: string;
  locale: string;
}

const DEFAULT_SETTINGS: WebmailSettings = {
  readMark: 'instant',
  listDensity: 'default',
  defaultSort: 'newest',
  quoteOnReply: true,
  signature: '',
  theme: 'system',
  notifications: false,
  accentColor: '#2F6EE0',
  locale: 'ko',
};

const ACCENT_PRESETS = [
  { name: '블루',  swatch: '#2F6EE0', light: { accent: '#2F6EE0', hover: '#2560C8', subtle: '#EBF1FD' }, dark: { accent: '#5B8EF0', hover: '#6B9AF4', subtle: '#1E2B45' } },
  { name: '틸',   swatch: '#0D9488', light: { accent: '#0D9488', hover: '#0F766E', subtle: '#CCFBF1' }, dark: { accent: '#14B8A6', hover: '#2DD4BF', subtle: '#1A3D38' } },
  { name: '보라', swatch: '#7C3AED', light: { accent: '#7C3AED', hover: '#6D28D9', subtle: '#EDE9FE' }, dark: { accent: '#A78BFA', hover: '#C4B5FD', subtle: '#2E1B4E' } },
  { name: '주황', swatch: '#EA580C', light: { accent: '#EA580C', hover: '#C2410C', subtle: '#FFEDD5' }, dark: { accent: '#FB923C', hover: '#FDBA74', subtle: '#451A03' } },
  { name: '핑크', swatch: '#DB2777', light: { accent: '#DB2777', hover: '#BE185D', subtle: '#FCE7F3' }, dark: { accent: '#F472B6', hover: '#FBCFE8', subtle: '#4A1032' } },
  { name: '그린', swatch: '#059669', light: { accent: '#059669', hover: '#047857', subtle: '#D1FAE5' }, dark: { accent: '#34D399', hover: '#6EE7B7', subtle: '#1A3B30' } },
];

function applyAccent(swatch: string) {
  const preset = ACCENT_PRESETS.find((p) => p.swatch === swatch) ?? ACCENT_PRESETS[0];
  const id = 'webmail-accent-override';
  let el = document.getElementById(id) as HTMLStyleElement | null;
  if (!el) { el = document.createElement('style'); el.id = id; document.head.appendChild(el); }
  el.textContent = `:root { --color-accent: ${preset.light.accent}; --color-accent-hover: ${preset.light.hover}; --color-accent-subtle: ${preset.light.subtle}; } [data-theme="dark"] { --color-accent: ${preset.dark.accent}; --color-accent-hover: ${preset.dark.hover}; --color-accent-subtle: ${preset.dark.subtle}; }`;
  try { localStorage.setItem('webmail_accent', swatch); } catch { /* */ }
}

function getInitialLocale(): string {
  if (typeof document === 'undefined') return 'ko';
  const match = document.cookie.match(/(?:^|;\s*)webmail_locale=([^;]+)/);
  return match?.[1] ?? 'ko';
}

function loadSettings(): WebmailSettings {
  try {
    const raw = localStorage.getItem('webmail_settings');
    return { ...DEFAULT_SETTINGS, locale: getInitialLocale(), ...(raw ? JSON.parse(raw) : {}) };
  } catch { /* */ }
  return { ...DEFAULT_SETTINGS, locale: getInitialLocale() };
}

function saveSettings(s: WebmailSettings) {
  try {
    localStorage.setItem('webmail_settings', JSON.stringify(s));
  } catch { /* */ }
}

type Category = 'mailbox' | 'compose' | 'theme' | 'notifications' | 'account';

const CATEGORIES: { id: Category; label: string }[] = [
  { id: 'mailbox', label: '메일함' },
  { id: 'compose', label: '메일 쓰기' },
  { id: 'theme', label: '테마' },
  { id: 'notifications', label: '알림' },
  { id: 'account', label: '계정' },
];

interface SettingsModalProps {
  onClose: () => void;
  userEmail?: string;
}

export function SettingsModal({ onClose, userEmail }: SettingsModalProps) {
  const [activeCategory, setActiveCategory] = useState<Category>('mailbox');
  const [settings, setSettings] = useState<WebmailSettings>(DEFAULT_SETTINGS);
  const [hoveredCategory, setHoveredCategory] = useState<Category | null>(null);

  useEffect(() => {
    setSettings(loadSettings());
  }, []);

  useEffect(() => {
    if (settings.accentColor) applyAccent(settings.accentColor);
  }, [settings.accentColor]);

  function update<K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) {
    setSettings((prev) => {
      const next = { ...prev, [key]: value };
      saveSettings(next);
      return next;
    });
  }

  function applyTheme(theme: WebmailSettings['theme']) {
    update('theme', theme);
    if (theme === 'dark') {
      document.documentElement.setAttribute('data-theme', 'dark');
    } else if (theme === 'light') {
      document.documentElement.setAttribute('data-theme', 'light');
    } else {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    }
  }

  function handleNotificationToggle(checked: boolean) {
    if (checked && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
      Notification.requestPermission().then((perm) => {
        update('notifications', perm === 'granted');
      });
    } else {
      update('notifications', checked);
    }
  }

  const labelStyle: React.CSSProperties = {
    fontSize: '13px',
    fontWeight: 500,
    color: 'var(--color-text-primary)',
    marginBottom: '8px',
    display: 'block',
  };

  const sectionStyle: React.CSSProperties = {
    marginBottom: '24px',
  };

  const radioGroupStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: '6px',
  };

  const radioLabelStyle: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    fontSize: '13px',
    color: 'var(--color-text-secondary)',
    cursor: 'pointer',
  };

  function renderContent() {
    switch (activeCategory) {
      case 'mailbox':
        return (
          <>
            <div style={sectionStyle}>
              <span style={labelStyle}>읽음 처리</span>
              <div style={radioGroupStyle}>
                {([['instant', '즉시'], ['2s', '2초 후'], ['manual', '수동']] as const).map(([val, lbl]) => (
                  <label key={val} style={radioLabelStyle}>
                    <input
                      type="radio"
                      name="readMark"
                      value={val}
                      checked={settings.readMark === val}
                      onChange={() => update('readMark', val)}
                    />
                    {lbl}
                  </label>
                ))}
              </div>
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>목록 밀도</span>
              <div style={radioGroupStyle}>
                {([['default', '기본'], ['compact', '컴팩트']] as const).map(([val, lbl]) => (
                  <label key={val} style={radioLabelStyle}>
                    <input
                      type="radio"
                      name="listDensity"
                      value={val}
                      checked={settings.listDensity === val}
                      onChange={() => update('listDensity', val)}
                    />
                    {lbl}
                  </label>
                ))}
              </div>
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>기본 정렬</span>
              <div style={radioGroupStyle}>
                {([['newest', '최신순'], ['oldest', '오래된순']] as const).map(([val, lbl]) => (
                  <label key={val} style={radioLabelStyle}>
                    <input
                      type="radio"
                      name="defaultSort"
                      value={val}
                      checked={settings.defaultSort === val}
                      onChange={() => update('defaultSort', val)}
                    />
                    {lbl}
                  </label>
                ))}
              </div>
            </div>
          </>
        );
      case 'compose':
        return (
          <>
            <div style={sectionStyle}>
              <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={settings.quoteOnReply}
                  onChange={(e) => update('quoteOnReply', e.target.checked)}
                />
                <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>
                  회신 시 인용문 포함
                </span>
              </label>
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>서명</span>
              <textarea
                value={settings.signature}
                onChange={(e) => update('signature', e.target.value)}
                maxLength={500}
                rows={5}
                placeholder="서명을 입력하세요..."
                style={{
                  width: '100%',
                  fontSize: '13px',
                  padding: '8px 10px',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '6px',
                  background: 'var(--color-bg-secondary)',
                  color: 'var(--color-text-primary)',
                  resize: 'vertical',
                  outline: 'none',
                  fontFamily: 'inherit',
                  boxSizing: 'border-box',
                }}
              />
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '4px', textAlign: 'right' }}>
                {settings.signature.length}/500
              </div>
            </div>
          </>
        );
      case 'theme':
        return (
          <>
            <div style={sectionStyle}>
              <span style={labelStyle}>테마</span>
              <div style={radioGroupStyle}>
                {([['light', '라이트'], ['dark', '다크'], ['system', '시스템']] as const).map(([val, lbl]) => (
                  <label key={val} style={radioLabelStyle}>
                    <input
                      type="radio"
                      name="theme"
                      value={val}
                      data-theme={val}
                      checked={settings.theme === val}
                      onChange={() => applyTheme(val)}
                    />
                    {lbl}
                  </label>
                ))}
              </div>
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>프라이머리 색상</span>
              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                {ACCENT_PRESETS.map((preset) => (
                  <button
                    key={preset.swatch}
                    title={preset.name}
                    onClick={() => { applyAccent(preset.swatch); update('accentColor', preset.swatch); }}
                    style={{
                      width: '28px', height: '28px', borderRadius: '50%',
                      background: preset.swatch, border: 'none', cursor: 'pointer',
                      boxShadow: settings.accentColor === preset.swatch
                        ? `0 0 0 2px var(--color-bg-primary), 0 0 0 4px ${preset.swatch}`
                        : 'none',
                      transition: 'box-shadow 100ms ease',
                    }}
                  />
                ))}
              </div>
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>언어</span>
              <div style={radioGroupStyle}>
                {([['ko', '한국어'], ['en', 'English'], ['ja', '日本語'], ['zh-CN', '中文(简体)']] as const).map(([code, label]) => (
                  <label key={code} style={radioLabelStyle}>
                    <input
                      type="radio"
                      name="locale"
                      value={code}
                      checked={settings.locale === code}
                      onChange={() => {
                        update('locale', code);
                        document.cookie = `webmail_locale=${code}; path=/; max-age=31536000; SameSite=Lax`;
                        window.location.reload();
                      }}
                    />
                    {label}
                  </label>
                ))}
              </div>
            </div>
          </>
        );
      case 'notifications':
        return (
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={settings.notifications}
                onChange={(e) => handleNotificationToggle(e.target.checked)}
              />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>
                새 메일 알림
              </span>
            </label>
            {settings.notifications && (
              <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '8px', marginLeft: '24px' }}>
                브라우저 알림이 활성화되어 있습니다.
              </p>
            )}
          </div>
        );
      case 'account':
        return (
          <>
            <div style={sectionStyle}>
              <span style={labelStyle}>이메일</span>
              <input
                type="email"
                readOnly
                value={userEmail ?? ''}
                style={{
                  width: '100%',
                  fontSize: '13px',
                  padding: '8px 10px',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '6px',
                  background: 'var(--color-bg-tertiary)',
                  color: 'var(--color-text-secondary)',
                  outline: 'none',
                  boxSizing: 'border-box',
                  cursor: 'default',
                }}
              />
            </div>
            <div style={sectionStyle}>
              <span style={labelStyle}>표시 이름</span>
              <input
                type="text"
                readOnly
                value={userEmail ? userEmail.split('@')[0] : ''}
                style={{
                  width: '100%',
                  fontSize: '13px',
                  padding: '8px 10px',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '6px',
                  background: 'var(--color-bg-tertiary)',
                  color: 'var(--color-text-secondary)',
                  outline: 'none',
                  boxSizing: 'border-box',
                  cursor: 'default',
                }}
              />
            </div>
          </>
        );
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="설정"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        style={{
          width: '680px',
          height: '520px',
          borderRadius: '12px',
          background: 'var(--color-bg-primary)',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 20px 60px rgba(0,0,0,0.25)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--color-border-subtle)',
            flexShrink: 0,
          }}
        >
          <span style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>설정</span>
          <button
            aria-label="닫기"
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--color-text-tertiary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '4px',
              borderRadius: '6px',
            }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Body */}
        <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
          {/* Left nav */}
          <div
            style={{
              width: '160px',
              flexShrink: 0,
              borderRight: '1px solid var(--color-border-subtle)',
              padding: '8px 0',
              overflowY: 'auto',
            }}
          >
            {CATEGORIES.map(({ id, label }) => {
              const isActive = activeCategory === id;
              const isHovered = hoveredCategory === id;
              return (
                <button
                  key={id}
                  onClick={() => setActiveCategory(id)}
                  onMouseEnter={() => setHoveredCategory(id)}
                  onMouseLeave={() => setHoveredCategory(null)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    padding: '9px 16px',
                    fontSize: '13px',
                    fontWeight: isActive ? 500 : 400,
                    color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                    background: isActive
                      ? 'var(--color-bg-tertiary)'
                      : isHovered
                      ? 'var(--color-bg-secondary)'
                      : 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    transition: 'background 100ms ease',
                  }}
                >
                  {label}
                </button>
              );
            })}
          </div>

          {/* Right content */}
          <div
            style={{
              flex: 1,
              overflowY: 'auto',
              padding: '24px',
            }}
          >
            {renderContent()}
          </div>
        </div>
      </div>
    </div>
  );
}
