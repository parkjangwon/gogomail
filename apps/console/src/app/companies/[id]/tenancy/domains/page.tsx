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
  StatusIndicator,
  Alert,
  Flashbar,
  FlashbarProps,
  ButtonDropdown,
  Pagination,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useRouter } from 'next/navigation';
import { useCompany } from '@/contexts/CompanyContext';

interface Domain {
  id: string;
  company_id: string;
  company_name: string;
  name: string;
  status: string;
  last_dns_check_status: string;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

const dnsBadge = (status: string, t: (key: string, defaultValue?: string) => string) => {
  if (status === 'pass') return <Badge color="green">{t('domains_page.dns_pass')}</Badge>;
  if (status === 'fail') return <Badge color="red">{t('domains_page.dns_fail')}</Badge>;
  if (status === 'partial') return <Badge color="severity-high">{t('domains_page.dns_partial')}</Badge>;
  return <Badge color="grey">{t('domains_page.dns_unchecked')}</Badge>;
};

const fmtQuota = (used: number, limit: number) => {
  const gb = (b: number) => `${(b / 1073741824).toFixed(1)} GB`;
  if (limit <= 0) return `${gb(used)} (unlimited)`;
  const pct = Math.round((used / limit) * 100);
  return `${gb(used)} / ${gb(limit)} (${pct}%)`;
};

export default function DomainsPage() {
  const { t } = useI18n();
  const statusOptions = [
    { label: t('status.active'), value: 'active' },
    { label: t('users.suspended'), value: 'suspended' },
  ];
  const router = useRouter();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const PAGE_SIZE = 25;
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [selected, setSelected] = useState<Domain[]>([]);
  const [bulkLoading, setBulkLoading] = useState(false);
  const [verifying, setVerifying] = useState<string | null>(null);

  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ name: '', quota_gb: '100' });
  const [creating, setCreating] = useState(false);

  const [editTarget, setEditTarget] = useState<Domain | null>(null);
  const [editForm, setEditForm] = useState({ quota_gb: '', status: 'active' });
  const [saving, setSaving] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<Domain | null>(null);
  const [deleting, setDeleting] = useState(false);

  const ok = (msg: string) => setFlash([{ type: 'success', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);
  const err = (msg: string) => setFlash([{ type: 'error', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const url = cid === 'default'
        ? '/admin/v1/domains?limit=200'
        : `/admin/v1/domains?company_id=${cid}&limit=200`;
      const res = await fetch(url);
      const data = await res.json();
      setDomains(data.domains ?? []);
      setSelected([]);
    } catch {
      err(t('domains_page.failed_load'));
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { load(); }, [load]);

  const filtered = useMemo(() => {
    if (!filter) return domains;
    const q = filter.toLowerCase();
    return domains.filter(d => d.name.toLowerCase().includes(q) || d.company_name?.toLowerCase().includes(q));
  }, [domains, filter]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paged = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  const handleCreate = async () => {
    if (!createForm.name.trim()) return;
    setCreating(true);
    try {
      const res = await fetch('/admin/v1/domains', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: createForm.name.trim(),
          company_id: cid,
          quota_limit: parseInt(createForm.quota_gb) * 1073741824 || 0,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      ok(t('domains_page.created'));
      setShowCreate(false);
      setCreateForm({ name: '', quota_gb: '100' });
      load();
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setCreating(false);
    }
  };

  const handleVerifyDNS = async (id: string) => {
    setVerifying(id);
    try {
      await fetch(`/admin/v1/domains/${id}/dns-check`, { method: 'GET' });
      load();
    } catch {
      err(t('domains_page.dns_check_failed'));
    } finally {
      setVerifying(null);
    }
  };

  const openEdit = (d: Domain) => {
    setEditTarget(d);
    setEditForm({
      status: d.status,
      quota_gb: d.quota_limit > 0 ? String(Math.round(d.quota_limit / 1073741824)) : '',
    });
  };

  const handleSaveEdit = async () => {
    if (!editTarget) return;
    setSaving(true);
    try {
      const calls: Promise<Response>[] = [];
      if (editTarget.status !== editForm.status) {
        calls.push(fetch(`/admin/v1/domains/${editTarget.id}/status`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: editForm.status }),
        }));
      }
      if (editForm.quota_gb) {
        calls.push(fetch(`/admin/v1/domains/${editTarget.id}/quota`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ quota_limit: parseInt(editForm.quota_gb) * 1073741824 }),
        }));
      }
      await Promise.all(calls);
      ok(t('domains_page.updated'));
      setEditTarget(null);
      load();
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await fetch(`/admin/v1/domains/${deleteTarget.id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error(await res.text());
      ok(t('domains_page.deleted'));
      setDeleteTarget(null);
      load();
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setDeleting(false);
    }
  };

  const handleBulk = async (action: 'activate' | 'suspend' | 'delete') => {
    if (selected.length === 0) return;
    if (action === 'delete' && !confirm(`${t('domains_page.confirm_bulk_delete')} ${selected.length}`)) return;
    setBulkLoading(true);
    try {
      const res = await fetch('/admin/v1/domains/bulk', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: selected.map(d => d.id), action }),
      });
      const data = await res.json();
      const s = data.succeeded?.length ?? 0;
      const f = data.failed?.length ?? 0;
      ok(`${t(`domains_page.bulk_${action}`)}: ${s} ${t('domains_page.succeeded')}${f > 0 ? `, ${f} ${t('domains_page.failed')}` : ''}`);
      load();
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setBulkLoading(false);
    }
  };

  if (loading && domains.length === 0) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          counter={`(${domains.length})`}
          actions={<Button variant="primary" onClick={() => setShowCreate(true)}>{t('domains_page.add_domain')}</Button>}
        >
          {t('pages.domains.title')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <Table
          selectionType="multi"
          selectedItems={selected}
          onSelectionChange={({ detail }) => setSelected(detail.selectedItems)}
          trackBy="id"
          items={paged}
          loading={loading}
          columnDefinitions={[
            {
              id: 'name', header: t('domains_page.domain'),
              cell: (d) => (
                <Button variant="inline-link" onClick={() => router.push(`/companies/${d.company_id}/domains/${d.id}`)}>
                  {d.name}
                </Button>
              ),
            },
            {
              id: 'company', header: t('domains_page.company'),
              cell: (d) => d.company_name || d.company_id,
            },
            {
              id: 'status', header: t('domains_page.status'),
              cell: (d) => <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>,
            },
            {
              id: 'dns', header: 'DNS',
              cell: (d) => dnsBadge(d.last_dns_check_status, t),
            },
            {
              id: 'quota', header: t('domains_page.quota'),
              cell: (d) => fmtQuota(d.quota_used, d.quota_limit),
            },
            {
              id: 'created', header: t('domains_page.created_at'),
              cell: (d) => new Date(d.created_at).toLocaleDateString(),
            },
            {
              id: 'actions', header: '',
              cell: (d) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" loading={verifying === d.id} onClick={() => handleVerifyDNS(d.id)}>{t('domains_page.verify_dns')}</Button>
                  <Button variant="inline-link" onClick={() => openEdit(d)}>{t('common.edit')}</Button>
                  <Button variant="inline-link" onClick={() => setDeleteTarget(d)}>{t('common.delete')}</Button>
                </SpaceBetween>
              ),
            },
          ]}
          header={
            <Header
              variant="h2"
              counter={selected.length > 0 ? `(${selected.length}/${filtered.length})` : `(${filtered.length})`}
              actions={
                selected.length > 0 ? (
                  <SpaceBetween size="xs" direction="horizontal">
                    <Box color="text-status-inactive" padding={{ top: 'xs' }}>{selected.length} {t('domains_page.selected')}</Box>
                    <ButtonDropdown
                      loading={bulkLoading}
                      items={[
                        { id: 'activate', text: t('pages.domains_page.activate_selected') },
                        { id: 'suspend', text: t('pages.domains_page.suspend_selected') },
                        { id: 'delete', text: t('pages.domains_page.delete_selected'), disabled: false },
                      ]}
                      onItemClick={({ detail }) => handleBulk(detail.id as 'activate' | 'suspend' | 'delete')}
                    >
                      {t('domains_page.bulk_actions')}
                    </ButtonDropdown>
                  </SpaceBetween>
                ) : undefined
              }
            >
              {t('pages.domains.title')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('domains_page.search_placeholder')}
              onChange={(e) => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
              countText={`${filtered.length} ${t('pages.domains.title')}`}
            />
          }
          pagination={
            pageCount > 1 ? (
              <Pagination
                currentPageIndex={currentPage}
                pagesCount={pageCount}
                onChange={(e) => setCurrentPage(e.detail.currentPageIndex)}
              />
            ) : undefined
          }
          empty={<Box textAlign="center" padding="l"><StatusIndicator type="info">{t('domains_page.empty')}</StatusIndicator></Box>}
        />

        <Modal
          visible={showCreate}
          onDismiss={() => setShowCreate(false)}
          header={t('domains_page.add_domain')}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowCreate(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate} disabled={!createForm.name.trim()}>{t('common.create')}</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.domains_page.domain_name')}>
              <Input value={createForm.name} placeholder="example.com" onChange={(e) => setCreateForm(f => ({ ...f, name: e.detail.value }))} />
            </FormField>
            <FormField label={t('pages.domains_page.storage_quota')} constraintText={t('pages.domains_page.quota_hint')}>
              <Input type="number" value={createForm.quota_gb} onChange={(e) => setCreateForm(f => ({ ...f, quota_gb: e.detail.value }))} />
            </FormField>
          </SpaceBetween>
        </Modal>

        <Modal
          visible={!!editTarget}
          onDismiss={() => setEditTarget(null)}
          header={`${t('common.edit')} — ${editTarget?.name ?? ''}`}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setEditTarget(null)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={saving} onClick={handleSaveEdit}>{t('common.save')}</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.domains_page.domain_status')}>
              <Select
                selectedOption={statusOptions.find(o => o.value === editForm.status) ?? statusOptions[0]}
                options={statusOptions}
                onChange={(e) => setEditForm(f => ({ ...f, status: e.detail.selectedOption.value ?? 'active' }))}
              />
            </FormField>
            <FormField label={t('domains_page.storage_quota_gb')} constraintText={t('domains_page.quota_hint')}>
              <Input type="number" value={editForm.quota_gb} onChange={(e) => setEditForm(f => ({ ...f, quota_gb: e.detail.value }))} />
            </FormField>
          </SpaceBetween>
        </Modal>

        <Modal
          visible={!!deleteTarget}
          onDismiss={() => setDeleteTarget(null)}
          header={t('domains_page.delete_domain')}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={deleting} onClick={handleDelete}>{t('common.delete')}</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <Alert type="warning">
            {t('domains_page.delete_confirm_prefix')} <strong>{deleteTarget?.name}</strong>? {t('domains_page.cannot_undo')}
          </Alert>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
