'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
} from '@cloudscape-design/components';
import { useMemo, useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
import { type DirectoryPrincipal, useDirectoryPrincipals } from '@/hooks/useDirectory';

export default function DirectoryPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data: principals = [], isLoading: loading } = useDirectoryPrincipals(companyId);
  const [filter, setFilter] = useState('');

  const filteredPrincipals = useMemo(() => principals.filter(p =>
    (p.primary_email || '').toLowerCase().includes(filter.toLowerCase()) ||
    (p.display_name || '').toLowerCase().includes(filter.toLowerCase())
  ), [principals, filter]);

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
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.directory_page.email'),
              cell: (item: DirectoryPrincipal) => item.primary_email,
              width: '30%',
            },
            {
              header: t('pages.directory_page.name'),
              cell: (item: DirectoryPrincipal) => item.display_name,
              width: '25%',
            },
            {
              header: t('pages.directory_page.type'),
              cell: (item: DirectoryPrincipal) => (
                <Badge color="blue">{item.kind}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.directory_page.status'),
              cell: (item: DirectoryPrincipal) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
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
