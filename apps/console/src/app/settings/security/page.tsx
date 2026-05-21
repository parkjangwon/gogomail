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
import { useI18n } from '@/app/i18n-provider';

type MFAStatus = { enabled: boolean };
type SetupData = { secret: string; qr_image: string; recovery_codes: string[] };
type View = 'idle' | 'setup' | 'codes';

export default function SecurityPage() {
  const { t } = useI18n();
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
        setError(t('security.settings.mfa_setup_failed'));
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
        setError(t('security.settings.invalid_code'));
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
    if (!confirm(t('security.settings.disable_confirm'))) return;
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
      return <StatusIndicator type="loading">{t('common.loading')}</StatusIndicator>;
    }

    if (view === 'idle') {
      return (
        <SpaceBetween size="m">
          <ColumnLayout columns={2}>
            <div>
              <Box variant="awsui-key-label">{t('common.status')}</Box>
              <StatusIndicator type={status.enabled ? 'success' : 'stopped'}>
                {status.enabled ? t('common.enabled') : t('security.settings.not_enabled')}
              </StatusIndicator>
            </div>
          </ColumnLayout>
          {error && <Alert type="error" onDismiss={() => setError('')} dismissible>{error}</Alert>}
          {status.enabled ? (
            <Button onClick={disableMFA} loading={loading}>{t('security.settings.disable_mfa')}</Button>
          ) : (
            <Button variant="primary" onClick={startSetup} loading={loading}>{t('security.settings.enable_mfa')}</Button>
          )}
        </SpaceBetween>
      );
    }

    if (view === 'setup' && setupData) {
      return (
        <SpaceBetween size="m">
          <Box>{t('security.settings.setup_instructions')}</Box>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={setupData.qr_image} alt={t('security.settings.qr_alt')} width={180} height={180} />
          <Box variant="code">{t('security.settings.secret_label')}: {setupData.secret}</Box>
          <FormField label={t('security.settings.confirmation_code')} description={t('security.settings.confirmation_desc')}>
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
            <Button onClick={() => { setView('idle'); setError(''); setConfirmCode(''); }}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={confirmSetup} loading={loading}>{t('common.confirm')}</Button>
          </SpaceBetween>
        </SpaceBetween>
      );
    }

    if (view === 'codes' && setupData) {
      return (
        <SpaceBetween size="m">
          <Alert type="success">{t('security.settings.enabled_success')}</Alert>
          <Box variant="h3">{t('security.settings.recovery_codes')}</Box>
          <Box>{t('security.settings.recovery_desc')}</Box>
          <SpaceBetween size="xs">
            {setupData.recovery_codes.map((c) => (
              <Box key={c} variant="code">{c}</Box>
            ))}
          </SpaceBetween>
          <Button onClick={() => setView('idle')}>{t('common.done')}</Button>
        </SpaceBetween>
      );
    }

    return null;
  };

  return (
    <SpaceBetween size="l">
      <Header variant="h1">{t('nav.security')}</Header>
      <Container header={<Header variant="h2">{t('security.settings.mfa_header')}</Header>}>
        {renderContent()}
      </Container>
    </SpaceBetween>
  );
}
