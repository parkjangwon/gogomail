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
  const [retryingId, setRetryingId] = useState<string | null>(null);

  useEffect(() => {
    fetchOutboxEvents();
  }, []);

  const fetchOutboxEvents = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/outbox-events?limit=100', {
        credentials: 'include',
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

  const handleRetry = async (item: OutboxEvent) => {
    setRetryingId(item.id);
    try {
      await fetch(`/api/admin/outbox/${item.id}/retry`, {
        method: 'POST',
        credentials: 'include',
      });
      fetchOutboxEvents();
    } catch (error) {
      console.error('Failed to retry outbox event:', error);
    } finally {
      setRetryingId(null);
    }
  };

  const getStatusColor = (status: string): 'green' | 'red' | 'blue' | 'grey' => {
    switch (status) {
      case 'success': return 'green';
      case 'failed': return 'red';
      case 'pending': return 'blue';
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
        >
          {t('pages.outbox.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.outbox_page.message_id'),
              cell: (item: OutboxEvent) => item.message_id,
              width: '20%',
            },
            {
              header: t('pages.outbox_page.recipient'),
              cell: (item: OutboxEvent) => item.recipient,
              width: '22%',
            },
            {
              header: t('pages.outbox_page.event_type'),
              cell: (item: OutboxEvent) => item.event_type,
              width: '15%',
            },
            {
              header: t('pages.outbox_page.status'),
              cell: (item: OutboxEvent) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.outbox_page.timestamp'),
              cell: (item: OutboxEvent) => new Date(item.timestamp).toLocaleString(),
              width: '18%',
            },
            {
              header: t('pages.outbox_page.actions'),
              cell: (item: OutboxEvent) =>
                (item.status === 'failed' || item.status === 'pending') ? (
                  <Button
                    variant="inline-link"
                    onClick={() => handleRetry(item)}
                    loading={retryingId === item.id}
                  >
                    {t('pages.outbox_page.retry')}
                  </Button>
                ) : null,
              width: '13%',
            },
          ]}
          items={filteredEvents}
          header={
            <Header variant="h2" counter={`(${filteredEvents.length})`}>
              {t('pages.outbox_page.events')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              {t('pages.outbox_page.no_events')}
            </Box>
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
