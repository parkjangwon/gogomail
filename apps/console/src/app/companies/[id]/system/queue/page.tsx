'use client';

import { DataTable } from '@/components/DataTable';
import {
  ContentLayout,
  Header,
  Container,
  ColumnLayout,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  StatusIndicator,
  Button,
} from '@cloudscape-design/components';
import { useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useAdminQueueStats, type QueueStat } from '@/hooks';

export default function QueueStatsPage() {
  const { t } = useI18n();
  const queueQuery = useAdminQueueStats(5_000);
  const queues = queueQuery.data?.queues ?? [];
  const loading = queueQuery.isLoading;
  const lastUpdated = queueQuery.dataUpdatedAt ? new Date(queueQuery.dataUpdatedAt) : null;

  const totals = useMemo(() => ({
    count: queues.reduce((sum, q) => sum + (q.count || 0), 0),
    ready: queues.reduce((sum, q) => sum + (q.ready_count || 0), 0),
    delayed: queues.reduce((sum, q) => sum + (q.delayed_count || 0), 0),
    stale: queues.reduce((sum, q) => sum + (q.stale_processing_count || 0), 0),
  }), [queues]);

  const getStatusIndicator = (q: QueueStat) => {
    if (q.stale_processing_count > 0) return <StatusIndicator type="error">{t('pages.queue_stats_page.stale')}</StatusIndicator>;
    if (q.count === 0) return <StatusIndicator type="success">{t('pages.queue_stats.pending')}</StatusIndicator>;
    if (q.ready_count > 0) return <StatusIndicator type="in-progress">{t('pages.queue_stats.processing')}</StatusIndicator>;
    return <StatusIndicator type="pending">{t('pages.queue_stats_page.status')}</StatusIndicator>;
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.queue_stats.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (queueQuery.isError && queues.length === 0) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.queue_stats.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="m" alignItems="center">
            <Box color="text-status-error">{t('pages.queue_stats.description')}</Box>
            <Button iconName="refresh" onClick={() => queueQuery.refetch()}>{t('common.retry')}</Button>
          </SpaceBetween>
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.queue_stats.description')}
          info={lastUpdated ? <Box color="text-body-secondary" fontSize="body-s">{t('pages.queue_stats_page.updated')} {lastUpdated.toLocaleTimeString()}</Box> : undefined}
        >
          {t('pages.queue_stats.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <ColumnLayout columns={4}>
          <Container header={<Header variant="h3">{t('pages.queue_stats.total_messages')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold">{totals.count.toLocaleString()}</Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.pending')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color={totals.ready > 0 ? 'text-status-warning' : 'text-body-secondary'}>
              {totals.ready.toLocaleString()}
            </Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.processing')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color="text-status-info">
              {totals.delayed.toLocaleString()}
            </Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.failed')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color={totals.stale > 0 ? 'text-status-error' : 'text-body-secondary'}>
              {totals.stale.toLocaleString()}
            </Box>
          </Container>
        </ColumnLayout>

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.queue_stats_page.topic'),
              cell: (q: QueueStat) => <Box fontWeight="bold">{q.topic}</Box>,
              width: '25%',
            },
            {
              header: t('pages.queue_stats_page.status'),
              cell: (q: QueueStat) => getStatusIndicator(q),
              width: '15%',
            },
            {
              header: t('pages.queue_stats_page.total'),
              cell: (q: QueueStat) => (q.count ?? 0).toLocaleString(),
              width: '12%',
            },
            {
              header: t('pages.queue_stats_page.ready'),
              cell: (q: QueueStat) => (
                <Badge color={(q.ready_count ?? 0) > 0 ? 'severity-high' : 'grey'}>
                  {(q.ready_count ?? 0).toLocaleString()}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.queue_stats_page.delayed'),
              cell: (q: QueueStat) => (q.delayed_count ?? 0).toLocaleString(),
              width: '12%',
            },
            {
              header: t('pages.queue_stats_page.stale'),
              cell: (q: QueueStat) => (
                <Badge color={(q.stale_processing_count ?? 0) > 0 ? 'red' : 'grey'}>
                  {(q.stale_processing_count ?? 0).toLocaleString()}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.queue_stats_page.oldest_ready'),
              cell: (q: QueueStat) => q.oldest_ready_at
                ? new Date(q.oldest_ready_at).toLocaleString()
                : '—',
              width: '12%',
            },
          ]}
          items={queues}
          header={
            <Header variant="h2" counter={`(${queues.length})`}>
              {t('pages.queue_stats.queue_status')}
            </Header>
          }
          empty={
            <Box textAlign="center" padding="xl">
              <StatusIndicator type="success">{t('pages.queue_stats_page.all_queues_idle')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
