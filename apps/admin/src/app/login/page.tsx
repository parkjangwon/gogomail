'use client';

import { useState, useRef } from 'react';
import { useRouter } from 'next/navigation';
import {
  FormField,
  Input,
  InputProps,
  Button,
  Alert,
} from '@cloudscape-design/components';
import './login.css';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [emailError, setEmailError] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const passwordRef = useRef<InputProps.Ref>(null);

  const validate = () => {
    let valid = true;
    if (!email.trim()) {
      setEmailError('Email address is required.');
      valid = false;
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setEmailError('Enter a valid email address.');
      valid = false;
    } else {
      setEmailError('');
    }
    if (!password) {
      setPasswordError('Password is required.');
      valid = false;
    } else if (password.length < 6) {
      setPasswordError('Password must be at least 6 characters.');
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
        const message = typeof errorData?.error === 'string' ? errorData.error : 'Invalid credentials';
        throw new Error(message);
      }

      const data = await res.json();

      if (data.access_token) {
        document.cookie = `admin_access_token=${data.access_token}; path=/; secure; samesite=strict`;
        router.push('/companies/default/dashboard');
      } else {
        throw new Error('No access token received');
      }
    } catch (err) {
      let errorMessage = 'Login failed';
      if (err instanceof Error) {
        errorMessage = err.message || 'An error occurred';
      } else if (typeof err === 'string') {
        errorMessage = err;
      }
      setError(errorMessage || 'Login failed');
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
          <p>Admin Console</p>
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
            <FormField label="Email Address" errorText={emailError}>
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

            <FormField label="Password" errorText={passwordError}>
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
              {loading ? 'Signing in...' : 'Sign in'}
            </Button>
          </div>

          {/* Demo Credentials */}
          <div className="aws-login-footer">
            <div className="aws-login-divider"></div>
            <p className="aws-login-hint">Demo Credentials</p>
            <div className="aws-login-credentials">
              <span>admin@system / admin1234</span>
            </div>
          </div>
        </div>

        {/* Copyright */}
        <div className="aws-login-copyright">
          © 2026 GoGoMail Inc. All rights reserved.
        </div>
      </div>
    </div>
  );
}
