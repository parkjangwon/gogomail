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
  Checkbox,
  Container,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface AuthPolicy {
  min_length: number;
  require_uppercase: boolean;
  require_numbers: boolean;
  require_symbols: boolean;
  max_age_days: number;
  history_count: number;
  mfa_required: boolean;
  mfa_methods: string[];
  session_timeout_minutes: number;
  max_concurrent_sessions: number;
}

const DEFAULT_POLICY: AuthPolicy = {
  min_length: 8,
  require_uppercase: false,
  require_numbers: false,
  require_symbols: false,
  max_age_days: 0,
  history_count: 0,
  mfa_required: false,
  mfa_methods: ['totp'],
  session_timeout_minutes: 480,
  max_concurrent_sessions: 0,
};

export default function AuthPolicyPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<AuthPolicy>(DEFAULT_POLICY);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetchPolicy();
  }, [companyId]);

  const fetchPolicy = async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/security/auth-policy`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setPolicy(data.policy);
      }
    } catch (error) {
      console.error('Failed to fetch auth policy:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/security/auth-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
    } catch (error) {
      console.error('Failed to save auth policy:', error);
    } finally {
      setSaving(false);
    }
  };

  const toggleMFAMethod = (method: string, checked: boolean) => {
    const mfa_methods = checked
      ? [...policy.mfa_methods, method]
      : policy.mfa_methods.filter((m) => m !== method);
    setPolicy({ ...policy, mfa_methods });
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.auth_policy_page.title')}</Header>}>
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
          description={t('pages.auth_policy_page.description')}
          actions={
            <Button variant="primary" onClick={handleSave} loading={saving}>
              {t('pages.auth_policy_page.save')}
            </Button>
          }
        >
          {t('pages.auth_policy_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.auth_policy_page.password_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.auth_policy_page.min_length')}>
              <Input
                type="number"
                value={String(policy.min_length)}
                onChange={(e) => setPolicy({ ...policy, min_length: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.auth_policy_page.require_uppercase')}>
              <Toggle
                checked={policy.require_uppercase}
                onChange={(e) => setPolicy({ ...policy, require_uppercase: e.detail.checked })}
              >
                {policy.require_uppercase ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField label={t('pages.auth_policy_page.require_numbers')}>
              <Toggle
                checked={policy.require_numbers}
                onChange={(e) => setPolicy({ ...policy, require_numbers: e.detail.checked })}
              >
                {policy.require_numbers ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField label={t('pages.auth_policy_page.require_symbols')}>
              <Toggle
                checked={policy.require_symbols}
                onChange={(e) => setPolicy({ ...policy, require_symbols: e.detail.checked })}
              >
                {policy.require_symbols ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField label={t('pages.auth_policy_page.max_age_days')}>
              <Input
                type="number"
                value={String(policy.max_age_days)}
                onChange={(e) => setPolicy({ ...policy, max_age_days: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.auth_policy_page.history_count')}>
              <Input
                type="number"
                value={String(policy.history_count)}
                onChange={(e) => setPolicy({ ...policy, history_count: parseInt(e.detail.value) || 0 })}
              />
            </FormField>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">{t('pages.auth_policy_page.mfa_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.auth_policy_page.mfa_required')}>
              <Toggle
                checked={policy.mfa_required}
                onChange={(e) => setPolicy({ ...policy, mfa_required: e.detail.checked })}
              >
                {policy.mfa_required ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField label={t('pages.auth_policy_page.mfa_methods')}>
              <SpaceBetween direction="horizontal" size="m">
                <Checkbox
                  checked={policy.mfa_methods.includes('totp')}
                  onChange={(e) => toggleMFAMethod('totp', e.detail.checked)}
                >
                  TOTP
                </Checkbox>
                <Checkbox
                  checked={policy.mfa_methods.includes('fido2')}
                  onChange={(e) => toggleMFAMethod('fido2', e.detail.checked)}
                >
                  FIDO2
                </Checkbox>
              </SpaceBetween>
            </FormField>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">{t('pages.auth_policy_page.session_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.auth_policy_page.session_timeout')}>
              <Input
                type="number"
                value={String(policy.session_timeout_minutes)}
                onChange={(e) =>
                  setPolicy({ ...policy, session_timeout_minutes: parseInt(e.detail.value) || 0 })
                }
              />
            </FormField>

            <FormField label={t('pages.auth_policy_page.max_concurrent')}>
              <Input
                type="number"
                value={String(policy.max_concurrent_sessions)}
                onChange={(e) =>
                  setPolicy({ ...policy, max_concurrent_sessions: parseInt(e.detail.value) || 0 })
                }
              />
            </FormField>
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
