'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { ThemeToggle } from '@/components/ThemeToggle';

const inputStyle: React.CSSProperties = {
  padding: '10px 12px',
  borderRadius: '6px',
  border: '1px solid var(--color-border-default)',
  background: 'var(--color-bg-primary)',
  color: 'var(--color-text-primary)',
  fontSize: '14px',
  transition: 'border-color 100ms ease',
  outline: 'none',
};

export default function ForgotPasswordPage() {
  const t = useTranslations();
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  async function handleSubmit(e: { preventDefault(): void }) {
    e.preventDefault();
    if (!email.trim()) {
      setError(t('misc.forgotPassword.emailEmpty'));
      return;
    }
    setError('');
    setLoading(true);
    try {
      const res = await fetch('/api/auth/password-reset/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim() }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({})) as { error?: string };
        throw new Error(data.error ?? t('misc.forgotPassword.requestFailed'));
      }
      setSent(true);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('misc.forgotPassword.requestFailed');
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--color-bg-primary)',
        padding: '24px',
      }}
    >
      <ThemeToggle />
      <div style={{ width: '100%', maxWidth: '400px' }}>
        <div style={{ textAlign: 'center', marginBottom: '40px' }}>
          <span
            style={{
              fontSize: '28px',
              fontWeight: 600,
              color: 'var(--color-accent)',
              letterSpacing: '-0.5px',
            }}
          >
            GoGoMail
          </span>
          <p style={{ marginTop: '8px', fontSize: '14px', color: 'var(--color-text-secondary)' }}>
            {t('misc.forgotPassword.title')}
          </p>
        </div>

        {sent ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div
              style={{
                padding: '16px',
                borderRadius: '6px',
                background: 'rgba(22,163,74,0.08)',
                border: '1px solid rgba(22,163,74,0.2)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                lineHeight: 1.6,
              }}
            >
              <strong>{t('misc.forgotPassword.checkEmailTitle')}</strong>
              <br />
              {t('misc.forgotPassword.checkEmailBody')}
            </div>
            <Link
              href="/login"
              style={{
                display: 'block',
                textAlign: 'center',
                color: 'var(--color-accent)',
                fontSize: '14px',
                textDecoration: 'none',
                padding: '8px 0',
              }}
            >
              {t('misc.forgotPassword.backToLogin')}
            </Link>
          </div>
        ) : (
          <form
            onSubmit={handleSubmit}
            style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
          >
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              {t('misc.forgotPassword.instructions')}
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label
                htmlFor="email"
                style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}
              >
                {t('misc.forgotPassword.labelEmail')}
              </label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                autoFocus
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="user@example.com"
                style={inputStyle}
                onFocus={(e) => { e.target.style.borderColor = 'var(--color-accent)'; }}
                onBlur={(e) => { e.target.style.borderColor = 'var(--color-border-default)'; }}
              />
            </div>

            {error && (
              <div
                role="alert"
                style={{
                  padding: '10px 12px',
                  borderRadius: '6px',
                  background: 'rgba(217,79,61,0.08)',
                  border: '1px solid rgba(217,79,61,0.2)',
                  color: 'var(--color-destructive)',
                  fontSize: '13px',
                }}
              >
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              style={{
                marginTop: '4px',
                padding: '11px 16px',
                borderRadius: '6px',
                border: 'none',
                background: loading ? 'var(--color-border-default)' : 'var(--color-accent)',
                color: '#fff',
                fontSize: '14px',
                fontWeight: 500,
                cursor: loading ? 'not-allowed' : 'pointer',
                transition: 'background 100ms ease',
              }}
              onMouseEnter={(e) => {
                if (!loading) (e.target as HTMLButtonElement).style.background = 'var(--color-accent-hover)';
              }}
              onMouseLeave={(e) => {
                if (!loading) (e.target as HTMLButtonElement).style.background = 'var(--color-accent)';
              }}
            >
              {loading ? t('misc.forgotPassword.processing') : t('misc.forgotPassword.submit')}
            </button>

            <Link
              href="/login"
              style={{
                display: 'block',
                textAlign: 'center',
                color: 'var(--color-text-secondary)',
                fontSize: '13px',
                textDecoration: 'none',
                padding: '4px 0',
              }}
            >
              {t('misc.forgotPassword.backToLogin')}
            </Link>
          </form>
        )}
      </div>
    </div>
  );
}
