'use client';

import { useState, useEffect, KeyboardEvent } from 'react';
import { useRouter } from 'next/navigation';
import { loginUser } from '@/lib/api';
import { ThemeToggle } from '@/components/ThemeToggle';

const DEV_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID || '';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Dev mode: skip login and go directly to mail
  useEffect(() => {
    if (DEV_USER_ID) {
      localStorage.setItem('webmail_token', '__dev__');
      if (DEV_USER_ID.includes('@')) {
        localStorage.setItem('webmail_email', DEV_USER_ID);
      }
      router.push('/mail');
    }
  }, [router]);

  async function handleSubmit(e: { preventDefault(): void }) {
    e.preventDefault();
    if (!email.trim() || !password.trim()) {
      setError('이메일과 비밀번호를 입력하세요.');
      return;
    }
    setError('');
    setLoading(true);
    try {
      const result = await loginUser(email.trim(), password);
      localStorage.setItem('webmail_token', result.token);
      localStorage.setItem('webmail_email', email.trim());
      localStorage.setItem('webmail_login_at', new Date().toISOString());
      if (result.expires_at) localStorage.setItem('webmail_token_expires_at', result.expires_at);
      if (result.client_ip) localStorage.setItem('webmail_login_ip', result.client_ip);
      if (result.must_change_password) {
        localStorage.setItem('webmail_must_change_password', '1');
      }
      router.push('/mail');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '로그인에 실패했습니다.';
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  function handleKeyDown(e: KeyboardEvent<HTMLFormElement>) {
    if (e.key === 'Enter') {
      void handleSubmit(e);
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
      <div
        style={{
          width: '100%',
          maxWidth: '400px',
        }}
      >
        {/* Logo */}
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
          <p
            style={{
              marginTop: '8px',
              fontSize: '14px',
              color: 'var(--color-text-secondary)',
            }}
          >
            계정에 로그인하세요
          </p>
        </div>

        {/* Form */}
        <form
          onSubmit={handleSubmit}
          onKeyDown={handleKeyDown}
          style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label
              htmlFor="email"
              style={{
                fontSize: '13px',
                fontWeight: 500,
                color: 'var(--color-text-primary)',
              }}
            >
              이메일
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="user@example.com"
              style={{
                padding: '10px 12px',
                borderRadius: '6px',
                border: '1px solid var(--color-border-default)',
                background: 'var(--color-bg-primary)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                transition: 'border-color 100ms ease',
                outline: 'none',
              }}
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--color-accent)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--color-border-default)';
              }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label
              htmlFor="password"
              style={{
                fontSize: '13px',
                fontWeight: 500,
                color: 'var(--color-text-primary)',
              }}
            >
              비밀번호
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              style={{
                padding: '10px 12px',
                borderRadius: '6px',
                border: '1px solid var(--color-border-default)',
                background: 'var(--color-bg-primary)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                transition: 'border-color 100ms ease',
                outline: 'none',
              }}
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--color-accent)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--color-border-default)';
              }}
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
              if (!loading) {
                (e.target as HTMLButtonElement).style.background = 'var(--color-accent-hover)';
              }
            }}
            onMouseLeave={(e) => {
              if (!loading) {
                (e.target as HTMLButtonElement).style.background = 'var(--color-accent)';
              }
            }}
          >
            {loading ? '로그인 중...' : '로그인'}
          </button>
        </form>
      </div>
    </div>
  );
}
