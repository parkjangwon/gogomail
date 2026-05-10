'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { FormField, Input, Button } from '@cloudscape-design/components';

export default function SetupPage() {
  const router = useRouter();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    setError('');

    // Validation
    if (!username.trim()) {
      setError('Username is required');
      return;
    }

    if (username.length < 3) {
      setError('Username must be at least 3 characters');
      return;
    }

    if (!password) {
      setError('Password is required');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    setLoading(true);

    try {
      const res = await fetch('/api/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
        credentials: 'include',
      });

      if (!res.ok) {
        const data = await res.json();
        setError(data.error || 'Setup failed');
        return;
      }

      router.push('/dashboard');
    } catch (err) {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-gradient-to-br from-slate-900 to-slate-800">
      <div className="w-full max-w-md p-8 bg-slate-700 rounded-lg shadow-lg">
        <h1 className="text-2xl font-bold text-white mb-2">Initial Setup</h1>
        <p className="text-gray-300 mb-6">Create your admin account credentials</p>

        {error && (
          <div className="mb-4 p-3 bg-red-500/20 border border-red-500 text-red-200 rounded text-sm">
            {error}
          </div>
        )}

        <div className="space-y-4">
          <FormField label="New Username">
            <Input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.detail.value)}
              placeholder="Enter your username"
              disabled={loading}
              autoFocus
            />
          </FormField>

          <FormField label="New Password">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.detail.value)}
              placeholder="Enter a strong password"
              disabled={loading}
            />
          </FormField>

          <FormField label="Confirm Password">
            <Input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.detail.value)}
              placeholder="Confirm your password"
              disabled={loading}
            />
          </FormField>

          <Button
            variant="primary"
            fullWidth
            loading={loading}
            onClick={handleSubmit}
          >
            Complete Setup
          </Button>
        </div>

        <p className="text-xs text-gray-400 mt-4 text-center">
          Password must be at least 8 characters
        </p>
      </div>
    </div>
  );
}
