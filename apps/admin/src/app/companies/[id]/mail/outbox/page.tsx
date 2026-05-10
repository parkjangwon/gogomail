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
  Button,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface OutboxEvent {
  id: string;
  message_id: string;
  recipient: string;
  event_type: string;
  status: string;
  timestamp: string;
}

export default function OutboxEventsPage() {
  const { t } = useI18n();
  const [events, setEvents] = useState<OutboxEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchOutboxEvents();
  }, []);

  const fetchOutboxEvents = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/outbox-events?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setEvents(data.events || []);
      }
    } catch (error) {
      console.error('Failed to fetch outbox events:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'green';
      case 'failed': return 'red';
      case 'pending': return 'blue';
      case 'retry': return 'severity-high';
      default: return 'grey';
    }
  };

  const filteredEvents = events.filter(e =>
    e.recipient.toLowerCase().includes(filter.toLowerCase()) ||
    e.message_id.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.outbox.title')}</Header>}>
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
          description={t('pages.outbox_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.outbox_page.retry_failed')}
            </Button>
          }
        >
          {t('pages.outbox.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.outbox_page.message_id'),
              cell: (item: OutboxEvent) => item.message_id,
              width: '20%',
            },
            {
              header: t('pages.outbox_page.recipient'),
              cell: (item: OutboxEvent) => item.recipient,
              width: '25%',
            },
            {
              header: t('pages.outbox_page.event_type'),
              cell: (item: OutboxEvent) => item.event_type,
              width: '20%',
            },
            {
              header: t('pages.outbox_page.status'),
              cell: (item: OutboxEvent) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.outbox_page.timestamp'),
              cell: (item: OutboxEvent) => new Date(item.timestamp).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredEvents}
          header={<Header variant="h2" counter={`(${filteredEvents.length})`}>{t('pages.outbox_page.events')}</Header>}
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
