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

interface DKIMKey {
  id: string;
  domain: string;
  selector: string;
  status: string;
  dns_verified: boolean;
  created_at: string;
}

export default function DKIMKeysPage() {
  const { t } = useI18n();
  const [keys, setKeys] = useState<DKIMKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDKIMKeys();
  }, []);

  const fetchDKIMKeys = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/dkim-keys?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys || []);
      }
    } catch (error) {
      console.error('Failed to fetch DKIM keys:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredKeys = keys.filter(k =>
    k.domain.toLowerCase().includes(filter.toLowerCase()) ||
    k.selector.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.dkim_keys.title')}</Header>}>
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
          description="Manage DKIM signing keys for domain authentication"
          actions={
            <Button variant="primary" disabled>
              + Generate Key
            </Button>
          }
        >
          DKIM Keys
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.dkim_keys.domain'),
              cell: (item: DKIMKey) => item.domain,
              width: '25%',
            },
            {
              header: 'Selector',
              cell: (item: DKIMKey) => item.selector,
              width: '20%',
            },
            {
              header: t('pages.dkim_keys.status'),
              cell: (item: DKIMKey) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'DNS Verified',
              cell: (item: DKIMKey) => (
                <Badge color={item.dns_verified ? 'green' : 'red'}>
                  {item.dns_verified ? 'Verified' : 'Not Verified'}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: 'Created',
              cell: (item: DKIMKey) => new Date(item.created_at).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredKeys}
          header={<Header variant="h2" counter={`(${filteredKeys.length})`}>Keys</Header>}
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
