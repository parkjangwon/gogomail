'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import {
  FormField,
  Input,
  Button,
  SpaceBetween,
  Alert,
} from '@cloudscape-design/components';
import './login.css';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    // Detect dark mode preference
    const darkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
    setIsDark(darkMode);
  }, []);

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
      console.log('Login response:', data);

      if (data.access_token) {
        document.cookie = `admin_access_token=${data.access_token}; path=/; secure; samesite=strict`;
        router.push('/companies/default/dashboard');
      } else {
        throw new Error('No access token received');
      }
    } catch (err) {
      console.error('Login error:', err);
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
    <div className="login-container" data-dark={isDark}>
      <div className="login-background">
        <div className="login-blob login-blob-1"></div>
        <div className="login-blob login-blob-2"></div>
        <div className="login-blob login-blob-3"></div>
      </div>

      <div className="login-wrapper">
        <div className="login-card">
          {/* Header */}
          <div className="login-header">
            <div className="login-logo">
              <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
                <rect width="48" height="48" rx="8" fill="url(#grad1)" />
                <path d="M12 24L20 32L36 16" stroke="white" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
                <defs>
                  <linearGradient id="grad1" x1="0" y1="0" x2="48" y2="48">
                    <stop offset="0%" stopColor="#2563eb" />
                    <stop offset="100%" stopColor="#7c3aed" />
                  </linearGradient>
                </defs>
              </svg>
            </div>
            <h1 className="login-title">GoGoMail</h1>
            <p className="login-subtitle">Admin Console</p>
            <p className="login-description">Enterprise Email Management Platform</p>
          </div>

          {/* Form */}
          <div className="login-form-container">
            {error && (
              <div className="login-alert">
                <Alert
                  type="error"
                  dismissible
                  onDismiss={() => setError('')}
                >
                  {error}
                </Alert>
              </div>
            )}

            <SpaceBetween size="m">
              <FormField
                label="Email Address"
                description="Your administrator account"
              >
                <Input
                  value={email}
                  onChange={(e) => setEmail(e.detail.value)}
                  placeholder="admin@system"
                  type="email"
                  disabled={loading}
                  autoFocus
                />
              </FormField>

              <FormField
                label="Password"
                description="Enter your password"
              >
                <Input
                  value={password}
                  onChange={(e) => setPassword(e.detail.value)}
                  type="password"
                  disabled={loading}
                />
              </FormField>

              <div className="login-button">
                <Button
                  variant="primary"
                  onClick={() => handleSubmit()}
                  loading={loading}
                  fullWidth
                >
                  {loading ? 'Signing in...' : 'Sign In'}
                </Button>
              </div>
            </SpaceBetween>
          </div>

          {/* Footer */}
          <div className="login-footer">
            <div className="login-divider"></div>
            <p className="login-hint">Demo Credentials</p>
            <div className="login-credentials">
              <code>admin@system</code>
              <span className="login-separator">/</span>
              <code>admin1234</code>
            </div>
            <p className="login-footer-text">
              Version 1.0 • © 2026 GoGoMail Inc. All rights reserved.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
