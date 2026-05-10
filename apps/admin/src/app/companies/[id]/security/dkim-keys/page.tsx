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
          description={t('pages.dkim_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.dkim_page.generate_key')}
            </Button>
          }
        >
          {t('pages.dkim_page.title')}
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
              header: t('pages.dkim_page.selector'),
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
              header: t('pages.dkim_page.dns_verified'),
              cell: (item: DKIMKey) => (
                <Badge color={item.dns_verified ? 'green' : 'red'}>
                  {item.dns_verified ? t('pages.dkim_page.verified') : t('pages.dkim_page.not_verified')}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.dkim_page.created'),
              cell: (item: DKIMKey) => new Date(item.created_at).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredKeys}
          header={<Header variant="h2" counter={`(${filteredKeys.length})`}>{t('pages.dkim_page.keys')}</Header>}
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
