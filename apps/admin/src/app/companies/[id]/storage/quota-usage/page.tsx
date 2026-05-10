'use client';

import {
  ContentLayout,
  Header,
  Table,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  ProgressBar,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface QuotaUsage {
  id: string;
  tenant: string;
  used_gb: number;
  quota_gb: number;
  percentage: number;
  last_updated: string;
}

export default function QuotaUsagePage() {
  const [quotas, setQuotas] = useState<QuotaUsage[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchQuotaUsage();
  }, []);

  const fetchQuotaUsage = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/quota-usage?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setQuotas(data.quotas || []);
      }
    } catch (error) {
      console.error('Failed to fetch quota usage:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredQuotas = quotas.filter(q =>
    q.tenant.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Quota Usage</Header>}>
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
          description="Monitor storage quota usage across tenants"
        >
          Quota Usage
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Tenant',
              cell: (item: QuotaUsage) => item.tenant,
              width: '25%',
            },
            {
              header: 'Used',
              cell: (item: QuotaUsage) => `${item.used_gb.toFixed(2)} GB`,
              width: '15%',
            },
            {
              header: 'Quota',
              cell: (item: QuotaUsage) => `${item.quota_gb.toFixed(2)} GB`,
              width: '15%',
            },
            {
              header: 'Usage',
              cell: (item: QuotaUsage) => (
                <Box>
                  <ProgressBar value={item.percentage} />
                  <Box color="text-body-secondary" fontSize="body-s">
                    {item.percentage.toFixed(1)}%
                  </Box>
                </Box>
              ),
              width: '30%',
            },
            {
              header: 'Last Updated',
              cell: (item: QuotaUsage) => new Date(item.last_updated).toLocaleString(),
              width: '15%',
            },
          ]}
          items={filteredQuotas}
          header={<Header variant="h2" counter={`(${filteredQuotas.length})`}>Quotas</Header>}
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
