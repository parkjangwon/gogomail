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

interface Principal {
  id: string;
  email: string;
  name: string;
  type: string;
  status: string;
  created_at: string;
}

export default function DirectoryPage() {
  const { t } = useI18n();
  const [principals, setPrincipals] = useState<Principal[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchPrincipals();
  }, []);

  const fetchPrincipals = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/principals?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setPrincipals(data.principals || []);
      }
    } catch (error) {
      console.error('Failed to fetch principals:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredPrincipals = principals.filter(p =>
    p.email.toLowerCase().includes(filter.toLowerCase()) ||
    p.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.directory_page.title')}</Header>}>
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
          description={t('pages.directory_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.directory_page.add_principal')}
            </Button>
          }
        >
          {t('pages.directory_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.directory_page.email'),
              cell: (item: Principal) => item.email,
              width: '30%',
            },
            {
              header: t('pages.directory_page.name'),
              cell: (item: Principal) => item.name,
              width: '25%',
            },
            {
              header: t('pages.directory_page.type'),
              cell: (item: Principal) => (
                <Badge color="blue">{item.type}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.directory_page.status'),
              cell: (item: Principal) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.directory_page.created'),
              cell: (item: Principal) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredPrincipals}
          header={<Header variant="h2" counter={`(${filteredPrincipals.length})`}>{t('pages.directory_page.principals')}</Header>}
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
