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

interface GroupMembership {
  id: string;
  group_name: string;
  member_email: string;
  role: string;
  joined_at: string;
}

export default function GroupMembershipsPage() {
  const { t } = useI18n();
  const [memberships, setMemberships] = useState<GroupMembership[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchMemberships();
  }, []);

  const fetchMemberships = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/group-memberships?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setMemberships(data.memberships || []);
      }
    } catch (error) {
      console.error('Failed to fetch group memberships:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredMemberships = memberships.filter(m =>
    m.group_name.toLowerCase().includes(filter.toLowerCase()) ||
    m.member_email.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.groups.title')}</Header>}>
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
          description={t('pages.groups.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.groups.create_group')}
            </Button>
          }
        >
          {t('pages.groups.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.groups.group_name'),
              cell: (item: GroupMembership) => item.group_name,
              width: '25%',
            },
            {
              header: t('pages.groups.members'),
              cell: (item: GroupMembership) => item.member_email,
              width: '35%',
            },
            {
              header: t('pages.groups_page.role'),
              cell: (item: GroupMembership) => (
                <Badge color="blue">{item.role}</Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.groups.created'),
              cell: (item: GroupMembership) => new Date(item.joined_at).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredMemberships}
          header={<Header variant="h2" counter={`(${filteredMemberships.length})`}>{t('pages.groups_page.memberships')}</Header>}
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
