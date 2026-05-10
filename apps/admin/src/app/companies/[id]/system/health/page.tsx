'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Table,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface HealthCheck {
  service: string;
  status: 'healthy' | 'degraded' | 'unhealthy';
  response_time_ms: number;
  last_check: string;
}

export default function APIHealthPage() {
  const { t } = useI18n();
  const [checks, setChecks] = useState<HealthCheck[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 10000);
    return () => clearInterval(interval);
  }, []);

  const fetchHealth = async () => {
    try {
      const res = await fetch('/api/admin/health', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setChecks(data.checks || []);
      }
    } catch (error) {
      console.error('Failed to fetch health:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy': return 'green';
      case 'degraded': return 'severity-high';
      case 'unhealthy': return 'red';
      default: return 'grey';
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_health.title')}</Header>}>
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
          description={t('pages.api_health.description')}
        >
          {t('pages.api_health.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Service',
              cell: (item: HealthCheck) => item.service,
              width: '25%',
            },
            {
              header: t('pages.api_health.status'),
              cell: (item: HealthCheck) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: 'Response Time (ms)',
              cell: (item: HealthCheck) => item.response_time_ms,
              width: '20%',
            },
            {
              header: 'Last Check',
              cell: (item: HealthCheck) => new Date(item.last_check).toLocaleString(),
              width: '35%',
            },
          ]}
          items={checks}
          header={<Header variant="h2">{t('pages.api_health.title')}</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
