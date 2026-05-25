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
  Select,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface RateLimitPolicy {
  enabled: boolean;
  max_per_hour: number;
  max_per_day: number;
  max_recipients_per_msg: number;
  max_message_size_mb: number;
  action_on_exceed: string;
  per_user_max_per_hour: number;
  per_user_max_per_day: number;
}

const DEFAULT_POLICY: RateLimitPolicy = {
  enabled: false,
  max_per_hour: 0,
  max_per_day: 0,
  max_recipients_per_msg: 100,
  max_message_size_mb: 25,
  action_on_exceed: 'queue',
  per_user_max_per_hour: 0,
  per_user_max_per_day: 500,
};

export default function RateLimitsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<RateLimitPolicy>(DEFAULT_POLICY);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const fetchPolicy = useCallback(async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/security/rate-limit`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setPolicy(data.policy);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  }, [companyId]);

  useEffect(() => {
    fetchPolicy();
  }, [fetchPolicy]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/security/rate-limit`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
    } catch {
      // mutation error handled by caller
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.rate_limit_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  const actionOptions = [
    { value: 'queue', label: t('pages.rate_limit_page.action_queue') },
    { value: 'reject', label: t('pages.rate_limit_page.action_reject') },
  ];

  const selectedAction = actionOptions.find((o) => o.value === policy.action_on_exceed) ?? actionOptions[0];

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.rate_limit_page.description')}>
          {t('pages.rate_limit_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.rate_limit_page.company_limits')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.rate_limit_page.enabled')}>
              <Toggle
                checked={policy.enabled}
                onChange={(e) => setPolicy({ ...policy, enabled: e.detail.checked })}
              >
                {policy.enabled ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField label={t('pages.rate_limit_page.max_per_hour')}>
              <Input
                type="number"
                value={String(policy.max_per_hour)}
                onChange={(e) => setPolicy({ ...policy, max_per_hour: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.rate_limit_page.max_per_day')}>
              <Input
                type="number"
                value={String(policy.max_per_day)}
                onChange={(e) => setPolicy({ ...policy, max_per_day: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.rate_limit_page.max_recipients')}>
              <Input
                type="number"
                value={String(policy.max_recipients_per_msg)}
                onChange={(e) => setPolicy({ ...policy, max_recipients_per_msg: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.rate_limit_page.max_size_mb')}>
              <Input
                type="number"
                value={String(policy.max_message_size_mb)}
                onChange={(e) => setPolicy({ ...policy, max_message_size_mb: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.rate_limit_page.action_on_exceed')}>
              <Select
                selectedOption={selectedAction}
                options={actionOptions}
                onChange={(e) => setPolicy({ ...policy, action_on_exceed: e.detail.selectedOption.value ?? 'queue' })}
              />
            </FormField>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">{t('pages.rate_limit_page.per_user_limits')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.rate_limit_page.per_user_hour')}>
              <Input
                type="number"
                value={String(policy.per_user_max_per_hour)}
                onChange={(e) => setPolicy({ ...policy, per_user_max_per_hour: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.rate_limit_page.per_user_day')}>
              <Input
                type="number"
                value={String(policy.per_user_max_per_day)}
                onChange={(e) => setPolicy({ ...policy, per_user_max_per_day: parseInt(e.detail.value) || 0 })}
              />
            </FormField>
          </SpaceBetween>
        </Container>

        <Button variant="primary" onClick={handleSave} loading={saving}>
          {t('pages.rate_limit_page.save')}
        </Button>
      </SpaceBetween>
    </ContentLayout>
  );
}
