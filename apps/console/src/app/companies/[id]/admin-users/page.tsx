'use client';
import { DataTable } from '@/components/DataTable';
import { ConfirmModal } from '@/components/ConfirmModal';


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
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
}

const ROLE_LABEL_KEYS: Record<string, string> = {
  system_admin: 'pages.admin_users_page.system_admin',
  company_admin: 'pages.admin_users_page.admin',
  admin: 'pages.admin_users_page.admin',
  readonly: 'pages.admin_users_page.read_only',
};

export default function AdminUsersPage() {
  const { t } = useI18n();
  const { isSystemAdmin, currentUserId } = useCompany();
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newAdmin, setNewAdmin] = useState({ username: '', email: '', password: '', role: 'admin' });
  const [createError, setCreateError] = useState('');
  const [creating, setCreating] = useState(false);

  // Delete confirmation + failure feedback.
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [actionError, setActionError] = useState('');

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
        setFetchError(t('pages.admin_users_page.load_failed', 'Failed to load data. Please try again in a moment.'));
      }
    } catch {
      setFetchError(t('pages.admin_users_page.load_failed', 'Failed to load data. Please try again in a moment.'));
    } finally {
      setLoading(false);
    }
  };

  const extractError = async (res: Response, fallback: string): Promise<string> => {
    const data = (await res.json().catch(() => ({}))) as { error?: { message?: string } | string };
    const msg = typeof data.error === 'string' ? data.error : data.error?.message;
    return msg || fallback;
  };

  const handleCreateAdmin = async () => {
    setCreateError('');
    setCreating(true);
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
      } else {
        // Keep the modal open so the admin can correct the input and retry.
        setCreateError(await extractError(res, t('pages.admin_users_page.create_failed', 'Failed to create admin user.')));
      }
    } catch {
      setCreateError(t('pages.admin_users_page.create_failed', 'Failed to create admin user.'));
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteAdmin = async (user: AdminUser) => {
    setDeleteError('');
    setDeleting(true);
    try {
      const res = await fetch(`/api/admin/admin-users/${user.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        setDeleteTarget(null);
        fetchAdminUsers();
      } else {
        setDeleteError(await extractError(res, t('pages.admin_users_page.delete_failed', 'Failed to remove admin user.')));
      }
    } catch {
      setDeleteError(t('pages.admin_users_page.delete_failed', 'Failed to remove admin user.'));
    } finally {
      setDeleting(false);
    }
  };

  const roleLabel = (role: string) => t(ROLE_LABEL_KEYS[role] ?? '', role);

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
            isSystemAdmin ? (
              <Button variant="primary" onClick={() => { setCreateError(''); setShowModal(true); }}>
                {t('pages.admin_users_page.add_admin_btn')}
              </Button>
            ) : undefined
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
        {actionError && (
          <Flashbar items={[{ type: 'error', content: actionError, id: 'action-error', dismissible: true, onDismiss: () => setActionError('') }]} />
        )}
        {!isSystemAdmin && (
          <Alert type="info">{t('pages.admin_users_page.read_only_notice', 'Only a system administrator can add or remove admin users.')}</Alert>
        )}
        <DataTable
          columnDefinitions={[
            { header: t('pages.admin_users_page.username'), cell: (u: AdminUser) => u.username, width: '20%' },
            { header: t('pages.admin_users_page.email'), cell: (u: AdminUser) => u.email, width: '30%' },
            {
              header: t('pages.admin_users_page.role'),
              cell: (u: AdminUser) => (
                <Badge color={u.role === 'system_admin' ? 'blue' : 'grey'}>
                  {roleLabel(u.role)}
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
              cell: (u: AdminUser) => {
                if (!isSystemAdmin) return null;
                // Self-delete guard: the backend does not stop an admin from
                // removing their own admin role, so block it in the UI.
                const isSelf = !!currentUserId && u.id === currentUserId;
                return (
                  <Button
                    variant="inline-link"
                    disabled={isSelf}
                    onClick={() => { setDeleteError(''); setDeleteTarget(u); }}
                  >
                    {t('pages.admin_users_page.remove')}
                  </Button>
                );
              },
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
                <Button onClick={() => setShowModal(false)} disabled={creating}>{t('common.cancel')}</Button>
                <Button variant="primary" onClick={handleCreateAdmin} loading={creating}>
                  {t('pages.admin_users_page.create_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          }
          header={t('pages.admin_users_page.add_admin_modal')}
        >
          <SpaceBetween size="m">
            {createError && <Alert type="error">{createError}</Alert>}
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
                selectedOption={{ label: roleLabel(newAdmin.role), value: newAdmin.role }}
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

        <ConfirmModal
          visible={!!deleteTarget}
          header={t('pages.admin_users_page.remove_modal_title', 'Remove admin user')}
          onConfirm={() => deleteTarget && handleDeleteAdmin(deleteTarget)}
          onDismiss={() => { setDeleteTarget(null); setDeleteError(''); }}
          loading={deleting}
          confirmLabel={t('pages.admin_users_page.remove')}
          error={deleteError || undefined}
        >
          {t('pages.admin_users_page.remove_confirm', 'Remove admin access for')}{' '}
          <strong>{deleteTarget?.email || deleteTarget?.username}</strong>?
        </ConfirmModal>
      </SpaceBetween>
    </ContentLayout>
  );
}
