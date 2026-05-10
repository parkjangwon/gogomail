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
  const { t: _unused } = useI18n(); _unused;
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
      <ContentLayout header={<Header variant="h1">Group Memberships</Header>}>
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
          description="Manage group memberships and roles"
          actions={
            <Button variant="primary" disabled>
              + Add Member
            </Button>
          }
        >
          Group Memberships
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Group',
              cell: (item: GroupMembership) => item.group_name,
              width: '25%',
            },
            {
              header: 'Member Email',
              cell: (item: GroupMembership) => item.member_email,
              width: '35%',
            },
            {
              header: 'Role',
              cell: (item: GroupMembership) => (
                <Badge color="blue">{item.role}</Badge>
              ),
              width: '20%',
            },
            {
              header: 'Joined',
              cell: (item: GroupMembership) => new Date(item.joined_at).toLocaleDateString(),
              width: '20%',
            },
          ]}
          items={filteredMemberships}
          header={<Header variant="h2" counter={`(${filteredMemberships.length})`}>Memberships</Header>}
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
