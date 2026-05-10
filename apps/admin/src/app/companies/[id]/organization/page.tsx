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
              {editing ? 'Cancel' : 'Edit Settings'}
            </Button>
          }
        >
          Organization Settings
        </Header>
      }
    >
      <SpaceBetween size="l">
        {settings && (
          <>
            {!editing ? (
              <Container header={<Header variant="h3">Current Settings</Header>}>
                <KeyValuePairs
                  items={[
                    { label: 'Organization Name', value: settings.name },
                    { label: 'Description', value: settings.description },
                    { label: 'Max Users', value: settings.max_users },
                    { label: 'Max Domains', value: settings.max_domains },
                    { label: 'Created', value: new Date(settings.created_at).toLocaleString() },
                  ]}
                />
              </Container>
            ) : (
              <Container header={<Header variant="h3">Edit Settings</Header>}>
                <SpaceBetween size="m">
                  <FormField label="Organization Name">
                    <Input value={settings.name} placeholder="Organization name" />
                  </FormField>
                  <FormField label="Description">
                    <Input value={settings.description} placeholder="Description" />
                  </FormField>
                  <FormField label="Max Users">
                    <Input type="number" value={settings.max_users.toString()} />
                  </FormField>
                  <FormField label="Max Domains">
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
