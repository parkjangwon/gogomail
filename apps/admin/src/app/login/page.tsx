'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Container,
  FormField,
  Input,
  Button,
  SpaceBetween,
  Box,
  Alert,
} from '@cloudscape-design/components';

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
        const errorData = await res.json().catch(() => ({ error: 'Login failed' }));
        throw new Error(errorData.error || 'Login failed');
      }

      const data = await res.json();

      // Store token in cookie
      if (data.access_token) {
        document.cookie = `admin_access_token=${data.access_token}; path=/; secure; samesite=strict`;
      }

      // Redirect to dashboard
      router.push('/companies/default/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };


  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      backgroundColor: '#f5f5f5',
    }}>
      <Container>
        <div style={{ maxWidth: '450px', padding: '40px 0' }}>
          <Box margin={{ bottom: 'l' }} textAlign="center">
            <h1 style={{ fontSize: '28px', marginBottom: '8px', color: '#232f3e' }}>
              GoGoMail Admin Console
            </h1>
            <p style={{ color: '#666', margin: 0 }}>Enterprise Email Management</p>
          </Box>

          <SpaceBetween size="l">
            {error && (
              <Alert type="error" dismissible onDismiss={() => setError('')}>
                {error}
              </Alert>
            )}

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
              onClick={handleSubmit}
              loading={loading}
              fullWidth
            >
              {loading ? 'Signing in...' : 'Sign in'}
            </Button>

            <Box textAlign="center" color="text-body-secondary">
              <small>Default: admin@system / admin1234</small>
            </Box>
          </SpaceBetween>
        </div>
      </Container>
    </div>
  );
}
