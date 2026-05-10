'use client';

import {
  ContentLayout,
  Header,
  Container,
  ColumnLayout,
  Box,
  Spinner,
  SpaceBetween,
  ProgressBar,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useState, useEffect } from 'react';

interface QueueStats {
  total_messages: number;
  pending: number;
  processing: number;
  failed: number;
  processing_rate: number;
  average_wait_time: number;
  last_updated: string;
}

export default function QueueStatsPage() {
  const { t } = useI18n();
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchQueueStats();
    const interval = setInterval(fetchQueueStats, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchQueueStats = async () => {
    try {
      const res = await fetch('/api/admin/queue', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch (error) {
      console.error('Failed to fetch queue stats:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.queue_stats.title')}</Header>}>
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
          description={t('pages.queue_stats.description')}
        >
          {t('pages.queue_stats.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {stats && (
          <>
            <ColumnLayout columns={4}>
              <Container header={<Header variant="h3">{t('pages.queue_stats.total_messages')}</Header>}>
                <Box fontSize="display-l" fontWeight="bold">
                  {stats.total_messages.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">{t('pages.queue_stats.pending')}</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-warning">
                  {stats.pending.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">{t('pages.queue_stats.processing')}</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-info">
                  {stats.processing.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">{t('pages.queue_stats.failed')}</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-error">
                  {stats.failed.toLocaleString()}
                </Box>
              </Container>
            </ColumnLayout>

            <Container header={<Header variant="h3">{t('pages.queue_stats.queue_status')}</Header>}>
              <SpaceBetween size="m">
                <Box>
                  <Box>{t('pages.queue_stats.processing_rate')}: {stats.processing_rate} {t('pages.queue_stats.msg_per_sec')}</Box>
                  <ProgressBar value={stats.processing_rate} label={t('pages.queue_stats.current_rate')} />
                </Box>
                <Box>
                  <Box>{t('pages.queue_stats.average_wait_time')}: {stats.average_wait_time} {t('pages.queue_stats.seconds')}</Box>
                  <ProgressBar value={Math.min(stats.average_wait_time, 100)} label={t('pages.queue_stats.wait_time')} />
                </Box>
              </SpaceBetween>
            </Container>

            <Container header={<Header variant="h3">{t('pages.queue_stats.last_updated')}</Header>}>
              <Box color="text-body-secondary">
                {new Date(stats.last_updated).toLocaleString()}
              </Box>
            </Container>
          </>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
