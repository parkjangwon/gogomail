'use client';

import { DataTable } from '@/components/DataTable';
import {
  ContentLayout,
  Header,
  Container,
  ColumnLayout,
  Box,
  ProgressBar,
  KeyValuePairs,
  StatusIndicator,
  SpaceBetween,
} from '@cloudscape-design/components';
import { useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import {
  useAdminQueueStats,
  useAdminHealth,
  useAdminSystemMetrics,
  type QueueStat,
} from '@/hooks';
import { type AdminHealthCheck } from '@/hooks/useSystem';

export default function MonitoringPage() {
  const { t } = useI18n();
  const queueQuery = useAdminQueueStats(5_000);
  const healthQuery = useAdminHealth(15_000);
  const metricsQuery = useAdminSystemMetrics(10_000);

  const queues = queueQuery.data?.queues ?? [];

  const stats = useMemo(() => {
    const total = queues.reduce((sum: number, q: QueueStat) => sum + (q.count || 0), 0);
    const pending = queues.reduce((sum: number, q: QueueStat) => sum + (q.ready_count || 0), 0);
    const delayed = queues.reduce((sum: number, q: QueueStat) => sum + (q.delayed_count || 0), 0);
    const stale = queues.reduce((sum: number, q: QueueStat) => sum + (q.stale_processing_count || 0), 0);
    return { total, pending, delayed, stale };
  }, [queues]);

  const memPct = metricsQuery.data?.memory?.usage_pct ?? 0;
  const goroutines = metricsQuery.data?.goroutines ?? 0;

  const dbCheck = healthQuery.data?.checks?.find((c: AdminHealthCheck) => c.service === 'database');
  const dbStatus = dbCheck?.status === 'healthy' ? 'success' : dbCheck?.status === 'degraded' ? 'warning' : 'pending';
  const dbResponseTime = dbCheck?.response_time_ms != null ? `${dbCheck.response_time_ms}ms` : '—';

  return (
    <ContentLayout header={<Header variant="h1">{t('pages.monitoring.title')}</Header>}>
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.monitoring_page.system_resources')}</Header>}>
          <ColumnLayout columns={2} variant="text-grid">
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>{t('pages.monitoring_page.memory_usage')}</small>
              </Box>
              <ProgressBar
                value={memPct}
                label={`${Math.round(memPct)}%`}
                status={memPct > 85 ? 'error' : 'success'}
              />
            </Box>
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>{t('pages.monitoring_page.goroutines')}</small>
              </Box>
              <Box variant="awsui-value-large">{goroutines}</Box>
            </Box>
          </ColumnLayout>
        </Container>

        <Container header={<Header variant="h2">{t('pages.monitoring_page.message_queue')}</Header>}>
          <KeyValuePairs
            items={[
              { label: t('pages.monitoring_page.total_messages'), value: stats.total },
              {
                label: t('pages.monitoring_page.queue_status'),
                value: stats.stale > 0 ? (
                  <StatusIndicator type="warning">
                    {stats.stale} {t('pages.monitoring_page.failed_label')}
                  </StatusIndicator>
                ) : (
                  <StatusIndicator type="success">{t('pages.monitoring_page.healthy')}</StatusIndicator>
                ),
              },
              { label: t('pages.monitoring_page.pending'), value: stats.pending },
              { label: t('pages.monitoring_page.delayed'), value: stats.delayed },
            ]}
          />
        </Container>

        <Container header={<Header variant="h2">{t('pages.monitoring_page.queue_breakdown')}</Header>}>
          <DataTable
            columnDefinitions={[
              { header: t('pages.monitoring_page.queue_topic'), cell: (item: QueueStat) => item.topic, width: '30%' },
              { header: t('pages.monitoring_page.total_messages'), cell: (item: QueueStat) => item.count, width: '17%' },
              { header: t('pages.monitoring_page.pending'), cell: (item: QueueStat) => item.ready_count, width: '17%' },
              { header: t('pages.monitoring_page.delayed'), cell: (item: QueueStat) => item.delayed_count, width: '17%' },
              {
                header: t('pages.monitoring_page.queue_status'),
                cell: (item: QueueStat) => (
                  <StatusIndicator type={item.stale_processing_count > 0 ? 'warning' : 'success'}>
                    {item.stale_processing_count > 0 ? t('pages.monitoring_page.failed_label') : t('pages.monitoring_page.healthy')}
                  </StatusIndicator>
                ),
                width: '19%',
              },
            ]}
            items={queues}
            header={<Header variant="h3">{t('pages.monitoring_page.active_queues')}</Header>}
          />
        </Container>

        <Container header={<Header variant="h2">{t('pages.monitoring_page.database')}</Header>}>
          <KeyValuePairs
            items={[
              {
                label: t('pages.monitoring_page.queue_status'),
                value: (
                  <StatusIndicator type={dbStatus}>
                    {dbCheck?.status ?? t('pages.monitoring_page.pending')}
                  </StatusIndicator>
                ),
              },
              { label: t('pages.monitoring_page.response_time'), value: dbResponseTime },
            ]}
          />
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
