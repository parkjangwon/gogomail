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
  Modal,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompanySSOConfig, useUpdateCompanySSOConfig, useTestCompanySSOConfig } from '@/hooks';

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
  const params = useParams();
  // Route param is the authoritative tenant id. Never fall back to a
  // placeholder such as 'default' — that would fetch/PUT another tenant's
  // auth config. Empty until the route resolves; the query is gated on it.
  const cid = (params?.id as string) ?? '';

  const [config, setConfig] = useState<SSOConfig>(defaultConfig());
  const configQuery = useCompanySSOConfig(cid || undefined);
  const updateConfig = useUpdateCompanySSOConfig();
  const testConfig = useTestCompanySSOConfig();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [urlErrors, setUrlErrors] = useState<{ metadata_url?: string; sso_login_url?: string }>({});
  const [disableConfirmVisible, setDisableConfirmVisible] = useState(false);

  const fetchConfig = useCallback(async () => {
    setLoading(configQuery.isLoading);
    if (configQuery.data) {
      setConfig({ ...defaultConfig(), ...(configQuery.data as Partial<SSOConfig>) });
    }
  }, [configQuery.data, configQuery.isLoading]);

  useEffect(() => { fetchConfig(); }, [fetchConfig]);

  // Optional http(s) URL validation. Empty is allowed (fields are optional).
  const isValidHttpUrl = (value: string): boolean => {
    if (!value.trim()) return true;
    try {
      const u = new URL(value);
      return u.protocol === 'http:' || u.protocol === 'https:';
    } catch {
      return false;
    }
  };

  const validateUrls = (): boolean => {
    const errors: { metadata_url?: string; sso_login_url?: string } = {};
    if (!isValidHttpUrl(config.metadata_url)) errors.metadata_url = t('pages.sso_page.url_invalid');
    if (!isValidHttpUrl(config.sso_login_url)) errors.sso_login_url = t('pages.sso_page.url_invalid');
    setUrlErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const persistConfig = async () => {
    setSaving(true);
    try {
      await updateConfig.mutateAsync({ companyId: cid, data: config as unknown as Record<string, never> });
      setNotifications([{ type: 'success', content: t('pages.sso_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      setConfig({ ...config });
    } catch {
      // A failed save must be surfaced — an admin must never believe SSO
      // saved when it did not.
      setNotifications([{ type: 'error', content: t('pages.sso_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (!validateUrls()) return;
    // Disabling SSO is a lockout-grade action — require explicit confirmation.
    if (!config.enabled && (configQuery.data as Partial<SSOConfig> | null)?.enabled) {
      setDisableConfirmVisible(true);
      return;
    }
    await persistConfig();
  };

  const confirmDisableAndSave = async () => {
    setDisableConfirmVisible(false);
    await persistConfig();
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const data = await testConfig.mutateAsync({ companyId: cid });
      setTestResult({ success: !!data.success, message: data.message ?? t('pages.sso_page.test_error') });
    } finally {
      setTesting(false);
    }
  };

  if (!cid || loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.sso_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="s">
            <Spinner />
            {!cid && <Box color="text-body-secondary">{t('pages.sso_page.loading_company')}</Box>}
          </SpaceBetween>
        </Box>
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
                  <Input value={config.entity_id || `urn:gogomail:sp:${cid}`} readOnly />
                </ColumnLayout>
              </FormField>
              <Alert type="info">{t('pages.sso_page.entity_id_hint')}</Alert>

              <FormField
                label={t('pages.sso_page.metadata_url_label')}
                description={t('pages.sso_page.metadata_url_desc')}
                errorText={urlErrors.metadata_url}
              >
                <Input
                  value={config.metadata_url}
                  onChange={e => { setConfig(c => ({ ...c, metadata_url: e.detail.value })); setUrlErrors(prev => ({ ...prev, metadata_url: undefined })); }}
                  placeholder="https://idp.example.com/metadata"
                />
              </FormField>

              <FormField
                label={t('pages.sso_page.sso_login_url_label')}
                description={t('pages.sso_page.sso_login_url_desc')}
                errorText={urlErrors.sso_login_url}
              >
                <Input
                  value={config.sso_login_url}
                  onChange={e => { setConfig(c => ({ ...c, sso_login_url: e.detail.value })); setUrlErrors(prev => ({ ...prev, sso_login_url: undefined })); }}
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

      {/* Disable SSO confirmation — lockout-grade action */}
      <Modal
        visible={disableConfirmVisible}
        onDismiss={() => setDisableConfirmVisible(false)}
        header={t('pages.sso_page.disable_confirm_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setDisableConfirmVisible(false)}>
                {t('pages.sso_page.cancel')}
              </Button>
              <Button variant="primary" onClick={confirmDisableAndSave} loading={saving}>
                {t('pages.sso_page.disable_confirm_action')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Alert type="warning">{t('pages.sso_page.disable_confirm_body')}</Alert>
      </Modal>
    </ContentLayout>
  );
}
