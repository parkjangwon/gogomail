'use client';
import {
  Container,
  Header,
  SpaceBetween,
  Alert,
  ColumnLayout,
  Box,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { CreatedCompany, CreatedDomain, DnsCheckResult } from './types';
import { formatBytes } from './types';

interface Props {
  createdCompany: CreatedCompany | null;
  createdDomain: CreatedDomain | null;
  dnsCheck: DnsCheckResult;
  dkimCreated: boolean;
  step4Selector: string;
  domainName: string;
  step5Skip: boolean;
  createdUserCount: number;
}

export function OnboardingReview({
  createdCompany,
  createdDomain,
  dnsCheck,
  dkimCreated,
  step4Selector,
  domainName,
  step5Skip,
  createdUserCount,
}: Props) {
  const { t } = useI18n();

  return (
    <Container header={<Header variant="h2">{t('onboarding.step_review_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.review_info')}</Alert>
        <ColumnLayout columns={2} variant="text-grid">
          <SpaceBetween size="s">
            <Box variant="awsui-key-label">{t('onboarding.review_company')}</Box>
            <div>{createdCompany?.name ?? '—'}</div>
            <Box variant="awsui-key-label">ID</Box>
            <div>{createdCompany?.id ?? '—'}</div>
            <Box variant="awsui-key-label">{t('onboarding.quota_gb')}</Box>
            <div>
              {createdCompany
                ? formatBytes(createdCompany.quota_limit, t('pages.company_overview.quota_unlimited', 'Unlimited'))
                : '—'}
            </div>
            <Box variant="awsui-key-label">{t('onboarding.status')}</Box>
            <div>{createdCompany?.status ?? '—'}</div>
          </SpaceBetween>
          <SpaceBetween size="s">
            <Box variant="awsui-key-label">{t('onboarding.review_domain')}</Box>
            <div>{createdDomain?.name ?? '—'}</div>
            <Box variant="awsui-key-label">ID</Box>
            <div>{createdDomain?.id ?? '—'}</div>
            <Box variant="awsui-key-label">{t('onboarding.dns_status')}</Box>
            <div>
              {dnsCheck.checked ? (
                dnsCheck.mx && dnsCheck.spf ? (
                  <StatusIndicator type="success">{t('onboarding.dns_verified')}</StatusIndicator>
                ) : (
                  <StatusIndicator type="warning">{t('onboarding.dns_partial')}</StatusIndicator>
                )
              ) : (
                <StatusIndicator type="pending">{t('onboarding.dns_pending')}</StatusIndicator>
              )}
            </div>
            <Box variant="awsui-key-label">{t('onboarding.review_dkim')}</Box>
            <div>{dkimCreated ? `${step4Selector}._domainkey.${domainName}` : t('onboarding.not_configured')}</div>
            <Box variant="awsui-key-label">{t('onboarding.review_users')}</Box>
            <div>{step5Skip ? t('onboarding.not_configured') : `${createdUserCount}`}</div>
          </SpaceBetween>
        </ColumnLayout>
      </SpaceBetween>
    </Container>
  );
}
