'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  Box,
  Button,
  ColumnLayout,
  Container,
  Header,
  SpaceBetween,
  StatusIndicator,
  Input,
  FormField,
  Alert,
} from '@cloudscape-design/components';

type MFAStatus = { enabled: boolean };
type SetupData = { secret: string; qr_image: string; recovery_codes: string[] };
type View = 'idle' | 'setup' | 'codes';

export default function SecurityPage() {
  const [status, setStatus] = useState<MFAStatus | null>(null);
  const [view, setView] = useState<View>('idle');
  const [setupData, setSetupData] = useState<SetupData | null>(null);
  const [confirmCode, setConfirmCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const fetchStatus = useCallback(async () => {
    const res = await fetch('/api/admin/auth/mfa/status', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json() as { mfa_status: MFAStatus };
      setStatus(data.mfa_status);
    }
  }, []);

  useEffect(() => { fetchStatus(); }, [fetchStatus]);

  async function startSetup() {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/auth/mfa/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({}),
      });
      if (!res.ok) {
        setError('Failed to start MFA setup');
        return;
      }
      const data = await res.json() as SetupData;
      setSetupData(data);
      setView('setup');
    } finally {
      setLoading(false);
    }
  }

  async function confirmSetup() {
    if (!confirmCode.trim()) return;
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/auth/mfa/setup/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ code: confirmCode }),
      });
      if (!res.ok) {
        setError('Invalid code — try again');
        return;
      }
      localStorage.removeItem('console_mfa_setup_required');
      setView('codes');
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function disableMFA() {
    if (!confirm('Disable MFA? You will no longer be challenged at login.')) return;
    setLoading(true);
    setError('');
    try {
      await fetch('/api/admin/auth/mfa', { method: 'DELETE', credentials: 'include' });
      await fetchStatus();
      setView('idle');
    } finally {
      setLoading(false);
    }
  }

  const renderContent = () => {
    if (status === null) {
      return <StatusIndicator type="loading">Loading…</StatusIndicator>;
    }

    if (view === 'idle') {
      return (
        <SpaceBetween size="m">
          <ColumnLayout columns={2}>
            <div>
              <Box variant="awsui-key-label">Status</Box>
              <StatusIndicator type={status.enabled ? 'success' : 'stopped'}>
                {status.enabled ? 'Enabled' : 'Not enabled'}
              </StatusIndicator>
            </div>
          </ColumnLayout>
          {error && <Alert type="error" onDismiss={() => setError('')} dismissible>{error}</Alert>}
          {status.enabled ? (
            <Button onClick={disableMFA} loading={loading}>Disable MFA</Button>
          ) : (
            <Button variant="primary" onClick={startSetup} loading={loading}>Enable MFA</Button>
          )}
        </SpaceBetween>
      );
    }

    if (view === 'setup' && setupData) {
      return (
        <SpaceBetween size="m">
          <Box>Scan this QR code with your authenticator app, then enter the 6-digit code to confirm.</Box>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={setupData.qr_image} alt="TOTP QR code" width={180} height={180} />
          <Box variant="code">Secret: {setupData.secret}</Box>
          <FormField label="Confirmation code" description="Enter the 6-digit code from your authenticator app">
            <Input
              value={confirmCode}
              onChange={({ detail }) => setConfirmCode(detail.value)}
              onKeyDown={({ detail }) => { if (detail.key === 'Enter') confirmSetup(); }}
              inputMode="numeric"
              autoFocus
            />
          </FormField>
          {error && <Alert type="error" onDismiss={() => setError('')} dismissible>{error}</Alert>}
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={() => { setView('idle'); setError(''); setConfirmCode(''); }}>Cancel</Button>
            <Button variant="primary" onClick={confirmSetup} loading={loading}>Confirm</Button>
          </SpaceBetween>
        </SpaceBetween>
      );
    }

    if (view === 'codes' && setupData) {
      return (
        <SpaceBetween size="m">
          <Alert type="success">MFA enabled successfully.</Alert>
          <Box variant="h3">Recovery codes</Box>
          <Box>Each code can be used once if you lose access to your authenticator. Save these somewhere safe.</Box>
          <SpaceBetween size="xs">
            {setupData.recovery_codes.map((c) => (
              <Box key={c} variant="code">{c}</Box>
            ))}
          </SpaceBetween>
          <Button onClick={() => setView('idle')}>Done</Button>
        </SpaceBetween>
      );
    }

    return null;
  };

  return (
    <SpaceBetween size="l">
      <Header variant="h1">Security</Header>
      <Container header={<Header variant="h2">Two-factor authentication (MFA)</Header>}>
        {renderContent()}
      </Container>
    </SpaceBetween>
  );
}
