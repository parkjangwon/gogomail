'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Button,
  FormField,
  Input,
  KeyValuePairs,
  Alert,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useConsoleCapabilities } from '@/hooks/useConsoleCapabilities';
import { useOrganizationSettings, useUpdateOrganizationSettings } from '@/hooks';

export default function OrganizationSettingsPage() {
  const { t } = useI18n();
  const { data: capabilities } = useConsoleCapabilities();
  const settingsQuery = useOrganizationSettings();
  const updateSettings = useUpdateOrganizationSettings();
  const settings = settingsQuery.data?.settings ?? null;
  const [draft, setDraft] = useState(settings);
  const loading = settingsQuery.isLoading;
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);

  const handleSave = async () => {
    if (!draft) return;
    setSaving(true);
    try {
      await updateSettings.mutateAsync(draft);
      setEditing(false);
    } catch (error) {
      console.error('Failed to save organization settings:', error);
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setDraft(settings);
    setEditing(false);
  };

  useEffect(() => {
    setDraft(settings);
  }, [settings]);

  const integrationStatus = capabilities?.integrations.organization_sync ?? 'planned';
  const canEdit = integrationStatus === 'available';

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.organization.title')}</Header>}>
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
          description={t('pages.organization.description')}
          actions={
            editing ? (
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={handleCancel}>{t('pages.organization_page.cancel')}</Button>
                <Button variant="primary" onClick={handleSave} loading={saving} disabled={!canEdit}>
                  {t('common.save')}
                </Button>
              </SpaceBetween>
            ) : (
              <Button variant="primary" onClick={() => setEditing(true)} disabled={!canEdit}>
                {t('pages.organization_page.edit_settings')}
              </Button>
            )
          }
        >
          {t('pages.organization.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {integrationStatus !== 'available' && (
          <Alert
            type={integrationStatus === 'planned' ? 'warning' : 'info'}
            header={t('pages.organization_page.integration_status_header')}
          >
            {integrationStatus === 'placeholder'
              ? t('pages.organization_page.integration_status_placeholder')
              : 'Organization sync is planned but not available yet.'}
          </Alert>
        )}

        {settings && draft && (
          <>
            {!editing ? (
              <Container header={<Header variant="h3">{t('pages.organization_page.current_settings')}</Header>}>
                <KeyValuePairs
                  items={[
                    { label: t('pages.organization_page.org_name'), value: settings.name },
                    { label: t('pages.organization_page.description_label'), value: settings.description || '—' },
                    { label: t('pages.organization_page.max_users'), value: settings.max_users },
                    { label: t('pages.organization_page.max_domains'), value: settings.max_domains },
                    { label: t('pages.organization_page.integration_status_header'), value: <Badge color={integrationStatus === 'available' ? 'green' : integrationStatus === 'placeholder' ? 'blue' : 'grey'}>{integrationStatus}</Badge> },
                    { label: t('pages.organization_page.created'), value: new Date(settings.created_at ?? Date.now()).toLocaleString() },
                  ]}
                />
              </Container>
            ) : (
              <Container header={<Header variant="h3">{t('pages.organization_page.edit_settings_header')}</Header>}>
                <SpaceBetween size="m">
                  <FormField label={t('pages.organization_page.org_name')}>
                    <Input
                      value={draft.name ?? ''}
                      onChange={(e) => setDraft({ ...draft, name: e.detail.value })}
                      placeholder={t('pages.organization_page.org_name_placeholder')}
                    />
                  </FormField>
                  <FormField label={t('pages.organization_page.description_label')}>
                    <Input
                      value={draft.description ?? ''}
                      onChange={(e) => setDraft({ ...draft, description: e.detail.value })}
                      placeholder={t('pages.organization_page.desc_placeholder')}
                    />
                  </FormField>
                  <FormField label={t('pages.organization_page.max_users')}>
                    <Input
                      type="number"
                      value={(draft.max_users ?? 0).toString()}
                      onChange={(e) => setDraft({ ...draft, max_users: parseInt(e.detail.value) || 0 })}
                    />
                  </FormField>
                  <FormField label={t('pages.organization_page.max_domains')}>
                    <Input
                      type="number"
                      value={(draft.max_domains ?? 0).toString()}
                      onChange={(e) => setDraft({ ...draft, max_domains: parseInt(e.detail.value) || 0 })}
                    />
                  </FormField>
                </SpaceBetween>
              </Container>
            )}
          </>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
