'use client';

import {
  ContentLayout,
  Header,
  Table,
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
  const { t: _unused } = useI18n(); _unused;
  const [metrics, setMetrics] = useState<PushMetric[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchPushMetrics();
  }, []);

  const fetchPushMetrics = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/push-notifications?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setMetrics(data.metrics || []);
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
      <ContentLayout header={<Header variant="h1">Push Notifications</Header>}>
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
          description="Monitor push notification delivery and metrics"
        >
          Push Notifications
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'User Email',
              cell: (item: PushMetric) => item.user_email,
              width: '25%',
            },
            {
              header: 'Device ID',
              cell: (item: PushMetric) => item.device_id,
              width: '25%',
            },
            {
              header: 'Type',
              cell: (item: PushMetric) => item.notification_type,
              width: '15%',
            },
            {
              header: 'Status',
              cell: (item: PushMetric) => (
                <Badge color={getStatusColor(item.attempt_status)}>
                  {item.attempt_status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Timestamp',
              cell: (item: PushMetric) => new Date(item.timestamp).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredMetrics}
          header={<Header variant="h2" counter={`(${filteredMetrics.length})`}>Notifications</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
