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
  const { t: _unused } = useI18n(); _unused;
  const [records, setRecords] = useState<APIUsageRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchAPIUsage();
  }, []);

  const fetchAPIUsage = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/api-usage?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRecords(data.records || []);
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
      <ContentLayout header={<Header variant="h1">API Usage</Header>}>
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
          description="Monitor API usage patterns and statistics"
        >
          API Usage Analytics
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">Daily API Usage</Header>}>
          <Box>View API usage broken down by principal and endpoint</Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: 'Principal',
              cell: (item: APIUsageRecord) => item.principal,
              width: '25%',
            },
            {
              header: 'Endpoint',
              cell: (item: APIUsageRecord) => item.endpoint,
              width: '30%',
            },
            {
              header: 'Method',
              cell: (item: APIUsageRecord) => item.method,
              width: '10%',
            },
            {
              header: 'Requests',
              cell: (item: APIUsageRecord) => item.request_count.toLocaleString(),
              width: '15%',
            },
            {
              header: 'Date',
              cell: (item: APIUsageRecord) => new Date(item.date).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredRecords}
          header={<Header variant="h2" counter={`(${filteredRecords.length})`}>Records</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
