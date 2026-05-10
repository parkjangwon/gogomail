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
  const { t } = useI18n();
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
      <ContentLayout header={<Header variant="h1">{t('pages.relays.title')}</Header>}>
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
          description={t('pages.relays_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.relays.create_relay')}
            </Button>
          }
        >
          {t('pages.relays_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.relays_page.host'),
              cell: (item: TrustedRelay) => item.host,
              width: '30%',
            },
            {
              header: t('pages.relays_page.port'),
              cell: (item: TrustedRelay) => item.port,
              width: '10%',
            },
            {
              header: t('pages.relays_page.protocol'),
              cell: (item: TrustedRelay) => item.protocol,
              width: '12%',
            },
            {
              header: t('pages.relays.status'),
              cell: (item: TrustedRelay) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.relays_page.active_connections'),
              cell: (item: TrustedRelay) => item.active_connections,
              width: '15%',
            },
            {
              header: t('pages.relays_page.created'),
              cell: (item: TrustedRelay) => new Date(item.created_at).toLocaleDateString(),
              width: '21%',
            },
          ]}
          items={filteredRelays}
          header={<Header variant="h2" counter={`(${filteredRelays.length})`}>{t('pages.relays_page.relays')}</Header>}
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
