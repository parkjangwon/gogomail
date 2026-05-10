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
import { useI18n } from '@/app/i18n-provider';

interface Alias {
  id: string;
  alias_email: string;
  target_email: string;
  status: string;
  created_at: string;
}

export default function AliasesPage() {
  const { t } = useI18n();
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
      <ContentLayout header={<Header variant="h1">{t('pages.aliases_page.title')}</Header>}>
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
          description={t('pages.aliases_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.aliases_page.create_alias')}
            </Button>
          }
        >
          {t('pages.aliases_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.aliases_page.alias_email'),
              cell: (item: Alias) => item.alias_email,
              width: '35%',
            },
            {
              header: t('pages.aliases_page.target_email'),
              cell: (item: Alias) => item.target_email,
              width: '35%',
            },
            {
              header: t('pages.aliases_page.status'),
              cell: (item: Alias) => item.status,
              width: '15%',
            },
            {
              header: t('pages.aliases_page.created'),
              cell: (item: Alias) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredAliases}
          header={<Header variant="h2" counter={`(${filteredAliases.length})`}>{t('pages.aliases_page.aliases')}</Header>}
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
