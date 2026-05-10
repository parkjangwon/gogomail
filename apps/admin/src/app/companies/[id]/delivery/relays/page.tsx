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
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface TrustedRelay {
  id: string;
  host: string;
  port: number;
  protocol: string;
  status: string;
  active_connections: number;
  created_at: string;
}

export default function TrustedRelaysPage() {
  const { t: _unused } = useI18n(); _unused;
  const [relays, setRelays] = useState<TrustedRelay[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchRelays();
  }, []);

  const fetchRelays = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/trusted-relays?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRelays(data.relays || []);
      }
    } catch (error) {
      console.error('Failed to fetch trusted relays:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredRelays = relays.filter(r =>
    r.host.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Trusted Relays</Header>}>
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
          description="Manage trusted mail relays"
          actions={
            <Button variant="primary" disabled>
              + Add Relay
            </Button>
          }
        >
          Trusted Relays
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Host',
              cell: (item: TrustedRelay) => item.host,
              width: '30%',
            },
            {
              header: 'Port',
              cell: (item: TrustedRelay) => item.port,
              width: '10%',
            },
            {
              header: 'Protocol',
              cell: (item: TrustedRelay) => item.protocol,
              width: '12%',
            },
            {
              header: 'Status',
              cell: (item: TrustedRelay) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: 'Active Connections',
              cell: (item: TrustedRelay) => item.active_connections,
              width: '15%',
            },
            {
              header: 'Created',
              cell: (item: TrustedRelay) => new Date(item.created_at).toLocaleDateString(),
              width: '21%',
            },
          ]}
          items={filteredRelays}
          header={<Header variant="h2" counter={`(${filteredRelays.length})`}>Relays</Header>}
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
