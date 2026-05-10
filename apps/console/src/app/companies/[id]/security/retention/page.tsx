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
  Container,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface RetentionPolicy {
  mail_retention_days: number;
  deleted_items_retention_days: number;
  audit_log_retention_days: number;
  attachment_retention_days: number;
  auto_purge_enabled: boolean;
}

const DEFAULT_POLICY: RetentionPolicy = {
  mail_retention_days: 0,
  deleted_items_retention_days: 30,
  audit_log_retention_days: 365,
  attachment_retention_days: 0,
  auto_purge_enabled: false,
};

export default function RetentionPolicyPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<RetentionPolicy>(DEFAULT_POLICY);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetchPolicy();
  }, [companyId]);

  const fetchPolicy = async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/security/retention-policy`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setPolicy(data.policy);
      }
    } catch (error) {
      console.error('Failed to fetch retention policy:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/security/retention-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
    } catch (error) {
      console.error('Failed to save retention policy:', error);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.retention_page.title')}</Header>}>
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
          description={t('pages.retention_page.description')}
          actions={
            <Button variant="primary" onClick={handleSave} loading={saving}>
              {t('pages.retention_page.save')}
            </Button>
          }
        >
          {t('pages.retention_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="warning">
          {t('pages.retention_page.warning')}
        </Alert>

        <Container>
          <SpaceBetween size="m">
            <FormField label={t('pages.retention_page.mail_retention')}>
              <Input
                type="number"
                value={String(policy.mail_retention_days)}
                onChange={(e) => setPolicy({ ...policy, mail_retention_days: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.retention_page.deleted_retention')}>
              <Input
                type="number"
                value={String(policy.deleted_items_retention_days)}
                onChange={(e) => setPolicy({ ...policy, deleted_items_retention_days: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.retention_page.audit_retention')}>
              <Input
                type="number"
                value={String(policy.audit_log_retention_days)}
                onChange={(e) => setPolicy({ ...policy, audit_log_retention_days: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.retention_page.attachment_retention')}>
              <Input
                type="number"
                value={String(policy.attachment_retention_days)}
                onChange={(e) => setPolicy({ ...policy, attachment_retention_days: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField
              label={t('pages.retention_page.auto_purge')}
              description={t('pages.retention_page.auto_purge_desc')}
            >
              <Toggle
                checked={policy.auto_purge_enabled}
                onChange={(e) => setPolicy({ ...policy, auto_purge_enabled: e.detail.checked })}
              >
                {policy.auto_purge_enabled ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>
          </SpaceBetween>
        </Container>

        <Box color="text-body-secondary" fontSize="body-s">
          {t('pages.retention_page.scope_note')}
        </Box>
      </SpaceBetween>
    </ContentLayout>
  );
}
