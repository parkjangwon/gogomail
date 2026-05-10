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

interface UserConfig {
  id: string;
  user_email: string;
  config_key: string;
  config_value: string;
  last_updated: string;
}

export default function UserConfigPage() {
  const [configs, setConfigs] = useState<UserConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchUserConfig();
  }, []);

  const fetchUserConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/config/user?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch user config:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredConfigs = configs.filter(c =>
    c.user_email.toLowerCase().includes(filter.toLowerCase()) ||
    c.config_key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">User Config</Header>}>
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
          description="Configure per-user settings"
          actions={
            <Button variant="primary" disabled>
              + Add Config
            </Button>
          }
        >
          User Config
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'User Email',
              cell: (item: UserConfig) => item.user_email,
              width: '30%',
            },
            {
              header: 'Key',
              cell: (item: UserConfig) => item.config_key,
              width: '25%',
            },
            {
              header: 'Value',
              cell: (item: UserConfig) => item.config_value,
              width: '25%',
            },
            {
              header: 'Last Updated',
              cell: (item: UserConfig) => new Date(item.last_updated).toLocaleString(),
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
