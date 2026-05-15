'use client';

import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Toggle,
  FormField,
  Input,
  Textarea,
  Container,
  Alert,
  Select,
} from '@cloudscape-design/components';
import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useDomains } from '@/hooks/useDomains';
import { useAPISettings, useUpdateAPISettings, type APISettings } from '@/hooks/useAPISettings';

const DEFAULT_SETTINGS: APISettings = {
  domain_id: '',
  rate_limit_rps: 100,
  rate_limit_bps: 0,
  cidr_allowlist_enabled: false,
  cidr_allowlist: [],
  require_api_key: true,
  updated_at: '',
  updated_by: '',
};

export default function APISettingsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data: domains = [], isLoading: loadingDomains } = useDomains(companyId);
  const [selectedDomainId, setSelectedDomainId] = useState('');
  const { data: settings, isLoading: loadingSettings } = useAPISettings(selectedDomainId);
  const updateSettings = useUpdateAPISettings();
  const [form, setForm] = useState<APISettings>(DEFAULT_SETTINGS);
  const [allowlistText, setAllowlistText] = useState('');
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState('');

  useEffect(() => {
    if (!selectedDomainId && domains.length > 0) {
      setSelectedDomainId(domains[0].id);
    }
  }, [domains, selectedDomainId]);

  useEffect(() => {
    if (!settings) return;
    const nextSettings = {
      ...DEFAULT_SETTINGS,
      ...settings,
      cidr_allowlist: settings.cidr_allowlist ?? [],
    };
    setForm(nextSettings);
    setAllowlistText(nextSettings.cidr_allowlist?.join(', ') ?? '');
    setSaveError('');
    setSaveSuccess(false);
  }, [settings]);

  const domainOptions = useMemo(
    () =>
      domains.map((domain) => ({
        label: domain.name,
        value: domain.id,
        description: domain.status,
      })),
    [domains]
  );

  const handleSave = async () => {
    if (!selectedDomainId) return;
    setSaveError('');
    setSaveSuccess(false);
    try {
      await updateSettings.mutateAsync({
        domainId: selectedDomainId,
        data: {
          domain_id: selectedDomainId,
          rate_limit_rps: form.rate_limit_rps,
          rate_limit_bps: form.rate_limit_bps,
          cidr_allowlist_enabled: form.cidr_allowlist_enabled,
          cidr_allowlist: allowlistText
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean),
          require_api_key: form.require_api_key,
        },
      });
      setSaveSuccess(true);
    } catch {
      setSaveError(t('pages.api_settings_page.save_error'));
    }
  };

  if (loadingDomains) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_settings_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.api_settings_page.description')}
          actions={
            <Button variant="primary" onClick={handleSave} loading={updateSettings.isPending}>
              {t('pages.api_settings_page.save')}
            </Button>
          }
        >
          {t('pages.api_settings_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {saveSuccess && <Alert type="success">{t('pages.api_settings_page.saved')}</Alert>}
        {saveError && <Alert type="error">{saveError}</Alert>}

        <Container header={<Header variant="h2">{t('pages.api_settings_page.domain_scope')}</Header>}>
          <FormField label={t('pages.api_settings_page.select_domain_label')}>
            <Select
              selectedOption={domainOptions.find((option) => option.value === selectedDomainId) ?? null}
              options={domainOptions}
              onChange={(event) => setSelectedDomainId(event.detail.selectedOption.value ?? '')}
              placeholder={t('pages.api_settings_page.select_domain_placeholder')}
              expandToViewport
            />
          </FormField>
        </Container>

        {loadingSettings && (
          <Box textAlign="center" padding="xl">
            <Spinner />
          </Box>
        )}

        {!loadingSettings && selectedDomainId && (
          <Container header={<Header variant="h2">{t('pages.api_settings_page.settings_section')}</Header>}>
            <SpaceBetween size="m">
              <FormField label={t('pages.api_settings_page.rate_limit_rps')}>
                <Input
                  type="number"
                  value={String(form.rate_limit_rps)}
                  onChange={(event) => setForm({ ...form, rate_limit_rps: parseInt(event.detail.value) || 0 })}
                />
              </FormField>

              <FormField label={t('pages.api_settings_page.rate_limit_bps')}>
                <Input
                  type="number"
                  value={String(form.rate_limit_bps)}
                  onChange={(event) => setForm({ ...form, rate_limit_bps: parseInt(event.detail.value) || 0 })}
                />
              </FormField>

              <FormField
                label={t('pages.api_settings_page.cidr_allowlist_enabled')}
                description={t('pages.api_settings_page.cidr_allowlist_enabled_desc')}
              >
                <Toggle
                  checked={form.cidr_allowlist_enabled}
                  onChange={(event) => setForm({ ...form, cidr_allowlist_enabled: event.detail.checked })}
                >
                  {form.cidr_allowlist_enabled ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>

              <FormField
                label={t('pages.api_settings_page.cidr_allowlist')}
                description={t('pages.api_settings_page.cidr_allowlist_desc')}
              >
                <Textarea
                  value={allowlistText}
                  onChange={(event) => setAllowlistText(event.detail.value)}
                  placeholder={t('pages.api_settings_page.cidr_allowlist_placeholder')}
                />
              </FormField>

              <FormField
                label={t('pages.api_settings_page.require_api_key')}
                description={t('pages.api_settings_page.require_api_key_desc')}
              >
                <Toggle
                  checked={form.require_api_key}
                  onChange={(event) => setForm({ ...form, require_api_key: event.detail.checked })}
                >
                  {form.require_api_key ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>
            </SpaceBetween>
          </Container>
        )}

        {settings?.updated_at && (
          <Box color="text-body-secondary" fontSize="body-s">
            {t('pages.api_settings_page.last_updated')}: {new Date(settings.updated_at).toLocaleString()}
            {settings.updated_by && <> · {settings.updated_by}</>}
          </Box>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
