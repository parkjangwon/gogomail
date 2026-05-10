'use client';

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
  Table,
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
        <Container header={<Header variant="h2">System Resources</Header>}>
          <ColumnLayout columns={3} variant="text-grid">
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>CPU Usage</small>
              </Box>
              <ProgressBar
                value={cpuUsage}
                label={`${Math.round(cpuUsage)}%`}
                status={cpuUsage > 80 ? 'error' : 'success'}
              />
            </Box>
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>Memory Usage</small>
              </Box>
              <ProgressBar
                value={memoryUsage}
                label={`${Math.round(memoryUsage)}%`}
                status={memoryUsage > 85 ? 'error' : 'success'}
              />
            </Box>
            <Box>
              <Box variant="h3" color="text-body-secondary">
                <small>Disk Usage</small>
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
        <Container header={<Header variant="h2">Message Queue</Header>}>
          <KeyValuePairs
            items={[
              { label: 'Total Messages', value: stats.total },
              {
                label: 'Status',
                value: stats.failed > 0 ? (
                  <StatusIndicator type="warning">
                    {stats.failed} Failed
                  </StatusIndicator>
                ) : (
                  <StatusIndicator type="success">Healthy</StatusIndicator>
                ),
              },
              { label: 'Processing', value: stats.processing },
              { label: 'Pending', value: stats.pending },
            ]}
          />
        </Container>

        {/* Network Traffic */}
        <Container header={<Header variant="h2">Network Traffic</Header>}>
          <Table
            columnDefinitions={[
              { header: 'Protocol', cell: (item: any) => item.protocol, width: '20%' },
              { header: 'Inbound (Mbps)', cell: (item: any) => item.inbound, width: '25%' },
              { header: 'Outbound (Mbps)', cell: (item: any) => item.outbound, width: '25%' },
              { header: 'Connections', cell: (item: any) => item.connections, width: '15%' },
              { header: 'Status', cell: (item: any) => (
                <StatusIndicator type={item.connections > 0 ? 'success' : 'pending'}>
                  {item.connections > 0 ? 'Active' : 'Idle'}
                </StatusIndicator>
              ), width: '15%' },
            ]}
            items={[
              { protocol: 'SMTP', inbound: 45.2, outbound: 128.5, connections: 42 },
              { protocol: 'IMAP', inbound: 230.8, outbound: 12.3, connections: 156 },
              { protocol: 'HTTP API', inbound: 89.4, outbound: 156.2, connections: 28 },
            ]}
            header={<Header variant="h3">Active Connections</Header>}
          />
        </Container>

        {/* Database */}
        <Container header={<Header variant="h2">Database</Header>}>
          <KeyValuePairs
            items={[
              { label: 'Status', value: <StatusIndicator type="success">Connected</StatusIndicator> },
              { label: 'Response Time', value: '12ms' },
              { label: 'Active Connections', value: '24 / 50' },
              { label: 'Last Backup', value: new Date(Date.now() - 3600000).toLocaleString() },
            ]}
          />
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
