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
  TextFilter,
  Select,
  ColumnLayout,
  Container,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface User {
  id: string;
  domain_id: string;
  username: string;
  display_name: string;
  role: string;
  status: string;
  password_configured: boolean;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

const STATUS_COLORS: Record<string, 'green' | 'red' | 'grey' | 'blue'> = {
  active: 'green',
  suspended: 'red',
  inactive: 'grey',
};

export default function UsersPage() {
  const { t } = useI18n();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
  const [creating, setCreating] = useState(false);

  const [editUser, setEditUser] = useState<User | null>(null);
  const [editForm, setEditForm] = useState({ display_name: '', quota_gb: '0' });
  const [saving, setSaving] = useState(false);

  const [togglingId, setTogglingId] = useState<string | null>(null);

  useEffect(() => { fetchUsers(); }, []);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/users?limit=500', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      }
    } catch (e) {
      console.error('Failed to fetch users:', e);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async () => {
    if (!newUser.username.trim() || !newUser.domain_id.trim()) return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: newUser.username,
          display_name: newUser.display_name,
          domain_id: newUser.domain_id,
          password: newUser.password,
          quota_limit: parseInt(newUser.quota_gb) * 1073741824,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewUser({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
        fetchUsers();
      }
    } catch (e) {
      console.error('Failed to create user:', e);
    } finally {
      setCreating(false);
    }
  };

  const handleEditSave = async () => {
    if (!editUser) return;
    setSaving(true);
    try {
      await fetch(`/api/admin/users/${editUser.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          display_name: editForm.display_name,
          quota_limit: parseInt(editForm.quota_gb) * 1073741824,
        }),
        credentials: 'include',
      });
      setEditUser(null);
      fetchUsers();
    } catch (e) {
      console.error('Failed to update user:', e);
    } finally {
      setSaving(false);
    }
  };

  const handleToggleStatus = async (user: User) => {
    setTogglingId(user.id);
    const nextStatus = user.status === 'active' ? 'suspended' : 'active';
    try {
      await fetch(`/api/admin/users/${user.id}/status`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: nextStatus }),
        credentials: 'include',
      });
      fetchUsers();
    } catch (e) {
      console.error('Failed to toggle status:', e);
    } finally {
      setTogglingId(null);
    }
  };

  const openEdit = (user: User) => {
    setEditUser(user);
    setEditForm({
      display_name: user.display_name,
      quota_gb: user.quota_limit > 0 ? String(Math.round(user.quota_limit / 1073741824)) : '0',
    });
  };

  const statusOptions = [
    { label: t('pages.users_page.all_statuses'), value: '' },
    { label: t('pages.users_page.active'), value: 'active' },
    { label: t('pages.users_page.suspended'), value: 'suspended' },
    { label: t('pages.users_page.inactive'), value: 'inactive' },
  ];

  const filteredUsers = useMemo(() => {
    return users.filter(u => {
      const matchesText = !filter || u.username.toLowerCase().includes(filter.toLowerCase())
        || (u.display_name || '').toLowerCase().includes(filter.toLowerCase());
      const matchesStatus = !statusFilter || u.status === statusFilter;
      return matchesText && matchesStatus;
    });
  }, [users, filter, statusFilter]);

  const totalUsers = users.length;
  const activeUsers = users.filter(u => u.status === 'active').length;
  const suspendedUsers = users.filter(u => u.status === 'suspended' || u.status === 'inactive').length;

  const formatStorage = (used: number, limit: number) => {
    const usedGb = (used / 1073741824).toFixed(1);
    if (!limit) return `${usedGb} GB`;
    const limitGb = (limit / 1073741824).toFixed(1);
    const pct = Math.round((used / limit) * 100);
    return `${usedGb} / ${limitGb} GB (${pct}%)`;
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.users_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          counter={`(${totalUsers})`}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.users_page.create_user_btn')}
            </Button>
          }
        >
          {t('pages.users_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* KPI Summary */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={140}>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{totalUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.users_page.total_label')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{activeUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.users_page.active_label')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{suspendedUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.users_page.suspended_label')}</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* User Table */}
        <Table
          columnDefinitions={[
            {
              header: t('pages.users_page.username'),
              cell: (u: User) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{u.username}</Box>
                  {u.display_name && (
                    <Box color="text-body-secondary" fontSize="body-s">{u.display_name}</Box>
                  )}
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: t('pages.users_page.domain'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">{u.domain_id || '—'}</Box>
              ),
              width: '18%',
            },
            {
              header: t('pages.users_page.role'),
              cell: (u: User) => u.role ? (
                <Badge color="blue">{u.role}</Badge>
              ) : <Box color="text-body-secondary">—</Box>,
              width: '10%',
            },
            {
              header: t('pages.users_page.status'),
              cell: (u: User) => (
                <Badge color={STATUS_COLORS[u.status] ?? 'grey'}>{u.status}</Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.users_page.storage'),
              cell: (u: User) => (
                <Box fontSize="body-s" color={
                  u.quota_limit > 0 && u.quota_used / u.quota_limit > 0.8
                    ? 'text-status-error'
                    : 'text-body-secondary'
                }>
                  {formatStorage(u.quota_used ?? 0, u.quota_limit ?? 0)}
                </Box>
              ),
              width: '20%',
            },
            {
              header: t('pages.users_page.created'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {new Date(u.created_at).toLocaleDateString()}
                </Box>
              ),
              width: '10%',
            },
            {
              header: t('pages.users_page.actions'),
              cell: (u: User) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => openEdit(u)}>
                    {t('pages.users_page.edit')}
                  </Button>
                  <Button
                    variant="inline-link"
                    onClick={() => handleToggleStatus(u)}
                    loading={togglingId === u.id}
                  >
                    {u.status === 'active'
                      ? t('pages.users_page.toggle_suspend')
                      : t('pages.users_page.toggle_activate')}
                  </Button>
                </SpaceBetween>
              ),
              width: '10%',
            },
          ]}
          items={filteredUsers}
          header={
            <Header variant="h2" counter={`(${filteredUsers.length})`}>
              {t('pages.users_page.user_list')}
            </Header>
          }
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('pages.users_page.search_placeholder')}
                onChange={(e) => setFilter(e.detail.filteringText)}
              />
              <Select
                selectedOption={statusOptions.find(o => o.value === statusFilter) ?? statusOptions[0]}
                options={statusOptions}
                onChange={(e) => setStatusFilter(e.detail.selectedOption.value ?? '')}
                expandToViewport
              />
            </SpaceBetween>
          }
          empty={
            <Box textAlign="center" padding="l">
              <StatusIndicator type="info">{t('pages.users_page.no_users')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Create Modal */}
      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        size="medium"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreateUser}
                loading={creating}
                disabled={!newUser.username.trim() || !newUser.domain_id.trim()}
              >
                {t('pages.users_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.users_page.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.users_page.domain_label')}>
            <Input
              value={newUser.domain_id}
              onChange={(e) => setNewUser({ ...newUser, domain_id: e.detail.value })}
              placeholder="domain-id"
            />
          </FormField>
          <FormField label={t('pages.users_page.username_label')}>
            <Input
              value={newUser.username}
              onChange={(e) => setNewUser({ ...newUser, username: e.detail.value })}
              placeholder="john.doe"
            />
          </FormField>
          <FormField label={t('pages.users_page.display_name_label')}>
            <Input
              value={newUser.display_name}
              onChange={(e) => setNewUser({ ...newUser, display_name: e.detail.value })}
              placeholder="John Doe"
            />
          </FormField>
          <FormField label={t('pages.users_page.password_label')}>
            <Input
              type="password"
              value={newUser.password}
              onChange={(e) => setNewUser({ ...newUser, password: e.detail.value })}
            />
          </FormField>
          <FormField label={t('pages.users_page.quota_label')}>
            <Input
              type="number"
              value={newUser.quota_gb}
              onChange={(e) => setNewUser({ ...newUser, quota_gb: e.detail.value })}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Edit Modal */}
      <Modal
        onDismiss={() => setEditUser(null)}
        visible={!!editUser}
        size="medium"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setEditUser(null)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleEditSave} loading={saving}>
                {t('pages.users_page.save_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={`${t('pages.users_page.edit_modal_title')} — ${editUser?.username ?? ''}`}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.users_page.display_name_label')}>
            <Input
              value={editForm.display_name}
              onChange={(e) => setEditForm({ ...editForm, display_name: e.detail.value })}
            />
          </FormField>
          <FormField label={t('pages.users_page.quota_label')}>
            <Input
              type="number"
              value={editForm.quota_gb}
              onChange={(e) => setEditForm({ ...editForm, quota_gb: e.detail.value })}
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
