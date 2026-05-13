'use client';

import {
  ContentLayout,
  Header,
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
import { Fragment, useState, useEffect } from 'react';
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
  password_reset_token_ttl_minutes: number;
  updated_at: string;
  updated_by: string;
}

const BYTES_PER_MB = 1048576;
const QUOTA_UNITS = {
  MB: BYTES_PER_MB,
  GB: BYTES_PER_MB * 1024,
  TB: BYTES_PER_MB * 1024 * 1024,
} as const;

type QuotaUnit = keyof typeof QUOTA_UNITS;

const bestQuotaUnit = (bytes: number): QuotaUnit => {
  if (bytes >= QUOTA_UNITS.TB && bytes % QUOTA_UNITS.TB === 0) return 'TB';
  if (bytes >= QUOTA_UNITS.GB && bytes % QUOTA_UNITS.GB === 0) return 'GB';
  return 'MB';
};

const formatQuotaValue = (bytes: number, unit: QuotaUnit): string => {
  const value = bytes / QUOTA_UNITS[unit];
  return Number.isInteger(value) ? String(value) : String(Number(value.toFixed(2)));
};

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
  const [quotaUnit, setQuotaUnit] = useState<QuotaUnit>('GB');

  const tlsOptions = [
    { label: t('pages.domain_settings_page.tls_opportunistic'), value: 'opportunistic' },
    { label: t('pages.domain_settings_page.tls_require'), value: 'require' },
    { label: t('pages.domain_settings_page.tls_disable'), value: 'disable' },
  ];

  const registrationModeOptions = [
    { label: t('pages.domain_settings_page.registration_temp_password'), value: 'temp_password' },
    { label: t('pages.domain_settings_page.registration_email_invite'), value: 'email_invite' },
  ];

  const quotaUnitOptions = [
    { label: 'MB', value: 'MB' },
    { label: 'GB', value: 'GB' },
    { label: 'TB', value: 'TB' },
  ];

  const apiErrorMessage = (value: unknown, fallback: string): string => {
    if (!value || typeof value !== 'object') return fallback;
    const body = value as { error?: unknown; error_message?: unknown; message?: unknown };
    if (typeof body.error_message === 'string' && body.error_message.trim()) return body.error_message;
    if (typeof body.error === 'string' && body.error.trim()) return body.error;
    if (body.error && typeof body.error === 'object') {
      const error = body.error as { message?: unknown; status_text?: unknown };
      if (typeof error.message === 'string' && error.message.trim()) return error.message;
      if (typeof error.status_text === 'string' && error.status_text.trim()) return error.status_text;
    }
    if (typeof body.message === 'string' && body.message.trim()) return body.message;
    return fallback;
  };

  const normalizeSettings = (value: DomainSettings): DomainSettings => ({
    ...value,
    ip_whitelist: value.ip_whitelist ?? [],
    user_registration_mode: value.user_registration_mode ?? 'temp_password',
    password_reset_token_ttl_minutes: value.password_reset_token_ttl_minutes ?? 60,
  });

  useEffect(() => {
    fetchDomains();
  }, [companyId]);

  useEffect(() => {
    if (selectedDomainId) fetchSettings(selectedDomainId);
  }, [selectedDomainId]);

  const fetchDomains = async () => {
    setLoadingDomains(true);
    try {
      const url = companyId
        ? `/api/admin/domains?company_id=${companyId}&limit=100`
        : '/api/admin/domains?limit=100';
      const res = await fetch(url, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        const nextDomains: Domain[] = data.domains || [];
        setDomains(nextDomains);
        if (!selectedDomainId && nextDomains.length > 0) {
          setSelectedDomainId(nextDomains[0].id);
        }
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
        const nextSettings = normalizeSettings(data.settings);
        setSettings(nextSettings);
        setForm(nextSettings);
        setQuotaUnit(bestQuotaUnit(nextSettings.quota_per_user));
      } else {
        const err = await res.json().catch(() => ({}));
        setSaveError(apiErrorMessage(err, t('pages.domain_settings_page.load_error')));
      }
    } catch (e) {
      console.error('Failed to fetch domain settings:', e);
      setSaveError(t('pages.domain_settings_page.load_error'));
    } finally {
      setLoadingSettings(false);
    }
  };

  const handleDomainChange = (domainId: string) => {
    setSelectedDomainId(domainId);
    setSettings(null);
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
        setSaveError(apiErrorMessage(err, t('pages.domain_settings_page.save_error')));
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
  const handleQuotaUnitChange = (unit: QuotaUnit) => {
    const value = parseFloat(formatQuotaValue(Number(f('quota_per_user') ?? 10737418240), quotaUnit));
    setQuotaUnit(unit);
    set('quota_per_user', Math.max(1, Math.round((Number.isFinite(value) ? value : 1) * QUOTA_UNITS[unit])));
  };

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
      <div style={{ display: 'grid', gap: '24px' }}>
        <Fragment key="domain-selector">
          <Container key="domain-selector-card">
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
        </Fragment>

        {loadingSettings && (
          <Fragment key="loading-settings">
            <Box key="loading-settings-box" textAlign="center" padding="xl"><Spinner /></Box>
          </Fragment>
        )}

        {!loadingSettings && saveError && (
          <Fragment key="load-error">
            <Alert key="load-error-alert" type="error">{saveError}</Alert>
          </Fragment>
        )}

        {!loadingSettings && domains.length === 0 && (
          <Fragment key="empty-domains">
            <Alert key="empty-domains-alert" type="info">{t('pages.domain_settings_page.no_domains')}</Alert>
          </Fragment>
        )}

        {settings && form && (
          <Fragment key="settings-form">
            <div key="settings-stack" style={{ display: 'grid', gap: '24px' }}>
              {saveSuccess && (
                <Fragment key="save-success">
                  <Alert key="save-success-alert" type="success">{t('pages.domain_settings_page.save_success')}</Alert>
                </Fragment>
              )}

              {/* User Registration */}
              <Container key="registration-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_registration')}</Header>}>
              <div key="registration-stack" style={{ display: 'grid', gap: '16px' }}>
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
              </div>
            </Container>

            {/* Security */}
            <Container key="security-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_security')}</Header>}>
              <ColumnLayout columns={2}>
                <div key="security-left" style={{ display: 'grid', gap: '16px' }}>
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
                </div>
                <div key="security-right" style={{ display: 'grid', gap: '16px' }}>
                  <FormField label={t('pages.domain_settings_page.session_timeout_label')} description={t('pages.domain_settings_page.minutes')}>
                    <Input
                      type="number"
                      value={String(f('session_timeout_minutes') ?? 480)}
                      onChange={(e) => set('session_timeout_minutes', parseInt(e.detail.value) || 480)}
                    />
                  </FormField>
                </div>
              </ColumnLayout>
            </Container>

            {/* Password Policy */}
            <Container key="password-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_password')}</Header>}>
              <ColumnLayout columns={2}>
                <div key="password-left" style={{ display: 'grid', gap: '16px' }}>
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
                  <FormField
                    label={t('pages.domain_settings_page.password_reset_ttl_label')}
                    description={t('pages.domain_settings_page.password_reset_ttl_desc')}
                  >
                    <Input
                      type="number"
                      value={String(f('password_reset_token_ttl_minutes') ?? 60)}
                      onChange={(e) => set('password_reset_token_ttl_minutes', parseInt(e.detail.value) || 60)}
                    />
                  </FormField>
                </div>
                <div key="password-right" style={{ display: 'grid', gap: '16px' }}>
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
                </div>
              </ColumnLayout>
            </Container>

            {/* Quota */}
            <Container key="quota-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_quota')}</Header>}>
              <ColumnLayout columns={2}>
                <FormField label={t('pages.domain_settings_page.quota_per_user_label')}>
                  <Input
                    type="number"
                    value={formatQuotaValue(Number(f('quota_per_user') ?? 10737418240), quotaUnit)}
                    onChange={(e) => {
                      const value = parseFloat(e.detail.value);
                      set('quota_per_user', Math.max(1, Math.round((Number.isFinite(value) ? value : 1) * QUOTA_UNITS[quotaUnit])));
                    }}
                  />
                </FormField>
                <FormField label={t('pages.domain_settings_page.quota_unit_label')}>
                  <Select
                    selectedOption={quotaUnitOptions.find(o => o.value === quotaUnit) ?? quotaUnitOptions[1]}
                    onChange={(e) => handleQuotaUnitChange((e.detail.selectedOption.value as QuotaUnit) ?? 'GB')}
                    options={quotaUnitOptions}
                  />
                </FormField>
              </ColumnLayout>
            </Container>

            {/* Footer */}
            <Box key="settings-footer" float="right">
              <div key="settings-footer-stack" style={{ alignItems: 'center', display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                {settings.updated_at && (
                  <Box color="text-body-secondary" fontSize="body-s" padding={{ top: 'xs' }}>
                    {t('pages.domain_settings_page.last_updated')}: {new Date(settings.updated_at).toLocaleString()}
                    {settings.updated_by && <> · <Badge color="grey">{settings.updated_by.slice(0, 8)}</Badge></>}
                  </Box>
                )}
                <Button variant="primary" onClick={handleSave} loading={saving}>
                  {t('pages.domain_settings_page.save_btn')}
                </Button>
              </div>
              </Box>
            </div>
          </Fragment>
        )}
      </div>
    </ContentLayout>
  );
}
