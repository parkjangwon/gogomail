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

const STATUS_OPTIONS = [
  { label: 'active', value: 'active' },
  { label: 'suspended', value: 'suspended' },
];

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

const dnsBadge = (status: string) => {
  if (status === 'pass') return <Badge color="green">Pass</Badge>;
  if (status === 'fail') return <Badge color="red">Fail</Badge>;
  if (status === 'partial') return <Badge color="severity-high">Partial</Badge>;
  return <Badge color="grey">Unchecked</Badge>;
};

const fmtQuota = (used: number, limit: number) => {
  const gb = (b: number) => `${(b / 1073741824).toFixed(1)} GB`;
  if (limit <= 0) return `${gb(used)} (unlimited)`;
  const pct = Math.round((used / limit) * 100);
  return `${gb(used)} / ${gb(limit)} (${pct}%)`;
};

export default function DomainsPage() {
  const { t } = useI18n();
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
      err('Failed to load domains');
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
      ok('Domain created');
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
      await fetch(`/admin/v1/domains/${id}/dns-check`, { method: 'POST' });
      load();
    } catch {
      err('DNS check failed');
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
      ok('Domain updated');
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
      ok('Domain deleted');
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
    if (action === 'delete' && !confirm(`Delete ${selected.length} domain(s)?`)) return;
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
      ok(`${action}: ${s} succeeded${f > 0 ? `, ${f} failed` : ''}`);
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
          actions={<Button variant="primary" onClick={() => setShowCreate(true)}>Add Domain</Button>}
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
              id: 'name', header: 'Domain',
              cell: (d) => (
                <Button variant="inline-link" onClick={() => router.push(`/companies/${d.company_id}/domains/${d.id}`)}>
                  {d.name}
                </Button>
              ),
            },
            {
              id: 'company', header: 'Company',
              cell: (d) => d.company_name || d.company_id,
            },
            {
              id: 'status', header: 'Status',
              cell: (d) => <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>,
            },
            {
              id: 'dns', header: 'DNS',
              cell: (d) => dnsBadge(d.last_dns_check_status),
            },
            {
              id: 'quota', header: 'Quota',
              cell: (d) => fmtQuota(d.quota_used, d.quota_limit),
            },
            {
              id: 'created', header: 'Created',
              cell: (d) => new Date(d.created_at).toLocaleDateString(),
            },
            {
              id: 'actions', header: '',
              cell: (d) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" loading={verifying === d.id} onClick={() => handleVerifyDNS(d.id)}>Verify DNS</Button>
                  <Button variant="inline-link" onClick={() => openEdit(d)}>Edit</Button>
                  <Button variant="inline-link" onClick={() => setDeleteTarget(d)}>Delete</Button>
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
                    <Box color="text-status-inactive" padding={{ top: 'xs' }}>{selected.length} selected</Box>
                    <ButtonDropdown
                      loading={bulkLoading}
                      items={[
                        { id: 'activate', text: t('pages.domains_page.activate_selected') },
                        { id: 'suspend', text: t('pages.domains_page.suspend_selected') },
                        { id: 'delete', text: t('pages.domains_page.delete_selected'), disabled: false },
                      ]}
                      onItemClick={({ detail }) => handleBulk(detail.id as 'activate' | 'suspend' | 'delete')}
                    >
                      Bulk Actions
                    </ButtonDropdown>
                  </SpaceBetween>
                ) : undefined
              }
            >
              Domains
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder="Search domains…"
              onChange={(e) => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
              countText={`${filtered.length} domains`}
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
          empty={<Box textAlign="center" padding="l"><StatusIndicator type="info">No domains found</StatusIndicator></Box>}
        />

        <Modal
          visible={showCreate}
          onDismiss={() => setShowCreate(false)}
          header="Add Domain"
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowCreate(false)}>Cancel</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate} disabled={!createForm.name.trim()}>Create</Button>
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
          header={`Edit — ${editTarget?.name ?? ''}`}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setEditTarget(null)}>Cancel</Button>
                <Button variant="primary" loading={saving} onClick={handleSaveEdit}>Save</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.domains_page.domain_status')}>
              <Select
                selectedOption={STATUS_OPTIONS.find(o => o.value === editForm.status) ?? STATUS_OPTIONS[0]}
                options={STATUS_OPTIONS}
                onChange={(e) => setEditForm(f => ({ ...f, status: e.detail.selectedOption.value ?? 'active' }))}
              />
            </FormField>
            <FormField label="Storage Quota (GB)" constraintText="0 = unlimited">
              <Input type="number" value={editForm.quota_gb} onChange={(e) => setEditForm(f => ({ ...f, quota_gb: e.detail.value }))} />
            </FormField>
          </SpaceBetween>
        </Modal>

        <Modal
          visible={!!deleteTarget}
          onDismiss={() => setDeleteTarget(null)}
          header="Delete Domain"
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setDeleteTarget(null)}>Cancel</Button>
                <Button variant="primary" loading={deleting} onClick={handleDelete}>Delete</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <Alert type="warning">
            Delete <strong>{deleteTarget?.name}</strong>? This cannot be undone.
          </Alert>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
