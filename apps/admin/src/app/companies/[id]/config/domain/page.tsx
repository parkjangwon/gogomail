'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface DomainConfig {
  id: string;
  domain: string;
  config_key: string;
  config_value: string;
  last_updated: string;
}

export default function DomainConfigPage() {
  const [configs, setConfigs] = useState<DomainConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDomainConfig();
  }, []);

  const fetchDomainConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/config/domain?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch domain config:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredConfigs = configs.filter(c =>
    c.domain.toLowerCase().includes(filter.toLowerCase()) ||
    c.config_key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Domain Config</Header>}>
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
          description="Configure domain-specific settings"
          actions={
            <Button variant="primary" disabled>
              + Add Config
            </Button>
          }
        >
          Domain Config
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Domain',
              cell: (item: DomainConfig) => item.domain,
              width: '20%',
            },
            {
              header: 'Key',
              cell: (item: DomainConfig) => item.config_key,
              width: '25%',
            },
            {
              header: 'Value',
              cell: (item: DomainConfig) => item.config_value,
              width: '35%',
            },
            {
              header: 'Last Updated',
              cell: (item: DomainConfig) => new Date(item.last_updated).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredConfigs}
          header={<Header variant="h2" counter={`(${filteredConfigs.length})`}>Config</Header>}
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
