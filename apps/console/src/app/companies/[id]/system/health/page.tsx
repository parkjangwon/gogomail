'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Table,
  Container,
  ColumnLayout,
  Button,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface HealthCheck {
  service: string;
  status: 'healthy' | 'degraded' | 'unhealthy';
  response_time_ms: number;
  last_check: string;
}

interface QueueStat {
  topic: string;
  status: string;
  count: number;
  ready_count: number;
  delayed_count: number;
  stale_processing_count: number;
  oldest_ready_at?: string;
}

const statusBadgeColor = (s: string) =>
  s === 'healthy' ? 'green' : s === 'degraded' ? 'severity-high' : 'red';

const latencyColor = (ms: number) =>
  ms < 50 ? 'text-status-success' : ms < 200 ? 'text-status-warning' : 'text-status-error';

export default function APIHealthPage() {
  const { t } = useI18n();
  const [checks, setChecks] = useState<HealthCheck[]>([]);
  const [queues, setQueues] = useState<QueueStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(false);
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null);

  const fetchAll = useCallback(async () => {
    setFetchError(false);
    try {
      const [healthRes, queueRes] = await Promise.all([
        fetch('/api/admin/health', { credentials: 'include' }),
        fetch('/api/admin/queue', { credentials: 'include' }),
      ]);
      if (healthRes.ok) {
        const data = await healthRes.json();
        setChecks(data.checks || []);
      } else {
        setFetchError(true);
      }
      if (queueRes.ok) {
        const data = await queueRes.json();
        setQueues(data.queues || []);
      }
      setLastRefreshed(new Date());
    } catch {
      setFetchError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    const interval = setInterval(fetchAll, 15_000);
    return () => clearInterval(interval);
  }, [fetchAll]);

  const overallStatus = checks.every(c => c.status === 'healthy')
    ? 'healthy'
    : checks.some(c => c.status === 'unhealthy')
    ? 'unhealthy'
    : 'degraded';

  const totalQueued = queues.reduce((s, q) => s + q.count, 0);
  const totalReady = queues.reduce((s, q) => s + q.ready_count, 0);
  const totalStale = queues.reduce((s, q) => s + q.stale_processing_count, 0);
  const maxLatency = checks.length > 0 ? Math.max(...checks.map(c => c.response_time_ms)) : 0;

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_health.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (fetchError && checks.length === 0) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_health.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="m" alignItems="center">
            <Box color="text-status-error">Failed to load health data.</Box>
            <Button iconName="refresh" onClick={fetchAll}>Retry</Button>
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
          description={t('pages.api_health.description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              {lastRefreshed && (
                <Box color="text-status-inactive" padding={{ top: 'xs' }} fontSize="body-s">
                  Updated {lastRefreshed.toLocaleTimeString()} · auto-refreshes 15s
                </Box>
              )}
              <Button iconName="refresh" onClick={fetchAll}>Refresh</Button>
            </SpaceBetween>
          }
        >
          {t('pages.api_health.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* KPI Summary */}
        <ColumnLayout columns={4} variant="text-grid" minColumnWidth={140}>
          <Container>
            <SpaceBetween size="xs">
              <Box color="text-body-secondary" fontSize="body-s">Overall Status</Box>
              <StatusIndicator type={overallStatus === 'healthy' ? 'success' : overallStatus === 'degraded' ? 'warning' : 'error'}>
                {overallStatus.charAt(0).toUpperCase() + overallStatus.slice(1)}
              </StatusIndicator>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box color={latencyColor(maxLatency)} fontSize="display-l" fontWeight="bold">
                {maxLatency}ms
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">Max DB Latency</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{totalQueued}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                Queue Depth ({totalReady} ready)
              </Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box
                fontSize="display-l"
                fontWeight="bold"
                color={totalStale > 0 ? 'text-status-error' : 'text-status-success'}
              >
                {totalStale}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">Stale Jobs</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* Service Health */}
        <Table
          columnDefinitions={[
            {
              header: t('pages.health_page.service'),
              cell: (item: HealthCheck) => item.service,
              width: '25%',
            },
            {
              header: t('pages.api_health.status'),
              cell: (item: HealthCheck) => (
                <Badge color={statusBadgeColor(item.status)}>{item.status}</Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.health_page.response_time_ms'),
              cell: (item: HealthCheck) => (
                <Box color={latencyColor(item.response_time_ms)}>{item.response_time_ms} ms</Box>
              ),
              width: '20%',
            },
            {
              header: t('pages.health_page.last_check'),
              cell: (item: HealthCheck) => new Date(item.last_check).toLocaleString(),
              width: '35%',
            },
          ]}
          items={checks}
          header={<Header variant="h2">Service Checks</Header>}
          empty={<Box textAlign="center" padding="l" color="text-body-secondary">No health data</Box>}
        />

        {/* Queue Stats */}
        {queues.length > 0 && (
          <Table
            columnDefinitions={[
              {
                header: 'Topic',
                cell: (q: QueueStat) => <Box fontWeight="bold">{q.topic}</Box>,
                width: '28%',
              },
              {
                header: 'Total',
                cell: (q: QueueStat) => q.count,
                width: '12%',
              },
              {
                header: 'Ready',
                cell: (q: QueueStat) => (
                  <Box color={q.ready_count > 100 ? 'text-status-warning' : undefined}>{q.ready_count}</Box>
                ),
                width: '12%',
              },
              {
                header: 'Delayed',
                cell: (q: QueueStat) => q.delayed_count,
                width: '12%',
              },
              {
                header: 'Stale',
                cell: (q: QueueStat) => (
                  <Box color={q.stale_processing_count > 0 ? 'text-status-error' : undefined}>
                    {q.stale_processing_count}
                  </Box>
                ),
                width: '12%',
              },
              {
                header: 'Oldest Ready',
                cell: (q: QueueStat) => q.oldest_ready_at
                  ? new Date(q.oldest_ready_at).toLocaleTimeString()
                  : '—',
                width: '24%',
              },
            ]}
            items={queues}
            header={<Header variant="h2" counter={`(${queues.length})`}>Queue Stats</Header>}
          />
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
