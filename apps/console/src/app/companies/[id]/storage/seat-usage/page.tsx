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
import { useMemo } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useSeatUsage } from '@/hooks';

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
  const { data, isLoading, error } = useSeatUsage(companyId);
  const totalUsers = data?.total_users ?? 0;
  const activeUsers = data?.active_users ?? 0;
  const suspendedUsers = data?.suspended_users ?? 0;
  const domainCount = data?.domain_count ?? 0;
  const storageUsed = data?.storage_used ?? 0;
  const storageLimit = data?.storage_limit ?? 0;

  const storagePercent = useMemo(() => {
    if (!storageLimit || storageLimit <= 0) return 0;
    return Math.min(100, Math.round((storageUsed / storageLimit) * 100));
  }, [storageLimit, storageUsed]);

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('seat_usage.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">{t('seat_usage.title')}</Header>}>
        <Box color="text-status-error">{String(error?.message || error || t('seat_usage.failed_load'))}</Box>
      </ContentLayout>
    );
  }

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
              <Box fontSize="display-l" fontWeight="bold">{totalUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {t('seat_usage.across_domains')} {domainCount} {domainCount !== 1 ? t('seat_usage.domain_plural') : t('seat_usage.domain_singular')}
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('seat_usage.active_header')}</Header>}>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-status-success">
                {activeUsers}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {totalUsers > 0 ? `${Math.round((activeUsers / totalUsers) * 100)}%` : '—'} {t('seat_usage.of_total')}
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('seat_usage.suspended_header')}</Header>}>
            <SpaceBetween size="xxs">
              <Box
                fontSize="display-l"
                fontWeight="bold"
                color={suspendedUsers > 0 ? 'text-status-warning' : undefined}
              >
                {suspendedUsers}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {suspendedUsers > 0 ? t('seat_usage.suspended_cleanup') : t('seat_usage.no_suspended')}
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
                <Box fontWeight="bold">{formatBytes(storageUsed)}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('seat_usage.allocated_label')}</Box>
                <Box fontWeight="bold">{storageLimit > 0 ? formatBytes(storageLimit) : t('seat_usage.unlimited')}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('seat_usage.available_label')}</Box>
                <Box fontWeight="bold">
                  {storageLimit > 0 ? formatBytes(Math.max(0, storageLimit - storageUsed)) : '—'}
                </Box>
              </SpaceBetween>
            </ColumnLayout>

            {storageLimit > 0 && (
              <ProgressBar
                value={storagePercent}
                label={`${t('seat_usage.storage_usage_label')} (${storagePercent}%)`}
                status={storageStatus}
                additionalInfo={`${formatBytes(storageUsed)} ${t('seat_usage.of_total')} ${formatBytes(storageLimit)}`}
              />
            )}
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
