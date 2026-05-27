'use client';

import { Container, Box, ColumnLayout } from '@cloudscape-design/components';
import { SpamFilterStats as SpamFilterStatsType } from './spamFilterTypes';

interface SpamFilterStatsProps {
  stats: SpamFilterStatsType | null;
  lastUpdated: Date | null;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterStats({ stats, t }: SpamFilterStatsProps) {
  const filteredRate = stats?.total_messages
    ? Math.round(((stats.filtered ?? 0) / stats.total_messages) * 100)
    : 0;

  return (
    <ColumnLayout columns={5} variant="text-grid" minColumnWidth={140}>
      <Container>
        <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_total')}</Box>
        <Box variant="h2">{(stats?.total_messages ?? 0).toLocaleString()}</Box>
      </Container>
      <Container>
        <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_filtered')}</Box>
        <Box variant="h2">{(stats?.filtered ?? 0).toLocaleString()}</Box>
      </Container>
      <Container>
        <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_rejected')}</Box>
        <Box variant="h2">{(stats?.rejected ?? 0).toLocaleString()}</Box>
      </Container>
      <Container>
        <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_delivered')}</Box>
        <Box variant="h2">{(stats?.delivered ?? 0).toLocaleString()}</Box>
      </Container>
      <Container>
        <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_filter_rate')}</Box>
        <Box variant="h2">{filteredRate}%</Box>
      </Container>
    </ColumnLayout>
  );
}
