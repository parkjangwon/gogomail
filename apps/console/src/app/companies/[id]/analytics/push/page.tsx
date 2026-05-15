'use client';
import { DataTable } from '@/components/DataTable';
import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
} from '@cloudscape-design/components';
import { useMemo, useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useAdminPushNotificationAttempts, type PushNotificationAttempt } from '@/hooks';

export default function PushNotificationsPage() {
  const { t } = useI18n();
  const [filter, setFilter] = useState('');
  const attemptsQuery = useAdminPushNotificationAttempts({ limit: 100 });
  const metrics = attemptsQuery.data ?? [];
  const loading = attemptsQuery.isLoading;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'delivered': return 'green';
      case 'failed': return 'red';
      case 'pending': return 'blue';
      default: return 'grey';
    }
  };

  const filteredMetrics = useMemo(
    () => metrics.filter((m: PushNotificationAttempt) =>
      (m.recipient || '').toLowerCase().includes(filter.toLowerCase()) ||
      (m.device_id || '').toLowerCase().includes(filter.toLowerCase())
    ),
    [filter, metrics]
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.push.title')}</Header>}>
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
          description={t('pages.push_page.description')}
        >
          {t('pages.push_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.push_page.user_email'),
              cell: (item: PushNotificationAttempt) => item.recipient,
              width: '25%',
            },
            {
              header: t('pages.push_page.device_id'),
              cell: (item: PushNotificationAttempt) => item.device_id ?? '—',
              width: '25%',
            },
            {
              header: t('pages.push_page.type'),
              cell: (item: PushNotificationAttempt) => item.subject,
              width: '15%',
            },
            {
              header: t('pages.push_page.status'),
              cell: (item: PushNotificationAttempt) => (
                <Badge color={getStatusColor(String(item.status))}>
                  {String(item.status)}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.push_page.timestamp'),
              cell: (item: PushNotificationAttempt) => new Date(item.attempted_at).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredMetrics}
          header={<Header variant="h2" counter={`(${filteredMetrics.length})`}>{t('pages.push_page.notifications')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
