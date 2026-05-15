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
  Select,
  type SelectProps,
  Container,
  Alert,
  ColumnLayout,
} from '@cloudscape-design/components';
import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useAuditPolicy, useUpdateAuditPolicy, type AuditLevel } from '@/hooks/useAuditPolicy';

const AUDIT_LEVEL_OPTIONS: SelectProps.Option[] = [
  { label: 'Level 1', value: 'level_1' },
  { label: 'Level 2', value: 'level_2' },
  { label: 'Level 3', value: 'level_3' },
];

const DEFAULT_POLICY = {
  company_id: '',
  audit_level: 'level_2' as AuditLevel,
  audit_admin_actions: true,
  audit_security_events: true,
  retention_days: 90,
  mask_mail_content: true,
  mask_recipient_emails: false,
};

function toOption(level: AuditLevel): SelectProps.Option {
  return AUDIT_LEVEL_OPTIONS.find((option) => option.value === level) ?? AUDIT_LEVEL_OPTIONS[0];
}

export default function AuditPolicyPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data, isLoading } = useAuditPolicy(companyId);
  const updatePolicy = useUpdateAuditPolicy();
  const [policy, setPolicy] = useState(DEFAULT_POLICY);

  useEffect(() => {
    if (!companyId) return;
    setPolicy((current) => ({ ...current, company_id: companyId }));
  }, [companyId]);

  useEffect(() => {
    if (!data) return;
    setPolicy({
      company_id: data.company_id || companyId,
      audit_level: data.audit_level,
      audit_admin_actions: data.audit_admin_actions,
      audit_security_events: data.audit_security_events,
      retention_days: data.retention_days,
      mask_mail_content: data.mask_mail_content,
      mask_recipient_emails: data.mask_recipient_emails,
    });
  }, [data]);

  const selectedLevel = useMemo(() => toOption(policy.audit_level), [policy.audit_level]);

  const handleSave = async () => {
    await updatePolicy.mutateAsync(policy);
  };

  if (isLoading && !data) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.audit_policy_page.title')}</Header>}>
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
          description={t('pages.audit_policy_page.description')}
          actions={
            <Button variant="primary" onClick={handleSave} loading={updatePolicy.isPending}>
              {t('pages.audit_policy_page.save')}
            </Button>
          }
        >
          {t('pages.audit_policy_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="info">
          {t('pages.audit_policy_page.info')}
        </Alert>

        <Container header={<Header variant="h2">{t('pages.audit_policy_page.company_scope')}</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <FormField label={t('pages.audit_policy_page.audit_level')}>
                <Select
                  selectedOption={selectedLevel}
                  options={AUDIT_LEVEL_OPTIONS}
                  onChange={(e) => setPolicy({ ...policy, audit_level: e.detail.selectedOption.value as AuditLevel })}
                />
              </FormField>

              <FormField
                label={t('pages.audit_policy_page.retention_days')}
                description={t('pages.audit_policy_page.retention_days_desc')}
              >
                <Input
                  type="number"
                  value={String(policy.retention_days)}
                  onChange={(e) => setPolicy({ ...policy, retention_days: parseInt(e.detail.value) || 0 })}
                />
              </FormField>
            </ColumnLayout>

            <FormField
              label={t('pages.audit_policy_page.audit_admin_actions')}
              description={t('pages.audit_policy_page.audit_admin_actions_desc')}
            >
              <Toggle
                checked={policy.audit_admin_actions}
                onChange={(e) => setPolicy({ ...policy, audit_admin_actions: e.detail.checked })}
              >
                {policy.audit_admin_actions ? t('common.enabled') : t('common.disabled')}
              </Toggle>
            </FormField>

            <FormField
              label={t('pages.audit_policy_page.audit_security_events')}
              description={t('pages.audit_policy_page.audit_security_events_desc')}
            >
              <Toggle
                checked={policy.audit_security_events}
                onChange={(e) => setPolicy({ ...policy, audit_security_events: e.detail.checked })}
              >
                {policy.audit_security_events ? t('common.enabled') : t('common.disabled')}
              </Toggle>
            </FormField>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">{t('pages.audit_policy_page.masking_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField
              label={t('pages.audit_policy_page.mask_mail_content')}
              description={t('pages.audit_policy_page.mask_mail_content_desc')}
            >
              <Toggle
                checked={policy.mask_mail_content}
                onChange={(e) => setPolicy({ ...policy, mask_mail_content: e.detail.checked })}
              >
                {policy.mask_mail_content ? t('common.enabled') : t('common.disabled')}
              </Toggle>
            </FormField>

            <FormField
              label={t('pages.audit_policy_page.mask_recipient_emails')}
              description={t('pages.audit_policy_page.mask_recipient_emails_desc')}
            >
              <Toggle
                checked={policy.mask_recipient_emails}
                onChange={(e) => setPolicy({ ...policy, mask_recipient_emails: e.detail.checked })}
              >
                {policy.mask_recipient_emails ? t('common.enabled') : t('common.disabled')}
              </Toggle>
            </FormField>
          </SpaceBetween>
        </Container>

        <Box color="text-body-secondary" fontSize="body-s">
          {t('pages.audit_policy_page.scope_note')}
        </Box>
      </SpaceBetween>
    </ContentLayout>
  );
}
