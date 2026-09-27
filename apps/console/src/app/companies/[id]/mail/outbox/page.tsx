'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  TextFilter,
  Badge,
  Button,
  Modal,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface OutboxEvent {
  id: string;
  topic: string;
  partition_key: string;
  status: string;
  attempts: number;
  last_error?: string;
  created_at: string;
  available_at: string;
  locked_at?: string;
  processed_at?: string;
}

const PAGE_SIZE = 100;

export default function OutboxEventsPage() {
  const { t } = useI18n();
  const [events, setEvents] = useState<OutboxEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const [limit, setLimit] = useState(PAGE_SIZE);
  const [filter, setFilter] = useState('');
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<OutboxEvent | null>(null);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const fetchOutboxEvents = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/outbox-events?limit=${limit}`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setEvents(data.outbox_events || []);
        setHasMore(Boolean(data.has_more));
      } else {
        const body = await res.json().catch(() => ({}));
        const msg = (body as { error?: string }).error ?? `HTTP ${res.status}`;
        setFlash([{
          type: 'error',
          header: t('pages.outbox_page.fetch_error_header', 'Failed to load outbox events'),
          content: msg,
          dismissible: true,
          onDismiss: () => setFlash([]),
        }]);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'An unexpected error occurred.';
      setFlash([{
        type: 'error',
        header: t('pages.outbox_page.fetch_error_header', 'Failed to load outbox events'),
        content: msg,
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
    } finally {
      setLoading(false);
    }
  }, [limit, t]);

  useEffect(() => {
    fetchOutboxEvents();
  }, [fetchOutboxEvents]);

  const performRetry = async (item: OutboxEvent) => {
    setRetryingId(item.id);
    try {
      const res = await fetch(`/api/admin/outbox/${item.id}/retry`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        setFlash([{
          type: 'success',
          header: t('pages.outbox_page.retry_success_header', 'Retry scheduled'),
          content: t('pages.outbox_page.retry_success', 'The outbox event has been requeued for delivery.'),
          dismissible: true,
          onDismiss: () => setFlash([]),
        }]);
        await fetchOutboxEvents();
      } else {
        const body = await res.json().catch(() => ({}));
        const msg = (body as { error?: string }).error ?? `HTTP ${res.status}`;
        setFlash([{
          type: 'error',
          header: t('pages.outbox_page.retry_error_header', 'Retry failed'),
          content: msg,
          dismissible: true,
          onDismiss: () => setFlash([]),
        }]);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'An unexpected error occurred.';
      setFlash([{
        type: 'error',
        header: t('pages.outbox_page.retry_error_header', 'Retry failed'),
        content: msg,
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
    } finally {
      setRetryingId(null);
      setConfirmTarget(null);
    }
  };

  const getStatusColor = (status: string): 'green' | 'red' | 'blue' | 'grey' => {
    switch (status) {
      case 'done': return 'green';
      case 'failed': return 'red';
      case 'pending': return 'blue';
      case 'processing': return 'grey';
      default: return 'grey';
    }
  };

  const statusLabel = (status: string): string => {
    switch (status) {
      case 'done': return t('pages.outbox_page.status_done', 'Done');
      case 'failed': return t('pages.outbox_page.status_failed', 'Failed');
      case 'pending': return t('pages.outbox_page.status_pending', 'Pending');
      case 'processing': return t('pages.outbox_page.status_processing', 'Processing');
      default: return status || '—';
    }
  };

  const needle = filter.trim().toLowerCase();
  const filteredEvents = needle
    ? events.filter(e =>
        (e.topic || '').toLowerCase().includes(needle) ||
        (e.partition_key || '').toLowerCase().includes(needle) ||
        e.id.toLowerCase().includes(needle)
      )
    : events;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.outbox_page.description')}
          actions={
            <Button iconName="refresh" onClick={fetchOutboxEvents} loading={loading}>
              {t('common.refresh', 'Refresh')}
            </Button>
          }
        >
          {t('pages.outbox.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}

        <DataTable
          columnDefinitions={[
            {
              id: 'topic',
              header: t('pages.outbox_page.topic', 'Topic'),
              cell: (item: OutboxEvent) => item.topic || '—',
              width: '18%',
            },
            {
              id: 'partition_key',
              header: t('pages.outbox_page.partition_key', 'Partition Key'),
              cell: (item: OutboxEvent) => item.partition_key || '—',
              width: '18%',
            },
            {
              id: 'status',
              header: t('pages.outbox_page.status'),
              cell: (item: OutboxEvent) => (
                <Badge color={getStatusColor(item.status)}>
                  {statusLabel(item.status)}
                </Badge>
              ),
              width: '12%',
            },
            {
              id: 'attempts',
              header: t('pages.outbox_page.attempts', 'Attempts'),
              cell: (item: OutboxEvent) => String(item.attempts ?? 0),
              width: '8%',
            },
            {
              id: 'last_error',
              header: t('pages.outbox_page.last_error', 'Last Error'),
              cell: (item: OutboxEvent) => item.last_error || '—',
              width: '20%',
            },
            {
              id: 'created_at',
              header: t('pages.outbox_page.created_at'),
              cell: (item: OutboxEvent) =>
                item.created_at ? new Date(item.created_at).toLocaleString() : '—',
              width: '14%',
            },
            {
              id: 'actions',
              header: t('pages.outbox_page.actions'),
              cell: (item: OutboxEvent) =>
                (item.status === 'failed' || item.status === 'pending') ? (
                  <Button
                    variant="inline-link"
                    onClick={() => setConfirmTarget(item)}
                    loading={retryingId === item.id}
                  >
                    {t('pages.outbox_page.retry')}
                  </Button>
                ) : null,
              width: '10%',
            },
          ]}
          items={filteredEvents}
          loading={loading}
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
          footer={
            hasMore ? (
              <Box textAlign="center">
                <Button
                  onClick={() => setLimit((prev) => prev + PAGE_SIZE)}
                  loading={loading}
                >
                  {t('common.load_more', 'Load more')}
                </Button>
              </Box>
            ) : undefined
          }
          empty={
            <Box textAlign="center" padding="l">
              {t('pages.outbox_page.no_events')}
            </Box>
          }
        />
      </SpaceBetween>

      <Modal
        visible={confirmTarget !== null}
        onDismiss={() => setConfirmTarget(null)}
        header={t('pages.outbox_page.retry_confirm_header', 'Retry outbox event?')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setConfirmTarget(null)}>
                {t('common.cancel', 'Cancel')}
              </Button>
              <Button
                variant="primary"
                loading={retryingId !== null}
                onClick={() => confirmTarget && performRetry(confirmTarget)}
              >
                {t('pages.outbox_page.retry')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        {t(
          'pages.outbox_page.retry_confirm_body',
          'Retrying resets this event to pending and may re-send the same mail. Continue?'
        )}
      </Modal>
    </ContentLayout>
  );
}
