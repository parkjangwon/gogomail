'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  Modal,
  FormField,
  Input,
  Select,
  Flashbar,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
}

export default function AdminUsersPage() {
  const { t } = useI18n();
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newAdmin, setNewAdmin] = useState({ username: '', email: '', password: '', role: 'admin' });

  useEffect(() => {
    fetchAdminUsers();
  }, []);

  const fetchAdminUsers = async () => {
    setLoading(true);
    setFetchError('');
    try {
      const res = await fetch('/api/admin/admin-users', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      } else {
        setFetchError('데이터를 불러오지 못했습니다. 잠시 후 다시 시도해주세요.');
      }
    } catch (error) {
      setFetchError('데이터를 불러오지 못했습니다. 잠시 후 다시 시도해주세요.');
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
    } catch {
      // mutation error handled by caller
    }
  };

  const handleDeleteAdmin = async (userId: string) => {
    try {
      await fetch(`/api/admin/admin-users/${userId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchAdminUsers();
    } catch {
      // mutation error handled by caller
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.admin_users_page.title')}</Header>}>
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
              {t('pages.admin_users_page.add_admin_btn')}
            </Button>
          }
        >
          {t('pages.admin_users_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {fetchError && (
          <Flashbar items={[{ type: 'error', content: fetchError, id: 'fetch-error', dismissible: true, onDismiss: () => setFetchError('') }]} />
        )}
        <DataTable
          columnDefinitions={[
            { header: t('pages.admin_users_page.username'), cell: (u: AdminUser) => u.username, width: '20%' },
            { header: t('pages.admin_users_page.email'), cell: (u: AdminUser) => u.email, width: '30%' },
            {
              header: t('pages.admin_users_page.role'),
              cell: (u: AdminUser) => (
                <Badge color={u.role === 'system_admin' ? 'blue' : 'grey'}>
                  {u.role}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.admin_users_page.status'),
              cell: (u: AdminUser) => (
                <Badge color={u.status === 'active' ? 'green' : 'red'}>
                  {u.status}
                </Badge>
              ),
              width: '15%',
            },
            { header: t('pages.admin_users_page.created'), cell: (u: AdminUser) => new Date(u.created_at).toLocaleDateString(), width: '15%' },
            {
              header: t('pages.admin_users_page.actions'),
              cell: (u: AdminUser) => (
                <Button variant="inline-link" onClick={() => handleDeleteAdmin(u.id)}>
                  {t('pages.admin_users_page.remove')}
                </Button>
              ),
              width: '10%',
            },
          ]}
          items={users}
          header={<Header variant="h2">{t('pages.admin_users_page.admin_accounts')}</Header>}
        />

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" onClick={handleCreateAdmin}>
                  {t('pages.admin_users_page.create_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          }
          header={t('pages.admin_users_page.add_admin_modal')}
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.admin_users_page.username_label')}>
              <Input
                value={newAdmin.username}
                onChange={(e) => setNewAdmin({ ...newAdmin, username: e.detail.value })}
              />
            </FormField>
            <FormField label={t('pages.admin_users_page.email_label')}>
              <Input
                type="email"
                value={newAdmin.email}
                onChange={(e) => setNewAdmin({ ...newAdmin, email: e.detail.value })}
              />
            </FormField>
            <FormField label={t('pages.admin_users_page.password_label')}>
              <Input
                type="password"
                value={newAdmin.password}
                onChange={(e) => setNewAdmin({ ...newAdmin, password: e.detail.value })}
              />
            </FormField>
            <FormField label={t('pages.admin_users_page.role_label')}>
              <Select
                selectedOption={{ label: newAdmin.role, value: newAdmin.role }}
                options={[
                  { label: t('pages.admin_users_page.system_admin'), value: 'system_admin' },
                  { label: t('pages.admin_users_page.admin'), value: 'admin' },
                  { label: t('pages.admin_users_page.read_only'), value: 'readonly' },
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
