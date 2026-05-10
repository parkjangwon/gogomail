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

interface DeliveryAttempt {
  id: string;
  message_id: string;
  recipient: string;
  attempt_number: number;
  status: string;
  error_message?: string;
  timestamp: string;
}

export default function DeliveryAttemptsPage() {
  const { t } = useI18n();
  const [attempts, setAttempts] = useState<DeliveryAttempt[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDeliveryAttempts();
  }, []);

  const fetchDeliveryAttempts = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/delivery-attempts?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setAttempts(data.attempts || []);
      }
    } catch (error) {
      console.error('Failed to fetch delivery attempts:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'green';
      case 'failed': return 'red';
      case 'permanent_failure': return 'severity-critical';
      case 'temporary_failure': return 'severity-high';
      default: return 'grey';
    }
  };

  const filteredAttempts = attempts.filter(a =>
    a.recipient.toLowerCase().includes(filter.toLowerCase()) ||
    a.message_id.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.delivery_attempts.title')}</Header>}>
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
          description={t('pages.delivery_attempts_page.description')}
        >
          {t('pages.delivery_attempts.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.delivery_attempts_page.message_id'),
              cell: (item: DeliveryAttempt) => item.message_id,
              width: '20%',
            },
            {
              header: t('pages.delivery_attempts.recipient'),
              cell: (item: DeliveryAttempt) => item.recipient,
              width: '25%',
            },
            {
              header: t('pages.delivery_attempts_page.attempt'),
              cell: (item: DeliveryAttempt) => `#${item.attempt_number}`,
              width: '10%',
            },
            {
              header: t('pages.delivery_attempts.status'),
              cell: (item: DeliveryAttempt) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.delivery_attempts_page.error'),
              cell: (item: DeliveryAttempt) => item.error_message || '-',
              width: '15%',
            },
            {
              header: t('pages.delivery_attempts_page.timestamp'),
              cell: (item: DeliveryAttempt) => new Date(item.timestamp).toLocaleString(),
              width: '10%',
            },
          ]}
          items={filteredAttempts}
          header={<Header variant="h2" counter={`(${filteredAttempts.length})`}>{t('pages.delivery_attempts_page.attempts')}</Header>}
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
