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

interface Alias {
  id: string;
  alias_email: string;
  target_email: string;
  status: string;
  created_at: string;
}

export default function AliasesPage() {
  const [aliases, setAliases] = useState<Alias[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchAliases();
  }, []);

  const fetchAliases = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/aliases?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setAliases(data.aliases || []);
      }
    } catch (error) {
      console.error('Failed to fetch aliases:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredAliases = aliases.filter(a =>
    a.alias_email.toLowerCase().includes(filter.toLowerCase()) ||
    a.target_email.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Aliases</Header>}>
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
          description="Manage email aliases"
          actions={
            <Button variant="primary" disabled>
              + Create Alias
            </Button>
          }
        >
          Aliases
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Alias Email',
              cell: (item: Alias) => item.alias_email,
              width: '35%',
            },
            {
              header: 'Target Email',
              cell: (item: Alias) => item.target_email,
              width: '35%',
            },
            {
              header: 'Status',
              cell: (item: Alias) => item.status,
              width: '15%',
            },
            {
              header: 'Created',
              cell: (item: Alias) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredAliases}
          header={<Header variant="h2" counter={`(${filteredAliases.length})`}>Aliases</Header>}
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
