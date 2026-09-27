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
import {
  BYTES_PER_MB,
  QuotaUnit,
  bestQuotaUnit,
  formatQuotaValue,
  parseQuotaInput,
  validateIntField,
} from '@/lib/quota';

const COMPANY_DOMAIN_SETTINGS_KEY = 'domain_settings_defaults';

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
  const [quotaUnit, setQuotaUnit] = useState<QuotaUnit>('GB');
  // Raw text of the quota input so we can validate before converting to bytes.
  // Empty string = "unlimited"; invalid text blocks save with an errorText.
  const [quotaInput, setQuotaInput] = useState('');
  // Per-field validation messages. A non-empty entry blocks save and renders
  // as the FormField errorText — invalid input is never silently coerced.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

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
      const nextSettings = coerceSettings(entry.Value);
      const unit = bestQuotaUnit(nextSettings.quota_per_user);
      setSettings(nextSettings);
      setQuotaUnit(unit);
      setQuotaInput(formatQuotaValue(nextSettings.quota_per_user, unit));
      setFieldErrors({});
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

  const setFieldError = (key: string, message: string) => {
    setFieldErrors((prev) => {
      if (message) return { ...prev, [key]: message };
      if (!(key in prev)) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const intFieldError = (code: string, min: number, max: number): string => {
    switch (code) {
      case 'required':
        return t('pages.domain_settings_page.field_required', 'Value is required');
      case 'invalid_integer':
        return t('pages.domain_settings_page.field_invalid_integer', 'Enter a whole number');
      case 'below_min':
      case 'above_max':
        return t('pages.domain_settings_page.field_out_of_range', 'Enter a value between {min} and {max}')
          .replace('{min}', String(min))
          .replace('{max}', String(max));
      default:
        return t('pages.domain_settings_page.field_invalid_integer', 'Enter a whole number');
    }
  };

  // Validate + commit an integer field. Invalid input (blank, non-integer,
  // negative, out of range) records an errorText and blocks save instead of
  // silently falling back to a default.
  const setIntField = (
    key: keyof CompanyDomainSettings,
    raw: string,
    min: number,
    max: number,
  ) => {
    const result = validateIntField(raw, { min, max });
    setSaveSuccess(false);
    setSaveError('');
    if (!result.valid || result.value === null) {
      // Keep the raw text visible by storing the parsed number when possible,
      // but surface the error so the user cannot save a bad value.
      setSettings((prev) => ({ ...prev, [key]: Number(raw) as never }));
      setFieldError(key as string, intFieldError(result.error, min, max));
      return;
    }
    setSettings((prev) => ({ ...prev, [key]: result.value as never }));
    setFieldError(key as string, '');
  };

  const quotaError = (code: string): string => {
    switch (code) {
      case 'invalid_number':
        return t('pages.domain_settings_page.quota_invalid_number', 'Enter a valid number, or leave blank for unlimited');
      case 'negative':
        return t('pages.domain_settings_page.quota_negative', 'Quota cannot be negative');
      case 'fraction':
        return t('pages.domain_settings_page.quota_fraction', 'Enter a whole number of {unit}').replace('{unit}', quotaUnit);
      default:
        return t('pages.domain_settings_page.quota_invalid_number', 'Enter a valid number, or leave blank for unlimited');
    }
  };

  // Validate + commit the quota input. Empty = unlimited (stored as 0 by the
  // backend contract); invalid input blocks save with an errorText and never
  // silently becomes 0/unlimited.
  const handleQuotaInputChange = (raw: string) => {
    setQuotaInput(raw);
    setSaveSuccess(false);
    setSaveError('');
    const result = parseQuotaInput(raw, quotaUnit);
    if (!result.valid) {
      setFieldError('quota_per_user', quotaError(result.error));
      return;
    }
    setFieldError('quota_per_user', '');
    // Empty input => unlimited. The company settings type stores a number, so
    // represent unlimited as 0 (the wire contract used across the console).
    set('quota_per_user', result.bytes ?? 0);
  };

  // Changing the unit is a display-only concern: it must NEVER change the
  // stored byte value. We re-render the SAME byte value in the new unit rather
  // than re-interpreting the on-screen number under the new unit (which would
  // silently shrink/grow the quota by 1024x on every toggle).
  const handleQuotaUnitChange = (unit: QuotaUnit) => {
    setQuotaUnit(unit);
    // Re-derive the displayed text from the unchanged stored bytes so a
    // GB -> MB -> GB round trip is a no-op on quota_per_user.
    if (settings.quota_per_user > 0) {
      setQuotaInput(formatQuotaValue(settings.quota_per_user, unit));
      setFieldError('quota_per_user', '');
    }
    // quota_per_user itself is deliberately left untouched.
  };

  const hasFieldErrors = Object.values(fieldErrors).some(Boolean);

  const handleSave = async () => {
    if (hasFieldErrors) {
      setSaveError(t('pages.domain_settings_page.fix_errors_before_save', 'Fix the highlighted fields before saving'));
      return;
    }
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
              <FormField key="password-min-length" label={t('pages.domain_settings_page.password_min_length_label')} errorText={fieldErrors.password_min_length}>
                <Input
                  type="number"
                  value={String(settings.password_min_length)}
                  onChange={(e) => setIntField('password_min_length', e.detail.value, 4, 128)}
                />
              </FormField>
              <FormField key="password-expiry" label={t('pages.domain_settings_page.password_expiry_label')} description={t('pages.domain_settings_page.password_expiry_desc')} errorText={fieldErrors.password_expiry_days}>
                <Input
                  type="number"
                  value={String(settings.password_expiry_days)}
                  onChange={(e) => setIntField('password_expiry_days', e.detail.value, 0, 3650)}
                />
              </FormField>
              <FormField key="session-timeout" label={t('pages.domain_settings_page.session_timeout_label')} description={t('pages.domain_settings_page.minutes')} errorText={fieldErrors.session_timeout_minutes}>
                <Input
                  type="number"
                  value={String(settings.session_timeout_minutes)}
                  onChange={(e) => setIntField('session_timeout_minutes', e.detail.value, 1, 43200)}
                />
              </FormField>
              <FormField
                key="reset-ttl"
                label={t('pages.domain_settings_page.password_reset_ttl_label')}
                description={t('pages.domain_settings_page.password_reset_ttl_desc')}
                errorText={fieldErrors.password_reset_token_ttl_minutes}
              >
                <Input
                  type="number"
                  value={String(settings.password_reset_token_ttl_minutes)}
                  onChange={(e) => setIntField('password_reset_token_ttl_minutes', e.detail.value, 1, 10080)}
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
          <ColumnLayout columns={2}>
            <FormField
              label={t('pages.domain_settings_page.quota_per_user_label')}
              description={t('pages.tenancy_domains.quota_zero_unlimited', '0 = unlimited')}
              errorText={fieldErrors.quota_per_user}
            >
              <Input
                type="number"
                value={quotaInput}
                onChange={(e) => handleQuotaInputChange(e.detail.value)}
                placeholder={t('pages.tenancy_domains.quota_zero_unlimited', '0 = unlimited')}
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

        <Box key="settings-footer" float="right">
          <Button variant="primary" onClick={handleSave} loading={saving} disabled={hasFieldErrors}>
            {t('pages.domain_settings_page.save_btn')}
          </Button>
        </Box>
      </div>
    </ContentLayout>
  );
}
