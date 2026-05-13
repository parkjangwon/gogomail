'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  ColumnLayout,
  ProgressBar,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface SeatUsage {
  total_users: number;
  active_users: number;
  suspended_users: number;
  domain_count: number;
  storage_used: number;
  storage_limit: number;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export default function SeatUsagePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [data, setData] = useState<SeatUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!companyId) return;
    fetch(`/api/admin/companies/${companyId}/seat-usage`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(setData)
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false));
  }, [companyId]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('seat_usage.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">{t('seat_usage.title')}</Header>}>
        <Box color="text-status-error">{error || t('seat_usage.failed_load')}</Box>
      </ContentLayout>
    );
  }

  const storagePercent = data.storage_limit > 0
    ? Math.min(100, Math.round((data.storage_used / data.storage_limit) * 100))
    : 0;

  const storageStatus: 'error' | 'success' | 'in-progress' = storagePercent >= 90 ? 'error' : storagePercent >= 75 ? 'in-progress' : 'success';

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('seat_usage.description')}>
          {t('seat_usage.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* User seats */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={160}>
          <Container header={<Header variant="h3">{t('seat_usage.total_users_header')}</Header>}>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{data.total_users}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {t('seat_usage.across_domains')} {data.domain_count} {data.domain_count !== 1 ? t('seat_usage.domain_plural') : t('seat_usage.domain_singular')}
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('seat_usage.active_header')}</Header>}>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-status-success">
                {data.active_users}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {data.total_users > 0 ? `${Math.round((data.active_users / data.total_users) * 100)}%` : '—'} {t('seat_usage.of_total')}
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('seat_usage.suspended_header')}</Header>}>
            <SpaceBetween size="xxs">
              <Box
                fontSize="display-l"
                fontWeight="bold"
                color={data.suspended_users > 0 ? 'text-status-warning' : undefined}
              >
                {data.suspended_users}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {data.suspended_users > 0 ? t('seat_usage.suspended_cleanup') : t('seat_usage.no_suspended')}
              </Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* Storage */}
        <Container header={<Header variant="h2">{t('seat_usage.storage_header')}</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={3} variant="text-grid">
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('seat_usage.used_label')}</Box>
                <Box fontWeight="bold">{formatBytes(data.storage_used)}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('seat_usage.allocated_label')}</Box>
                <Box fontWeight="bold">{data.storage_limit > 0 ? formatBytes(data.storage_limit) : t('seat_usage.unlimited')}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('seat_usage.available_label')}</Box>
                <Box fontWeight="bold">
                  {data.storage_limit > 0 ? formatBytes(Math.max(0, data.storage_limit - data.storage_used)) : '—'}
                </Box>
              </SpaceBetween>
            </ColumnLayout>

            {data.storage_limit > 0 && (
              <ProgressBar
                value={storagePercent}
                label={`${t('seat_usage.storage_usage_label')} (${storagePercent}%)`}
                status={storageStatus}
                additionalInfo={`${formatBytes(data.storage_used)} ${t('seat_usage.of_total')} ${formatBytes(data.storage_limit)}`}
              />
            )}
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
