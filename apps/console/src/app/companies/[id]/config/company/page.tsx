'use client';

import {
  Alert,
  Box,
  Button,
  ColumnLayout,
  Container,
  ContentLayout,
  FormField,
  Header,
  Input,
  Select,
  Spinner,
  Toggle,
} from '@cloudscape-design/components';
import { useEffect, useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

const COMPANY_DOMAIN_SETTINGS_KEY = 'domain_settings_defaults';
const BYTES_PER_MB = 1048576;

interface CompanyDomainSettings {
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
}

interface ConfigEntry {
  Value: unknown;
}

const defaultSettings: CompanyDomainSettings = {
  tls_policy: 'opportunistic',
  quota_per_user: 10240 * BYTES_PER_MB,
  ip_whitelist_enabled: false,
  ip_whitelist: [],
  require_2fa: false,
  session_timeout_minutes: 480,
  password_min_length: 8,
  password_require_uppercase: true,
  password_require_numbers: true,
  password_require_special_chars: false,
  password_expiry_days: 0,
  user_registration_mode: 'temp_password',
  password_reset_token_ttl_minutes: 60,
};

const coerceSettings = (value: unknown): CompanyDomainSettings => {
  let raw = value;
  if (typeof raw === 'string') {
    try {
      raw = JSON.parse(raw);
    } catch {
      raw = {};
    }
  }
  const parsed = raw && typeof raw === 'object' ? raw as Partial<CompanyDomainSettings> : {};
  return {
    ...defaultSettings,
    ...parsed,
    ip_whitelist: Array.isArray(parsed.ip_whitelist) ? parsed.ip_whitelist : [],
    quota_per_user: Number(parsed.quota_per_user) > 0 ? Number(parsed.quota_per_user) : defaultSettings.quota_per_user,
    session_timeout_minutes: Number(parsed.session_timeout_minutes) > 0 ? Number(parsed.session_timeout_minutes) : defaultSettings.session_timeout_minutes,
    password_min_length: Number(parsed.password_min_length) > 0 ? Number(parsed.password_min_length) : defaultSettings.password_min_length,
    password_expiry_days: Number(parsed.password_expiry_days) >= 0 ? Number(parsed.password_expiry_days) : defaultSettings.password_expiry_days,
    password_reset_token_ttl_minutes: Number(parsed.password_reset_token_ttl_minutes) > 0
      ? Number(parsed.password_reset_token_ttl_minutes)
      : defaultSettings.password_reset_token_ttl_minutes,
  };
};

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

export default function CompanyConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [settings, setSettings] = useState<CompanyDomainSettings>(defaultSettings);
  const [loading, setLoading] = useState(true);
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
    fetchCompanySettings();
  }, [companyId]);

  const fetchCompanySettings = async () => {
    if (!companyId) return;
    setLoading(true);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/config/${COMPANY_DOMAIN_SETTINGS_KEY}`, {
        credentials: 'include',
      });
      if (res.status === 404) {
        setSettings(defaultSettings);
        return;
      }
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(apiErrorMessage(body, t('pages.domain_settings_page.load_error')));
      }
      const data = await res.json();
      const entry: ConfigEntry = data.config ?? data;
      setSettings(coerceSettings(entry.Value));
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : t('pages.domain_settings_page.load_error'));
    } finally {
      setLoading(false);
    }
  };

  const set = <K extends keyof CompanyDomainSettings>(key: K, value: CompanyDomainSettings[K]) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    setSaveSuccess(false);
    setSaveError('');
  };

  const quotaMb = Math.max(1, Math.round(settings.quota_per_user / BYTES_PER_MB));

  const handleSave = async () => {
    setSaving(true);
    setSaveSuccess(false);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/config/${COMPANY_DOMAIN_SETTINGS_KEY}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ value: coerceSettings(settings) }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(apiErrorMessage(body, t('pages.domain_settings_page.save_error')));
      }
      setSaveSuccess(true);
      await fetchCompanySettings();
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : t('pages.domain_settings_page.save_error'));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.config_company.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.config_company_page.description')}>
          {t('pages.config_company.title')}
        </Header>
      }
    >
      <div style={{ display: 'grid', gap: '24px' }}>
        {saveSuccess && <Alert key="save-success" type="success">{t('pages.domain_settings_page.save_success')}</Alert>}
        {saveError && <Alert key="save-error" type="error">{saveError}</Alert>}

        <Container key="registration-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_registration')}</Header>}>
          <FormField
            label={t('pages.domain_settings_page.registration_mode_label')}
            description={t('pages.domain_settings_page.registration_mode_desc')}
          >
            <Select
              selectedOption={registrationModeOptions.find(o => o.value === settings.user_registration_mode) ?? registrationModeOptions[0]}
              onChange={(e) => set('user_registration_mode', e.detail.selectedOption.value ?? 'temp_password')}
              options={registrationModeOptions}
            />
          </FormField>
        </Container>

        <Container key="security-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_security')}</Header>}>
          <div style={{ display: 'grid', gap: '16px' }}>
            <FormField key="tls-policy" label={t('pages.domain_settings_page.tls_policy_label')}>
              <Select
                selectedOption={tlsOptions.find(o => o.value === settings.tls_policy) ?? tlsOptions[0]}
                onChange={(e) => set('tls_policy', e.detail.selectedOption.value ?? 'opportunistic')}
                options={tlsOptions}
              />
            </FormField>
            <Toggle
              key="require-2fa"
              checked={settings.require_2fa}
              onChange={(e) => set('require_2fa', e.detail.checked)}
            >
              {t('pages.domain_settings_page.require_2fa_label')}
            </Toggle>
            <Toggle
              key="ip-whitelist-enabled"
              checked={settings.ip_whitelist_enabled}
              onChange={(e) => set('ip_whitelist_enabled', e.detail.checked)}
            >
              {t('pages.domain_settings_page.ip_whitelist_label')}
            </Toggle>
            {settings.ip_whitelist_enabled && (
              <FormField key="ip-whitelist" label="IP/CIDR">
                <Input
                  value={settings.ip_whitelist.join(', ')}
                  onChange={(e) => set('ip_whitelist', e.detail.value.split(',').map(v => v.trim()).filter(Boolean))}
                  placeholder="192.168.1.0/24, 10.0.0.1"
                />
              </FormField>
            )}
          </div>
        </Container>

        <Container key="password-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_password')}</Header>}>
          <ColumnLayout columns={2}>
            <div key="password-left" style={{ display: 'grid', gap: '16px' }}>
              <FormField key="password-min-length" label={t('pages.domain_settings_page.password_min_length_label')}>
                <Input
                  type="number"
                  value={String(settings.password_min_length)}
                  onChange={(e) => set('password_min_length', parseInt(e.detail.value) || 8)}
                />
              </FormField>
              <FormField key="password-expiry" label={t('pages.domain_settings_page.password_expiry_label')} description={t('pages.domain_settings_page.password_expiry_desc')}>
                <Input
                  type="number"
                  value={String(settings.password_expiry_days)}
                  onChange={(e) => set('password_expiry_days', parseInt(e.detail.value) || 0)}
                />
              </FormField>
              <FormField key="session-timeout" label={t('pages.domain_settings_page.session_timeout_label')} description={t('pages.domain_settings_page.minutes')}>
                <Input
                  type="number"
                  value={String(settings.session_timeout_minutes)}
                  onChange={(e) => set('session_timeout_minutes', parseInt(e.detail.value) || 480)}
                />
              </FormField>
              <FormField
                key="reset-ttl"
                label={t('pages.domain_settings_page.password_reset_ttl_label')}
                description={t('pages.domain_settings_page.password_reset_ttl_desc')}
              >
                <Input
                  type="number"
                  value={String(settings.password_reset_token_ttl_minutes)}
                  onChange={(e) => set('password_reset_token_ttl_minutes', parseInt(e.detail.value) || 60)}
                />
              </FormField>
            </div>
            <div key="password-right" style={{ display: 'grid', gap: '16px' }}>
              <Toggle
                key="require-uppercase"
                checked={settings.password_require_uppercase}
                onChange={(e) => set('password_require_uppercase', e.detail.checked)}
              >
                {t('pages.domain_settings_page.require_uppercase_label')}
              </Toggle>
              <Toggle
                key="require-numbers"
                checked={settings.password_require_numbers}
                onChange={(e) => set('password_require_numbers', e.detail.checked)}
              >
                {t('pages.domain_settings_page.require_numbers_label')}
              </Toggle>
              <Toggle
                key="require-special"
                checked={settings.password_require_special_chars}
                onChange={(e) => set('password_require_special_chars', e.detail.checked)}
              >
                {t('pages.domain_settings_page.require_special_chars_label')}
              </Toggle>
            </div>
          </ColumnLayout>
        </Container>

        <Container key="quota-settings" header={<Header variant="h2">{t('pages.domain_settings_page.section_quota')}</Header>}>
          <FormField label={t('pages.domain_settings_page.quota_per_user_label')} description="MB">
            <Input
              type="number"
              value={String(quotaMb)}
              onChange={(e) => set('quota_per_user', Math.max(1, parseInt(e.detail.value) || 1) * BYTES_PER_MB)}
            />
          </FormField>
        </Container>

        <Box key="settings-footer" float="right">
          <Button variant="primary" onClick={handleSave} loading={saving}>
            {t('pages.domain_settings_page.save_btn')}
          </Button>
        </Box>
      </div>
    </ContentLayout>
  );
}
