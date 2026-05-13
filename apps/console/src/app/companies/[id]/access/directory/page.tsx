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
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface Principal {
  ID: string;
  Kind: string;
  CompanyID: string;
  DomainID: string;
  OrganizationID: string;
  DisplayName: string;
  PrimaryEmail: string;
  Status: string;
}

export default function DirectoryPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [principals, setPrincipals] = useState<Principal[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchPrincipals();
  }, [companyId]);

  const fetchPrincipals = async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/directory/principals?company_id=${companyId}&limit=100`, {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setPrincipals(data.directory_principals || []);
      }
    } catch (error) {
      console.error('Failed to fetch principals:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredPrincipals = principals.filter(p =>
    p.PrimaryEmail.toLowerCase().includes(filter.toLowerCase()) ||
    p.DisplayName.toLowerCase().includes(filter.toLowerCase())
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
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.directory_page.email'),
              cell: (item: Principal) => item.PrimaryEmail,
              width: '30%',
            },
            {
              header: t('pages.directory_page.name'),
              cell: (item: Principal) => item.DisplayName,
              width: '25%',
            },
            {
              header: t('pages.directory_page.type'),
              cell: (item: Principal) => (
                <Badge color="blue">{item.Kind}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.directory_page.status'),
              cell: (item: Principal) => (
                <Badge color={item.Status === 'active' ? 'green' : 'grey'}>
                  {item.Status}
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
