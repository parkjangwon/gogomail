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
  const { t: _unused } = useI18n(); _unused;
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
      <ContentLayout header={<Header variant="h1">Outbox Events</Header>}>
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
          description="Monitor and manage outbox delivery events"
          actions={
            <Button variant="primary" disabled>
              Retry Failed
            </Button>
          }
        >
          Outbox Events
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Message ID',
              cell: (item: OutboxEvent) => item.message_id,
              width: '20%',
            },
            {
              header: 'Recipient',
              cell: (item: OutboxEvent) => item.recipient,
              width: '25%',
            },
            {
              header: 'Event Type',
              cell: (item: OutboxEvent) => item.event_type,
              width: '20%',
            },
            {
              header: 'Status',
              cell: (item: OutboxEvent) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Timestamp',
              cell: (item: OutboxEvent) => new Date(item.timestamp).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredEvents}
          header={<Header variant="h2" counter={`(${filteredEvents.length})`}>Events</Header>}
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
