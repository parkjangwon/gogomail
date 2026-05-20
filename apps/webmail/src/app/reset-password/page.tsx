'use client';

import { useState, useEffect, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
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

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get('token') ?? '';

  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!token) {
      setError('유효하지 않거나 만료된 토큰입니다. 비밀번호 찾기 페이지에서 다시 시도해 주세요.');
    }
  }, [token]);

  async function handleSubmit(e: { preventDefault(): void }) {
    e.preventDefault();
    if (!newPassword || !confirmPassword) {
      setError('모든 항목을 입력하세요.');
      return;
    }
    if (newPassword.length < 8) {
      setError('새 비밀번호는 8자 이상이어야 합니다.');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('비밀번호가 일치하지 않습니다.');
      return;
    }
    if (!token) {
      setError('유효하지 않거나 만료된 토큰입니다.');
      return;
    }
    setError('');
    setLoading(true);
    try {
      const res = await fetch('/api/auth/password-reset/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: newPassword }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({})) as { error?: string };
        throw new Error(data.error ?? '유효하지 않거나 만료된 토큰입니다.');
      }
      router.push('/login');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '유효하지 않거나 만료된 토큰입니다.';
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label
          htmlFor="new-password"
          style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}
        >
          새 비밀번호
        </label>
        <input
          id="new-password"
          type="password"
          autoComplete="new-password"
          autoFocus
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          placeholder="8자 이상"
          style={inputStyle}
          disabled={!token}
          onFocus={(e) => { e.target.style.borderColor = 'var(--color-accent)'; }}
          onBlur={(e) => { e.target.style.borderColor = 'var(--color-border-default)'; }}
        />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label
          htmlFor="confirm-password"
          style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}
        >
          새 비밀번호 확인
        </label>
        <input
          id="confirm-password"
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          placeholder="비밀번호 재입력"
          style={inputStyle}
          disabled={!token}
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
        disabled={loading || !token}
        style={{
          marginTop: '4px',
          padding: '11px 16px',
          borderRadius: '6px',
          border: 'none',
          background: (loading || !token) ? 'var(--color-border-default)' : 'var(--color-accent)',
          color: '#fff',
          fontSize: '14px',
          fontWeight: 500,
          cursor: (loading || !token) ? 'not-allowed' : 'pointer',
          transition: 'background 100ms ease',
        }}
        onMouseEnter={(e) => {
          if (!loading && token) (e.target as HTMLButtonElement).style.background = 'var(--color-accent-hover)';
        }}
        onMouseLeave={(e) => {
          if (!loading && token) (e.target as HTMLButtonElement).style.background = 'var(--color-accent)';
        }}
      >
        {loading ? '처리 중...' : '비밀번호 재설정'}
      </button>

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
        ← 비밀번호 찾기로 돌아가기
      </Link>
    </form>
  );
}

export default function ResetPasswordPage() {
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
            새 비밀번호 설정
          </p>
        </div>

        <Suspense fallback={<div style={{ textAlign: 'center', color: 'var(--color-text-secondary)', fontSize: '14px' }}>로딩 중...</div>}>
          <ResetPasswordForm />
        </Suspense>
      </div>
    </div>
  );
}
