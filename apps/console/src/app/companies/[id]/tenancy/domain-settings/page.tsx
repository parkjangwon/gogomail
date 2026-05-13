'use client';

import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  Select,
  FormField,
  Input,
  Toggle,
  Button,
  Container,
  ColumnLayout,
  Alert,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface Domain {
  id: string;
  name: string;
  status: string;
}

interface DomainSettings {
  domain_id: string;
  tls_policy: string;
  quota_per_user: number;
  ip_whitelist_enabled: boolean;
  ip_whitelist: string[];
  require_2fa: boolean;
  session_timeout_minutes: number;
  password_min_length: number;
  password_require_uppercase: boolean;
  password_require_numbers: boolean;
  password_require_special_chars: boolean;
  password_expiry_days: number;
  user_registration_mode: string;
  updated_at: string;
  updated_by: string;
}

export default function DomainSettingsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [domains, setDomains] = useState<Domain[]>([]);
  const [selectedDomainId, setSelectedDomainId] = useState('');
  const [settings, setSettings] = useState<DomainSettings | null>(null);
  const [form, setForm] = useState<Partial<DomainSettings>>({});
  const [loadingDomains, setLoadingDomains] = useState(true);
  const [loadingSettings, setLoadingSettings] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState('');

  const tlsOptions = [
    { label: t('pages.domain_settings_page.tls_opportunistic'), value: 'opportunistic' },
    { label: t('pages.domain_settings_page.tls_require'), value: 'require' },
    { label: t('pages.domain_settings_page.tls_disable'), value: 'disable' },
  ];

  const registrationModeOptions = [
    { label: t('pages.domain_settings_page.registration_temp_password'), value: 'temp_password' },
    { label: t('pages.domain_settings_page.registration_email_invite'), value: 'email_invite' },
  ];

  useEffect(() => {
    fetchDomains();
  }, [companyId]);

  const fetchDomains = async () => {
    setLoadingDomains(true);
    try {
      const url = companyId
        ? `/api/admin/domains?company_id=${companyId}&limit=100`
        : '/api/admin/domains?limit=100';
      const res = await fetch(url, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setDomains(data.domains || []);
      }
    } catch (e) {
      console.error('Failed to fetch domains:', e);
    } finally {
      setLoadingDomains(false);
    }
  };

  const fetchSettings = async (domainId: string) => {
    setLoadingSettings(true);
    setSaveSuccess(false);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/settings`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setSettings(data.settings);
        setForm(data.settings);
      }
    } catch (e) {
      console.error('Failed to fetch domain settings:', e);
    } finally {
      setLoadingSettings(false);
    }
  };

  const handleDomainChange = (domainId: string) => {
    setSelectedDomainId(domainId);
    setSettings(null);
    if (domainId) fetchSettings(domainId);
  };

  const handleSave = async () => {
    if (!selectedDomainId) return;
    setSaving(true);
    setSaveSuccess(false);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomainId}/settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
        credentials: 'include',
      });
      if (res.ok) {
        setSaveSuccess(true);
        fetchSettings(selectedDomainId);
      } else {
        const err = await res.json().catch(() => ({}));
        setSaveError(err.error || t('pages.domain_settings_page.save_error'));
      }
    } catch (e) {
      setSaveError(t('pages.domain_settings_page.save_error'));
    } finally {
      setSaving(false);
    }
  };

  const domainOptions = domains.map(d => ({
    label: d.name,
    value: d.id,
    description: d.status,
  }));

  const f = <K extends keyof DomainSettings>(key: K) => form[key] as DomainSettings[K];
  const set = <K extends keyof DomainSettings>(key: K, value: DomainSettings[K]) =>
    setForm(prev => ({ ...prev, [key]: value }));

  if (loadingDomains) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domain_settings_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.domain_settings_page.description')}>
          {t('pages.domain_settings_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container>
          <FormField label={t('pages.domain_settings_page.select_domain_label')}>
            <Select
              selectedOption={domainOptions.find(o => o.value === selectedDomainId) ?? null}
              options={domainOptions}
              onChange={(e) => handleDomainChange(e.detail.selectedOption?.value ?? '')}
              placeholder={t('pages.domain_settings_page.select_domain_placeholder')}
              expandToViewport
            />
          </FormField>
        </Container>

        {loadingSettings && (
          <Box textAlign="center" padding="xl"><Spinner /></Box>
        )}

        {settings && form && (
          <SpaceBetween size="l">
            {saveSuccess && <Alert type="success">{t('pages.domain_settings_page.save_success')}</Alert>}
            {saveError && <Alert type="error">{saveError}</Alert>}

            {/* User Registration */}
            <Container header={<Header variant="h2">{t('pages.domain_settings_page.section_registration')}</Header>}>
              <SpaceBetween size="m">
                <FormField
                  label={t('pages.domain_settings_page.registration_mode_label')}
                  description={t('pages.domain_settings_page.registration_mode_desc')}
                >
                  <Select
                    selectedOption={registrationModeOptions.find(o => o.value === f('user_registration_mode')) ?? registrationModeOptions[0]}
                    options={registrationModeOptions}
                    onChange={(e) => set('user_registration_mode', e.detail.selectedOption.value as string)}
                    expandToViewport
                  />
                </FormField>
                <Box color="text-body-secondary" fontSize="body-s">
                  {f('user_registration_mode') === 'email_invite'
                    ? t('pages.domain_settings_page.mode_invite_hint')
                    : t('pages.domain_settings_page.mode_temp_hint')}
                </Box>
              </SpaceBetween>
            </Container>

            {/* Security */}
            <Container header={<Header variant="h2">{t('pages.domain_settings_page.section_security')}</Header>}>
              <ColumnLayout columns={2}>
                <SpaceBetween size="m">
                  <FormField label={t('pages.domain_settings_page.tls_policy_label')}>
                    <Select
                      selectedOption={tlsOptions.find(o => o.value === f('tls_policy')) ?? tlsOptions[0]}
                      options={tlsOptions}
                      onChange={(e) => set('tls_policy', e.detail.selectedOption.value as string)}
                      expandToViewport
                    />
                  </FormField>
                  <Toggle
                    checked={!!f('require_2fa')}
                    onChange={(e) => set('require_2fa', e.detail.checked)}
                  >
                    {t('pages.domain_settings_page.require_2fa_label')}
                  </Toggle>
                  <Toggle
                    checked={!!f('ip_whitelist_enabled')}
                    onChange={(e) => set('ip_whitelist_enabled', e.detail.checked)}
                  >
                    {t('pages.domain_settings_page.ip_whitelist_label')}
                  </Toggle>
                </SpaceBetween>
                <SpaceBetween size="m">
                  <FormField label={t('pages.domain_settings_page.session_timeout_label')} description={t('pages.domain_settings_page.minutes')}>
                    <Input
                      type="number"
                      value={String(f('session_timeout_minutes') ?? 480)}
                      onChange={(e) => set('session_timeout_minutes', parseInt(e.detail.value) || 480)}
                    />
                  </FormField>
                </SpaceBetween>
              </ColumnLayout>
            </Container>

            {/* Password Policy */}
            <Container header={<Header variant="h2">{t('pages.domain_settings_page.section_password')}</Header>}>
              <ColumnLayout columns={2}>
                <SpaceBetween size="m">
                  <FormField label={t('pages.domain_settings_page.password_min_length_label')}>
                    <Input
                      type="number"
                      value={String(f('password_min_length') ?? 8)}
                      onChange={(e) => set('password_min_length', parseInt(e.detail.value) || 8)}
                    />
                  </FormField>
                  <FormField label={t('pages.domain_settings_page.password_expiry_label')} description={t('pages.domain_settings_page.password_expiry_desc')}>
                    <Input
                      type="number"
                      value={String(f('password_expiry_days') ?? 0)}
                      onChange={(e) => set('password_expiry_days', parseInt(e.detail.value) || 0)}
                    />
                  </FormField>
                </SpaceBetween>
                <SpaceBetween size="m">
                  <Toggle
                    checked={!!f('password_require_uppercase')}
                    onChange={(e) => set('password_require_uppercase', e.detail.checked)}
                  >
                    {t('pages.domain_settings_page.require_uppercase_label')}
                  </Toggle>
                  <Toggle
                    checked={!!f('password_require_numbers')}
                    onChange={(e) => set('password_require_numbers', e.detail.checked)}
                  >
                    {t('pages.domain_settings_page.require_numbers_label')}
                  </Toggle>
                  <Toggle
                    checked={!!f('password_require_special_chars')}
                    onChange={(e) => set('password_require_special_chars', e.detail.checked)}
                  >
                    {t('pages.domain_settings_page.require_special_chars_label')}
                  </Toggle>
                </SpaceBetween>
              </ColumnLayout>
            </Container>

            {/* Quota */}
            <Container header={<Header variant="h2">{t('pages.domain_settings_page.section_quota')}</Header>}>
              <FormField label={t('pages.domain_settings_page.quota_per_user_label')} description="bytes">
                <Input
                  type="number"
                  value={String(f('quota_per_user') ?? 10737418240)}
                  onChange={(e) => set('quota_per_user', parseInt(e.detail.value) || 10737418240)}
                />
              </FormField>
            </Container>

            {/* Footer */}
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                {settings.updated_at && (
                  <Box color="text-body-secondary" fontSize="body-s" padding={{ top: 'xs' }}>
                    {t('pages.domain_settings_page.last_updated')}: {new Date(settings.updated_at).toLocaleString()}
                    {settings.updated_by && <> · <Badge color="grey">{settings.updated_by.slice(0, 8)}</Badge></>}
                  </Box>
                )}
                <Button variant="primary" onClick={handleSave} loading={saving}>
                  {t('pages.domain_settings_page.save_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          </SpaceBetween>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
