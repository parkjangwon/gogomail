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
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface PushMetric {
  id: string;
  device_id: string;
  user_email: string;
  attempt_status: string;
  notification_type: string;
  timestamp: string;
}

export default function PushNotificationsPage() {
  const { t } = useI18n();
  const [metrics, setMetrics] = useState<PushMetric[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchPushMetrics();
  }, []);

  const fetchPushMetrics = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/push-notification-attempts?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setMetrics(data.push_notification_attempts || []);
      }
    } catch (error) {
      console.error('Failed to fetch push metrics:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'delivered': return 'green';
      case 'failed': return 'red';
      case 'pending': return 'blue';
      default: return 'grey';
    }
  };

  const filteredMetrics = metrics.filter(m =>
    m.user_email.toLowerCase().includes(filter.toLowerCase()) ||
    m.device_id.toLowerCase().includes(filter.toLowerCase())
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
              cell: (item: PushMetric) => item.user_email,
              width: '25%',
            },
            {
              header: t('pages.push_page.device_id'),
              cell: (item: PushMetric) => item.device_id,
              width: '25%',
            },
            {
              header: t('pages.push_page.type'),
              cell: (item: PushMetric) => item.notification_type,
              width: '15%',
            },
            {
              header: t('pages.push_page.status'),
              cell: (item: PushMetric) => (
                <Badge color={getStatusColor(item.attempt_status)}>
                  {item.attempt_status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.push_page.timestamp'),
              cell: (item: PushMetric) => new Date(item.timestamp).toLocaleString(),
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
