'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  Modal,
  FormField,
  Input,
  Select,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [newAdmin, setNewAdmin] = useState({ username: '', email: '', password: '', role: 'admin' });

  useEffect(() => {
    fetchAdminUsers();
  }, []);

  const fetchAdminUsers = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/admin-users', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      }
    } catch (error) {
      console.error('Failed to fetch admin users:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateAdmin = async () => {
    try {
      const res = await fetch('/api/admin/admin-users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newAdmin),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewAdmin({ username: '', email: '', password: '', role: 'admin' });
        fetchAdminUsers();
      }
    } catch (error) {
      console.error('Failed to create admin user:', error);
    }
  };

  const handleDeleteAdmin = async (userId: string) => {
    try {
      await fetch(`/api/admin/admin-users/${userId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchAdminUsers();
    } catch (error) {
      console.error('Failed to delete admin user:', error);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Admin Users</Header>}>
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
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              + Add Admin
            </Button>
          }
        >
          Admin Users
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            { header: 'Username', cell: (u: AdminUser) => u.username, width: '20%' },
            { header: 'Email', cell: (u: AdminUser) => u.email, width: '30%' },
            {
              header: 'Role',
              cell: (u: AdminUser) => (
                <Badge color={u.role === 'system_admin' ? 'blue' : 'grey'}>
                  {u.role}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Status',
              cell: (u: AdminUser) => (
                <Badge color={u.status === 'active' ? 'green' : 'red'}>
                  {u.status}
                </Badge>
              ),
              width: '15%',
            },
            { header: 'Created', cell: (u: AdminUser) => new Date(u.created_at).toLocaleDateString(), width: '15%' },
            {
              header: 'Actions',
              cell: (u: AdminUser) => (
                <Button variant="inline-link" onClick={() => handleDeleteAdmin(u.id)}>
                  Remove
                </Button>
              ),
              width: '10%',
            },
          ]}
          items={users}
          header={<Header variant="h2">Administrator Accounts</Header>}
        />

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>Cancel</Button>
                <Button variant="primary" onClick={handleCreateAdmin}>
                  Create
                </Button>
              </SpaceBetween>
            </Box>
          }
          header="Add Administrator"
        >
          <SpaceBetween size="m">
            <FormField label="Username">
              <Input
                value={newAdmin.username}
                onChange={(e) => setNewAdmin({ ...newAdmin, username: e.detail.value })}
              />
            </FormField>
            <FormField label="Email">
              <Input
                type="email"
                value={newAdmin.email}
                onChange={(e) => setNewAdmin({ ...newAdmin, email: e.detail.value })}
              />
            </FormField>
            <FormField label="Password">
              <Input
                type="password"
                value={newAdmin.password}
                onChange={(e) => setNewAdmin({ ...newAdmin, password: e.detail.value })}
              />
            </FormField>
            <FormField label="Role">
              <Select
                selectedOption={{ label: newAdmin.role, value: newAdmin.role }}
                options={[
                  { label: 'System Admin', value: 'system_admin' },
                  { label: 'Admin', value: 'admin' },
                  { label: 'Read-Only', value: 'readonly' },
                ]}
                onChange={(e) => setNewAdmin({ ...newAdmin, role: e.detail.selectedOption?.value || 'admin' })}
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
