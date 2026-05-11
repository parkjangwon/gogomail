'use client';

import { useState, useEffect } from 'react';
import { CheckIcon } from '@heroicons/react/24/outline';

interface SettingsViewProps {
  userEmail?: string;
  userName?: string;
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: '12px' }}>
      {children}
    </div>
  );
}

function SettingRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '24px', padding: '12px 0', borderBottom: '1px solid var(--color-border-subtle)' }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: '14px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{label}</div>
        {description && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{description}</div>}
      </div>
      <div style={{ flexShrink: 0 }}>{children}</div>
    </div>
  );
}

function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      role="switch"
      aria-checked={value}
      onClick={() => onChange(!value)}
      style={{
        width: '40px', height: '22px', borderRadius: '11px',
        background: value ? 'var(--color-accent)' : 'var(--color-bg-tertiary)',
        border: 'none', cursor: 'pointer', position: 'relative',
        transition: 'background 150ms ease', flexShrink: 0,
      }}
    >
      <span style={{
        position: 'absolute', top: '3px',
        left: value ? '21px' : '3px',
        width: '16px', height: '16px', borderRadius: '50%',
        background: '#fff',
        boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
        transition: 'left 150ms ease',
      }} />
    </button>
  );
}

const ACCENT_COLORS = [
  { value: '#2563eb', label: '파랑' },
  { value: '#7c3aed', label: '보라' },
  { value: '#0d9488', label: '청록' },
  { value: '#16a34a', label: '초록' },
  { value: '#dc2626', label: '빨강' },
];

export function SettingsView({ userEmail, userName }: SettingsViewProps) {
  const [signature, setSignature] = useState('');
  const [sigSaved, setSigSaved] = useState(false);
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [accent, setAccent] = useState('#2563eb');
  const [compact, setCompact] = useState(false);
  const [convMode, setConvMode] = useState(true);
  const [notifPerm, setNotifPerm] = useState<NotificationPermission>('default');
  const [displayName, setDisplayName] = useState('');
  const [nameSaved, setNameSaved] = useState(false);

  useEffect(() => {
    try {
      setSignature(localStorage.getItem('webmail_signature') ?? '');
      setTheme((localStorage.getItem('webmail_theme') as 'light' | 'dark') ?? 'light');
      setAccent(localStorage.getItem('webmail_accent') ?? '#2563eb');
      setCompact(localStorage.getItem('webmail_compact') === '1');
      setConvMode(localStorage.getItem('webmail_conv_mode') !== '0');
      setDisplayName(localStorage.getItem('webmail_display_name') ?? userName ?? '');
    } catch { /* ignore */ }
    if (typeof Notification !== 'undefined') setNotifPerm(Notification.permission);
  }, [userName]);

  function saveSignature() {
    try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ }
    setSigSaved(true);
    setTimeout(() => setSigSaved(false), 2000);
  }

  function saveDisplayName() {
    try { localStorage.setItem('webmail_display_name', displayName); } catch { /* ignore */ }
    setNameSaved(true);
    setTimeout(() => setNameSaved(false), 2000);
  }

  function applyTheme(t: 'light' | 'dark') {
    setTheme(t);
    try { localStorage.setItem('webmail_theme', t); } catch { /* ignore */ }
    document.documentElement.setAttribute('data-theme', t);
  }

  function applyAccent(color: string) {
    setAccent(color);
    try { localStorage.setItem('webmail_accent', color); } catch { /* ignore */ }
    document.documentElement.style.setProperty('--color-accent', color);
    const hex = color.replace('#', '');
    const r = parseInt(hex.slice(0, 2), 16);
    const g = parseInt(hex.slice(2, 4), 16);
    const b = parseInt(hex.slice(4, 6), 16);
    document.documentElement.style.setProperty('--color-accent-subtle', `rgba(${r},${g},${b},0.1)`);
  }

  function applyCompact(v: boolean) {
    setCompact(v);
    try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* ignore */ }
  }

  function applyConvMode(v: boolean) {
    setConvMode(v);
    try { localStorage.setItem('webmail_conv_mode', v ? '1' : '0'); } catch { /* ignore */ }
  }

  async function requestNotif() {
    if (typeof Notification === 'undefined') return;
    const p = await Notification.requestPermission();
    setNotifPerm(p);
  }

  return (
    <div style={{ flex: 1, minWidth: 0, height: '100%', overflowY: 'auto', background: 'var(--color-bg-primary)', display: 'flex', justifyContent: 'center' }}>
      <div style={{ width: '100%', maxWidth: '640px', padding: '32px 24px' }}>
        <h1 style={{ fontSize: '20px', fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: '32px' }}>설정</h1>

        {/* Account */}
        <section style={{ marginBottom: '32px' }}>
          <SectionTitle>계정</SectionTitle>
          <div style={{ padding: '16px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', marginBottom: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
              <div style={{ width: '44px', height: '44px', borderRadius: '50%', background: 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px', fontWeight: 700, flexShrink: 0 }}>
                {(displayName || userEmail || '?')[0].toUpperCase()}
              </div>
              <div>
                <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{displayName || userName || '(이름 없음)'}</div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{userEmail}</div>
              </div>
            </div>
          </div>
          <SettingRow label="표시 이름" description="메일 발송 시 표시되는 이름">
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="이름 입력"
                style={{ padding: '5px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '160px' }}
              />
              <button
                onClick={saveDisplayName}
                style={{ padding: '5px 12px', borderRadius: '5px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px' }}
              >
                {nameSaved ? <><CheckIcon style={{ width: '14px', height: '14px' }} /> 저장됨</> : '저장'}
              </button>
            </div>
          </SettingRow>
        </section>

        {/* Signature */}
        <section style={{ marginBottom: '32px' }}>
          <SectionTitle>서명</SectionTitle>
          <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '8px' }}>
            메일 작성 시 자동으로 추가되는 서명입니다.
          </div>
          <textarea
            value={signature}
            onChange={(e) => setSignature(e.target.value)}
            placeholder="서명을 입력하세요 (예: 감사합니다. 홍길동 드림)"
            rows={5}
            style={{
              width: '100%', padding: '10px 12px', borderRadius: '6px',
              border: '1px solid var(--color-border-default)',
              background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)',
              fontSize: '13px', lineHeight: 1.6, resize: 'vertical', boxSizing: 'border-box',
              fontFamily: 'inherit',
            }}
          />
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '8px' }}>
            <button
              onClick={saveSignature}
              style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '5px' }}
            >
              {sigSaved ? <><CheckIcon style={{ width: '14px', height: '14px' }} /> 저장됨</> : '서명 저장'}
            </button>
          </div>
        </section>

        {/* Appearance */}
        <section style={{ marginBottom: '32px' }}>
          <SectionTitle>외관</SectionTitle>
          <SettingRow label="테마" description="다크/라이트 모드">
            <div style={{ display: 'flex', gap: '6px' }}>
              {(['light', 'dark'] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => applyTheme(t)}
                  style={{
                    padding: '5px 14px', borderRadius: '6px', border: `1.5px solid ${theme === t ? 'var(--color-accent)' : 'var(--color-border-default)'}`,
                    background: theme === t ? 'var(--color-accent-subtle)' : 'transparent',
                    color: theme === t ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                    fontSize: '12px', fontWeight: theme === t ? 600 : 400, cursor: 'pointer',
                  }}
                >
                  {t === 'light' ? '라이트' : '다크'}
                </button>
              ))}
            </div>
          </SettingRow>
          <SettingRow label="강조 색상" description="버튼, 링크, 선택 영역에 사용">
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              {ACCENT_COLORS.map((c) => (
                <button
                  key={c.value}
                  title={c.label}
                  onClick={() => applyAccent(c.value)}
                  style={{
                    width: '22px', height: '22px', borderRadius: '50%',
                    background: c.value, border: `2px solid ${accent === c.value ? 'var(--color-text-primary)' : 'transparent'}`,
                    cursor: 'pointer', padding: 0,
                    boxShadow: accent === c.value ? `0 0 0 1px ${c.value}` : 'none',
                    transition: 'border-color 100ms ease',
                  }}
                />
              ))}
            </div>
          </SettingRow>
          <SettingRow label="컴팩트 보기" description="메일 목록 행 높이를 줄여 더 많은 메일을 표시">
            <Toggle value={compact} onChange={applyCompact} />
          </SettingRow>
          <SettingRow label="대화 모드" description="같은 제목의 메일을 묶어서 표시">
            <Toggle value={convMode} onChange={applyConvMode} />
          </SettingRow>
        </section>

        {/* Notifications */}
        <section style={{ marginBottom: '32px' }}>
          <SectionTitle>알림</SectionTitle>
          <SettingRow
            label="브라우저 알림"
            description={
              notifPerm === 'granted' ? '알림이 허용되어 있습니다'
              : notifPerm === 'denied' ? '알림이 차단됨 — 브라우저 설정에서 변경하세요'
              : '새 메일 도착 시 알림을 받습니다'
            }
          >
            {notifPerm === 'granted' ? (
              <span style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <CheckIcon style={{ width: '14px', height: '14px' }} /> 허용됨
              </span>
            ) : notifPerm === 'denied' ? (
              <span style={{ fontSize: '12px', color: 'var(--color-destructive)', fontWeight: 500 }}>차단됨</span>
            ) : (
              <button
                onClick={requestNotif}
                style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
              >
                허용하기
              </button>
            )}
          </SettingRow>
        </section>

        {/* About */}
        <section>
          <SectionTitle>정보</SectionTitle>
          <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', lineHeight: 1.7 }}>
            <div>GoGoMail Webmail</div>
            <div>Next.js 15 · TypeScript · Tailwind v4</div>
          </div>
        </section>
      </div>
    </div>
  );
}
