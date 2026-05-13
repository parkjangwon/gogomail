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
  Alert,
  CopyToClipboard,
  Flashbar,
  Pagination,
  ButtonDropdown,
} from '@cloudscape-design/components';
import { useState, useEffect, useMemo, useRef } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface User {
  id: string;
  domain_id: string;
  username: string;
  display_name: string;
  role: string;
  status: UserStatus;
  password_configured: boolean;
  must_change_password: boolean;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

interface Domain {
  id: string;
  name: string;
  status: string;
}

type UserStatus = 'active' | 'suspended' | 'disabled';

const STATUS_COLORS: Record<UserStatus, 'green' | 'red' | 'grey' | 'blue'> = {
  active: 'green',
  suspended: 'red',
  disabled: 'red',
};

function normalizeUserStatus(rawStatus: unknown): UserStatus {
  switch (String(rawStatus).trim().toLowerCase()) {
    case 'active':
      return 'active';
    case 'suspended':
      return 'suspended';
    case 'disabled':
      return 'disabled';
    default:
      return 'disabled';
  }
}

export default function UsersPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [users, setUsers] = useState<User[]>([]);
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');

  // Bulk import/export state
  const [showImportModal, setShowImportModal] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<{ total: number; success: number; failed: number; failures: Array<{ email: string; error: string }> } | null>(null);
  const [flashItems, setFlashItems] = useState<Array<{ type: 'success' | 'error' | 'info'; content: string; id: string; dismissible: boolean; onDismiss: () => void }>>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
  const [registrationMode, setRegistrationMode] = useState<'temp_password' | 'email_invite'>('temp_password');
  const [loadingDomainSettings, setLoadingDomainSettings] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [inviteLink, setInviteLink] = useState('');

  const [editUser, setEditUser] = useState<User | null>(null);
  const [editForm, setEditForm] = useState({ display_name: '', quota_gb: '0', role: 'user' });
  const [saving, setSaving] = useState(false);

  const [togglingId, setTogglingId] = useState<string | null>(null);

  // Offboarding modal
  const [offboardTarget, setOffboardTarget] = useState<User | null>(null);
  const [offboarding, setOffboarding] = useState(false);

  // Bulk selection
  const [selectedUsers, setSelectedUsers] = useState<User[]>([]);
  const [bulkLoading, setBulkLoading] = useState(false);

  // Pagination
  const PAGE_SIZE = 25;
  const [currentPage, setCurrentPage] = useState(1);

  useEffect(() => {
    fetchUsers();
    fetchDomains();
  }, []);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const domainRes = await fetch(`/api/admin/domains?company_id=${encodeURIComponent(companyId)}&limit=200`, { credentials: 'include' });
      if (!domainRes.ok) return;
      const domainData = await domainRes.json();
      const companyDomains: Domain[] = domainData.domains || [];
      const userLists = await Promise.all(
        companyDomains.map((domain) =>
          fetch(`/api/admin/users?domain_id=${encodeURIComponent(domain.id)}&limit=200`, { credentials: 'include' })
            .then((res) => res.ok ? res.json() : { users: [] })
        )
      );
      setUsers(userLists.flatMap((data: { users?: User[] }) => data.users || []).map((u: User) => ({
          ...u,
          status: normalizeUserStatus(u.status),
        })));
    } catch (e) {
      console.error('Failed to fetch users:', e);
    } finally {
      setLoading(false);
    }
  };

  const fetchDomains = async () => {
    try {
      const res = await fetch(`/api/admin/domains?company_id=${encodeURIComponent(companyId)}&limit=200`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setDomains(data.domains || []);
      }
    } catch (e) {
      console.error('Failed to fetch domains:', e);
    }
  };

  const handleDomainChange = async (domainId: string) => {
    setNewUser(u => ({ ...u, domain_id: domainId }));
    if (!domainId) return;
    setLoadingDomainSettings(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/settings`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setRegistrationMode(data.settings?.user_registration_mode ?? 'temp_password');
      }
    } catch (e) {
      console.error('Failed to fetch domain settings:', e);
    } finally {
      setLoadingDomainSettings(false);
    }
  };

  const selectedDomain = domains.find(d => d.id === newUser.domain_id);
  const autoAddress = selectedDomain && newUser.username.trim()
    ? `${newUser.username.trim().toLowerCase()}@${selectedDomain.name}`
    : '';

  const handleCreateUser = async () => {
    if (!newUser.username.trim() || !newUser.domain_id.trim()) return;
    if (!autoAddress) return;
    setCreating(true);
    setCreateError('');
    setInviteLink('');
    try {
      const body: Record<string, unknown> = {
        username: newUser.username.trim(),
        display_name: newUser.display_name.trim() || newUser.username.trim(),
        domain_id: newUser.domain_id,
        address: autoAddress,
        quota_limit: parseInt(newUser.quota_gb) * 1073741824,
      };

      if (registrationMode === 'temp_password') {
        if (!newUser.password.trim()) {
          setCreateError(t('pages.users_page.password_required'));
          setCreating(false);
          return;
        }
        body.password = newUser.password;
      }

      const res = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        credentials: 'include',
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        setCreateError(err.error || t('pages.users_page.create_failed'));
        return;
      }

      const created = await res.json();
      const userId = created.user?.id;

      if (registrationMode === 'email_invite' && userId) {
        const invRes = await fetch(`/api/admin/users/${userId}/invite`, {
          method: 'POST',
          credentials: 'include',
        });
        if (invRes.ok) {
          const invData = await invRes.json();
          const token = invData.invite_token?.token;
          if (token) {
            const url = `${window.location.origin}/invite/${token}`;
            setInviteLink(url);
          }
        }
      }

      if (registrationMode === 'temp_password') {
        setShowCreateModal(false);
        setNewUser({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
      }
      fetchUsers();
    } catch (e) {
      console.error('Failed to create user:', e);
      setCreateError(t('pages.users_page.create_failed'));
    } finally {
      setCreating(false);
    }
  };

  const handleEditSave = async () => {
    if (!editUser) return;
    setSaving(true);
    try {
      await fetch(`/api/admin/users/${editUser.id}/quota`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quota_limit: parseInt(editForm.quota_gb) * 1073741824 }),
        credentials: 'include',
      });
      if (editForm.role !== editUser.role) {
        await fetch(`/api/admin/users/${editUser.id}/role`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ role: editForm.role }),
          credentials: 'include',
        });
      }
      setEditUser(null);
      fetchUsers();
    } catch (e) {
      console.error('Failed to update user:', e);
    } finally {
      setSaving(false);
    }
  };

  const handleToggleStatus = async (user: User) => {
    if (user.status === 'active') {
      setOffboardTarget(user);
      return;
    }
    setTogglingId(user.id);
    try {
      await fetch(`/api/admin/users/${user.id}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: 'active' }),
        credentials: 'include',
      });
      fetchUsers();
    } catch (e) {
      console.error('Failed to activate user:', e);
    } finally {
      setTogglingId(null);
    }
  };

  const handleOffboard = async () => {
    if (!offboardTarget) return;
    setOffboarding(true);
    try {
      const res = await fetch(`/api/admin/users/${offboardTarget.id}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: 'suspended' }),
        credentials: 'include',
      });
      if (!res.ok) {
        addFlash('error', t('pages.users_page.suspend_failed'));
        return;
      }
      setOffboardTarget(null);
      fetchUsers();
    } catch {
      addFlash('error', t('pages.users_page.suspend_failed'));
    } finally {
      setOffboarding(false);
    }
  };

  const openEdit = (user: User) => {
    setEditUser(user);
    setEditForm({
      display_name: user.display_name,
      quota_gb: user.quota_limit > 0 ? String(Math.round(user.quota_limit / 1073741824)) : '0',
      role: user.role || 'user',
    });
  };

  const ROLE_OPTIONS = [
    { label: t('pages.users_page.role_user_email'), value: 'user' },
    { label: t('pages.users_page.role_company_admin'), value: 'company_admin' },
    { label: t('pages.users_page.role_system_admin'), value: 'system_admin' },
  ];

  const addFlash = (type: 'success' | 'error' | 'info', content: string) => {
    const id = Date.now().toString();
    setFlashItems(prev => [...prev, { type, content, id, dismissible: true, onDismiss: () => setFlashItems(f => f.filter(i => i.id !== id)) }]);
  };

  const handleBulkAction = async (action: 'activate' | 'suspend') => {
    if (selectedUsers.length === 0) return;
    setBulkLoading(true);
    try {
      const res = await fetch('/api/admin/users/bulk', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: selectedUsers.map(u => u.id), action }),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        const succeeded: number = data.succeeded?.length ?? 0;
        const failed: number = data.failed?.length ?? 0;
        if (failed === 0) {
          addFlash('success', t('pages.users_page.bulk_updated')
            .replace('{action}', t(`pages.users_page.bulk_${action}`))
            .replace('{succeeded}', String(succeeded)));
        } else {
          addFlash('error', t('pages.users_page.bulk_partial')
            .replace('{action}', t(`pages.users_page.bulk_${action}`))
            .replace('{succeeded}', String(succeeded))
            .replace('{failed}', String(failed)));
        }
      } else {
        addFlash('error', t('pages.users_page.bulk_failed').replace('{action}', t(`pages.users_page.bulk_${action}`)));
      }
      setSelectedUsers([]);
      fetchUsers();
    } catch {
      addFlash('error', t('pages.users_page.bulk_failed').replace('{action}', t(`pages.users_page.bulk_${action}`)));
    } finally {
      setBulkLoading(false);
    }
  };

  const handleExportCSV = async () => {
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/users/bulk-export`, { credentials: 'include' });
      if (!res.ok) {
        addFlash('error', t('pages.users_page.export_failed'));
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'users-export.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      addFlash('error', t('pages.users_page.export_failed'));
    }
  };

  const handleImportCSV = async (file: File) => {
    setImporting(true);
    setImportResult(null);
    try {
      const text = await file.text();
      const lines = text.trim().split('\n').filter(Boolean);
      const usersToImport = lines.map(line => {
        const cols = line.split(',');
        return {
          email: (cols[0] ?? '').trim(),
          display_name: (cols[1] ?? '').trim(),
          domain_id: (cols[2] ?? '').trim(),
          password: (cols[3] ?? '').trim(),
        };
      }).filter(u => u.email);

      const res = await fetch(`/api/admin/companies/${companyId}/users/bulk-import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ users: usersToImport }),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setImportResult(data);
        if (data.success > 0) {
          fetchUsers();
        }
      } else {
        addFlash('error', t('pages.users_page.import_failed'));
      }
    } catch {
      addFlash('error', t('pages.users_page.import_failed'));
    } finally {
      setImporting(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const statusOptions = [
    { label: t('pages.users_page.all_statuses'), value: '' },
    { label: t('pages.users_page.active'), value: 'active' },
    { label: t('pages.users_page.suspended'), value: 'suspended' },
    { label: t('pages.users_page.disabled'), value: 'disabled' },
  ];

  const domainOptions = domains.map(d => ({
    label: d.name,
    value: d.id,
    description: d.status,
  }));

  const filteredUsers = useMemo(() => {
    return users.filter(u => {
      const matchesText = !filter || u.username.toLowerCase().includes(filter.toLowerCase())
        || (u.display_name || '').toLowerCase().includes(filter.toLowerCase());
      const matchesStatus = !statusFilter || u.status === statusFilter;
      return matchesText && matchesStatus;
    });
  }, [users, filter, statusFilter]);

  const pageCount = Math.max(1, Math.ceil(filteredUsers.length / PAGE_SIZE));
  const pagedUsers = filteredUsers.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  const totalUsers = users.length;
  const activeUsers = users.filter(u => u.status === 'active').length;
  const suspendedUsers = users.filter(u => u.status === 'suspended' || u.status === 'disabled').length;

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
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={handleExportCSV}>
                {t('pages.users_page.users_bulk.export_btn')}
              </Button>
              <Button onClick={() => { setShowImportModal(true); setImportResult(null); }}>
                {t('pages.users_page.users_bulk.import_btn')}
              </Button>
              <Button variant="primary" onClick={() => {
                setShowCreateModal(true);
                setCreateError('');
                setInviteLink('');
              }}>
                {t('pages.users_page.create_user_btn')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.users_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flashItems.length > 0 && <Flashbar items={flashItems} />}

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
          selectionType="multi"
          selectedItems={selectedUsers}
          onSelectionChange={e => setSelectedUsers(e.detail.selectedItems)}
          trackBy="id"
          columnDefinitions={[
            {
              header: t('pages.users_page.username'),
              cell: (u: User) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{u.username}</Box>
                  {u.display_name && (
                    <Box color="text-body-secondary" fontSize="body-s">{u.display_name}</Box>
                  )}
                  {u.must_change_password && (
                    <Badge color="severity-medium">{t('pages.users_page.must_change_pw')}</Badge>
                  )}
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: t('pages.users_page.domain'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {domains.find(d => d.id === u.domain_id)?.name ?? u.domain_id.slice(0, 8) + '…'}
                </Box>
              ),
              width: '18%',
            },
            {
              header: t('pages.users_page.role'),
              cell: (u: User) => {
                const roleColor = u.role === 'system_admin' ? 'red' : u.role === 'company_admin' ? 'green' : 'grey';
                return u.role ? <Badge color={roleColor}>{u.role}</Badge> : <Box color="text-body-secondary">—</Box>;
              },
              width: '12%',
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
          items={pagedUsers}
          header={
            <Header
              variant="h2"
              counter={selectedUsers.length > 0 ? `(${selectedUsers.length}/${filteredUsers.length})` : `(${filteredUsers.length})`}
              actions={
                selectedUsers.length > 0 ? (
                  <SpaceBetween direction="horizontal" size="xs">
                    <Box color="text-status-inactive" padding={{ top: 'xs' }}>
                      {t('pages.users_page.selected_count').replace('{n}', String(selectedUsers.length))}
                    </Box>
                    <ButtonDropdown
                      loading={bulkLoading}
                      items={[
                        { id: 'activate', text: t('pages.users_page.activate_selected') },
                        { id: 'suspend', text: t('pages.users_page.suspend_selected') },
                      ]}
                      onItemClick={({ detail }) => handleBulkAction(detail.id as 'activate' | 'suspend')}
                    >
                      {t('pages.users_page.bulk_actions')}
                    </ButtonDropdown>
                  </SpaceBetween>
                ) : undefined
              }
            >
              {t('pages.users_page.user_list')}
            </Header>
          }
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('pages.users_page.search_placeholder')}
                onChange={(e) => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
              />
              <Select
                selectedOption={statusOptions.find(o => o.value === statusFilter) ?? statusOptions[0]}
                options={statusOptions}
                onChange={(e) => { setStatusFilter(e.detail.selectedOption.value ?? ''); setCurrentPage(1); }}
                expandToViewport
              />
            </SpaceBetween>
          }
          pagination={
            pageCount > 1 ? (
              <Pagination
                currentPageIndex={currentPage}
                pagesCount={pageCount}
                onChange={e => setCurrentPage(e.detail.currentPageIndex)}
              />
            ) : undefined
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
        onDismiss={() => {
          setShowCreateModal(false);
          setNewUser({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
          setInviteLink('');
          setCreateError('');
        }}
        visible={showCreateModal}
        size="medium"
        footer={
          inviteLink ? (
            <Box float="right">
              <Button onClick={() => {
                setShowCreateModal(false);
                setNewUser({ username: '', display_name: '', domain_id: '', password: '', quota_gb: '0' });
                setInviteLink('');
              }}>{t('common.close')}</Button>
            </Box>
          ) : (
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
                <Button
                  variant="primary"
                  onClick={handleCreateUser}
                  loading={creating}
                  disabled={!newUser.username.trim() || !newUser.domain_id.trim() || loadingDomainSettings}
                >
                  {registrationMode === 'email_invite'
                    ? t('pages.users_page.create_and_invite_btn')
                    : t('pages.users_page.create_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          )
        }
        header={t('pages.users_page.create_modal_title')}
      >
        <SpaceBetween size="m">
          {inviteLink ? (
            <SpaceBetween size="m">
              <Alert type="success">{t('pages.users_page.invite_created')}</Alert>
              <FormField label={t('pages.users_page.invite_link_label')}
                description={t('pages.users_page.invite_link_desc')}>
                <CopyToClipboard
                  copyButtonText={t('pages.users_page.copy_invite')}
                  copySuccessText={t('pages.users_page.copy_success')}
                  copyErrorText={t('pages.users_page.copy_error')}
                  textToCopy={inviteLink}
                />
              </FormField>
              <Box color="text-body-secondary" fontSize="body-s">
                {inviteLink}
              </Box>
            </SpaceBetween>
          ) : (
            <>
              <FormField label={t('pages.users_page.domain_label')}>
                <Select
                  selectedOption={domainOptions.find(o => o.value === newUser.domain_id) ?? null}
                  options={domainOptions}
                  onChange={(e) => handleDomainChange(e.detail.selectedOption?.value ?? '')}
                  placeholder={t('pages.users_page.domain_placeholder')}
                  statusType={loadingDomainSettings ? 'loading' : 'finished'}
                  expandToViewport
                />
              </FormField>

              {newUser.domain_id && (
                <Alert type="info">
                  {registrationMode === 'email_invite'
                    ? t('pages.users_page.mode_email_invite_info')
                    : t('pages.users_page.mode_temp_password_info')}
                </Alert>
              )}

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
                  placeholder={t('pages.users_page.display_name_placeholder')}
                />
              </FormField>

              {autoAddress && (
                <FormField label={t('pages.users_page.address_label')}>
                  <Box color="text-body-secondary">{autoAddress}</Box>
                </FormField>
              )}

              {registrationMode === 'temp_password' && (
                <FormField
                  label={t('pages.users_page.password_label')}
                  description={t('pages.users_page.temp_password_desc')}
                >
                  <Input
                    type="password"
                    value={newUser.password}
                    onChange={(e) => setNewUser({ ...newUser, password: e.detail.value })}
                  />
                </FormField>
              )}

              <FormField label={t('pages.users_page.quota_label')} description={t('pages.users_page.quota_description')}>
                <Input
                  type="number"
                  value={newUser.quota_gb}
                  onChange={(e) => setNewUser({ ...newUser, quota_gb: e.detail.value })}
                />
              </FormField>

              {createError && <Alert type="error">{createError}</Alert>}
            </>
          )}
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
            <Box color="text-body-secondary">{editForm.display_name || '—'}</Box>
          </FormField>
          <FormField label={t('pages.users_page.quota_label')}>
            <Input
              type="number"
              value={editForm.quota_gb}
              onChange={(e) => setEditForm({ ...editForm, quota_gb: e.detail.value })}
            />
          </FormField>
          <FormField
            label={t('pages.users_page.role')}
            description={t('pages.users_page.admin_role_desc')}
          >
            <Select
              selectedOption={ROLE_OPTIONS.find(o => o.value === editForm.role) ?? ROLE_OPTIONS[0]}
              options={ROLE_OPTIONS}
              onChange={(e) => setEditForm({ ...editForm, role: e.detail.selectedOption.value ?? 'user' })}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Offboarding Modal */}
      <Modal
        visible={!!offboardTarget}
        onDismiss={() => setOffboardTarget(null)}
        size="medium"
        header={`${t('pages.users_page.offboard_title')} — ${offboardTarget?.username ?? ''}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setOffboardTarget(null)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleOffboard} loading={offboarding}>
                {t('pages.users_page.suspend_user')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <Alert type="warning">
            {t('pages.users_page.offboard_warning_prefix')} <strong>{offboardTarget?.username}</strong>{' '}
            {t('pages.users_page.offboard_warning_suffix')}
          </Alert>
          <Alert type="info">
            {t('pages.users_page.offboard_alias_prefix')} <strong>{t('pages.users_page.access_aliases')}</strong>{' '}
            {t('pages.users_page.offboard_alias_middle')} <strong>{offboardTarget?.username}</strong>{' '}
            {t('pages.users_page.offboard_alias_suffix')}
          </Alert>
        </SpaceBetween>
      </Modal>

      {/* Import CSV Modal */}
      <Modal
        onDismiss={() => { setShowImportModal(false); setImportResult(null); }}
        visible={showImportModal}
        size="medium"
        header={t('pages.users_page.users_bulk.import_modal')}
        footer={
          <Box float="right">
            <Button onClick={() => { setShowImportModal(false); setImportResult(null); }}>
              {t('pages.users_page.users_bulk.close')}
            </Button>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <Alert type="info">
            {t('pages.users_page.users_bulk.format_hint')}
          </Alert>

          {!importResult && (
            <FormField label={t('pages.users_page.users_bulk.drop_or_click')}>
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                style={{ display: 'block' }}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleImportCSV(file);
                }}
              />
              {importing && <Box padding={{ top: 's' }}><Spinner /> {t('pages.users_page.users_bulk.importing')}</Box>}
            </FormField>
          )}

          {importResult && (
            <SpaceBetween size="s">
              <Alert type={importResult.failed === 0 ? 'success' : 'warning'}>
                {t('pages.users_page.users_bulk.success_count').replace('{n}', String(importResult.success))}
                {importResult.failed > 0 && (
                  <> &mdash; {t('pages.users_page.users_bulk.failed_count').replace('{n}', String(importResult.failed))}</>
                )}
              </Alert>
              {importResult.failures && importResult.failures.length > 0 && (
                <Alert type="error" header={t('pages.users_page.users_bulk.import_result')}>
                  <ul style={{ margin: 0, paddingLeft: '1.2em' }}>
                    {importResult.failures.map((f, i) => (
                      <li key={i}><strong>{f.email}</strong>: {f.error}</li>
                    ))}
                  </ul>
                </Alert>
              )}
            </SpaceBetween>
          )}
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
