'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  FormField,
  Input,
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

  const handleSubmit = async () => {
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
            <FormField label="Email Address">
              <Input
                value={email}
                onChange={(e) => setEmail(e.detail.value)}
                placeholder="admin@system"
                type="email"
                disabled={loading}
                autoFocus
              />
            </FormField>

            <FormField label="Password">
              <Input
                value={password}
                onChange={(e) => setPassword(e.detail.value)}
                type="password"
                disabled={loading}
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
