'use client';

import {
  ContentLayout,
  Header,
  Table,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Container,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface APIUsageRecord {
  id: string;
  principal: string;
  endpoint: string;
  method: string;
  request_count: number;
  date: string;
}

export default function APIUsagePage() {
  const { t } = useI18n();
  const [records, setRecords] = useState<APIUsageRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchAPIUsage();
  }, []);

  const fetchAPIUsage = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/api-usage/daily?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRecords(data.api_usage_daily || []);
      }
    } catch (error) {
      console.error('Failed to fetch API usage:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredRecords = records.filter(r =>
    r.principal.toLowerCase().includes(filter.toLowerCase()) ||
    r.endpoint.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_usage.title')}</Header>}>
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
          description={t('pages.api_usage_analytics.description')}
        >
          {t('pages.api_usage_analytics.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">{t('pages.api_usage_analytics.daily_api_usage')}</Header>}>
          <Box>{t('pages.api_usage_analytics.daily_desc')}</Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: t('pages.api_usage_analytics.principal'),
              cell: (item: APIUsageRecord) => item.principal,
              width: '25%',
            },
            {
              header: t('pages.api_usage_analytics.endpoint'),
              cell: (item: APIUsageRecord) => item.endpoint,
              width: '30%',
            },
            {
              header: t('pages.api_usage_analytics.method'),
              cell: (item: APIUsageRecord) => item.method,
              width: '10%',
            },
            {
              header: t('pages.api_usage_analytics.requests'),
              cell: (item: APIUsageRecord) => item.request_count.toLocaleString(),
              width: '15%',
            },
            {
              header: t('pages.api_usage_analytics.date'),
              cell: (item: APIUsageRecord) => new Date(item.date).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredRecords}
          header={<Header variant="h2" counter={`(${filteredRecords.length})`}>{t('pages.api_usage_analytics.records')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
