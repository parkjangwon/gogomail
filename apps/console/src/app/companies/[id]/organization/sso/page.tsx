'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Textarea,
  Toggle,
  Select,
  SelectProps,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
  ExpandableSection,
  ColumnLayout,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface SSOConfig {
  enabled: boolean;
  provider: string;
  entity_id: string;
  metadata_url: string;
  sso_login_url: string;
  certificate: string;
  attribute_email: string;
  attribute_name: string;
  force_sso: boolean;
  auto_provision: boolean;
  default_role: string;
}

const defaultConfig = (): SSOConfig => ({
  enabled: false,
  provider: 'saml',
  entity_id: '',
  metadata_url: '',
  sso_login_url: '',
  certificate: '',
  attribute_email: 'email',
  attribute_name: 'displayName',
  force_sso: false,
  auto_provision: false,
  default_role: 'viewer',
});

const PROVIDER_OPTIONS: SelectProps.Option[] = [
  { value: 'saml', label: 'SAML 2.0' },
  { value: 'oidc', label: 'OpenID Connect' },
  { value: 'google', label: 'Google Workspace' },
  { value: 'azure_ad', label: 'Azure AD' },
  { value: 'okta', label: 'Okta' },
];

const ROLE_OPTIONS: SelectProps.Option[] = [
  { value: 'admin', label: 'Admin' },
  { value: 'operator', label: 'Operator' },
  { value: 'viewer', label: 'Viewer' },
];

export default function SSOPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [config, setConfig] = useState<SSOConfig>(defaultConfig());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const fetchConfig = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/sso/config`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setConfig({ ...defaultConfig(), ...(data.config ?? {}) });
      }
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { fetchConfig(); }, [fetchConfig]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/sso/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
        credentials: 'include',
      });
      if (res.ok) {
        setNotifications([{ type: 'success', content: t('pages.sso_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.sso_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/sso/test`, {
        method: 'POST',
        credentials: 'include',
      });
      const data = await res.json();
      if (res.ok) {
        setTestResult({ success: data.success, message: data.message });
      } else {
        setTestResult({ success: false, message: data.error ?? t('pages.sso_page.test_error') });
      }
    } finally {
      setTesting(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.sso_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const selectedProvider = PROVIDER_OPTIONS.find(o => o.value === config.provider) ?? PROVIDER_OPTIONS[0];
  const selectedRole = ROLE_OPTIONS.find(o => o.value === config.default_role) ?? ROLE_OPTIONS[2];

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.sso_page.description')}>
          {t('pages.sso_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        {/* Status Banner */}
        {config.enabled
          ? <Alert type="success">{t('pages.sso_page.status_enabled')}</Alert>
          : <Alert type="info">{t('pages.sso_page.status_disabled')}</Alert>
        }

        {/* Provider Setup */}
        <Container header={<Header variant="h2">{t('pages.sso_page.provider_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.sso_page.enabled_label')} description={t('pages.sso_page.enabled_desc')}>
              <Toggle
                checked={config.enabled}
                onChange={e => setConfig(c => ({ ...c, enabled: e.detail.checked }))}
              >
                {config.enabled ? t('pages.sso_page.enabled_on') : t('pages.sso_page.enabled_off')}
              </Toggle>
            </FormField>

            <FormField label={t('pages.sso_page.provider_label')}>
              <Select
                selectedOption={selectedProvider}
                onChange={e => setConfig(c => ({ ...c, provider: e.detail.selectedOption.value ?? 'saml' }))}
                options={PROVIDER_OPTIONS}
              />
            </FormField>

            <FormField
              label={t('pages.sso_page.force_sso_label')}
              description={t('pages.sso_page.force_sso_desc')}
            >
              <SpaceBetween size="xs">
                <Toggle
                  checked={config.force_sso}
                  onChange={e => setConfig(c => ({ ...c, force_sso: e.detail.checked }))}
                >
                  {config.force_sso ? t('pages.sso_page.enabled_on') : t('pages.sso_page.enabled_off')}
                </Toggle>
                {config.force_sso && (
                  <Alert type="warning">{t('pages.sso_page.force_sso_warning')}</Alert>
                )}
              </SpaceBetween>
            </FormField>

            <FormField
              label={t('pages.sso_page.auto_provision_label')}
              description={t('pages.sso_page.auto_provision_desc')}
            >
              <Toggle
                checked={config.auto_provision}
                onChange={e => setConfig(c => ({ ...c, auto_provision: e.detail.checked }))}
              >
                {config.auto_provision ? t('pages.sso_page.enabled_on') : t('pages.sso_page.enabled_off')}
              </Toggle>
            </FormField>

            {config.auto_provision && (
              <FormField label={t('pages.sso_page.default_role_label')} description={t('pages.sso_page.default_role_desc')}>
                <Select
                  selectedOption={selectedRole}
                  onChange={e => setConfig(c => ({ ...c, default_role: e.detail.selectedOption.value ?? 'viewer' }))}
                  options={ROLE_OPTIONS}
                />
              </FormField>
            )}
          </SpaceBetween>
        </Container>

        {/* Identity Provider Settings */}
        {config.enabled && (
          <Container header={<Header variant="h2">{t('pages.sso_page.idp_section')}</Header>}>
            <SpaceBetween size="m">
              <FormField label={t('pages.sso_page.entity_id_label')}>
                <ColumnLayout columns={1}>
                  <Input value={config.entity_id || `urn:gogomail:sp:${cid}`} readOnly onChange={() => {}} />
                </ColumnLayout>
              </FormField>
              <Alert type="info">{t('pages.sso_page.entity_id_hint')}</Alert>

              <FormField
                label={t('pages.sso_page.metadata_url_label')}
                description={t('pages.sso_page.metadata_url_desc')}
              >
                <Input
                  value={config.metadata_url}
                  onChange={e => setConfig(c => ({ ...c, metadata_url: e.detail.value }))}
                  placeholder="https://idp.example.com/metadata"
                />
              </FormField>

              <FormField
                label={t('pages.sso_page.sso_login_url_label')}
                description={t('pages.sso_page.sso_login_url_desc')}
              >
                <Input
                  value={config.sso_login_url}
                  onChange={e => setConfig(c => ({ ...c, sso_login_url: e.detail.value }))}
                  placeholder="https://idp.example.com/sso"
                />
              </FormField>

              <FormField
                label={t('pages.sso_page.certificate_label')}
                description={t('pages.sso_page.certificate_desc')}
              >
                <Textarea
                  value={config.certificate}
                  onChange={e => setConfig(c => ({ ...c, certificate: e.detail.value }))}
                  placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                  rows={6}
                />
              </FormField>
            </SpaceBetween>
          </Container>
        )}

        {/* Attribute Mapping */}
        <ExpandableSection headerText={t('pages.sso_page.attribute_section')}>
          <SpaceBetween size="m">
            <FormField
              label={t('pages.sso_page.attribute_email_label')}
              constraintText={t('pages.sso_page.attribute_email_hint')}
            >
              <Input
                value={config.attribute_email}
                onChange={e => setConfig(c => ({ ...c, attribute_email: e.detail.value }))}
                placeholder="email"
              />
            </FormField>
            <FormField
              label={t('pages.sso_page.attribute_name_label')}
              constraintText={t('pages.sso_page.attribute_name_hint')}
            >
              <Input
                value={config.attribute_name}
                onChange={e => setConfig(c => ({ ...c, attribute_name: e.detail.value }))}
                placeholder="displayName"
              />
            </FormField>
          </SpaceBetween>
        </ExpandableSection>

        {/* Test Result */}
        {testResult !== null && (
          <Alert type={testResult.success ? 'success' : 'error'}>
            {testResult.message}
          </Alert>
        )}

        {/* Actions */}
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={handleTest} loading={testing} disabled={!config.enabled}>
              {t('pages.sso_page.test_connection')}
            </Button>
            <Button variant="primary" onClick={handleSave} loading={saving}>
              {t('pages.sso_page.save')}
            </Button>
          </SpaceBetween>
        </Box>
      </SpaceBetween>
    </ContentLayout>
  );
}
