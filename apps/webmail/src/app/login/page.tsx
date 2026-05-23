'use client';

import { useState, useEffect, KeyboardEvent } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { loginUser, verifyMFA } from '@/lib/api';
import { ThemeToggle } from '@/components/ThemeToggle';

const DEV_USER_ID = process.env.NODE_ENV !== 'production' ? (process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID || '') : '';
const DEV_SKIP_LOGIN = process.env.NODE_ENV !== 'production' && process.env.NEXT_PUBLIC_GOGOMAIL_DEV_SKIP_LOGIN === 'true';

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

export default function LoginPage() {
  const router = useRouter();
  const t = useTranslations();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const [step, setStep] = useState<'password' | 'mfa'>('password');
  const [pendingToken, setPendingToken] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [useRecovery, setUseRecovery] = useState(false);

  useEffect(() => {
    if (DEV_SKIP_LOGIN && DEV_USER_ID) {
      localStorage.setItem('webmail_authenticated', '1');
      if (DEV_USER_ID.includes('@')) {
        localStorage.setItem('webmail_email', DEV_USER_ID);
      }
      router.push('/mail');
    }
  }, [router]);

  async function handlePasswordSubmit(e: { preventDefault(): void }) {
    e.preventDefault();
    if (!email.trim() || !password.trim()) {
      setError(t('misc.login.emptyFieldsError'));
      return;
    }
    setError('');
    setLoading(true);
    try {
      const result = await loginUser(email.trim(), password);
      if (result.mfa_required && result.pending_token) {
        setPendingToken(result.pending_token);
        setStep('mfa');
        return;
      }
      localStorage.setItem('webmail_authenticated', '1');
      localStorage.setItem('webmail_email', email.trim());
      localStorage.setItem('webmail_login_at', new Date().toISOString());
      if (result.expires_at) localStorage.setItem('webmail_token_expires_at', result.expires_at);
      if (result.client_ip) localStorage.setItem('webmail_login_ip', result.client_ip);
      if (result.must_change_password) {
        localStorage.setItem('webmail_must_change_password', '1');
      }
      if (result.mfa_setup_required) {
        localStorage.setItem('webmail_mfa_setup_required', '1');
      } else {
        localStorage.removeItem('webmail_mfa_setup_required');
      }
      router.push('/mail');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('misc.login.loginFailed');
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  async function handleMFASubmit(e: { preventDefault(): void }) {
    e.preventDefault();
    const code = mfaCode.trim();
    if (!code) {
      setError(useRecovery ? t('misc.login.recoveryCodeEmpty') : t('misc.login.mfaCodeEmpty'));
      return;
    }
    setError('');
    setLoading(true);
    try {
      const result = await verifyMFA(pendingToken, code);
      localStorage.setItem('webmail_authenticated', '1');
      localStorage.setItem('webmail_email', email.trim());
      localStorage.setItem('webmail_login_at', new Date().toISOString());
      if (result.expires_at) localStorage.setItem('webmail_token_expires_at', result.expires_at);
      router.push('/mail');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('misc.login.mfaFailed');
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  function handleKeyDown(e: KeyboardEvent<HTMLFormElement>) {
    if (e.key === 'Enter') {
      void (step === 'password' ? handlePasswordSubmit(e) : handleMFASubmit(e));
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
            {step === 'password' ? t('misc.login.subtitleLogin') : t('misc.login.subtitleMfa')}
          </p>
        </div>

        {step === 'password' ? (
          <form
            onSubmit={handlePasswordSubmit}
            onKeyDown={handleKeyDown}
            style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label htmlFor="email" style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
                {t('misc.login.labelEmail')}
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

            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label htmlFor="password" style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
                {t('misc.login.labelPassword')}
              </label>
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                style={inputStyle}
                onFocus={(e) => { e.target.style.borderColor = 'var(--color-accent)'; }}
                onBlur={(e) => { e.target.style.borderColor = 'var(--color-border-default)'; }}
              />
            </div>

            {error && <ErrorAlert message={error} />}

            <SubmitButton loading={loading} label={t('misc.login.loginButton')} processingLabel={t('misc.login.processing')} />

            <Link
              href="/forgot-password"
              style={{
                display: 'block',
                textAlign: 'center',
                color: 'var(--color-text-secondary)',
                fontSize: '13px',
                textDecoration: 'none',
                padding: '4px 0',
              }}
            >
              {t('misc.login.forgotPassword')}
            </Link>
          </form>
        ) : (
          <form
            onSubmit={handleMFASubmit}
            onKeyDown={handleKeyDown}
            style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
          >
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              {useRecovery ? t('misc.login.mfaRecoveryHint') : t('misc.login.mfaCodeHint')}
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label htmlFor="mfa-code" style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
                {useRecovery ? t('misc.login.labelRecoveryCode') : t('misc.login.labelMfaCode')}
              </label>
              <input
                id="mfa-code"
                type="text"
                autoComplete="one-time-code"
                autoFocus
                inputMode={useRecovery ? 'text' : 'numeric'}
                pattern={useRecovery ? undefined : '[0-9]*'}
                maxLength={useRecovery ? 32 : 6}
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                placeholder={useRecovery ? 'xxxx-xxxx-xxxx' : '000000'}
                style={{ ...inputStyle, letterSpacing: useRecovery ? 'normal' : '0.3em', textAlign: 'center' }}
                onFocus={(e) => { e.target.style.borderColor = 'var(--color-accent)'; }}
                onBlur={(e) => { e.target.style.borderColor = 'var(--color-border-default)'; }}
              />
            </div>

            {error && <ErrorAlert message={error} />}

            <SubmitButton loading={loading} label={useRecovery ? t('misc.login.submitRecovery') : t('misc.login.submitMfa')} processingLabel={t('misc.login.processing')} />

            <button
              type="button"
              onClick={() => { setUseRecovery(!useRecovery); setMfaCode(''); setError(''); }}
              style={{
                background: 'none',
                border: 'none',
                color: 'var(--color-accent)',
                fontSize: '13px',
                cursor: 'pointer',
                textAlign: 'center',
                padding: '4px 0',
              }}
            >
              {useRecovery ? t('misc.login.switchToMfa') : t('misc.login.switchToRecovery')}
            </button>

            <button
              type="button"
              onClick={() => { setStep('password'); setError(''); setPendingToken(''); setMfaCode(''); }}
              style={{
                background: 'none',
                border: 'none',
                color: 'var(--color-text-secondary)',
                fontSize: '12px',
                cursor: 'pointer',
                textAlign: 'center',
                padding: '2px 0',
              }}
            >
              {t('misc.login.backToStart')}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

function ErrorAlert({ message }: { message: string }) {
  return (
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
      {message}
    </div>
  );
}

function SubmitButton({ loading, label, processingLabel }: { loading: boolean; label: string; processingLabel: string }) {
  return (
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
      {loading ? processingLabel : label}
    </button>
  );
}
