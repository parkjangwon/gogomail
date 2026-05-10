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
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface OrgSettings {
  name: string;
  description: string;
  max_users: number;
  max_domains: number;
  created_at: string;
}

export default function OrganizationSettingsPage() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<OrgSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);

  useEffect(() => {
    fetchOrgSettings();
  }, []);

  const fetchOrgSettings = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/organization/settings', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setSettings(data.settings);
      }
    } catch (error) {
      console.error('Failed to fetch organization settings:', error);
    } finally {
      setLoading(false);
    }
  };

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
            <Button variant="primary" onClick={() => setEditing(!editing)}>
              {editing ? t('pages.organization_page.cancel') : t('pages.organization_page.edit_settings')}
            </Button>
          }
        >
          {t('pages.organization.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {settings && (
          <>
            {!editing ? (
              <Container header={<Header variant="h3">{t('pages.organization_page.current_settings')}</Header>}>
                <KeyValuePairs
                  items={[
                    { label: t('pages.organization_page.org_name'), value: settings.name },
                    { label: t('pages.organization_page.description_label'), value: settings.description },
                    { label: t('pages.organization_page.max_users'), value: settings.max_users },
                    { label: t('pages.organization_page.max_domains'), value: settings.max_domains },
                    { label: t('pages.organization_page.created'), value: new Date(settings.created_at).toLocaleString() },
                  ]}
                />
              </Container>
            ) : (
              <Container header={<Header variant="h3">{t('pages.organization_page.edit_settings_header')}</Header>}>
                <SpaceBetween size="m">
                  <FormField label={t('pages.organization_page.org_name')}>
                    <Input value={settings.name} placeholder={t('pages.organization_page.org_name_placeholder')} />
                  </FormField>
                  <FormField label={t('pages.organization_page.description_label')}>
                    <Input value={settings.description} placeholder={t('pages.organization_page.desc_placeholder')} />
                  </FormField>
                  <FormField label={t('pages.organization_page.max_users')}>
                    <Input type="number" value={settings.max_users.toString()} />
                  </FormField>
                  <FormField label={t('pages.organization_page.max_domains')}>
                    <Input type="number" value={settings.max_domains.toString()} />
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
