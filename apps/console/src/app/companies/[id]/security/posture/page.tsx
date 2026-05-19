'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  ColumnLayout,
  StatusIndicator,
  ProgressBar,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface PostureData {
  score: number;
  mfa: { total: number; enabled: number; rate: number };
  ip_policy_configured: boolean;
  users_without_password: number;
  domain_count: number;
  active_domains: number;
}

export default function SecurityPosturePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [data, setData] = useState<PostureData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!companyId) return;
    fetch(`/api/admin/companies/${companyId}/security/posture`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(setData)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load security posture.'))
      .finally(() => setLoading(false));
  }, [companyId]);

  const scoreColor = (s: number) => s >= 80 ? 'text-status-success' : s >= 50 ? 'text-status-warning' : 'text-status-error';
  const scoreLabel = (s: number) => s >= 80 ? t('security_posture.score_good') : s >= 50 ? t('security_posture.score_fair') : t('security_posture.score_at_risk');

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('security_posture.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">{t('security_posture.title')}</Header>}>
        <StatusIndicator type="error">{error || t('security_posture.failed_load')}</StatusIndicator>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('security_posture.description')}>
          {t('security_posture.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* Score card */}
        <Container header={<Header variant="h2">{t('security_posture.score_header')}</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <SpaceBetween size="xs">
                <Box fontSize="display-l" fontWeight="bold" color={scoreColor(data.score)}>
                  {data.score}/100
                </Box>
                <Box color="text-body-secondary">{scoreLabel(data.score)}</Box>
              </SpaceBetween>
              <SpaceBetween size="s">
                <ProgressBar
                  value={data.score}
                  label={t('security_posture.score_label')}
                  status={data.score >= 80 ? 'success' : data.score >= 50 ? 'in-progress' : 'error'}
                />
              </SpaceBetween>
            </ColumnLayout>
          </SpaceBetween>
        </Container>

        {/* KPI grid */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={160}>
          <Container header={<Header variant="h3">{t('security_posture.mfa_header')}</Header>}>
            <SpaceBetween size="s">
              <Box fontSize="display-l" fontWeight="bold">
                {data.mfa.rate.toFixed(0)}%
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {data.mfa.enabled} {t('security_posture.mfa_enrolled_of')} {data.mfa.total} {t('security_posture.mfa_enrolled_users')}
              </Box>
              <StatusIndicator type={data.mfa.rate >= 80 ? 'success' : data.mfa.rate >= 50 ? 'warning' : 'error'}>
                {data.mfa.rate >= 80 ? t('security_posture.mfa_healthy') : data.mfa.rate >= 50 ? t('security_posture.mfa_partial') : t('security_posture.mfa_low')}
              </StatusIndicator>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('security_posture.ip_policy_header')}</Header>}>
            <SpaceBetween size="s">
              <StatusIndicator type={data.ip_policy_configured ? 'success' : 'warning'}>
                {data.ip_policy_configured ? t('security_posture.ip_configured') : t('security_posture.ip_not_configured')}
              </StatusIndicator>
              <Box color="text-body-secondary" fontSize="body-s">
                {t('security_posture.ip_desc')}
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('security_posture.password_header')}</Header>}>
            <SpaceBetween size="s">
              <Box fontSize="display-l" fontWeight="bold" color={data.users_without_password > 0 ? 'text-status-warning' : undefined}>
                {data.users_without_password}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('security_posture.password_without')}</Box>
              <StatusIndicator type={data.users_without_password === 0 ? 'success' : 'warning'}>
                {data.users_without_password === 0 ? t('security_posture.password_all_set') : t('security_posture.password_action')}
              </StatusIndicator>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* Domains */}
        <Container header={<Header variant="h2">{t('security_posture.domain_health_header')}</Header>}>
          <ColumnLayout columns={2} variant="text-grid">
            <SpaceBetween size="xxs">
              <Box fontWeight="bold">{data.domain_count}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('security_posture.total_domains')}</Box>
            </SpaceBetween>
            <SpaceBetween size="xxs">
              <Box fontWeight="bold" color="text-status-success">{data.active_domains}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('security_posture.active_domains')}</Box>
            </SpaceBetween>
          </ColumnLayout>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
