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
import { useI18n } from '@/app/i18n-provider';

interface User {
  id: string;
  username: string;
  email: string;
  status: string;
  created_at: string;
}

export default function UsersPage() {
  const { t } = useI18n();
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
      <ContentLayout header={<Header variant="h1">{t('pages.users_page.title')}</Header>}>
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
              {t('pages.users_page.create_user_btn')}
            </Button>
          }
        >
          {t('pages.users_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.users_page.username'),
              cell: (user: User) => user.username,
              width: '20%',
            },
            {
              header: t('pages.users_page.email'),
              cell: (user: User) => user.email,
              width: '30%',
            },
            {
              header: t('pages.users_page.status'),
              cell: (user: User) => (
                <Badge color={user.status === 'active' ? 'green' : 'grey'}>
                  {user.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.users_page.created'),
              cell: (user: User) => new Date(user.created_at).toLocaleDateString(),
              width: '20%',
            },
            {
              header: t('pages.users_page.actions'),
              cell: () => (
                <Button variant="inline-link">{t('pages.users_page.edit')}</Button>
              ),
              width: '15%',
            },
          ]}
          items={users}
          header={<Header variant="h2" counter={`(${users.length})`}>{t('pages.users_page.user_list')}</Header>}
        />

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" onClick={handleCreateUser}>
                  {t('pages.users_page.create_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          }
          header={t('pages.users_page.create_modal_title')}
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.users_page.username_label')}>
              <Input
                value={newUser.username}
                onChange={(e) => setNewUser({ ...newUser, username: e.detail.value })}
              />
            </FormField>
            <FormField label={t('pages.users_page.email_label')}>
              <Input
                type="email"
                value={newUser.email}
                onChange={(e) => setNewUser({ ...newUser, email: e.detail.value })}
              />
            </FormField>
            <FormField label={t('pages.users_page.password_label')}>
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
