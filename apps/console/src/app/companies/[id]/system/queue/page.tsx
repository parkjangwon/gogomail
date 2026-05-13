'use client';

import {
  ContentLayout,
  Header,
  Container,
  ColumnLayout,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Table,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useState, useEffect } from 'react';

interface QueueStat {
  topic: string;
  status: string;
  count: number;
  ready_count: number;
  delayed_count: number;
  stale_processing_count: number;
  oldest_ready_at?: string;
  next_available_at?: string;
}

export default function QueueStatsPage() {
  const { t } = useI18n();
  const [queues, setQueues] = useState<QueueStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  useEffect(() => {
    fetchQueueStats();
    const interval = setInterval(fetchQueueStats, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchQueueStats = async () => {
    try {
      const res = await fetch('/api/admin/queue', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setQueues(data.queues || []);
        setLastUpdated(new Date());
      }
    } catch (error) {
      console.error('Failed to fetch queue stats:', error);
    } finally {
      setLoading(false);
    }
  };

  const totalCount = queues.reduce((sum, q) => sum + (q.count || 0), 0);
  const totalReady = queues.reduce((sum, q) => sum + (q.ready_count || 0), 0);
  const totalDelayed = queues.reduce((sum, q) => sum + (q.delayed_count || 0), 0);
  const totalStale = queues.reduce((sum, q) => sum + (q.stale_processing_count || 0), 0);

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
            <Box fontSize="display-l" fontWeight="bold">{totalCount.toLocaleString()}</Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.pending')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color={totalReady > 0 ? 'text-status-warning' : 'text-body-secondary'}>
              {totalReady.toLocaleString()}
            </Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.processing')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color="text-status-info">
              {totalDelayed.toLocaleString()}
            </Box>
          </Container>
          <Container header={<Header variant="h3">{t('pages.queue_stats.failed')}</Header>}>
            <Box fontSize="display-l" fontWeight="bold" color={totalStale > 0 ? 'text-status-error' : 'text-body-secondary'}>
              {totalStale.toLocaleString()}
            </Box>
          </Container>
        </ColumnLayout>

        <Table
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
