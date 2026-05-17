'use client';

import { useState, useRef } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import {
  FormField,
  Input,
  InputProps,
  Button,
  Alert,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import './login.css';

export default function LoginPage() {
  const { t } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [emailError, setEmailError] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const passwordRef = useRef<InputProps.Ref>(null);
  const showDemoCredentials = process.env.NODE_ENV !== 'production';

  const validate = () => {
    let valid = true;
    if (!email.trim()) {
      setEmailError(t('login.email_required'));
      valid = false;
    } else if (!/^[^\s@]+@[^\s@]+$/.test(email)) {
      setEmailError(t('login.email_invalid'));
      valid = false;
    } else {
      setEmailError('');
    }
    if (!password) {
      setPasswordError(t('login.password_required'));
      valid = false;
    } else if (password.length < 6) {
      setPasswordError(t('login.password_min_length'));
      valid = false;
    } else {
      setPasswordError('');
    }
    return valid;
  };

  const handleSubmit = async () => {
    if (!validate()) return;
    setError('');
    setLoading(true);

    try {
      const res = await fetch('/api/admin/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
        credentials: 'include',
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        const message = typeof errorData?.error === 'string' ? errorData.error : t('login.invalid_credentials');
        throw new Error(message);
      }

      if (res.ok) {
        // Token is stored as httpOnly cookie by the server
        const next = searchParams.get('next');
        const destination = next?.startsWith('/companies/') ? next : '/companies/default/dashboard';
        router.push(destination);
      } else {
        throw new Error(t('login.failed'));
      }
    } catch (err) {
      let errorMessage = t('login.failed');
      if (err instanceof Error) {
        errorMessage = err.message || t('common.error');
      } else if (typeof err === 'string') {
        errorMessage = err;
      }
      setError(errorMessage || t('login.failed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="aws-login-container">
      <div className="aws-login-wrapper">
        {/* Logo and Title */}
        <div className="aws-login-header">
          <h1>GoGoMail</h1>
          <p>{t('login.subtitle')}</p>
        </div>

        {/* Login Card */}
        <div className="aws-login-card">
          {error && (
            <div className="aws-login-alert">
              <Alert
                type="error"
                dismissible
                onDismiss={() => setError('')}
              >
                {error}
              </Alert>
            </div>
          )}

          <div className="aws-login-form">
            <FormField label={t('login.email_label')} errorText={emailError}>
              <Input
                value={email}
                onChange={(e) => { setEmail(e.detail.value); setEmailError(''); }}
                onKeyDown={(e) => { if (e.detail.key === 'Enter') passwordRef.current?.focus(); }}
                placeholder="admin@system"
                type="email"
                disabled={loading}
                invalid={!!emailError}
                autoFocus
              />
            </FormField>

            <FormField label={t('login.password_label')} errorText={passwordError}>
              <Input
                ref={passwordRef}
                value={password}
                onChange={(e) => { setPassword(e.detail.value); setPasswordError(''); }}
                onKeyDown={(e) => { if (e.detail.key === 'Enter') handleSubmit(); }}
                type="password"
                disabled={loading}
                invalid={!!passwordError}
              />
            </FormField>

            <Button
              variant="primary"
              onClick={() => handleSubmit()}
              loading={loading}
              fullWidth
            >
              {loading ? t('login.signing_in') : t('login.sign_in')}
            </Button>
          </div>

          {showDemoCredentials && (
            <div className="aws-login-footer">
              <div className="aws-login-divider"></div>
              <p className="aws-login-hint">{t('login.demo_credentials')}</p>
              <div className="aws-login-credentials">
                <span>admin@system / admin1234</span>
              </div>
            </div>
          )}
        </div>

        {/* Copyright */}
        <div className="aws-login-copyright">
          {t('login.copyright')}
        </div>
      </div>
    </div>
  );
}
