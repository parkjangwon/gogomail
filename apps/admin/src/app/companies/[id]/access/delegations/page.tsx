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

interface Delegation {
  id: string;
  delegator: string;
  delegate: string;
  permissions: string[];
  status: string;
  created_at: string;
}

export default function DelegationsPage() {
  const { t } = useI18n();
  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDelegations();
  }, []);

  const fetchDelegations = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/delegations?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setDelegations(data.delegations || []);
      }
    } catch (error) {
      console.error('Failed to fetch delegations:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredDelegations = delegations.filter(d =>
    d.delegator.toLowerCase().includes(filter.toLowerCase()) ||
    d.delegate.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.delegations.title')}</Header>}>
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
          description="Manage user delegations and permissions"
          actions={
            <Button variant="primary" disabled>
              {t('pages.delegations.create_delegation')}
            </Button>
          }
        >
          Delegations
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.delegations.delegator'),
              cell: (item: Delegation) => item.delegator,
              width: '25%',
            },
            {
              header: t('pages.delegations.delegate'),
              cell: (item: Delegation) => item.delegate,
              width: '25%',
            },
            {
              header: 'Permissions',
              cell: (item: Delegation) => item.permissions.join(', '),
              width: '35%',
            },
            {
              header: t('pages.delegations.status'),
              cell: (item: Delegation) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
          ]}
          items={filteredDelegations}
          header={<Header variant="h2" counter={`(${filteredDelegations.length})`}>{t('pages.delegations.title')}</Header>}
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
