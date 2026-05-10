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
      <ContentLayout header={<Header variant="h1">Queue Stats</Header>}>
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
          description="Real-time mail processing queue monitoring"
        >
          Queue Stats
        </Header>
      }
    >
      <SpaceBetween size="l">
        {stats && (
          <>
            <ColumnLayout columns={4}>
              <Container header={<Header variant="h3">Total Messages</Header>}>
                <Box fontSize="display-l" fontWeight="bold">
                  {stats.total_messages.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">Pending</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-warning">
                  {stats.pending.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">Processing</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-info">
                  {stats.processing.toLocaleString()}
                </Box>
              </Container>
              <Container header={<Header variant="h3">Failed</Header>}>
                <Box fontSize="display-l" fontWeight="bold" color="text-status-error">
                  {stats.failed.toLocaleString()}
                </Box>
              </Container>
            </ColumnLayout>

            <Container header={<Header variant="h3">Queue Status</Header>}>
              <SpaceBetween size="m">
                <Box>
                  <Box>Processing Rate: {stats.processing_rate} msg/sec</Box>
                  <ProgressBar value={stats.processing_rate} label="Current Rate" />
                </Box>
                <Box>
                  <Box>Avg Wait Time: {stats.average_wait_time}s</Box>
                  <ProgressBar value={Math.min(stats.average_wait_time, 100)} label="Wait Time" />
                </Box>
              </SpaceBetween>
            </Container>

            <Container header={<Header variant="h3">Last Updated</Header>}>
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
