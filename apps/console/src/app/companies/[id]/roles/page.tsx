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
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface Role {
  id: string;
  name: string;
  description: string;
  permissions_count: number;
  assigned_users: number;
  created_at: string;
}

export default function RolesPage() {
  const { t } = useI18n();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchRoles();
  }, []);

  const fetchRoles = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/roles?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRoles(data.roles || []);
      }
    } catch (error) {
      // Roles endpoint may not exist — silently use empty list
    } finally {
      setLoading(false);
    }
  };

  const filteredRoles = roles.filter((r) =>
    r.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.roles_page.title')}</Header>}>
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
          description={t('pages.roles_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.roles_page.create_role')}
            </Button>
          }
        >
          {t('pages.roles_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="warning">{t('pages.roles_page.backend_warning')}</Alert>

        <Table
          columnDefinitions={[
            {
              header: t('pages.roles_page.name'),
              cell: (item: Role) => item.name,
              width: '20%',
            },
            {
              header: t('pages.roles_page.description_col'),
              cell: (item: Role) => item.description,
              width: '35%',
            },
            {
              header: t('pages.roles_page.permissions'),
              cell: (item: Role) => item.permissions_count,
              width: '15%',
            },
            {
              header: t('pages.roles_page.assigned_users'),
              cell: (item: Role) => item.assigned_users,
              width: '15%',
            },
            {
              header: t('pages.roles_page.created'),
              cell: (item: Role) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredRoles}
          header={
            <Header variant="h2" counter={`(${filteredRoles.length})`}>
              {t('pages.roles_page.roles')}
            </Header>
          }
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
