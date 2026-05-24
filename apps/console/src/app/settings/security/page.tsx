'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
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
  const router = useRouter();
  const [status, setStatus] = useState<MFAStatus | null>(null);
  const [view, setView] = useState<View>('idle');
  const [setupData, setSetupData] = useState<SetupData | null>(null);
  const [confirmCode, setConfirmCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [accountEmail, setAccountEmail] = useState('');

  const fetchStatus = useCallback(async () => {
    const res = await fetch('/api/admin/auth/mfa/status', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json() as { mfa_status: MFAStatus };
      setStatus(data.mfa_status);
    }
  }, []);

  useEffect(() => { fetchStatus(); }, [fetchStatus]);

  useEffect(() => {
    setAccountEmail(localStorage.getItem('console_admin_email') || '');
  }, []);

  async function startSetup() {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/admin/auth/mfa/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(accountEmail ? { email: accountEmail } : {}),
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
      router.replace('/');
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
          <ColumnLayout key="status-layout" columns={2}>
            <div>
              <Box variant="awsui-key-label">{t('common.status')}</Box>
              <StatusIndicator type={status.enabled ? 'success' : 'stopped'}>
                {status.enabled ? t('common.enabled') : t('security.settings.not_enabled')}
              </StatusIndicator>
            </div>
          </ColumnLayout>
          {error && <Alert key="status-error" type="error" onDismiss={() => setError('')} dismissible>{error}</Alert>}
          {status.enabled ? (
            <Button key="disable-mfa" onClick={disableMFA} loading={loading}>{t('security.settings.disable_mfa')}</Button>
          ) : (
            <Button key="enable-mfa" variant="primary" onClick={startSetup} loading={loading}>{t('security.settings.enable_mfa')}</Button>
          )}
        </SpaceBetween>
      );
    }

    if (view === 'setup' && setupData) {
      return (
        <SpaceBetween size="m">
          <Box key="setup-instructions">{t('security.settings.setup_instructions')}</Box>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img key="setup-qr" src={setupData.qr_image} alt={t('security.settings.qr_alt')} width={180} height={180} />
          <Box key="setup-secret" variant="code">{t('security.settings.secret_label')}: {setupData.secret}</Box>
          <FormField key="setup-confirm-code" label={t('security.settings.confirmation_code')} description={t('security.settings.confirmation_desc')}>
            <Input
              value={confirmCode}
              onChange={({ detail }) => setConfirmCode(detail.value)}
              onKeyDown={({ detail }) => { if (detail.key === 'Enter') confirmSetup(); }}
              inputMode="numeric"
              autoFocus
            />
          </FormField>
          {error && <Alert key="setup-error" type="error" onDismiss={() => setError('')} dismissible>{error}</Alert>}
          <SpaceBetween key="setup-actions" direction="horizontal" size="xs">
            <Button key="setup-cancel" onClick={() => { setView('idle'); setError(''); setConfirmCode(''); }}>{t('common.cancel')}</Button>
            <Button key="setup-confirm" variant="primary" onClick={confirmSetup} loading={loading}>{t('common.confirm')}</Button>
          </SpaceBetween>
        </SpaceBetween>
      );
    }

    if (view === 'codes' && setupData) {
      return (
        <SpaceBetween size="m">
          <Alert key="codes-success" type="success">{t('security.settings.enabled_success')}</Alert>
          <Box key="codes-heading" variant="h3">{t('security.settings.recovery_codes')}</Box>
          <Box key="codes-description">{t('security.settings.recovery_desc')}</Box>
          <SpaceBetween key="codes-list" size="xs">
            {setupData.recovery_codes.map((c) => (
              <Box key={c} variant="code">{c}</Box>
            ))}
          </SpaceBetween>
          <Button key="codes-done" onClick={() => router.replace('/')}>{t('common.done')}</Button>
        </SpaceBetween>
      );
    }

    return null;
  };

  return (
    <SpaceBetween size="l">
      <Header key="security-title" variant="h1">{t('nav.security')}</Header>
      <Container key="mfa-container" header={<Header variant="h2">{t('security.settings.mfa_header')}</Header>}>
        {renderContent()}
      </Container>
    </SpaceBetween>
  );
}
