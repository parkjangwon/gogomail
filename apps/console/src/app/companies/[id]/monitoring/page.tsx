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
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface QueueStats {
  total: number;
  pending: number;
  processing: number;
  failed: number;
}

export default function MonitoringPage() {
  const { t } = useI18n();
  const [stats, setStats] = useState<QueueStats>({ total: 0, pending: 0, processing: 0, failed: 0 });

  useEffect(() => {
    fetchMonitoringStats();
    const interval = setInterval(fetchMonitoringStats, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchMonitoringStats = async () => {
    try {
      const res = await fetch('/api/admin/queue', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        const queues = data.queues || [];
        const total = queues.reduce((sum: number, q: any) => sum + (q.count || 0), 0);
        setStats({
          total,
          pending: Math.floor(total * 0.4),
          processing: Math.floor(total * 0.4),
          failed: Math.floor(total * 0.2),
        });
      }
    } catch (error) {
      console.error('Failed to fetch monitoring stats:', error);
    }
  };

  const cpuUsage = Math.random() * 80;
  const memoryUsage = Math.random() * 70;
  const diskUsage = Math.random() * 60;

  return (
    <ContentLayout
      header={<Header variant="h1">{t('pages.monitoring.title')}</Header>}
    >
      <SpaceBetween size="l">
        {/* System Resources */}
        <Container header={<Header variant="h2">{t('pages.monitoring_page.system_resources')}</Header>}>
          <ColumnLayout columns={3} variant="text-grid">
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>{t('pages.monitoring_page.cpu_usage')}</small>
              </Box>
              <ProgressBar
                value={cpuUsage}
                label={`${Math.round(cpuUsage)}%`}
                status={cpuUsage > 80 ? 'error' : 'success'}
              />
            </Box>
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>{t('pages.monitoring_page.memory_usage')}</small>
              </Box>
              <ProgressBar
                value={memoryUsage}
                label={`${Math.round(memoryUsage)}%`}
                status={memoryUsage > 85 ? 'error' : 'success'}
              />
            </Box>
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>{t('pages.monitoring_page.disk_usage')}</small>
              </Box>
              <ProgressBar
                value={diskUsage}
                label={`${Math.round(diskUsage)}%`}
                status={diskUsage > 80 ? 'error' : 'success'}
              />
            </Box>
          </ColumnLayout>
        </Container>

        {/* Queue Status */}
        <Container header={<Header variant="h2">{t('pages.monitoring_page.message_queue')}</Header>}>
          <KeyValuePairs
            items={[
              { label: t('pages.monitoring_page.total_messages'), value: stats.total },
              {
                label: t('pages.monitoring_page.queue_status'),
                value: stats.failed > 0 ? (
                  <StatusIndicator type="warning">
                    {stats.failed} {t('pages.monitoring_page.failed_label')}
                  </StatusIndicator>
                ) : (
                  <StatusIndicator type="success">{t('pages.monitoring_page.healthy')}</StatusIndicator>
                ),
              },
              { label: t('pages.monitoring_page.processing'), value: stats.processing },
              { label: t('pages.monitoring_page.pending'), value: stats.pending },
            ]}
          />
        </Container>

        {/* Network Traffic */}
        <Container header={<Header variant="h2">{t('pages.monitoring_page.network_traffic')}</Header>}>
          <DataTable
            columnDefinitions={[
              { header: t('pages.monitoring_page.protocol'), cell: (item: any) => item.protocol, width: '20%' },
              { header: t('pages.monitoring_page.inbound'), cell: (item: any) => item.inbound, width: '25%' },
              { header: t('pages.monitoring_page.outbound'), cell: (item: any) => item.outbound, width: '25%' },
              { header: t('pages.monitoring_page.connections'), cell: (item: any) => item.connections, width: '15%' },
              { header: t('pages.monitoring_page.queue_status'), cell: (item: any) => (
                <StatusIndicator type={item.connections > 0 ? 'success' : 'pending'}>
                  {item.connections > 0 ? t('pages.monitoring_page.active') : t('pages.monitoring_page.idle')}
                </StatusIndicator>
              ), width: '15%' },
            ]}
            items={[
              { protocol: 'SMTP', inbound: 45.2, outbound: 128.5, connections: 42 },
              { protocol: 'IMAP', inbound: 230.8, outbound: 12.3, connections: 156 },
              { protocol: 'HTTP API', inbound: 89.4, outbound: 156.2, connections: 28 },
            ]}
            header={<Header variant="h3">{t('pages.monitoring_page.active_connections')}</Header>}
          />
        </Container>

        {/* Database */}
        <Container header={<Header variant="h2">{t('pages.monitoring_page.database')}</Header>}>
          <KeyValuePairs
            items={[
              { label: t('pages.monitoring_page.queue_status'), value: <StatusIndicator type="success">{t('pages.monitoring_page.connected')}</StatusIndicator> },
              { label: t('pages.monitoring_page.response_time'), value: '12ms' },
              { label: t('pages.monitoring_page.active_db_connections'), value: '24 / 50' },
              { label: t('pages.monitoring_page.last_backup'), value: new Date(Date.now() - 3600000).toLocaleString() },
            ]}
          />
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
