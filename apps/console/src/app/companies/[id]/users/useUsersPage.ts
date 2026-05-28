'use client';
import { useState, useEffect, useMemo, useRef } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
import { User, Domain, normalizeUserStatus } from '@/lib/users/userUtils';
import {
  buildAutoAddress,
  createEmptyUserDraft,
  parseUsersCsv,
  USER_STORAGE_BYTES_PER_GB,
} from '@/lib/users/userPageUtils';
import type { EditUserFormState } from '@/components/users/EditUserModal';
import type { ImportUsersResult } from '@/components/users/ImportUsersModal';

type FlashItem = {
  type: 'success' | 'error' | 'info';
  content: string;
  id: string;
  dismissible: boolean;
  onDismiss: () => void;
};

const PAGE_SIZE = 25;

export function useUsersPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  // ── Data ─────────────────────────────────────────────────────────────────
  const [users, setUsers] = useState<User[]>([]);
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');

  // ── Filter / pagination ───────────────────────────────────────────────────
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);

  // ── Flash messages ────────────────────────────────────────────────────────
  const [flashItems, setFlashItems] = useState<FlashItem[]>([]);

  // ── Bulk import/export ────────────────────────────────────────────────────
  const [showImportModal, setShowImportModal] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<ImportUsersResult | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // ── Create user ───────────────────────────────────────────────────────────
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newUser, setNewUser] = useState(() => createEmptyUserDraft());
  const [registrationMode, setRegistrationMode] = useState<'temp_password' | 'email_invite'>('temp_password');
  const [loadingDomainSettings, setLoadingDomainSettings] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [inviteLink, setInviteLink] = useState('');

  // ── Edit user ─────────────────────────────────────────────────────────────
  const [editUser, setEditUser] = useState<User | null>(null);
  const [editForm, setEditForm] = useState<EditUserFormState>({ display_name: '', recovery_email: '', quota_gb: '0', role: 'user' });
  const [saving, setSaving] = useState(false);
  const [editSaveError, setEditSaveError] = useState('');

  // ── Status toggle ─────────────────────────────────────────────────────────
  const [togglingId, setTogglingId] = useState<string | null>(null);

  // ── Offboard ──────────────────────────────────────────────────────────────
  const [offboardTarget, setOffboardTarget] = useState<User | null>(null);
  const [offboarding, setOffboarding] = useState(false);

  // ── Bulk selection ────────────────────────────────────────────────────────
  const [selectedUsers, setSelectedUsers] = useState<User[]>([]);
  const [bulkLoading, setBulkLoading] = useState(false);

  // ── Fetch ─────────────────────────────────────────────────────────────────
  const fetchUsers = async () => {
    setLoading(true);
    setFetchError('');
    try {
      const domainRes = await fetch(`/api/admin/domains?company_id=${encodeURIComponent(companyId)}&limit=200`, { credentials: 'include' });
      if (!domainRes.ok) {
        setFetchError(t('pages.users_page.load_failed'));
        return;
      }
      const domainData = await domainRes.json();
      const companyDomains: Domain[] = domainData.domains || [];
      const userLists = await Promise.all(
        companyDomains.map((domain) =>
          fetch(`/api/admin/users?domain_id=${encodeURIComponent(domain.id)}&limit=200`, { credentials: 'include' })
            .then((res) => res.ok ? res.json() : { users: [] })
        )
      );
      setUsers(
        userLists
          .flatMap((data: { users?: User[] }) => data.users || [])
          .map((u: User) => ({ ...u, status: normalizeUserStatus(u.status) }))
      );
    } catch {
      setFetchError(t('pages.users_page.load_failed'));
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
    } catch {
      // silently ignored
    }
  };

  useEffect(() => {
    fetchUsers();
    fetchDomains();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ── Flash helper ──────────────────────────────────────────────────────────
  const addFlash = (type: 'success' | 'error' | 'info', content: string) => {
    const id = Date.now().toString();
    setFlashItems(prev => [
      ...prev,
      { type, content, id, dismissible: true, onDismiss: () => setFlashItems(f => f.filter(i => i.id !== id)) },
    ]);
  };

  // ── Create user handlers ──────────────────────────────────────────────────
  const openCreateModal = () => {
    setShowCreateModal(true);
    setCreateError('');
    setInviteLink('');
  };

  const resetCreateUserForm = () => {
    setShowCreateModal(false);
    setNewUser(createEmptyUserDraft());
    setInviteLink('');
    setCreateError('');
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
    } catch {
      // silently ignored
    } finally {
      setLoadingDomainSettings(false);
    }
  };

  const selectedDomain = domains.find(d => d.id === newUser.domain_id);
  const autoAddress = buildAutoAddress(newUser.username, selectedDomain?.name);

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
        recovery_email: newUser.recovery_email.trim(),
        quota_limit: parseInt(newUser.quota_gb) * USER_STORAGE_BYTES_PER_GB,
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
            setInviteLink(`${window.location.origin}/invite/${token}`);
          }
        }
      }

      if (registrationMode === 'temp_password') {
        resetCreateUserForm();
      }
      fetchUsers();
    } catch {
      setCreateError(t('pages.users_page.create_failed'));
    } finally {
      setCreating(false);
    }
  };

  // ── Edit user handlers ────────────────────────────────────────────────────
  const openEdit = (user: User) => {
    setEditUser(user);
    setEditForm({
      display_name: user.display_name,
      recovery_email: user.recovery_email ?? '',
      quota_gb: user.quota_limit > 0 ? String(Math.round(user.quota_limit / USER_STORAGE_BYTES_PER_GB)) : '0',
      role: user.role || 'user',
    });
  };

  const handleEditSave = async () => {
    if (!editUser) return;
    setSaving(true);
    setEditSaveError('');
    try {
      const quotaRes = await fetch(`/api/admin/users/${editUser.id}/quota`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quota_limit: parseInt(editForm.quota_gb) * USER_STORAGE_BYTES_PER_GB }),
        credentials: 'include',
      });
      if (!quotaRes.ok) throw new Error(await quotaRes.text());

      if (editForm.role !== editUser.role) {
        const roleRes = await fetch(`/api/admin/users/${editUser.id}/role`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ role: editForm.role }),
          credentials: 'include',
        });
        if (!roleRes.ok) throw new Error(await roleRes.text());
      }

      if (editForm.recovery_email.trim() !== (editUser.recovery_email ?? '')) {
        const emailRes = await fetch(`/api/admin/users/${editUser.id}/recovery-email`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ recovery_email: editForm.recovery_email.trim() }),
          credentials: 'include',
        });
        if (!emailRes.ok) throw new Error(await emailRes.text());
      }

      setEditUser(null);
      fetchUsers();
    } catch (e) {
      setEditSaveError(e instanceof Error ? e.message : t('common.error'));
    } finally {
      setSaving(false);
    }
  };

  // ── Status toggle / offboard handlers ─────────────────────────────────────
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
    } catch {
      // silently ignored
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

  // ── Bulk action handler ───────────────────────────────────────────────────
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

  // ── CSV export / import handlers ──────────────────────────────────────────
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
      const usersToImport = parseUsersCsv(text);
      const res = await fetch(`/api/admin/companies/${companyId}/users/bulk-import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ users: usersToImport }),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setImportResult(data);
        if (data.success > 0) fetchUsers();
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

  // ── Derived / static options ──────────────────────────────────────────────
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

  const ROLE_OPTIONS = [
    { label: t('pages.users_page.role_user_email'), value: 'user' },
    { label: t('pages.users_page.role_company_admin'), value: 'company_admin' },
    { label: t('pages.users_page.role_system_admin'), value: 'system_admin' },
  ];

  const filteredUsers = useMemo(() => {
    return users.filter(u => {
      const matchesText = !filter
        || u.username.toLowerCase().includes(filter.toLowerCase())
        || (u.display_name || '').toLowerCase().includes(filter.toLowerCase())
        || (u.recovery_email || '').toLowerCase().includes(filter.toLowerCase());
      const matchesStatus = !statusFilter || u.status === statusFilter;
      return matchesText && matchesStatus;
    });
  }, [users, filter, statusFilter]);

  const pageCount = Math.max(1, Math.ceil(filteredUsers.length / PAGE_SIZE));
  const pagedUsers = filteredUsers.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  const totalUsers = users.length;
  const activeUsers = users.filter(u => u.status === 'active').length;
  const suspendedUsers = users.filter(u => u.status === 'suspended' || u.status === 'disabled').length;

  return {
    // ── identity ────────────────────────────────────────────────────────────
    companyId,
    t,

    // ── data / loading ───────────────────────────────────────────────────────
    loading,
    fetchError,
    setFetchError,
    domains,

    // ── filter / pagination ──────────────────────────────────────────────────
    filter,
    setFilter,
    statusFilter,
    setStatusFilter,
    currentPage,
    setCurrentPage,
    filteredUsers,
    pagedUsers,
    pageCount,
    PAGE_SIZE,

    // ── stats ────────────────────────────────────────────────────────────────
    totalUsers,
    activeUsers,
    suspendedUsers,

    // ── flash ────────────────────────────────────────────────────────────────
    flashItems,

    // ── import / export ──────────────────────────────────────────────────────
    showImportModal,
    setShowImportModal,
    importing,
    importResult,
    setImportResult,
    fileInputRef,
    handleExportCSV,
    handleImportCSV,

    // ── create user ──────────────────────────────────────────────────────────
    showCreateModal,
    setShowCreateModal,
    openCreateModal,
    newUser,
    setNewUser,
    registrationMode,
    loadingDomainSettings,
    creating,
    createError,
    inviteLink,
    domainOptions,
    autoAddress,
    resetCreateUserForm,
    handleDomainChange,
    handleCreateUser,

    // ── edit user ────────────────────────────────────────────────────────────
    editUser,
    setEditUser,
    editForm,
    setEditForm,
    saving,
    editSaveError,
    setEditSaveError,
    ROLE_OPTIONS,
    openEdit,
    handleEditSave,

    // ── status toggle / offboard ─────────────────────────────────────────────
    togglingId,
    offboardTarget,
    setOffboardTarget,
    offboarding,
    handleToggleStatus,
    handleOffboard,

    // ── bulk selection ───────────────────────────────────────────────────────
    selectedUsers,
    setSelectedUsers,
    bulkLoading,
    handleBulkAction,

    // ── options ──────────────────────────────────────────────────────────────
    statusOptions,
  };
}
