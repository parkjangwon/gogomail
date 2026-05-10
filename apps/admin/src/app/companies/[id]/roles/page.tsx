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

interface Role {
  id: string;
  name: string;
  description: string;
  permissions_count: number;
  assigned_users: number;
  created_at: string;
}

export default function RolesPage() {
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
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRoles(data.roles || []);
      }
    } catch (error) {
      console.error('Failed to fetch roles:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredRoles = roles.filter(r =>
    r.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Roles</Header>}>
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
          description="Manage user roles and permissions"
          actions={
            <Button variant="primary" disabled>
              + Create Role
            </Button>
          }
        >
          Roles
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Name',
              cell: (item: Role) => item.name,
              width: '20%',
            },
            {
              header: 'Description',
              cell: (item: Role) => item.description,
              width: '35%',
            },
            {
              header: 'Permissions',
              cell: (item: Role) => item.permissions_count,
              width: '15%',
            },
            {
              header: 'Assigned Users',
              cell: (item: Role) => item.assigned_users,
              width: '15%',
            },
            {
              header: 'Created',
              cell: (item: Role) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredRoles}
          header={<Header variant="h2" counter={`(${filteredRoles.length})`}>Roles</Header>}
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
