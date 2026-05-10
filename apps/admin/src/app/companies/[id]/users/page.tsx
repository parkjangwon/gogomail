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
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface User {
  id: string;
  username: string;
  email: string;
  status: string;
  created_at: string;
}

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', email: '', password: '' });

  useEffect(() => {
    fetchUsers();
  }, []);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/users', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      }
    } catch (error) {
      console.error('Failed to fetch users:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async () => {
    try {
      const res = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newUser),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewUser({ username: '', email: '', password: '' });
        fetchUsers();
      }
    } catch (error) {
      console.error('Failed to create user:', error);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Users</Header>}>
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
              + Create User
            </Button>
          }
        >
          Users
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Username',
              cell: (user: User) => user.username,
              width: '20%',
            },
            {
              header: 'Email',
              cell: (user: User) => user.email,
              width: '30%',
            },
            {
              header: 'Status',
              cell: (user: User) => (
                <Badge color={user.status === 'active' ? 'green' : 'grey'}>
                  {user.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Created',
              cell: (user: User) => new Date(user.created_at).toLocaleDateString(),
              width: '20%',
            },
            {
              header: 'Actions',
              cell: () => (
                <Button variant="inline-link">Edit</Button>
              ),
              width: '15%',
            },
          ]}
          items={users}
          header={<Header variant="h2" counter={`(${users.length})`}>User List</Header>}
        />

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>Cancel</Button>
                <Button variant="primary" onClick={handleCreateUser}>
                  Create
                </Button>
              </SpaceBetween>
            </Box>
          }
          header="Create New User"
        >
          <SpaceBetween size="m">
            <FormField label="Username">
              <Input
                value={newUser.username}
                onChange={(e) => setNewUser({ ...newUser, username: e.detail.value })}
              />
            </FormField>
            <FormField label="Email">
              <Input
                type="email"
                value={newUser.email}
                onChange={(e) => setNewUser({ ...newUser, email: e.detail.value })}
              />
            </FormField>
            <FormField label="Password">
              <Input
                type="password"
                value={newUser.password}
                onChange={(e) => setNewUser({ ...newUser, password: e.detail.value })}
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
